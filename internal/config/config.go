// Package config loads and validates the service configuration.
//
// Configuration comes from environment variables only. There is no config file
// lookup, no defaults hidden in other packages and no fallback between
// environments: what the process sees in its environment is what it runs with.
// The committed files under config/ are plain KEY=VALUE env files that Docker
// Compose, the Makefile and the deployment tooling load into the environment.
//
// Every problem is reported at once (see Load), so a misconfigured deployment
// fails on its first start with the full list of what is wrong.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Environment names one isolated deployment of the platform.
type Environment string

const (
	EnvLocal      Environment = "local"
	EnvTest       Environment = "test"
	EnvStage      Environment = "stage"
	EnvProduction Environment = "production"
)

var environments = []Environment{EnvLocal, EnvTest, EnvStage, EnvProduction}

// IsDeployed reports whether the environment runs on shared cloud
// infrastructure. Deployed environments get the strict rules: TLS to the
// database, JSON logs and no debug logging.
func (e Environment) IsDeployed() bool {
	return e == EnvStage || e == EnvProduction
}

// Config is the complete runtime configuration of the service.
type Config struct {
	Env         Environment
	ServiceName string
	HTTP        HTTP
	Log         Log
	Database    Database
}

type HTTP struct {
	// Addr is the listen address, for example ":8080".
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	// ShutdownTimeout bounds how long in-flight requests may take to finish
	// after the process is asked to stop.
	ShutdownTimeout time.Duration
}

type Log struct {
	Level  slog.Level
	Format string // "json" or "text"
}

type Database struct {
	// URL is a PostgreSQL connection URL. It contains a password: never log it
	// directly, use RedactedURL.
	URL      string
	MaxConns int32
	// ConnectTimeout bounds the initial connection and each readiness ping.
	ConnectTimeout time.Duration
}

// Name returns the database name from the connection URL.
func (d Database) Name() string {
	u, err := url.Parse(d.URL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}

// RedactedURL returns the connection URL with the password removed.
func (d Database) RedactedURL() string {
	u, err := url.Parse(d.URL)
	if err != nil {
		return "<unparseable>"
	}
	return u.Redacted()
}

// LogValue keeps secrets out of logs when the whole config is logged.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("env", string(c.Env)),
		slog.String("service_name", c.ServiceName),
		slog.String("http_addr", c.HTTP.Addr),
		slog.String("log_level", c.Log.Level.String()),
		slog.String("log_format", c.Log.Format),
		slog.String("database_url", c.Database.RedactedURL()),
		slog.Int("database_max_conns", int(c.Database.MaxConns)),
	)
}

// Lookup reads one environment variable. os.LookupEnv satisfies it; tests
// pass a map instead so they never depend on the machine they run on.
type Lookup func(key string) (string, bool)

// FromMap adapts a map to Lookup.
func FromMap(m map[string]string) Lookup {
	return func(key string) (string, bool) {
		v, ok := m[key]
		return v, ok
	}
}

// Load reads and validates the configuration. The returned error lists every
// problem found, one per line.
func Load(lookup Lookup) (Config, error) {
	r := reader{lookup: lookup}

	cfg := Config{
		Env:         Environment(r.required("APP_ENV")),
		ServiceName: r.optional("APP_SERVICE_NAME", "platform-backend"),
		HTTP: HTTP{
			Addr:              r.optional("HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: r.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       r.duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      r.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       r.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   r.duration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
		},
		Log: Log{
			Level:  r.logLevel("LOG_LEVEL", slog.LevelInfo),
			Format: r.optional("LOG_FORMAT", "json"),
		},
		Database: Database{
			URL:            r.required("DATABASE_URL"),
			MaxConns:       r.int32("DATABASE_MAX_CONNS", 10),
			ConnectTimeout: r.duration("DATABASE_CONNECT_TIMEOUT", 5*time.Second),
		},
	}

	r.errs = append(r.errs, validate(cfg)...)

	if len(r.errs) > 0 {
		problems := make([]string, len(r.errs))
		for i, err := range r.errs {
			problems[i] = err.Error()
		}
		return Config{}, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

func validate(cfg Config) []error {
	var errs []error
	add := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	if cfg.Env != "" && !isKnownEnv(cfg.Env) {
		add("APP_ENV %q is not one of %v", cfg.Env, environments)
	}
	if cfg.Log.Format != "json" && cfg.Log.Format != "text" {
		add("LOG_FORMAT %q must be \"json\" or \"text\"", cfg.Log.Format)
	}
	if cfg.Database.MaxConns < 1 {
		add("DATABASE_MAX_CONNS must be at least 1")
	}

	if cfg.Database.URL != "" {
		errs = append(errs, validateDatabaseURL(cfg.Env, cfg.Database.URL)...)
	}

	if cfg.Env.IsDeployed() {
		if cfg.Log.Format != "json" {
			add("LOG_FORMAT must be \"json\" in %s", cfg.Env)
		}
		if cfg.Log.Level < slog.LevelInfo {
			add("LOG_LEVEL must not be debug in %s", cfg.Env)
		}
	}
	return errs
}

// validateDatabaseURL enforces the environment-isolation rule: the database
// name must end with "_<APP_ENV>". A stage process handed the production URL
// (or the reverse) refuses to start instead of silently using the wrong data.
func validateDatabaseURL(env Environment, raw string) []error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return []error{errors.New("DATABASE_URL must be a postgres:// URL with a host")}
	}

	var errs []error
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		errs = append(errs, errors.New("DATABASE_URL must name a database"))
	} else if isKnownEnv(env) && !strings.HasSuffix(name, "_"+string(env)) {
		errs = append(errs, fmt.Errorf(
			"database name %q must end with %q: each environment owns its own database (APP_ENV=%s)",
			name, "_"+string(env), env))
	}

	if env.IsDeployed() {
		switch u.Query().Get("sslmode") {
		case "require", "verify-ca", "verify-full":
		default:
			errs = append(errs, fmt.Errorf("DATABASE_URL must set sslmode=verify-full (or require/verify-ca) in %s", env))
		}
	}
	return errs
}

func isKnownEnv(e Environment) bool {
	for _, known := range environments {
		if e == known {
			return true
		}
	}
	return false
}

// reader collects parse errors instead of stopping at the first one.
type reader struct {
	lookup Lookup
	errs   []error
}

func (r *reader) get(key string) (string, bool) {
	v, ok := r.lookup(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (r *reader) required(key string) string {
	v, ok := r.get(key)
	if !ok {
		r.errs = append(r.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func (r *reader) optional(key, fallback string) string {
	if v, ok := r.get(key); ok {
		return v
	}
	return fallback
}

func (r *reader) duration(key string, fallback time.Duration) time.Duration {
	v, ok := r.get(key)
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		r.errs = append(r.errs, fmt.Errorf("%s=%q must be a positive duration such as 10s", key, v))
		return fallback
	}
	return d
}

func (r *reader) int32(key string, fallback int32) int32 {
	v, ok := r.get(key)
	if !ok {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s=%q must be an integer", key, v))
		return fallback
	}
	return int32(n)
}

func (r *reader) logLevel(key string, fallback slog.Level) slog.Level {
	v, ok := r.get(key)
	if !ok {
		return fallback
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(v)); err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s=%q must be debug, info, warn or error", key, v))
		return fallback
	}
	return level
}
