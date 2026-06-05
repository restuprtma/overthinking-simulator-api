// Package questdb is a thin, opinionated wrapper around
// github.com/questdb/go-questdb-client/v3 that centralizes how Tuai
// services talk to QuestDB.
//
// Two surfaces:
//
//  1. Sender — thread-safe ILP writer (HTTP transport). The underlying
//     qdb.LineSender is NOT concurrency-safe; this wrapper guards it
//     with a mutex so multiple goroutines can call Row(fn) safely.
//
//  2. QueryClient — HTTP /exec wrapper for ad-hoc reads. The official
//     client only ships ILP write support; reads go through /exec by
//     hand.
//
// Both share Config which loads from the QUESTDB_* env vars already
// used across the consumer binaries.
package questdb

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds the connection parameters for both the ILP Sender and
// the HTTP QueryClient. A single shared shape keeps the env-loading
// dance identical across services.
//
// Address (host:port) is used by Sender for ILP HTTP. HTTPBaseURL
// (full URL, e.g. http://10.10.8.51:9000) is used by QueryClient.
// If HTTPBaseURL is empty it is derived from Address with the
// http:// scheme — almost always what you want.
type Config struct {
	Address           string        // host:port for ILP HTTP transport (e.g. 10.10.8.51:9000)
	HTTPBaseURL       string        // full base URL for /exec queries (defaults from Address)
	Token             string        // bearer token; takes precedence over basic auth
	BasicAuthUser     string        // basic-auth user (used if Token empty)
	BasicAuthPassword string        // basic-auth password
	TLS               bool          // enable TLS on ILP HTTP transport
	AutoFlushRows     int           // ILP auto-flush row threshold; default 1000
	AutoFlushInterval time.Duration // ILP auto-flush interval; default 500ms
	HTTPTimeout       time.Duration // QueryClient per-request timeout; default 60s
}

// DefaultConfig returns the project-wide defaults. Address must be
// filled by the caller — there is no safe default for "where is your
// QuestDB."
func DefaultConfig() Config {
	return Config{
		AutoFlushRows:     1000,
		AutoFlushInterval: 500 * time.Millisecond,
		HTTPTimeout:       60 * time.Second,
	}
}

// LoadFromEnv populates a Config from the QUESTDB_* env vars used
// across the cmd/* consumer binaries:
//
//	QUESTDB_ADDRESS          host:port (required)
//	QUESTDB_HTTP_URL         full http(s):// base URL for /exec (optional)
//	QUESTDB_TOKEN            bearer token
//	QUESTDB_AUTH_USER        basic-auth user (alternative to token)
//	QUESTDB_AUTH_PASSWORD
//	QUESTDB_TLS              true|false (default false)
//	QUESTDB_AUTO_FLUSH_ROWS  default 1000
//	QUESTDB_AUTO_FLUSH_INT   default 500ms
//	QUESTDB_HTTP_TIMEOUT     default 60s
//
// The optional `prefix` is prepended to each name (e.g. "STOCK_" →
// STOCK_QUESTDB_ADDRESS) for services that need to point at multiple
// QuestDB instances. Pass "" for the default.
func LoadFromEnv(prefix string) (Config, error) {
	cfg := DefaultConfig()

	addr := os.Getenv(prefix + "QUESTDB_ADDRESS")
	if addr == "" {
		return cfg, errors.New("missing required env var " + prefix + "QUESTDB_ADDRESS")
	}
	cfg.Address = addr

	cfg.HTTPBaseURL = os.Getenv(prefix + "QUESTDB_HTTP_URL")
	cfg.Token = os.Getenv(prefix + "QUESTDB_TOKEN")
	cfg.BasicAuthUser = os.Getenv(prefix + "QUESTDB_AUTH_USER")
	cfg.BasicAuthPassword = os.Getenv(prefix + "QUESTDB_AUTH_PASSWORD")

	if v := os.Getenv(prefix + "QUESTDB_TLS"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid %sQUESTDB_TLS: %w", prefix, err)
		}
		cfg.TLS = b
	}
	if v := os.Getenv(prefix + "QUESTDB_AUTO_FLUSH_ROWS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid %sQUESTDB_AUTO_FLUSH_ROWS: %w", prefix, err)
		}
		cfg.AutoFlushRows = n
	}
	if v := os.Getenv(prefix + "QUESTDB_AUTO_FLUSH_INT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid %sQUESTDB_AUTO_FLUSH_INT: %w", prefix, err)
		}
		cfg.AutoFlushInterval = d
	}
	if v := os.Getenv(prefix + "QUESTDB_HTTP_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return cfg, fmt.Errorf("invalid %sQUESTDB_HTTP_TIMEOUT: %w", prefix, err)
		}
		cfg.HTTPTimeout = d
	}
	return cfg, nil
}

// Validate sanity-checks the config before opening a connection.
// Returned errors are wrapped with the env var name they came from
// so misconfiguration is easy to diagnose in production logs.
func (c Config) Validate() error {
	if c.Address == "" {
		return errors.New("questdb: Address is required")
	}
	if c.AutoFlushRows < 0 {
		return errors.New("questdb: AutoFlushRows must be >= 0")
	}
	if c.AutoFlushInterval < 0 {
		return errors.New("questdb: AutoFlushInterval must be >= 0")
	}
	if c.HTTPTimeout < 0 {
		return errors.New("questdb: HTTPTimeout must be >= 0")
	}
	return nil
}

// httpBaseURL returns the effective /exec base URL — explicit
// HTTPBaseURL wins, otherwise we synthesize http(s)://Address.
func (c Config) httpBaseURL() string {
	if c.HTTPBaseURL != "" {
		return strings.TrimRight(c.HTTPBaseURL, "/")
	}
	scheme := "http"
	if c.TLS {
		scheme = "https"
	}
	return scheme + "://" + c.Address
}
