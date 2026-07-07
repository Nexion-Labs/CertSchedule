package domain

import "errors"

var (
	ErrNotFound          = errors.New("resource not found")
	ErrAlreadyExists     = errors.New("resource already exists")
	ErrInvalidCredential = errors.New("invalid username or password")
	ErrInvalidInput      = errors.New("invalid input")
)
