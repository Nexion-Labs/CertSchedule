package application

import (
	"context"
	"testing"

	"certschedule/internal/domain"

	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_Login_Success(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	users := newFakeUserRepo()
	users.byUsername["admin"] = &domain.User{ID: "u1", Username: "admin", PasswordHash: string(hash)}

	svc := NewAuthService(users, &fakeTokenIssuer{token: "signed-jwt"})

	token, err := svc.Login(context.Background(), "admin", "correct-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "signed-jwt" {
		t.Errorf("expected token to be returned, got %q", token)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	users := newFakeUserRepo()
	users.byUsername["admin"] = &domain.User{ID: "u1", Username: "admin", PasswordHash: string(hash)}

	svc := NewAuthService(users, &fakeTokenIssuer{token: "signed-jwt"})

	_, err := svc.Login(context.Background(), "admin", "wrong-password")
	if err != domain.ErrInvalidCredential {
		t.Errorf("expected ErrInvalidCredential, got %v", err)
	}
}

func TestAuthService_Login_UnknownUser(t *testing.T) {
	users := newFakeUserRepo()
	svc := NewAuthService(users, &fakeTokenIssuer{token: "signed-jwt"})

	_, err := svc.Login(context.Background(), "ghost", "whatever")
	if err != domain.ErrInvalidCredential {
		t.Errorf("expected ErrInvalidCredential, got %v", err)
	}
}
