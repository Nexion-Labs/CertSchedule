package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"certschedule/internal/domain"

	"github.com/google/uuid"
)

// JobRepo implements domain.JobRepository backed by SQLite.
type JobRepo struct {
	db *sql.DB
}

func NewJobRepo(db *sql.DB) *JobRepo {
	return &JobRepo{db: db}
}

func (r *JobRepo) Create(ctx context.Context, j *domain.RenewalJob) error {
	if j.ID == "" {
		j.ID = uuid.NewString()
	}
	if j.StartedAt.IsZero() {
		j.StartedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO renewal_jobs (id, domain_id, trigger, status, message, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.DomainID, string(j.Trigger), string(j.Status), j.Message, j.StartedAt, j.FinishedAt,
	)
	if err != nil {
		return fmt.Errorf("insert renewal job: %w", err)
	}
	return nil
}

func (r *JobRepo) Update(ctx context.Context, j *domain.RenewalJob) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE renewal_jobs SET status=?, message=?, finished_at=? WHERE id=?`,
		string(j.Status), j.Message, j.FinishedAt, j.ID,
	)
	if err != nil {
		return fmt.Errorf("update renewal job: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *JobRepo) ListForDomain(ctx context.Context, domainID string) ([]*domain.RenewalJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, domain_id, trigger, status, message, started_at, finished_at
		FROM renewal_jobs WHERE domain_id=? ORDER BY started_at DESC`, domainID)
	if err != nil {
		return nil, fmt.Errorf("list renewal jobs: %w", err)
	}
	defer rows.Close()

	var out []*domain.RenewalJob
	for rows.Next() {
		var j domain.RenewalJob
		var trigger, status string
		if err := rows.Scan(&j.ID, &j.DomainID, &trigger, &status, &j.Message, &j.StartedAt, &j.FinishedAt); err != nil {
			return nil, err
		}
		j.Trigger = domain.TriggerType(trigger)
		j.Status = domain.JobStatus(status)
		out = append(out, &j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
