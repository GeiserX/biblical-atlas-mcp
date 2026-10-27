package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.DataURL != DefaultDataURL || c.Refresh != time.Hour || c.Timeout != 30*time.Second ||
		c.Transport != "http" || c.ListenAddr != "127.0.0.1:8080" || c.AuthToken != "" || c.DataFile != "" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if !strings.HasSuffix(c.CacheDir, "biblical-atlas-mcp") {
		t.Fatalf("cache dir = %q", c.CacheDir)
	}
}

func TestValues(t *testing.T) {
	c, err := Load(env(map[string]string{
		"BIBLICAL_ATLAS_DATA_URL":  "http://localhost:9000/data.json",
		"BIBLICAL_ATLAS_DATA_FILE": "/tmp/data.json",
		"BIBLICAL_ATLAS_CACHE_DIR": "off",
		"BIBLICAL_ATLAS_REFRESH":   "0",
		"BIBLICAL_ATLAS_TIMEOUT":   "5m",
		"TRANSPORT":                "STDIO",
		"LISTEN_ADDR":              "0.0.0.0:9090",
		"MCP_AUTH_TOKEN":           "secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.DataURL != "http://localhost:9000/data.json" || c.DataFile != "/tmp/data.json" || c.CacheDir != "" ||
		c.Refresh != 0 || c.Timeout != 5*time.Minute || c.Transport != "stdio" || c.ListenAddr != "0.0.0.0:9090" || c.AuthToken != "secret" {
		t.Fatalf("unexpected config: %+v", c)
	}
	c, err = Load(env(map[string]string{"BIBLICAL_ATLAS_CACHE_DIR": "/var/cache/x", "BIBLICAL_ATLAS_REFRESH": "10m", "TRANSPORT": "http"}))
	if err != nil || c.CacheDir != "/var/cache/x" || c.Refresh != 10*time.Minute || c.Transport != "http" {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestRejections(t *testing.T) {
	cases := []struct{ key, value string }{
		{"BIBLICAL_ATLAS_DATA_URL", "ftp://example.org/data.json"},
		{"BIBLICAL_ATLAS_DATA_URL", "not a url"},
		{"BIBLICAL_ATLAS_DATA_URL", "https://"},
		{"BIBLICAL_ATLAS_REFRESH", "30s"},
		{"BIBLICAL_ATLAS_REFRESH", "soon"},
		{"BIBLICAL_ATLAS_REFRESH", "-1h"},
		{"BIBLICAL_ATLAS_TIMEOUT", "500ms"},
		{"BIBLICAL_ATLAS_TIMEOUT", "6m"},
		{"BIBLICAL_ATLAS_TIMEOUT", "x"},
		{"TRANSPORT", "sse"},
		{"MCP_AUTH_TOKEN", "two words"},
		{"MCP_ALLOWED_ORIGINS", "https://ok.example, not an origin"},
		{"MCP_ALLOWED_ORIGINS", "https://app.example/path"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := Load(env(map[string]string{tc.key: tc.value}))
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("want an error naming %s, got %v", tc.key, err)
			}
		})
	}
}

func TestTokenRequiredOffLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8080", ":8080", "192.168.1.10:8080", "[::]:8080", "example.org:8080"} {
		_, err := Load(env(map[string]string{"LISTEN_ADDR": addr}))
		if err == nil || !strings.Contains(err.Error(), "MCP_AUTH_TOKEN is required") {
			t.Errorf("LISTEN_ADDR %s without a token: got %v", addr, err)
		}
		if _, err := Load(env(map[string]string{"LISTEN_ADDR": addr, "MCP_AUTH_TOKEN": "t0ken"})); err != nil {
			t.Errorf("LISTEN_ADDR %s with a token: %v", addr, err)
		}
		// stdio never listens, so it needs no token.
		if _, err := Load(env(map[string]string{"LISTEN_ADDR": addr, "TRANSPORT": "stdio"})); err != nil {
			t.Errorf("stdio with LISTEN_ADDR %s: %v", addr, err)
		}
	}
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080", "127.0.0.2:9000"} {
		if _, err := Load(env(map[string]string{"LISTEN_ADDR": addr})); err != nil {
			t.Errorf("loopback %s without a token: %v", addr, err)
		}
	}
}

func TestAllowedOrigins(t *testing.T) {
	c, err := Load(env(map[string]string{"MCP_ALLOWED_ORIGINS": " https://a.example , http://localhost:3000,"}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(c.AllowedOrigins, "|") != "https://a.example|http://localhost:3000" {
		t.Fatalf("origins %q", c.AllowedOrigins)
	}
}
