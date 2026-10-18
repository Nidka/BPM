package middleware

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type verifiedSessionToken struct{}

func (verifiedSessionToken) VerifyIDToken(context.Context, string) (contracts.Identity, error) {
	return contracts.Identity{UID: "verified", EmailVerified: true, AuthenticatedAt: time.Unix(1000, 0)}, nil
}

type sessionGate struct {
	activity bool
	err      error
	calls    int
}

func (s *sessionGate) Validate(_ context.Context, i contracts.Identity, activity bool) (contracts.SessionWindow, error) {
	if i.UID != "verified" {
		panic("unverified context")
	}
	s.calls++
	s.activity = activity
	return contracts.SessionWindow{AbsoluteExpiresAt: time.Unix(2000, 0), IdleExpiresAt: time.Unix(1500, 0)}, s.err
}
func TestSessionGateChecksEveryProtectedRequest(t *testing.T) {
	for _, tc := range []struct {
		path, method string
		activity     bool
	}{
		{"/api/v1/me", "GET", false}, {"/api/v1/organizations", "GET", true}, {"/api/v1/auth/activity", "POST", true}, {"/api/v1/auth/register", "POST", true},
		{"/api/v1/organizations/org/checklists", "GET", true},
		{"/api/v1/organizations/org/checklist-runs/run/answers", "PUT", true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			gate := &sessionGate{}
			called := false
			h := Authenticate(verifiedSessionToken{}, gate)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }))
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Authorization", "Bearer valid")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if !called || gate.calls != 1 || gate.activity != tc.activity || w.Header().Get("X-BPM-Session-Expires-At") != "2000" {
				t.Fatal("incorrect session gate", w.Code, gate)
			}
		})
	}
}
func TestSessionGateFailsClosedWithSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{contracts.ErrSessionExpired, 401, "SESSION_EXPIRED"}, {errors.New("postgres://private:secret@internal"), 503, "AUTHENTICATION_UNAVAILABLE"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			h := Authenticate(verifiedSessionToken{}, &sessionGate{err: tc.err})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("denied session reached feature") }))
			r := httptest.NewRequest("GET", "/api/v1/organizations", nil)
			r.Header.Set("Authorization", "Bearer valid")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "secret") {
				t.Fatal("unsafe denial", w.Code, w.Body.String())
			}
		})
	}
}
