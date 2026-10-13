package domain

import "errors"

var (
	ErrUnauthenticated = errors.New("identity is required")
	ErrUserNotFound    = errors.New("application user not found")
	ErrUserInactive    = errors.New("application user is inactive")
)
