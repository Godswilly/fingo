package config

import (
	"errors"
	"testing"
)

func TestLoadFromMap(t *testing.T) {
	base := map[string]string{
		"FINGO_APP_ENV":      "local",
		"FINGO_LOG_LEVEL":    "info",
		"FINGO_DATABASE_URL": "postgres://user:pass@localhost:5432/fingo?sslmode=disable",
		"FINGO_NATS_URL":     "nats://localhost:4222",
		"FINGO_API_ADDR":     ":8080",
	}

	tests := []struct {
		name      string
		role      ServiceRole
		patchEnv  map[string]string
		wantErrIs error
	}{
		{
			name: "api success",
			role: RoleAPI,
		},
		{
			name: "worker success",
			role: RoleWorker,
		},
		{
			name: "migrate success",
			role: RoleMigrate,
		},
		{
			name:      "missing app env",
			role:      RoleAPI,
			patchEnv:  map[string]string{"FINGO_APP_ENV": ""},
			wantErrIs: ErrMissingEnv,
		},
		{
			name:      "invalid app env",
			role:      RoleAPI,
			patchEnv:  map[string]string{"FINGO_APP_ENV": "production"},
			wantErrIs: ErrInvalidEnv,
		},
		{
			name:      "invalid log level",
			role:      RoleAPI,
			patchEnv:  map[string]string{"FINGO_LOG_LEVEL": "verbose"},
			wantErrIs: ErrInvalidEnv,
		},
		{
			name:      "api missing database url",
			role:      RoleAPI,
			patchEnv:  map[string]string{"FINGO_DATABASE_URL": ""},
			wantErrIs: ErrMissingEnv,
		},
		{
			name:      "api missing nats url",
			role:      RoleAPI,
			patchEnv:  map[string]string{"FINGO_NATS_URL": ""},
			wantErrIs: ErrMissingEnv,
		},
		{
			name:      "worker missing nats url",
			role:      RoleWorker,
			patchEnv:  map[string]string{"FINGO_NATS_URL": ""},
			wantErrIs: ErrMissingEnv,
		},
		{
			name:      "migrate missing database url",
			role:      RoleMigrate,
			patchEnv:  map[string]string{"FINGO_DATABASE_URL": ""},
			wantErrIs: ErrMissingEnv,
		},
		{
			name:      "unknown role",
			role:      ServiceRole("scheduler"),
			wantErrIs: ErrUnknownRole,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := cloneMap(base)
			for k, v := range tc.patchEnv {
				env[k] = v
			}

			cfg, err := LoadFromMap(tc.role, env)
			if tc.wantErrIs != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.wantErrIs)
				}
				if !errors.Is(err, tc.wantErrIs) {
					t.Fatalf("expected error to wrap %v, got %v", tc.wantErrIs, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}

			if cfg.AppEnv == "" || cfg.LogLevel == "" {
				t.Fatalf("expected populated common config, got %+v", cfg)
			}
		})
	}
}

func TestLoadFromMap_DefaultAPIAddr(t *testing.T) {
	env := map[string]string{
		"FINGO_APP_ENV":      "local",
		"FINGO_LOG_LEVEL":    "info",
		"FINGO_DATABASE_URL": "postgres://user:pass@localhost:5432/fingo?sslmode=disable",
		"FINGO_NATS_URL":     "nats://localhost:4222",
	}

	cfg, err := LoadFromMap(RoleAPI, env)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if cfg.APIAddr != ":8080" {
		t.Fatalf("expected default API addr :8080, got %q", cfg.APIAddr)
	}
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
