package models

// ServerConfig holds the server-wide configuration.
type ServerConfig struct {
	Transport           string            `json:"transport"`             // auto | stdio | http
	HTTPPort            int               `json:"http_port"`
	MaxConcurrency      int               `json:"max_concurrency"`      // parallel rows
	ProviderConcurrency int               `json:"provider_concurrency"` // parallel providers per row
	ConfidenceThreshold float64           `json:"confidence_threshold"` // min consensus score
	MaxRetries          int               `json:"max_retries"`
	SQLitePath          string            `json:"sqlite_path"`
	Providers           []ProviderConfig  `json:"providers"`
	VoyageConfig        VoyageConfig      `json:"voyage"`
}

// ProviderConfig configures a single search provider.
type ProviderConfig struct {
	Name    string `json:"name"`    // tavily | brave | exa
	APIKey  string `json:"api_key"`
	Enabled bool   `json:"enabled"`
}

// VoyageConfig configures Voyage AI embedding storage.
type VoyageConfig struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"` // e.g. voyage-4-large
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Transport:           "auto",
		HTTPPort:            3847,
		MaxConcurrency:      10,
		ProviderConcurrency: 3,
		ConfidenceThreshold: 0.7,
		MaxRetries:          2,
		SQLitePath:          "./ricerkatoro.db",
		VoyageConfig: VoyageConfig{
			Model: "voyage-3-large",
		},
	}
}
