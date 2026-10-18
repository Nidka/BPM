package firebaseauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/Nidka/BPM-service/internal/core/config"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"google.golang.org/api/option"
)

var ErrInvalidToken = errors.New("invalid authentication token")

// El puerto conserva la comprobación de revocación para cualquier proveedor.
type revocationTokenClient interface {
	VerifyIDTokenAndCheckRevoked(context.Context, string) (*auth.Token, error)
}

type Verifier struct {
	client revocationTokenClient
}

func New(ctx context.Context, cfg config.FirebaseConfig) (*Verifier, error) {
	credentials, err := base64.StdEncoding.DecodeString(cfg.ServiceAccountJSONBase64)
	if err != nil {
		return nil, errors.New("decode Firebase service account")
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.ProjectID}, option.WithCredentialsJSON(credentials))
	if err != nil {
		return nil, fmt.Errorf("initialize Firebase: %w", err)
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize Firebase Auth: %w", err)
	}
	return &Verifier{client: client}, nil
}

func (v *Verifier) VerifyIDToken(ctx context.Context, rawToken string) (contracts.Identity, error) {
	// Revocation checking is deliberate for sensitive operational data. If this
	// becomes a latency concern, add a short, bounded cache instead of skipping it.
	token, err := v.client.VerifyIDTokenAndCheckRevoked(ctx, rawToken)
	if err != nil || token == nil || token.UID == "" {
		return contracts.Identity{}, ErrInvalidToken
	}

	email, _ := token.Claims["email"].(string)
	emailVerified, _ := token.Claims["email_verified"].(bool)
	return contracts.Identity{
		UID:             token.UID,
		Email:           email,
		EmailVerified:   emailVerified,
		AuthenticatedAt: time.Unix(token.AuthTime, 0).UTC(),
	}, nil
}

var _ contracts.TokenVerifier = (*Verifier)(nil)
