package jobs

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("job not found")

type Job struct {
	ID      string     `json:"id"`
	Status  string     `json:"status"`
	Err     string     `json:"error,omitempty"`
	Started *time.Time `json:"started,omitempty"`
	Ended   *time.Time `json:"ended,omitempty"`
	LogPath string     `json:"logPath"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create jobs database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open jobs database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			started_at TEXT,
			ended_at TEXT,
			log_path TEXT NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize jobs database: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Create(job *Job) error {
	_, err := s.db.Exec(
		`INSERT INTO jobs (id, status, error, started_at, ended_at, log_path) VALUES (?, ?, ?, ?, ?, ?)`,
		job.ID, job.Status, job.Err, timeValue(job.Started), timeValue(job.Ended), job.LogPath,
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (s *Store) Update(job *Job) error {
	result, err := s.db.Exec(
		`UPDATE jobs SET status = ?, error = ?, started_at = ?, ended_at = ?, log_path = ? WHERE id = ?`,
		job.Status, job.Err, timeValue(job.Started), timeValue(job.Ended), job.LogPath, job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated job: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Get(id string) (*Job, error) {
	job := &Job{}
	var startedAt, endedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, status, error, started_at, ended_at, log_path FROM jobs WHERE id = ?`,
		id,
	).Scan(&job.ID, &job.Status, &job.Err, &startedAt, &endedAt, &job.LogPath)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}

	if job.Started, err = parseTime(startedAt); err != nil {
		return nil, fmt.Errorf("parse job start time: %w", err)
	}
	if job.Ended, err = parseTime(endedAt); err != nil {
		return nil, fmt.Errorf("parse job end time: %w", err)
	}
	return job, nil
}

func timeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339Nano)
}

func parseTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
