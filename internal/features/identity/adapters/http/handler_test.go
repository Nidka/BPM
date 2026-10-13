package identityhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nidka/BPM/internal/core/contracts"
	httpmiddleware "github.com/Nidka/BPM/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM/internal/core/httpserver/response"
	"github.com/Nidka/BPM/internal/features/identity/domain"
	"github.com/go-chi/chi/v5"
)

type currentProfileGetterStub struct {
	profile  domain.Profile
	err      error
	identity contracts.Identity
}

func (getter *currentProfileGetterStub) Execute(_ context.Context, identity contracts.Identity) (domain.Profile, error) {
	getter.identity = identity
	return getter.profile, getter.err
}

type tokenVerifierStub struct {
	identity contracts.Identity
	err      error
}

func (verifier tokenVerifierStub) VerifyIDToken(context.Context, string) (contracts.Identity, error) {
	return verifier.identity, verifier.err
}

func TestMeReturnsAuthenticatedProfile(t *testing.T) {
	getter := &currentProfileGetterStub{profile: domain.Profile{
		ID:           "firebase-uid",
		Email:        "usuario@example.com",
		DisplayName:  "Usuario BPM",
		PlatformRole: domain.PlatformRoleUser,
		Status:       domain.UserStatusActive,
		Organizations: []domain.OrganizationMembership{{
			Organization: domain.Organization{ID: "org-1", LegalName: "Restaurante BPM"},
			Role:         domain.MembershipRoleAdmin,
			Locations: []domain.Location{{
				ID:       "location-1",
				Name:     "Local Centro",
				Address:  "Av. Principal 123",
				Timezone: "America/Lima",
			}},
		}},
	}}
	recorder := serveMe(getter, tokenVerifierStub{identity: contracts.Identity{UID: "firebase-uid"}})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if getter.identity.UID != "firebase-uid" {
		t.Fatalf("expected verified identity, got %+v", getter.identity)
	}
	var body struct {
		Data struct {
			ID            string `json:"id"`
			Organizations []struct {
				ID        string `json:"id"`
				Locations []struct {
					ID string `json:"id"`
				} `json:"locations"`
			} `json:"organizations"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.ID != "firebase-uid" || len(body.Data.Organizations) != 1 || len(body.Data.Organizations[0].Locations) != 1 {
		t.Fatalf("unexpected profile response: %+v", body)
	}
}

func TestMeRejectsUnregisteredUserWithSafeError(t *testing.T) {
	getter := &currentProfileGetterStub{err: domain.ErrUserNotFound}
	recorder := serveMe(getter, tokenVerifierStub{identity: contracts.Identity{UID: "unknown-uid"}})

	assertAPIError(t, recorder, http.StatusForbidden, "USER_NOT_REGISTERED")
}

func TestMeSanitizesInternalErrors(t *testing.T) {
	getter := &currentProfileGetterStub{err: errors.New("password=secret host=database.internal")}
	recorder := serveMe(getter, tokenVerifierStub{identity: contracts.Identity{UID: "firebase-uid"}})

	assertAPIError(t, recorder, http.StatusInternalServerError, response.CodeInternalError)
	if strings.Contains(recorder.Body.String(), "secret") || strings.Contains(recorder.Body.String(), "database.internal") {
		t.Fatalf("response leaked internal error: %s", recorder.Body.String())
	}
}

func TestMeRequiresBearerToken(t *testing.T) {
	getter := &currentProfileGetterStub{}
	var logs bytes.Buffer
	handler := NewHandler(getter, slog.New(slog.NewTextHandler(&logs, nil)))
	routes := NewRoutes(handler, httpmiddleware.Authenticate(tokenVerifierStub{}))
	router := chi.NewRouter()
	router.Use(httpmiddleware.RequestID)
	routes.Register(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

	assertAPIError(t, recorder, http.StatusUnauthorized, "UNAUTHENTICATED")
}

func serveMe(getter CurrentProfileGetter, verifier tokenVerifierStub) *httptest.ResponseRecorder {
	var logs bytes.Buffer
	handler := NewHandler(getter, slog.New(slog.NewTextHandler(&logs, nil)))
	routes := NewRoutes(handler, httpmiddleware.Authenticate(verifier))
	router := chi.NewRouter()
	router.Use(httpmiddleware.RequestID)
	routes.Register(router)

	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer valid-test-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func assertAPIError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("expected %d, got %d: %s", status, recorder.Code, recorder.Body.String())
	}
	var body response.ErrorBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Code != code {
		t.Fatalf("unexpected error response: %+v", body)
	}
	if body.RequestID == "" {
		t.Fatal("expected requestId in error response")
	}
}
