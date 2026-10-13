package identityhttp

import (
	"context"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	m "github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"github.com/go-chi/chi/v5"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type registrarStub struct {
	calls    int
	identity contracts.Identity
	name     string
	err      error
}

func (s *registrarStub) Execute(_ context.Context, identity contracts.Identity, name string) (domain.Profile, error) {
	s.calls++
	s.identity = identity
	s.name = name
	return domain.Profile{ID: identity.UID, DisplayName: name, Organizations: []domain.OrganizationMembership{}}, s.err
}
func registerRequest(t *testing.T, body, token string, stub *registrarStub) *httptest.ResponseRecorder {
	t.Helper()
	h := NewRegisterHandler(stub, slog.New(slog.NewTextHandler(io.Discard, nil)))
	router := chi.NewRouter()
	router.Use(m.RequestID)
	router.With(m.Authenticate(tokenVerifierStub{identity: contracts.Identity{UID: "verified-uid", Email: "verified@example.com", EmailVerified: true}})).Post("/auth/register", h.Register)
	r := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}
func TestRegistrationCannotChooseIdentityOrPermissions(t *testing.T) {
	for _, body := range []string{`{"displayName":"Person","id":"other"}`, `{"displayName":"Person","platformRole":"SUPER_ADMIN"}`, `{"displayName":"Person","email":"other@example.com"}`, `{"displayName":"Person"} {}`, `{"displayName":`} {
		stub := &registrarStub{}
		w := registerRequest(t, body, "token", stub)
		assertAPIError(t, w, 400, "INVALID_REQUEST")
		if stub.calls != 0 {
			t.Fatal("invalid input reached use case")
		}
	}
	stub := &registrarStub{}
	w := registerRequest(t, `{"displayName":"Person"}`, "token", stub)
	if w.Code != 200 || stub.identity.UID != "verified-uid" || stub.identity.Email != "verified@example.com" || stub.name != "Person" {
		t.Fatal("identity must come from verified token")
	}
}
func TestRegistrationRequiresAuthenticationAndSanitizesErrors(t *testing.T) {
	stub := &registrarStub{}
	assertAPIError(t, registerRequest(t, `{"displayName":"Person"}`, "", stub), 401, "UNAUTHENTICATED")
	if stub.calls != 0 {
		t.Fatal("unauthenticated registration")
	}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrEmailUnverified, 403, "EMAIL_NOT_VERIFIED"}, {domain.ErrRegistrationConflict, 409, "REGISTRATION_CONFLICT"},
		{&domain.ValidationError{Fields: map[string]string{"displayName": "Invalid name"}}, 422, "VALIDATION_ERROR"},
	} {
		stub.err = tc.err
		assertAPIError(t, registerRequest(t, `{"displayName":"Person"}`, "token", stub), tc.status, tc.code)
	}
}
