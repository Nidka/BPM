package contracts

import (
	"context"
	"errors"
	"time"
)

// Identity is the identity proven by the external authentication provider.
// Organization access and roles are intentionally resolved from PostgreSQL.
type Identity struct {
	UID             string
	Email           string
	EmailVerified   bool
	AuthenticatedAt time.Time
}

var ErrSessionExpired = errors.New("session expired")

// SessionValidator is supplied by identity; every protected feature uses it.
type SessionValidator interface {
	Validate(context.Context, Identity, bool) (SessionWindow, error)
}

type SessionWindow struct{ AbsoluteExpiresAt, IdleExpiresAt time.Time }

// TokenVerifier is the inbound identity port used by HTTP authentication.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (Identity, error)
}
