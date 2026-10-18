package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	organizationhttp "github.com/Nidka/BPM-service/internal/features/organizations/adapters/http"
	"github.com/Nidka/BPM-service/internal/features/organizations/application"
	"github.com/Nidka/BPM-service/internal/features/organizations/domain"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Firebase is substituted here; authorization and queries use real PostgreSQL.
type tenantTestVerifier struct{}

func (tenantTestVerifier) VerifyIDToken(_ context.Context, token string) (contracts.Identity, error) {
	if token != "worker" && token != "admin" && token != "chef" {
		return contracts.Identity{}, errors.New("invalid token")
	}
	return contracts.Identity{UID: token, EmailVerified: true}, nil
}

// Solo una base explícitamente de pruebas. Cada ejecución usa un esquema propio
// con la migración real; no toca usuarios u organizaciones de public.
func TestWorkspacePostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || cfg.ConnConfig.Database != "bpm_test" {
		t.Fatal("requires database bpm_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("cannot connect to test database")
	}
	defer admin.Close(context.Background())
	schema := pgx.Identifier{fmt.Sprintf("workspace_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.MaxConns = 2
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	baseline, err := os.ReadFile("../../../../../migraciones/V1__baseline.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(baseline)); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO application_users(id,email,display_name,platform_role) VALUES
      ('root','root@example.invalid','Root','SUPER_ADMIN'),
      ('admin','admin@example.invalid','Admin','USER'),
      ('worker','worker@example.invalid','Worker','USER'),
      ('outsider','outsider@example.invalid','Outsider','USER')`)
	service := application.New(New(pool))
	org := domain.Organization{ID: "10000000-0000-0000-0000-000000000001", LegalName: "Restaurante Uno", BusinessType: "RESTAURANT"}
	other := domain.Organization{ID: "10000000-0000-0000-0000-000000000002", LegalName: "Restaurante Dos", BusinessType: "RESTAURANT"}
	loc := domain.Location{ID: "20000000-0000-0000-0000-000000000001", Name: "Centro", Address: "Av. Principal 123"}
	otherLoc := domain.Location{ID: "20000000-0000-0000-0000-000000000002", Name: "Norte", Address: "Av. Norte 123"}
	if _, err = service.CreateOrganization(ctx, "admin", org); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("user created tenant", err)
	}
	for _, input := range []domain.Organization{org, other, org} {
		if _, err = service.CreateOrganization(ctx, "root", input); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&count); err != nil || count != 2 {
		t.Fatal("replay duplicated audit", err, count)
	}
	changed := org
	changed.LegalName = "Different"
	if _, err = service.CreateOrganization(ctx, "root", changed); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("replay changed organization", err)
	}
	for _, attempt := range []struct {
		org      string
		location domain.Location
	}{{org.ID, loc}, {other.ID, otherLoc}, {org.ID, loc}} {
		if _, err = service.CreateLocation(ctx, "root", attempt.org, attempt.location); err != nil {
			t.Fatal(err)
		}
	}
	if err = service.AddMember(ctx, "root", org.ID, "admin@example.invalid", "ADMIN", nil); err != nil {
		t.Fatal(err)
	}
	if err = service.AddMember(ctx, "admin", org.ID, "worker@example.invalid", "WORKER", []string{otherLoc.ID}); err == nil {
		t.Fatal("accepted foreign location")
	}
	if err = service.AddMember(ctx, "admin", org.ID, "worker@example.invalid", "WORKER", []string{loc.ID, loc.ID}); err != nil {
		t.Fatal(err)
	}
	if err = service.AddMember(ctx, "admin", org.ID, "worker@example.invalid", "ADMIN", nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("duplicate changed role", err)
	}
	if _, err = service.CreateLocation(ctx, "worker", org.ID, otherLoc); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("worker wrote location", err)
	}
	if _, err = service.Workspace(ctx, "worker", other.ID); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("cross tenant read", err)
	}
	if _, err = service.Workspace(ctx, "outsider", org.ID); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("outsider read tenant", err)
	}
	exec(`INSERT INTO training_courses(organization_id,code,title,status) VALUES($1,'BPM','Lavado de manos','PUBLISHED'),($2,'BPM','Private course','PUBLISHED'),($1,'DRAFT','Hidden course','DRAFT')`, org.ID, other.ID)
	w, err := service.Workspace(ctx, "worker", org.ID)
	if err != nil || len(w.Locations) != 1 || len(w.Members) != 0 || len(w.Courses) != 1 || w.Courses[0].Title != "Lavado de manos" {
		t.Fatalf("worker scope: %+v %v", w, err)
	}
	w, err = service.Workspace(ctx, "admin", org.ID)
	if err != nil || len(w.Members) != 2 {
		t.Fatalf("admin workspace: %+v %v", w, err)
	}
	orgs, err := service.List(ctx, "worker")
	if err != nil || len(orgs) != 1 || orgs[0].ID != org.ID {
		t.Fatal("tenant list scope", err)
	}
	exec(`INSERT INTO application_users(id,email,display_name,platform_role) VALUES('chef','chef@example.invalid','Chef','USER')`)
	if err = service.AddMember(ctx, "admin", org.ID, "chef@example.invalid", "CHEF", []string{loc.ID}); err != nil {
		t.Fatal(err)
	}
	chef, err := service.Workspace(ctx, "chef", org.ID)
	if err != nil || len(chef.Locations) != 1 || len(chef.Members) != 0 {
		t.Fatal("chef scope", err)
	}
	router := chi.NewRouter()
	organizationhttp.New(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(router, middleware.Authenticate(tenantTestVerifier{}))
	for _, tc := range []struct {
		name, token, method, path, body string
		status                          int
	}{
		{"worker cannot change tenant in URL", "worker", "GET", "/organizations/" + other.ID + "/workspace", "", 403},
		{"admin cannot write another tenant", "admin", "POST", "/organizations/" + other.ID + "/locations", `{"id":"20000000-0000-0000-0000-000000000099","name":"Valid","address":"Valid address"}`, 403},
		{"worker cannot add members", "worker", "POST", "/organizations/" + org.ID + "/members", `{"email":"outsider@example.invalid","role":"ADMIN","locationIds":[]}`, 403},
		{"chef cannot write locations", "chef", "POST", "/organizations/" + org.ID + "/locations", `{"id":"20000000-0000-0000-0000-000000000099","name":"Valid","address":"Valid address"}`, 403},
		{"platform privilege cannot be assigned", "admin", "POST", "/organizations/" + org.ID + "/members", `{"email":"outsider@example.invalid","role":"SUPER_ADMIN","locationIds":[]}`, 422},
		{"worker cannot see team via spoofed role", "worker", "GET", "/organizations/" + org.ID + "/workspace", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Authorization", "Bearer "+tc.token)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Organization-ID", other.ID)
			request.Header.Set("X-Role", "SUPER_ADMIN")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatal("unexpected access", recorder.Code, recorder.Body.String())
			}
			if tc.status == 200 && (!strings.Contains(recorder.Body.String(), `"role":"WORKER"`) || !strings.Contains(recorder.Body.String(), `"members":[]`)) {
				t.Fatal("client headers granted permissions")
			}
		})
	}
	exec(`UPDATE organization_memberships SET status='INACTIVE' WHERE organization_id=$1 AND user_id='worker'`, org.ID)
	if _, err = service.Workspace(ctx, "worker", org.ID); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("inactive membership accepted")
	}
	exec(`UPDATE organizations SET status='INACTIVE' WHERE id=$1`, other.ID)
	if _, err = service.Workspace(ctx, "root", other.ID); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("inactive tenant accepted", err)
	}
	exec(`UPDATE application_users SET status='INACTIVE' WHERE id='root'`)
	if _, err = service.List(ctx, "root"); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("inactive superadmin accepted")
	}
	var actor string
	if err = pool.QueryRow(ctx, `SELECT created_by FROM locations WHERE id=$1`, loc.ID).Scan(&actor); err != nil || actor != "root" {
		t.Fatal("audit actor missing")
	}
}
