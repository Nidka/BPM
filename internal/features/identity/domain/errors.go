package domain

import "errors"

var (
	ErrUnauthenticated      = errors.New("identity is required")
	ErrUserNotFound         = errors.New("application user not found")
	ErrUserInactive         = errors.New("application user is inactive")
	ErrEmailUnverified      = errors.New("verified email is required")
	ErrRegistrationConflict = errors.New("registration conflict")
)

type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid registration fields" }

type NewUser struct{ ID, Email, DisplayName string }
