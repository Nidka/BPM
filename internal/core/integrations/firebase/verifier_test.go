package firebaseauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/Nidka/BPM-service/internal/core/contracts"
)

type tokenClientStub struct {
	token *auth.Token
	err   error
	calls int
}

func (c *tokenClientStub) VerifyIDTokenAndCheckRevoked(_ context.Context, raw string) (*auth.Token, error) {
	c.calls++
	if raw != "firebase-test-token" {
		return nil, errors.New("unexpected token")
	}
	return c.token, c.err
}

func TestVerifierUsesRevocationCheckForEveryProvider(t *testing.T) {
	for _, provider := range []string{"password", "google.com"} {
		t.Run(provider, func(t *testing.T) {
			client := &tokenClientStub{token: &auth.Token{UID: "firebase-uid", AuthTime: 12345, Claims: map[string]interface{}{
				"email": "person@example.com", "email_verified": true,
				"firebase": map[string]interface{}{"sign_in_provider": provider},
			}}}
			identity, err := (&Verifier{client: client}).VerifyIDToken(context.Background(), "firebase-test-token")
			if err != nil || identity.UID != "firebase-uid" || !identity.EmailVerified || client.calls != 1 {
				t.Fatalf("expected verified Firebase UID through revocation check; identity=%+v err=%v calls=%d", identity, err, client.calls)
			}
			if !identity.AuthenticatedAt.Equal(time.Unix(12345, 0)) {
				t.Fatal("auth_time was not propagated")
			}
		})
	}
}

func TestVerifierSanitizesRejectedAndMalformedTokens(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token *auth.Token
		err   error
	}{
		{name: "revoked", err: errors.New("private provider detail")},
		{name: "nil token"},
		{name: "empty UID", token: &auth.Token{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &tokenClientStub{token: tc.token, err: tc.err}
			identity, err := (&Verifier{client: client}).VerifyIDToken(context.Background(), "firebase-test-token")
			if err != ErrInvalidToken || identity != (contracts.Identity{}) {
				t.Fatalf("expected sanitized rejection, got identity=%+v err=%v", identity, err)
			}
		})
	}
}
