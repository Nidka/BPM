package contracts

import "context"

// Identity is the identity proven by the external authentication provider.
// Organization access and roles are intentionally resolved from PostgreSQL.
type Identity struct {
	UID           string
	Email         string
	EmailVerified bool
}

// TokenVerifier is the inbound identity port used by HTTP authentication.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (Identity, error)
}
