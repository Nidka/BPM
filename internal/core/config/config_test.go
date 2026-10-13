package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadFromDevelopment(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL": "postgresql://postgres:secret@localhost:5432/bpm",
	}

	cfg, err := loadFrom(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.HTTP.Port)
	}
	if cfg.Database.MaxConnections != 10 {
		t.Fatalf("expected default pool max 10, got %d", cfg.Database.MaxConnections)
	}
	if cfg.Firebase.Enabled || cfg.R2.Enabled {
		t.Fatal("external integrations must be opt-in during development")
	}
}

func TestProductionRequiresSecurityIntegrations(t *testing.T) {
	values := map[string]string{
		"APP_ENV":              Production,
		"DATABASE_URL":         "postgresql://postgres:secret@localhost:5432/bpm",
		"CORS_ALLOWED_ORIGINS": "https://app.example.com",
	}

	_, err := loadFrom(func(key string) string { return values[key] })
	if err == nil {
		t.Fatal("expected production validation error")
	}
	if !strings.Contains(err.Error(), "FIREBASE_ENABLED") || !strings.Contains(err.Error(), "R2_ENABLED") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProductionConfiguration(t *testing.T) {
	credentials := base64.StdEncoding.EncodeToString([]byte(`{"type":"service_account"}`))
	values := map[string]string{
		"APP_ENV":                              Production,
		"DATABASE_URL":                         "postgresql://postgres:secret@postgres.railway.internal:5432/bpm",
		"CORS_ALLOWED_ORIGINS":                 "https://app.example.com",
		"FIREBASE_ENABLED":                     "true",
		"FIREBASE_PROJECT_ID":                  "bpm-production",
		"FIREBASE_SERVICE_ACCOUNT_JSON_BASE64": credentials,
		"R2_ENABLED":                           "true",
		"R2_ACCOUNT_ID":                        "account",
		"R2_ACCESS_KEY_ID":                     "key",
		"R2_SECRET_ACCESS_KEY":                 "secret",
		"R2_BUCKET_NAME":                       "bpm-private",
		"R2_ENDPOINT":                          "https://account.r2.cloudflarestorage.com",
	}

	if _, err := loadFrom(func(key string) string { return values[key] }); err != nil {
		t.Fatalf("expected valid production config: %v", err)
	}
}

func TestProductionRejectsWildcardCORS(t *testing.T) {
	values := map[string]string{
		"APP_ENV":              Production,
		"DATABASE_URL":         "postgresql://postgres:secret@localhost:5432/bpm",
		"CORS_ALLOWED_ORIGINS": "*",
		"FIREBASE_ENABLED":     "false",
		"R2_ENABLED":           "false",
	}

	_, err := loadFrom(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "CORS wildcard") {
		t.Fatalf("expected wildcard CORS error, got %v", err)
	}
}
