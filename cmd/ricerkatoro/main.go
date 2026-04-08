package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	"github.com/autoschei/ricerkatoro-mcp/internal/server"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present
	godotenv.Load()

	cfg := models.DefaultConfig()
	loadEnvConfig(cfg)

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	defer srv.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func loadEnvConfig(cfg *models.ServerConfig) {
	if v := os.Getenv("TRANSPORT"); v != "" {
		cfg.Transport = v
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.HTTPPort = port
		}
	}
	if v := os.Getenv("MAX_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxConcurrency = n
		}
	}
	if v := os.Getenv("PROVIDER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.ProviderConcurrency = n
		}
	}
	if v := os.Getenv("CONFIDENCE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.ConfidenceThreshold = f
		}
	}
	if v := os.Getenv("MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxRetries = n
		}
	}
	if v := os.Getenv("SQLITE_PATH"); v != "" {
		cfg.SQLitePath = v
	}

	// Voyage
	if v := os.Getenv("VOYAGE_API_KEY"); v != "" {
		cfg.VoyageConfig.APIKey = v
	}
	if v := os.Getenv("VOYAGE_MODEL"); v != "" {
		cfg.VoyageConfig.Model = v
	}

	// Search providers from env
	providerNames := []struct {
		name   string
		envKey string
	}{
		{"tavily", "TAVILY_API_KEY"},
		{"brave", "BRAVE_API_KEY"},
		{"exa", "EXA_API_KEY"},
	}

	for _, p := range providerNames {
		apiKey := os.Getenv(p.envKey)
		if apiKey != "" {
			cfg.Providers = append(cfg.Providers, models.ProviderConfig{
				Name:    p.name,
				APIKey:  apiKey,
				Enabled: true,
			})
		}
	}
}
