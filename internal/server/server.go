package server

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/autoschei/ricerkatoro-mcp/internal/engine"
	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/providers"
	"github.com/autoschei/ricerkatoro-mcp/internal/storage"
	"github.com/autoschei/ricerkatoro-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// RicercatoreServer holds the full server state.
type RicercatoreServer struct {
	Config     *models.ServerConfig
	Store      *storage.SQLiteStore
	Voyage     *storage.VoyageStore
	Providers  []providers.SearchProvider
	Dispatcher *engine.Dispatcher
	Consensus  *engine.ConsensusEngine
	Tables     map[string]*models.ResearchTable
	MCPServer  *mcpserver.MCPServer
}

// New creates and configures the MCP server with all tools.
func New(cfg *models.ServerConfig) (*RicercatoreServer, error) {
	store, err := storage.NewSQLiteStore(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("init sqlite: %w", err)
	}

	voyage := storage.NewVoyageStore(cfg.VoyageConfig.APIKey, cfg.VoyageConfig.Model)

	var provs []providers.SearchProvider
	for _, pc := range cfg.Providers {
		if !pc.Enabled || pc.APIKey == "" {
			continue
		}
		switch pc.Name {
		case "tavily":
			provs = append(provs, providers.NewTavilyProvider(pc.APIKey))
		case "brave":
			provs = append(provs, providers.NewBraveProvider(pc.APIKey))
		case "exa":
			provs = append(provs, providers.NewExaProvider(pc.APIKey))
		default:
			log.Printf("[server] unknown provider: %s", pc.Name)
		}
	}

	consensus := engine.NewConsensusEngine(cfg.ConfidenceThreshold)
	dispatcher := engine.NewDispatcher(provs, consensus, store, cfg.MaxConcurrency, cfg.MaxRetries)

	srv := &RicercatoreServer{
		Config:     cfg,
		Store:      store,
		Voyage:     voyage,
		Providers:  provs,
		Dispatcher: dispatcher,
		Consensus:  consensus,
		Tables:     make(map[string]*models.ResearchTable),
	}

	mcpSrv := mcpserver.NewMCPServer(
		"ricerkatoro-mcp",
		"1.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	srv.registerTools(mcpSrv)
	srv.MCPServer = mcpSrv

	return srv, nil
}

// prop is a helper to build a JSON Schema property.
func prop(typ, desc string) any {
	return map[string]any{"type": typ, "description": desc}
}

func propEnum(typ, desc string, enum []string) any {
	e := make([]any, len(enum))
	for i, v := range enum {
		e[i] = v
	}
	return map[string]any{"type": typ, "description": desc, "enum": e}
}

func propArray(desc string, itemType string) any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]any{"type": itemType},
	}
}

func propArrayObj(desc string) any {
	return map[string]any{
		"type":        "array",
		"description": desc,
		"items":       map[string]any{"type": "object"},
	}
}

func (s *RicercatoreServer) registerTools(mcpSrv *mcpserver.MCPServer) {
	handler := tools.NewHandler(s.Config, s.Store, s.Voyage, s.Providers, s.Dispatcher, s.Consensus, s.Tables)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_config",
		Description: "Configure API keys, search providers, and server preferences.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"providers":            propArrayObj("List of provider configs: [{name, api_key, enabled}]"),
				"voyage_api_key":       prop("string", "Voyage AI API key for embedding storage"),
				"concurrency":          prop("number", "Max parallel rows to process (default: 10)"),
				"confidence_threshold": prop("number", "Min consensus score 0.0-1.0 (default: 0.7)"),
			},
		},
	}, handler.Config)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_load_table",
		Description: "Load a table of items to research. Each row has input fields (known data) and validate fields (to research).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"name":            prop("string", "Name for this research table"),
				"data":            propArrayObj("Array of objects, each representing a row"),
				"search_fields":   propArray("Fields to use for building search queries", "string"),
				"validate_fields": propArray("Fields to validate/research via search", "string"),
			},
			Required: []string{"name", "data", "search_fields", "validate_fields"},
		},
	}, handler.LoadTable)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_search_row",
		Description: "Search a single row across all configured providers.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"table_id":       prop("string", "ID of the research table"),
				"row_id":         prop("string", "ID of the row to search"),
				"query_template": prop("string", "Custom query template with {{field}} placeholders"),
			},
			Required: []string{"table_id", "row_id"},
		},
	}, handler.SearchRow)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_search_batch",
		Description: "Launch parallel search across all rows (or a range). Returns a job ID for tracking.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"table_id":       prop("string", "ID of the research table"),
				"query_template": prop("string", "Custom query template with {{field}} placeholders"),
				"start":          prop("number", "Start index (0-based, default: 0)"),
				"end":            prop("number", "End index (exclusive, default: all rows)"),
				"concurrency":    prop("number", "Override max parallel rows for this batch"),
			},
			Required: []string{"table_id"},
		},
	}, handler.SearchBatch)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_get_status",
		Description: "Get the status of a batch search job or overall table status.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"job_id":   prop("string", "Job ID from search_batch"),
				"table_id": prop("string", "Table ID for overall status"),
			},
		},
	}, handler.GetStatus)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_get_results",
		Description: "Retrieve validated results with consensus scores.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"table_id":       prop("string", "ID of the research table"),
				"row_ids":        propArray("Specific row IDs to retrieve", "string"),
				"min_confidence": prop("number", "Minimum consensus score filter (0.0-1.0)"),
			},
			Required: []string{"table_id"},
		},
	}, handler.GetResults)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_validate",
		Description: "Trigger cross-validation on specific rows. Optionally retry conflicts.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"table_id":        prop("string", "ID of the research table"),
				"row_ids":         propArray("Specific row IDs", "string"),
				"retry_conflicts": prop("boolean", "Re-search and re-validate conflicting rows"),
			},
			Required: []string{"table_id"},
		},
	}, handler.Validate)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_export",
		Description: "Export research results in JSON, CSV, or Markdown format.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"table_id":        prop("string", "ID of the research table"),
				"format":          propEnum("string", "Output format", []string{"json", "csv", "markdown"}),
				"include_sources": prop("boolean", "Include source URLs in output"),
				"include_scores":  prop("boolean", "Include confidence scores in output"),
			},
			Required: []string{"table_id", "format"},
		},
	}, handler.Export)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_voyage_store",
		Description: "Embed and store validated results in Voyage AI for future semantic retrieval.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"table_id":  prop("string", "ID of the research table"),
				"row_ids":   propArray("Specific rows to embed", "string"),
				"namespace": prop("string", "Namespace for organizing embeddings"),
			},
			Required: []string{"table_id"},
		},
	}, handler.VoyageStore)

	mcpSrv.AddTool(mcp.Tool{
		Name:        "ricerkatoro_voyage_search",
		Description: "Semantic search across previously stored research results.",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"query":     prop("string", "Natural language search query"),
				"top_k":     prop("number", "Number of results (default: 5)"),
				"namespace": prop("string", "Filter by namespace"),
				"min_score": prop("number", "Minimum similarity score (0.0-1.0)"),
			},
			Required: []string{"query"},
		},
	}, handler.VoyageSearch)
}

// Run starts the server with the configured transport.
func (s *RicercatoreServer) Run(ctx context.Context) error {
	transport := s.Config.Transport

	if transport == "auto" {
		fi, _ := os.Stdin.Stat()
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			transport = "stdio"
		} else {
			transport = "http"
		}
	}

	log.Printf("[ricerkatoro] starting with transport=%s", transport)

	switch transport {
	case "stdio":
		return mcpserver.ServeStdio(s.MCPServer)
	case "http":
		addr := fmt.Sprintf(":%d", s.Config.HTTPPort)
		log.Printf("[ricerkatoro] HTTP server listening on %s", addr)
		httpServer := mcpserver.NewStreamableHTTPServer(s.MCPServer)
		return httpServer.Start(addr)
	default:
		return fmt.Errorf("unknown transport: %s", transport)
	}
}

// Close cleans up resources.
func (s *RicercatoreServer) Close() error {
	if s.Store != nil {
		return s.Store.Close()
	}
	return nil
}
