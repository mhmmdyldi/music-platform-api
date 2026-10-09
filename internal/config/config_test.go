package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const localURL = "postgres://platform:local-only-password@localhost:5433/platform_local?sslmode=disable"

func validLocal() map[string]string {
	return map[string]string{
		"APP_ENV":      "local",
		"DATABASE_URL": localURL,
	}
}

func TestLoad_AppliesDefaults(t *testing.T) {
	cfg, err := Load(FromMap(validLocal()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Env != EnvLocal {
		t.Errorf("Env = %q, want local", cfg.Env)
	}
	if cfg.ServiceName != "platform-backend" {
		t.Errorf("ServiceName = %q", cfg.ServiceName)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.HTTP.ShutdownTimeout != 20*time.Second {
		t.Errorf("HTTP.ShutdownTimeout = %v", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Log.Level != slog.LevelInfo || cfg.Log.Format != "json" {
		t.Errorf("Log = %+v", cfg.Log)
	}
	if cfg.Database.MaxConns != 10 {
		t.Errorf("Database.MaxConns = %d", cfg.Database.MaxConns)
	}
	if cfg.Database.Name() != "platform_local" {
		t.Errorf("Database.Name() = %q", cfg.Database.Name())
	}
}

func TestLoad_ReadsOverrides(t *testing.T) {
	env := validLocal()
	env["HTTP_ADDR"] = ":9999"
	env["HTTP_WRITE_TIMEOUT"] = "45s"
	env["LOG_LEVEL"] = "debug"
	env["LOG_FORMAT"] = "text"
	env["DATABASE_MAX_CONNS"] = "3"

	cfg, err := Load(FromMap(env))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTP.Addr != ":9999" || cfg.HTTP.WriteTimeout != 45*time.Second {
		t.Errorf("HTTP = %+v", cfg.HTTP)
	}
	if cfg.Log.Level != slog.LevelDebug || cfg.Log.Format != "text" {
		t.Errorf("Log = %+v", cfg.Log)
	}
	if cfg.Database.MaxConns != 3 {
		t.Errorf("MaxConns = %d", cfg.Database.MaxConns)
	}
}

// Every problem is reported in one go, so a broken deployment is fixed in one
// round trip instead of one restart per mistake.
func TestLoad_ReportsAllProblemsAtOnce(t *testing.T) {
	_, err := Load(FromMap(map[string]string{
		"HTTP_READ_TIMEOUT":  "soon",
		"DATABASE_MAX_CONNS": "many",
		"LOG_FORMAT":         "xml",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"APP_ENV is required", "DATABASE_URL is required", "HTTP_READ_TIMEOUT", "DATABASE_MAX_CONNS", "LOG_FORMAT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func TestLoad_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "unknown environment",
			env:     map[string]string{"APP_ENV": "staging", "DATABASE_URL": "postgres://u:p@h/platform_staging"},
			wantErr: `APP_ENV "staging" is not one of`,
		},
		{
			name:    "non-postgres database URL",
			env:     map[string]string{"APP_ENV": "local", "DATABASE_URL": "mysql://u:p@h/platform_local"},
			wantErr: "must be a postgres:// URL",
		},
		{
			name:    "database of another environment",
			env:     map[string]string{"APP_ENV": "local", "DATABASE_URL": "postgres://u:p@h/platform_production"},
			wantErr: `must end with "_local"`,
		},
		{
			name:    "stage pointed at the production database",
			env:     map[string]string{"APP_ENV": "stage", "DATABASE_URL": "postgres://u:p@h/platform_production?sslmode=verify-full"},
			wantErr: `must end with "_stage"`,
		},
		{
			name:    "production without TLS to the database",
			env:     map[string]string{"APP_ENV": "production", "DATABASE_URL": "postgres://u:p@h/platform_production?sslmode=disable"},
			wantErr: "sslmode=verify-full",
		},
		{
			name: "text logs in stage",
			env: map[string]string{"APP_ENV": "stage", "LOG_FORMAT": "text",
				"DATABASE_URL": "postgres://u:p@h/platform_stage?sslmode=verify-full"},
			wantErr: `LOG_FORMAT must be "json" in stage`,
		},
		{
			name: "debug logs in production",
			env: map[string]string{"APP_ENV": "production", "LOG_LEVEL": "debug",
				"DATABASE_URL": "postgres://u:p@h/platform_production?sslmode=verify-full"},
			wantErr: "LOG_LEVEL must not be debug in production",
		},
		{
			name:    "negative duration",
			env:     map[string]string{"APP_ENV": "local", "DATABASE_URL": localURL, "HTTP_IDLE_TIMEOUT": "-1s"},
			wantErr: "HTTP_IDLE_TIMEOUT",
		},
		{
			name:    "zero pool size",
			env:     map[string]string{"APP_ENV": "local", "DATABASE_URL": localURL, "DATABASE_MAX_CONNS": "0"},
			wantErr: "DATABASE_MAX_CONNS must be at least 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(FromMap(tc.env))
			if err == nil {
				t.Fatalf("expected an error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestLogValue_HidesDatabasePassword(t *testing.T) {
	cfg, err := Load(FromMap(validLocal()))
	if err != nil {
		t.Fatal(err)
	}
	logged := cfg.LogValue().String()
	if strings.Contains(logged, "local-only-password") {
		t.Fatalf("password leaked into log value: %s", logged)
	}
	if !strings.Contains(logged, "platform_local") {
		t.Fatalf("redacted URL lost the database name: %s", logged)
	}
}

// The committed env files are loaded by Docker Compose and by deployments, so
// they are tested like code: each must produce a valid configuration for the
// environment it is named after.
func TestCommittedEnvFiles(t *testing.T) {
	// Secrets are injected at deploy time, never committed. These stand-ins
	// only make the files loadable.
	injectedSecrets := map[Environment]map[string]string{
		EnvStage:      {"DATABASE_URL": "postgres://u:p@stage-db.internal:5432/platform_stage?sslmode=verify-full"},
		EnvProduction: {"DATABASE_URL": "postgres://u:p@prod-db.internal:5432/platform_production?sslmode=verify-full"},
	}

	for _, env := range []Environment{EnvLocal, EnvStage, EnvProduction} {
		t.Run(string(env), func(t *testing.T) {
			values := readEnvFile(t, string(env)+".env")

			if got := values["APP_ENV"]; got != string(env) {
				t.Fatalf("%s.env sets APP_ENV=%q", env, got)
			}

			if env.IsDeployed() {
				for key := range values {
					if looksSecret(key) {
						t.Errorf("%s.env contains %s; secrets belong in AWS Secrets Manager, not in git", env, key)
					}
				}
			}

			for k, v := range injectedSecrets[env] {
				values[k] = v
			}
			if _, err := Load(FromMap(values)); err != nil {
				t.Fatalf("%s.env does not load: %v", env, err)
			}
		})
	}
}

func looksSecret(key string) bool {
	if key == "DATABASE_URL" {
		return true
	}
	for _, marker := range []string{"PASSWORD", "SECRET", "TOKEN", "PRIVATE", "_KEY"} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func readEnvFile(t *testing.T, name string) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "config", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	values, err := ParseEnvFile(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return values
}
