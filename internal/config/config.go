package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type ServiceRole string

const (
	RoleAPI     ServiceRole = "api"
	RoleWorker  ServiceRole = "worker"
	RoleMigrate ServiceRole = "migrate"
)

var (
	ErrMissingEnv    = errors.New("missing required environment variable")
	ErrInvalidEnv    = errors.New("invalid environment value")
	ErrUnknownRole   = errors.New("unknown service role")
	allowedAppEnv    = map[string]struct{}{"local": {}, "dev": {}, "staging": {}, "prod": {}}
	allowedLogLevels = map[string]struct{}{"debug": {}, "info": {}, "warn": {}, "error": {}}
)

type Config struct {
	AppEnv      string
	LogLevel    string
	DatabaseURL string
	NATSURL     string
	APIAddr     string
}

func Load(role ServiceRole) (Config, error) {
	return LoadFromMap(role, getOSEnvMap())
}

func LoadFromMap(role ServiceRole, env map[string]string) (Config, error) {
	cfg := Config{
		AppEnv:   strings.ToLower(strings.TrimSpace(env["FINGO_APP_ENV"])),
		LogLevel: strings.ToLower(strings.TrimSpace(env["FINGO_LOG_LEVEL"])),
	}

	if err := validateCommon(cfg); err != nil {
		return Config{}, err
	}

	switch role {
	case RoleAPI:
		cfg.DatabaseURL = strings.TrimSpace(env["FINGO_DATABASE_URL"])
		cfg.NATSURL = strings.TrimSpace(env["FINGO_NATS_URL"])
		cfg.APIAddr = strings.TrimSpace(env["FINGO_API_ADDR"])
		if cfg.APIAddr == "" {
			cfg.APIAddr = ":8080"
		}
		if err := require("FINGO_DATABASE_URL", cfg.DatabaseURL); err != nil {
			return Config{}, err
		}
		if err := require("FINGO_NATS_URL", cfg.NATSURL); err != nil {
			return Config{}, err
		}
	case RoleWorker:
		cfg.DatabaseURL = strings.TrimSpace(env["FINGO_DATABASE_URL"])
		cfg.NATSURL = strings.TrimSpace(env["FINGO_NATS_URL"])
		if err := require("FINGO_DATABASE_URL", cfg.DatabaseURL); err != nil {
			return Config{}, err
		}
		if err := require("FINGO_NATS_URL", cfg.NATSURL); err != nil {
			return Config{}, err
		}
	case RoleMigrate:
		cfg.DatabaseURL = strings.TrimSpace(env["FINGO_DATABASE_URL"])
		if err := require("FINGO_DATABASE_URL", cfg.DatabaseURL); err != nil {
			return Config{}, err
		}
	default:
		return Config{}, fmt.Errorf("%w: %q", ErrUnknownRole, role)
	}

	return cfg, nil
}

func validateCommon(cfg Config) error {
	if err := require("FINGO_APP_ENV", cfg.AppEnv); err != nil {
		return err
	}
	if _, ok := allowedAppEnv[cfg.AppEnv]; !ok {
		return fmt.Errorf("%w: FINGO_APP_ENV must be one of local|dev|staging|prod", ErrInvalidEnv)
	}

	if err := require("FINGO_LOG_LEVEL", cfg.LogLevel); err != nil {
		return err
	}
	if _, ok := allowedLogLevels[cfg.LogLevel]; !ok {
		return fmt.Errorf("%w: FINGO_LOG_LEVEL must be one of debug|info|warn|error", ErrInvalidEnv)
	}
	return nil
}

func require(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%w: %s", ErrMissingEnv, name)
	}
	return nil
}

func getOSEnvMap() map[string]string {
	out := make(map[string]string)
	for _, kv := range os.Environ() {
		key, val, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		out[key] = val
	}
	return out
}
