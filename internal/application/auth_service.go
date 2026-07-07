package application

import (
	"context"
	"fmt"

	"certschedule/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

// TokenIssuer issues signed session tokens for authenticated users.
type TokenIssuer interface {
	Issue(userID, username string) (string, error)
}

// AuthService implements username/password login backed by bcrypt-hashed
// passwords, issuing a JWT on success.
type AuthService struct {
	users  domain.UserRepository
	tokens TokenIssuer
}

func NewAuthService(users domain.UserRepository, tokens TokenIssuer) *AuthService {
	return &AuthService{users: users, tokens: tokens}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, error) {
	u, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		if err == domain.ErrNotFound {
			return "", domain.ErrInvalidCredential
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrInvalidCredential
	}

	token, err := s.tokens.Issue(u.ID, u.Username)
	if err != nil {
		return "", fmt.Errorf("issue token: %w", err)
	}
	return token, nil
}
