package questdb

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.AutoFlushRows != 1000 {
		t.Errorf("AutoFlushRows: got %d, want 1000", c.AutoFlushRows)
	}
	if c.AutoFlushInterval != 500*time.Millisecond {
		t.Errorf("AutoFlushInterval: got %s, want 500ms", c.AutoFlushInterval)
	}
	if c.HTTPTimeout != 60*time.Second {
		t.Errorf("HTTPTimeout: got %s, want 60s", c.HTTPTimeout)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"missing address", Config{}, true},
		{"happy path", Config{Address: "localhost:9000"}, false},
		{"negative flush rows", Config{Address: "x:1", AutoFlushRows: -1}, true},
		{"negative flush interval", Config{Address: "x:1", AutoFlushInterval: -time.Second}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("QUESTDB_ADDRESS", "10.10.8.51:9000")
	t.Setenv("QUESTDB_TOKEN", "abc")
	t.Setenv("QUESTDB_AUTO_FLUSH_ROWS", "5000")
	t.Setenv("QUESTDB_AUTO_FLUSH_INT", "1s")
	t.Setenv("QUESTDB_TLS", "true")

	cfg, err := LoadFromEnv("")
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if cfg.Address != "10.10.8.51:9000" {
		t.Errorf("Address: got %q", cfg.Address)
	}
	if cfg.Token != "abc" {
		t.Errorf("Token: got %q", cfg.Token)
	}
	if cfg.AutoFlushRows != 5000 {
		t.Errorf("AutoFlushRows: got %d", cfg.AutoFlushRows)
	}
	if cfg.AutoFlushInterval != time.Second {
		t.Errorf("AutoFlushInterval: got %s", cfg.AutoFlushInterval)
	}
	if !cfg.TLS {
		t.Errorf("TLS: got %v, want true", cfg.TLS)
	}
}

func TestLoadFromEnvMissingAddress(t *testing.T) {
	t.Setenv("QUESTDB_ADDRESS", "")
	if _, err := LoadFromEnv(""); err == nil {
		t.Fatal("expected error for missing QUESTDB_ADDRESS")
	}
}

func TestLoadFromEnvWithPrefix(t *testing.T) {
	t.Setenv("STOCK_QUESTDB_ADDRESS", "stock-host:9000")
	cfg, err := LoadFromEnv("STOCK_")
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if cfg.Address != "stock-host:9000" {
		t.Errorf("Address: got %q", cfg.Address)
	}
}

func TestHTTPBaseURL(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"explicit", Config{HTTPBaseURL: "http://questdb:9000/"}, "http://questdb:9000"},
		{"derived http", Config{Address: "10.10.8.51:9000"}, "http://10.10.8.51:9000"},
		{"derived https", Config{Address: "10.10.8.51:9000", TLS: true}, "https://10.10.8.51:9000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.httpBaseURL(); got != tc.want {
				t.Errorf("httpBaseURL: got %q, want %q", got, tc.want)
			}
		})
	}
}
