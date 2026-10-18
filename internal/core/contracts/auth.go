package contracts

import (
	"context"
	"errors"
	"time"
)

// Identity representa la identidad verificada por el proveedor de autenticación.
// El acceso a organizaciones y los roles se resuelven en PostgreSQL.
type Identity struct {
	UID             string
	Email           string
	EmailVerified   bool
	AuthenticatedAt time.Time
}

var ErrSessionExpired = errors.New("session expired")

// SessionValidator es el contrato de validación que identidad proporciona a los módulos protegidos.
type SessionValidator interface {
	Validate(context.Context, Identity, bool) (SessionWindow, error)
}

type SessionWindow struct{ AbsoluteExpiresAt, IdleExpiresAt time.Time }

// TokenVerifier es el puerto de verificación de identidad utilizado por la autenticación HTTP.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (Identity, error)
}
