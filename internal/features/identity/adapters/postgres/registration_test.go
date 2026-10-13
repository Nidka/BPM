package postgres

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

// Todos los datos de prueba viven en una tabla TEMP de una única conexión.
// Nunca se insertan cuentas en public.application_users.
func TestRegistrationPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.MaxConns = 1
	cfg.MinConns = 0
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `CREATE TEMP TABLE application_users (
   id VARCHAR(128) PRIMARY KEY, email VARCHAR(320) NOT NULL, display_name VARCHAR(200) NOT NULL,
   phone_number VARCHAR(30), platform_role VARCHAR(20) NOT NULL DEFAULT 'USER', status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
   created_by VARCHAR(128), updated_by VARCHAR(128));
   CREATE UNIQUE INDEX test_user_email_unique ON application_users (LOWER(email));`)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot initialize test pool")
	}
	defer pool.Close()
	repo := NewRepository(pool)
	user := domain.NewUser{ID: "test-uid-1", Email: "person@example.invalid", DisplayName: "Persona"}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatal("cannot register test user")
	}
	var role, status, actor, name string
	var created, updated time.Time
	err = pool.QueryRow(ctx, `SELECT platform_role,status,created_by,display_name,created_at,updated_at FROM application_users WHERE id=$1`, user.ID).Scan(&role, &status, &actor, &name, &created, &updated)
	if err != nil || role != "USER" || status != "ACTIVE" || actor != user.ID || created.IsZero() || updated.IsZero() {
		t.Fatal("registration did not enforce role/status/audit")
	}
	profile, err := repo.FindByUserID(ctx, user.ID)
	if err != nil || profile.ID != user.ID || profile.PlatformRole != domain.PlatformRoleUser || len(profile.Organizations) != 0 {
		t.Fatal("registered user must have a readable profile without organization access")
	}
	if _, err := pool.Exec(ctx, `UPDATE application_users SET status='INACTIVE' WHERE id=$1`, user.ID); err != nil {
		t.Fatal("cannot prepare replay fixture")
	}
	user.DisplayName = "Changed"
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatal("idempotent replay failed")
	}
	if err := pool.QueryRow(ctx, `SELECT status,display_name FROM application_users WHERE id=$1`, user.ID).Scan(&status, &name); err != nil || status != "INACTIVE" || name != "Persona" {
		t.Fatal("replay modified an existing user")
	}
	user.ID = "other-uid"
	user.Email = "PERSON@example.invalid"
	if !errors.Is(repo.CreateUser(ctx, user), domain.ErrRegistrationConflict) {
		t.Fatal("email collision must not merge identities")
	}
	var count int
	if pool.QueryRow(ctx, `SELECT COUNT(*) FROM application_users`).Scan(&count) != nil || count != 1 {
		t.Fatal("unexpected registration rows")
	}
}
