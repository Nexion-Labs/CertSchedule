package domain

import "time"

// User is an authenticated operator of the web app.
type User struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}
