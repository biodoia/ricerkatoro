// batch runner — loads a JSON table, searches all rows via Tavily/Brave, exports results
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/engine"
	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/providers"
	"github.com/autoschei/ricerkatoro-mcp/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: batch <table.json> [output.md]\n")
		os.Exit(1)
	}
	tableFile := os.Args[1]
	outputFile := ""
	if len(os.Args) > 2 {
		outputFile = os.Args[2]
	}

	// Load table data
	data, err := os.ReadFile(tableFile)
	if err != nil {
		log.Fatalf("read table: %v", err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(data, &rows); err != nil {
		log.Fatalf("parse table: %v", err)
	}
	log.Printf("Loaded %d rows from %s", len(rows), tableFile)

	// Setup providers from env
	var provs []providers.SearchProvider
	if key := os.Getenv("TAVILY_API_KEY"); key != "" {
		provs = append(provs, providers.NewTavilyProvider(key))
		log.Println("Provider: tavily")
	}
	if key := os.Getenv("BRAVE_API_KEY"); key != "" {
		provs = append(provs, providers.NewBraveProvider(key))
		log.Println("Provider: brave")
	}
	if key := os.Getenv("EXA_API_KEY"); key != "" {
		provs = append(provs, providers.NewExaProvider(key))
		log.Println("Provider: exa")
	}
	if len(provs) == 0 {
		log.Fatal("No search providers configured. Set TAVILY_API_KEY, BRAVE_API_KEY, or EXA_API_KEY")
	}

	// Init store + engine
	store, err := storage.NewSQLiteStore("./ricerkatoro-batch.db")
	if err != nil {
		log.Fatalf("init sqlite: %v", err)
	}
	defer store.Close()

	consensus := engine.NewConsensusEngine(0.5) // lower threshold for single-provider
	dispatcher := engine.NewDispatcher(provs, consensus, store, 5, 1)

	// Build research table
	searchFields := []string{"name", "url"}
	validateFields := []string{"claimed_wager", "claimed_bonus", "bonus_type", "jurisdiction"}
	tableID := fmt.Sprintf("batch_%d", time.Now().Unix())
	table := models.NewResearchTable(tableID, "casino-validation", searchFields, validateFields)

	for i, row := range rows {
		inputFields := make(map[string]string)
		for k, v := range row {
			inputFields[k] = fmt.Sprintf("%v", v)
		}
		item := &models.ResearchItem{
			ID:             fmt.Sprintf("row_%d", i),
			InputFields:    inputFields,
			ValidateFields: validateFields,
			Status:         models.StatusPending,
			Results:        make(map[string][]models.ProviderResult),
			Consensus:      make(map[string]models.ConsensusField),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		table.AddItem(item)
	}

	// Search all rows
	ctx := context.Background()
	log.Printf("Starting batch search on %d rows...", len(table.Items))
	start := time.Now()

	job, err := dispatcher.SearchBatch(ctx, table, "", 0, 0)
	if err != nil {
		log.Fatalf("batch search: %v", err)
	}
	log.Printf("Job %s started, waiting for completion...", job.ID)

	// Wait for completion
	for job.Status == "running" {
		time.Sleep(2 * time.Second)
		log.Printf("  progress: %d/%d completed, %d failed", job.Completed, job.Total, job.Failed)
	}
	elapsed := time.Since(start)
	log.Printf("Batch complete in %s: %d/%d succeeded, %d failed",
		elapsed.Round(time.Second), job.Completed, job.Total, job.Failed)

	// Generate markdown output
	var md string
	md += "# Casino Validation Results\n\n"
	md += fmt.Sprintf("**Date:** %s\n", time.Now().Format("2006-01-02 15:04"))
	md += fmt.Sprintf("**Rows:** %d | **Providers:** %d | **Duration:** %s\n\n", len(table.Items), len(provs), elapsed.Round(time.Second))
	md += "| # | Casino | URL | Bonus | Wager | Type | Status | Confidence | Sources |\n"
	md += "|---|--------|-----|-------|-------|------|--------|------------|--------|\n"

	for i, item := range table.Items {
		name := item.InputFields["name"]
		url := item.InputFields["url"]
		bonus := item.InputFields["claimed_bonus"]
		wager := item.InputFields["claimed_wager"]
		btype := item.InputFields["bonus_type"]
		note := item.InputFields["note"]

		conf := ""
		sources := ""

		// Check consensus for each field
		totalScore := 0.0
		fieldCount := 0
		for _, cf := range item.Consensus {
			totalScore += cf.Score
			fieldCount++
			if sources == "" {
				sources = fmt.Sprintf("%d", len(cf.Sources))
			}
		}
		if fieldCount > 0 {
			conf = fmt.Sprintf("%.0f%%", totalScore/float64(fieldCount)*100)
		}

		// Check for validated values
		if cf, ok := item.Consensus["claimed_wager"]; ok && cf.Value != "" {
			wager = cf.Value
		}
		if cf, ok := item.Consensus["claimed_bonus"]; ok && cf.Value != "" {
			bonus = cf.Value
		}

		statusEmoji := "?"
		switch item.Status {
		case models.StatusValidated:
			statusEmoji = "OK"
		case models.StatusConflict:
			statusEmoji = "CONFLICT"
		case models.StatusSearched:
			statusEmoji = "searched"
		default:
			statusEmoji = string(item.Status)
		}

		if note != "" {
			statusEmoji += " " + note
		}

		md += fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			i+1, name, url, bonus, wager, btype, statusEmoji, conf, sources)
	}

	md += "\n## Search Details\n\n"
	for _, item := range table.Items {
		name := item.InputFields["name"]
		if len(item.Results) == 0 {
			continue
		}
		md += fmt.Sprintf("### %s\n", name)
		for provName, provResults := range item.Results {
			for _, pr := range provResults {
				if pr.Error != "" {
					md += fmt.Sprintf("- **%s**: ERROR: %s\n", provName, pr.Error)
					continue
				}
				md += fmt.Sprintf("- **%s** (%s, %d results):\n", provName, pr.Duration.Round(time.Millisecond), len(pr.Results))
				for _, sr := range pr.Results {
					title := sr.Title
					if len(title) > 60 {
						title = title[:57] + "..."
					}
					md += fmt.Sprintf("  - [%s](%s)\n", title, sr.URL)
				}
			}
		}
		md += "\n"
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(md), 0644); err != nil {
			log.Fatalf("write output: %v", err)
		}
		log.Printf("Results written to %s", outputFile)
	} else {
		fmt.Print(md)
	}
}
