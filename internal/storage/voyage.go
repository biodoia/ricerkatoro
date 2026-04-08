package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const voyageEmbedURL = "https://api.voyageai.com/v1/embeddings"

// VoyageStore manages embeddings via Voyage AI API.
type VoyageStore struct {
	apiKey string
	model  string
	client *http.Client
}

// VoyageDocument represents a document to embed and store.
type VoyageDocument struct {
	ID       string            `json:"id"`
	Text     string            `json:"text"`
	Metadata map[string]string `json:"metadata"`
}

// VoyageEmbedding represents a stored embedding.
type VoyageEmbedding struct {
	ID        string            `json:"id"`
	Vector    []float64         `json:"vector"`
	Metadata  map[string]string `json:"metadata"`
	Text      string            `json:"text"`
}

// NewVoyageStore creates a new Voyage AI store.
func NewVoyageStore(apiKey, model string) *VoyageStore {
	if model == "" {
		model = "voyage-3-large"
	}
	return &VoyageStore{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// IsAvailable returns true if the API key is set.
func (v *VoyageStore) IsAvailable() bool {
	return v.apiKey != ""
}

type voyageEmbedRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type"`
}

type voyageEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed generates embeddings for the given texts.
func (v *VoyageStore) Embed(ctx context.Context, texts []string, inputType string) ([][]float64, error) {
	if !v.IsAvailable() {
		return nil, fmt.Errorf("voyage: API key not configured")
	}

	if inputType == "" {
		inputType = "document"
	}

	reqBody := voyageEmbedRequest{
		Input:     texts,
		Model:     v.model,
		InputType: inputType,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("voyage: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageEmbedURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("voyage: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("voyage: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var voyageResp voyageEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("voyage: decode: %w", err)
	}

	embeddings := make([][]float64, len(voyageResp.Data))
	for _, d := range voyageResp.Data {
		embeddings[d.Index] = d.Embedding
	}
	return embeddings, nil
}

// CosineSimilarity computes similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
