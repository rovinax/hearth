-- name: EnqueueJob :one
INSERT INTO jobs (kind, payload, max_attempts)
VALUES (?, ?, ?)
RETURNING id, kind, payload, status, attempts, max_attempts, last_error, run_after, created_at, updated_at;
