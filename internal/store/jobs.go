package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNoJob = errors.New("no job")

type Job struct {
	ID          int64
	Kind        string
	Payload     string
	Attempts    int
	MaxAttempts int
}

func (s *Store) RequeueRunning(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'pending',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE status = 'running'`)
	return err
}

func (s *Store) Enqueue(ctx context.Context, kind, payload string, maxAttempts int) (int64, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO jobs (kind, payload, max_attempts)
VALUES (?, ?, ?)
RETURNING id`, kind, payload, maxAttempts).Scan(&id)
	return id, err
}

func (s *Store) ClaimJob(ctx context.Context) (Job, error) {
	var job Job
	err := s.db.QueryRowContext(ctx, `
UPDATE jobs
SET status = 'running',
    attempts = attempts + 1,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
      AND run_after <= strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    ORDER BY id
    LIMIT 1
)
RETURNING id, kind, payload, attempts, max_attempts`).Scan(
		&job.ID, &job.Kind, &job.Payload, &job.Attempts, &job.MaxAttempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNoJob
	}
	return job, err
}

func (s *Store) FinishJob(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = 'done',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, id)
	return err
}

func (s *Store) FailJob(ctx context.Context, job Job, message string) error {
	status := "pending"
	modifier := fmt.Sprintf("+%d seconds", job.Attempts)
	if job.Attempts >= job.MaxAttempts {
		status = "failed"
		modifier = "+0 seconds"
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE jobs
SET status = ?,
    last_error = ?,
    run_after = strftime('%Y-%m-%dT%H:%M:%fZ', 'now', ?),
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?`, status, message, modifier, job.ID)
	return err
}
