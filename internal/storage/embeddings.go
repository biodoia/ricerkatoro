package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// EmbeddingRecord represents a persisted embedding in SQLite.
type EmbeddingRecord struct {
	ID        string            `json:"id"`
	Vector    []float64         `json:"vector"`
	Text      string            `json:"text"`
	Metadata  map[string]string `json:"metadata"`
	Namespace string            `json:"namespace"`
	CreatedAt time.Time         `json:"created_at"`
}

// migrateEmbeddings adds the embeddings table to the SQLite schema.
func (s *SQLiteStore) migrateEmbeddings() error {
	schema := `
	CREATE TABLE IF NOT EXISTS embeddings (
		id TEXT PRIMARY KEY,
		namespace TEXT NOT NULL DEFAULT '',
		vector BLOB NOT NULL,
		text TEXT NOT NULL,
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_embeddings_namespace ON embeddings(namespace);
	`
	_, err := s.db.Exec(schema)
	return err
}

// SaveEmbedding persists an embedding to SQLite.
func (s *SQLiteStore) SaveEmbedding(rec *EmbeddingRecord) error {
	vectorBlob := encodeVector(rec.Vector)
	metadataJSON, _ := json.Marshal(rec.Metadata)

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO embeddings (id, namespace, vector, text, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.Namespace, vectorBlob, rec.Text, string(metadataJSON), rec.CreatedAt,
	)
	return err
}

// LoadEmbeddings loads all embeddings, optionally filtered by namespace.
func (s *SQLiteStore) LoadEmbeddings(namespace string) ([]EmbeddingRecord, error) {
	var query string
	var args []interface{}

	if namespace != "" {
		query = `SELECT id, namespace, vector, text, metadata, created_at FROM embeddings WHERE namespace = ?`
		args = []interface{}{namespace}
	} else {
		query = `SELECT id, namespace, vector, text, metadata, created_at FROM embeddings`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("load embeddings: %w", err)
	}
	defer rows.Close()

	var records []EmbeddingRecord
	for rows.Next() {
		var rec EmbeddingRecord
		var vectorBlob []byte
		var metadataStr string

		if err := rows.Scan(&rec.ID, &rec.Namespace, &vectorBlob, &rec.Text, &metadataStr, &rec.CreatedAt); err != nil {
			return nil, err
		}

		rec.Vector = decodeVector(vectorBlob)
		json.Unmarshal([]byte(metadataStr), &rec.Metadata)
		records = append(records, rec)
	}
	return records, nil
}

// DeleteEmbedding removes an embedding by ID.
func (s *SQLiteStore) DeleteEmbedding(id string) error {
	_, err := s.db.Exec(`DELETE FROM embeddings WHERE id = ?`, id)
	return err
}

// DeleteEmbeddingsByNamespace removes all embeddings in a namespace.
func (s *SQLiteStore) DeleteEmbeddingsByNamespace(namespace string) error {
	_, err := s.db.Exec(`DELETE FROM embeddings WHERE namespace = ?`, namespace)
	return err
}

// CountEmbeddings returns the total count of stored embeddings.
func (s *SQLiteStore) CountEmbeddings() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&count)
	return count, err
}

// SearchEmbeddings performs brute-force cosine similarity search.
// Loads all vectors (optionally filtered by namespace), computes similarity, returns top-k.
func (s *SQLiteStore) SearchEmbeddings(queryVec []float64, namespace string, topK int, minScore float64) ([]ScoredEmbedding, error) {
	records, err := s.LoadEmbeddings(namespace)
	if err != nil {
		return nil, err
	}

	var results []ScoredEmbedding
	for _, rec := range records {
		sim := CosineSimilarity(queryVec, rec.Vector)
		if sim >= minScore {
			results = append(results, ScoredEmbedding{
				Record: rec,
				Score:  sim,
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// ScoredEmbedding is an embedding with a similarity score.
type ScoredEmbedding struct {
	Record EmbeddingRecord `json:"record"`
	Score  float64         `json:"score"`
}

// encodeVector encodes a float64 slice as binary (little-endian float64 values).
func encodeVector(v []float64) []byte {
	buf := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(val))
	}
	return buf
}

// decodeVector decodes a binary blob back to float64 slice.
func decodeVector(b []byte) []float64 {
	n := len(b) / 8
	v := make([]float64, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return v
}
