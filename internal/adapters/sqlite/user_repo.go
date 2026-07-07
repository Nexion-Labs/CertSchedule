package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"certschedule/internal/domain"

	"github.com/google/uuid"
)

// UserRepo implements domain.UserRepository backed by SQLite.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, created_at FROM users WHERE username=?`, username)
	var u domain.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// EnsureUser creates the user if it does not already exist (used to seed the
// single admin account from config on startup).
func (r *UserRepo) EnsureUser(ctx context.Context, username, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash`,
		uuid.NewString(), username, passwordHash, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	return nil
}
