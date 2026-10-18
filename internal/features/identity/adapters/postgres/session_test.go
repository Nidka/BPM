package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/features/identity/application"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
	"time"
)

func TestSessionPolicyPostgres(t *testing.T) {
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
	schema := pgx.Identifier{fmt.Sprintf("session_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = NewRepository(pool).RequireSessionSchema(ctx); err == nil {
		t.Fatal("missing migration was accepted")
	}
	migration, err := os.ReadFile("../../../../../migraciones/V2__authentication_sessions.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	if err = NewRepository(pool).RequireSessionSchema(ctx); err != nil {
		t.Fatal("applied migration not recognized", err)
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	identity := contracts.Identity{UID: "person", AuthenticatedAt: now}
	replica := func() *application.Sessions {
		return application.NewSessions(NewRepository(pool), 8*time.Hour, 15*time.Minute, func() time.Time { return now })
	}
	a, b := replica(), replica()
	if _, err = a.Validate(ctx, identity, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(14 * time.Minute)
	if _, err = b.Validate(ctx, identity, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	// Consultar /me no actualiza la actividad; al alcanzar el límite, el contexto queda vencido.
	for _, s := range []*application.Sessions{a, b, replica()} {
		if _, err = s.Validate(ctx, identity, true); !errors.Is(err, contracts.ErrSessionExpired) {
			t.Fatal("idle session reopened", err)
		}
	}
	// Una autenticación nueva crea otro contexto; reutilizar el UID no reabre el anterior.
	identity.AuthenticatedAt = now
	if _, err = a.Validate(ctx, identity, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(14 * time.Minute)
	if _, err = b.Validate(ctx, identity, true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err = a.Validate(ctx, identity, false); err != nil {
		t.Fatal("real activity did not extend idle window", err)
	}
	if err = a.End(ctx, identity); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, e := b.Validate(ctx, identity, true); !errors.Is(e, contracts.ErrSessionExpired) {
				t.Error("revoked context reopened", e)
			}
		})
	}
	wg.Wait()
	// Otra cuenta con el mismo auth_time conserva un contexto independiente.
	identity.UID = "other-person"
	if _, err = b.Validate(ctx, identity, true); err != nil {
		t.Fatal("revocation crossed identities", err)
	}
	now = identity.AuthenticatedAt.Add(8 * time.Hour)
	if _, err = a.Validate(ctx, identity, true); !errors.Is(err, contracts.ErrSessionExpired) {
		t.Fatal("absolute boundary bypassed", err)
	}
}
