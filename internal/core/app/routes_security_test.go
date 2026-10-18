package app

import (
	"context"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/features/checklists"
	"github.com/Nidka/BPM-service/internal/features/organizations"
	"github.com/go-chi/chi/v5"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

type validToken struct{}

func (validToken) VerifyIDToken(context.Context, string) (contracts.Identity, error) {
	return contracts.Identity{UID: "person", EmailVerified: true}, nil
}

type expiredSession struct{}

func (expiredSession) Validate(context.Context, contracts.Identity, bool) (contracts.SessionWindow, error) {
	return contracts.SessionWindow{}, contracts.ErrSessionExpired
}

func TestBusinessModulesDenyExpiredSessionsBeforeDatabaseAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := chi.NewRouter()
	organizations.Routes(nil, validToken{}, logger, expiredSession{})(r)
	checklists.Routes(nil, validToken{}, logger, expiredSession{})(r)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/organizations"}, {"POST", "/organizations/org/members"},
		{"GET", "/organizations/org/checklists"}, {"POST", "/organizations/org/checklist-runs"},
		{"PUT", "/organizations/org/checklist-runs/run/answers"}, {"POST", "/organizations/org/checklist-runs/run/submit"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != 401 || !strings.Contains(rec.Body.String(), "SESSION_EXPIRED") {
				t.Fatal("module did not enforce session", rec.Code, rec.Body.String())
			}
		})
	}
}
