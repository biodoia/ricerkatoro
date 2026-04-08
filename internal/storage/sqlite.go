package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/autoschei/ricerkatoro-mcp/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore manages persistent state in SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates a SQLite database.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrateEmbeddings(); err != nil {
		db.Close()
		return nil, err
	}
	// Clean up stale jobs from previous runs
	store.cleanupStaleJobs()
	return store, nil
}

// cleanupStaleJobs marks any "running" jobs as "failed" on startup.
func (s *SQLiteStore) cleanupStaleJobs() {
	s.db.Exec(`UPDATE jobs SET status = 'failed', completed_at = CURRENT_TIMESTAMP WHERE status = 'running'`)
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS research_tables (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		search_fields TEXT NOT NULL,
		validate_fields TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS research_items (
		id TEXT PRIMARY KEY,
		table_id TEXT NOT NULL,
		input_fields TEXT NOT NULL,
		validate_fields TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		results TEXT NOT NULL DEFAULT '{}',
		consensus TEXT NOT NULL DEFAULT '{}',
		retry_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (table_id) REFERENCES research_tables(id)
	);

	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		table_id TEXT NOT NULL,
		total INTEGER NOT NULL DEFAULT 0,
		completed INTEGER NOT NULL DEFAULT 0,
		failed INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'running',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		completed_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_items_table ON research_items(table_id);
	CREATE INDEX IF NOT EXISTS idx_items_status ON research_items(status);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("sqlite: migrate: %w", err)
	}
	return nil
}

// SaveTable persists a research table.
func (s *SQLiteStore) SaveTable(t *models.ResearchTable) error {
	searchFields, _ := json.Marshal(t.SearchFields)
	validateFields, _ := json.Marshal(t.ValidateFields)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO research_tables (id, name, search_fields, validate_fields, created_at) VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Name, string(searchFields), string(validateFields), t.CreatedAt,
	)
	return err
}

// SaveItem persists a single research item.
func (s *SQLiteStore) SaveItem(tableID string, item *models.ResearchItem) error {
	inputFields, _ := json.Marshal(item.InputFields)
	validateFields, _ := json.Marshal(item.ValidateFields)
	results, _ := json.Marshal(item.Results)
	consensus, _ := json.Marshal(item.Consensus)

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO research_items (id, table_id, input_fields, validate_fields, status, results, consensus, retry_count, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, tableID, string(inputFields), string(validateFields),
		string(item.Status), string(results), string(consensus),
		item.RetryCount, item.CreatedAt, time.Now(),
	)
	return err
}

// LoadItems loads all items for a table.
func (s *SQLiteStore) LoadItems(tableID string) ([]*models.ResearchItem, error) {
	rows, err := s.db.Query(
		`SELECT id, input_fields, validate_fields, status, results, consensus, retry_count, created_at, updated_at FROM research_items WHERE table_id = ?`,
		tableID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*models.ResearchItem
	for rows.Next() {
		var (
			item           models.ResearchItem
			inputFieldsStr string
			validateStr    string
			resultsStr     string
			consensusStr   string
			status         string
		)
		if err := rows.Scan(&item.ID, &inputFieldsStr, &validateStr, &status, &resultsStr, &consensusStr, &item.RetryCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Status = models.ItemStatus(status)
		json.Unmarshal([]byte(inputFieldsStr), &item.InputFields)
		json.Unmarshal([]byte(validateStr), &item.ValidateFields)
		json.Unmarshal([]byte(resultsStr), &item.Results)
		json.Unmarshal([]byte(consensusStr), &item.Consensus)
		items = append(items, &item)
	}
	return items, nil
}

// UpdateItemStatus updates just the status and results of an item.
func (s *SQLiteStore) UpdateItemStatus(item *models.ResearchItem) error {
	results, _ := json.Marshal(item.Results)
	consensus, _ := json.Marshal(item.Consensus)
	_, err := s.db.Exec(
		`UPDATE research_items SET status = ?, results = ?, consensus = ?, retry_count = ?, updated_at = ? WHERE id = ?`,
		string(item.Status), string(results), string(consensus), item.RetryCount, time.Now(), item.ID,
	)
	return err
}

// SaveJob persists a job status.
func (s *SQLiteStore) SaveJob(job *models.JobStatus) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO jobs (id, table_id, total, completed, failed, status, started_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.TableID, job.Total, job.Completed, job.Failed, job.Status, job.StartedAt, job.CompletedAt,
	)
	return err
}

// GetJob loads a job by ID.
func (s *SQLiteStore) GetJob(jobID string) (*models.JobStatus, error) {
	var job models.JobStatus
	err := s.db.QueryRow(
		`SELECT id, table_id, total, completed, failed, status, started_at, completed_at FROM jobs WHERE id = ?`,
		jobID,
	).Scan(&job.ID, &job.TableID, &job.Total, &job.Completed, &job.Failed, &job.Status, &job.StartedAt, &job.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetLatestJob returns the most recent job for a table.
func (s *SQLiteStore) GetLatestJob(tableID string) (*models.JobStatus, error) {
	var job models.JobStatus
	err := s.db.QueryRow(
		`SELECT id, table_id, total, completed, failed, status, started_at, completed_at FROM jobs WHERE table_id = ? ORDER BY started_at DESC LIMIT 1`,
		tableID,
	).Scan(&job.ID, &job.TableID, &job.Total, &job.Completed, &job.Failed, &job.Status, &job.StartedAt, &job.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
