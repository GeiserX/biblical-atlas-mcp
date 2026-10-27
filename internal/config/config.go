// Package config reads and validates the environment.
package config

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// DefaultDataURL is the atlas's public data file.
const DefaultDataURL = "https://biblical-atlas.geiser.cloud/data.json"

// Config is the validated environment.
type Config struct {
	DataURL    string
	DataFile   string
	CacheDir   string // "" when the disk cache is off
	Refresh    time.Duration
	Timeout    time.Duration
	Transport  string // "stdio" or "http"
	ListenAddr string
	AuthToken  string
	// AllowedOrigins are browser origins (scheme://host[:port]) that may call
	// /mcp cross-origin. Empty means none: a cross-origin browser request
	// gets 403.
	AllowedOrigins []string
}

// Help is the variable table printed by --help.
const Help = `Environment variables:
  BIBLICAL_ATLAS_DATA_URL   Data file URL (default ` + DefaultDataURL + `)
  BIBLICAL_ATLAS_DATA_FILE  Local copy of the data file; no network when set
  BIBLICAL_ATLAS_CACHE_DIR  Disk cache directory (default <user cache dir>/biblical-atlas-mcp; "off" disables it)
  BIBLICAL_ATLAS_REFRESH    How long a copy is served before asking again (default 1h, at least 1m, 0 = never)
  BIBLICAL_ATLAS_TIMEOUT    Limit for one download (default 30s, 1s to 5m)
  TRANSPORT                 stdio, or http (default)
  LISTEN_ADDR               HTTP listen address (default 127.0.0.1:8080)
  MCP_AUTH_TOKEN            HTTP requests need "Authorization: Bearer <token>"; required when LISTEN_ADDR is not loopback
  MCP_ALLOWED_ORIGINS       Comma-separated browser origins allowed to call /mcp (default none)
`

// Load reads the configuration through getenv. Any value that does not
// parse is an error naming the variable.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		DataURL:    DefaultDataURL,
		Refresh:    time.Hour,
		Timeout:    30 * time.Second,
		Transport:  "http",
		ListenAddr: "127.0.0.1:8080",
	}
	if v := strings.TrimSpace(getenv("BIBLICAL_ATLAS_DATA_URL")); v != "" {
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return c, fmt.Errorf("BIBLICAL_ATLAS_DATA_URL must be an http or https URL, got %q", v)
		}
		c.DataURL = v
	}
	c.DataFile = strings.TrimSpace(getenv("BIBLICAL_ATLAS_DATA_FILE"))

	switch v := strings.TrimSpace(getenv("BIBLICAL_ATLAS_CACHE_DIR")); v {
	case "off":
		c.CacheDir = ""
	case "":
		if dir, err := os.UserCacheDir(); err == nil {
			c.CacheDir = filepath.Join(dir, "biblical-atlas-mcp")
		}
	default:
		c.CacheDir = v
	}

	if v := strings.TrimSpace(getenv("BIBLICAL_ATLAS_REFRESH")); v != "" {
		if v == "0" {
			c.Refresh = 0
		} else {
			d, err := time.ParseDuration(v)
			if err != nil || d < time.Minute {
				return c, fmt.Errorf("BIBLICAL_ATLAS_REFRESH must be a Go duration of at least 1m, or 0 to never refresh, got %q", v)
			}
			c.Refresh = d
		}
	}
	if v := strings.TrimSpace(getenv("BIBLICAL_ATLAS_TIMEOUT")); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second || d > 5*time.Minute {
			return c, fmt.Errorf("BIBLICAL_ATLAS_TIMEOUT must be a Go duration from 1s to 5m, got %q", v)
		}
		c.Timeout = d
	}
	switch v := strings.ToLower(strings.TrimSpace(getenv("TRANSPORT"))); v {
	case "", "http":
		c.Transport = "http"
	case "stdio":
		c.Transport = "stdio"
	default:
		return c, fmt.Errorf("TRANSPORT must be stdio or http, got %q", v)
	}
	if v := strings.TrimSpace(getenv("LISTEN_ADDR")); v != "" {
		c.ListenAddr = v
	}
	c.AuthToken = getenv("MCP_AUTH_TOKEN")
	if c.AuthToken != "" && strings.IndexFunc(c.AuthToken, unicode.IsSpace) >= 0 {
		return c, fmt.Errorf("MCP_AUTH_TOKEN must not contain white space")
	}
	if v := strings.TrimSpace(getenv("MCP_ALLOWED_ORIGINS")); v != "" {
		check := http.NewCrossOriginProtection()
		for _, o := range strings.Split(v, ",") {
			o = strings.TrimSpace(o)
			if o == "" {
				continue
			}
			if err := check.AddTrustedOrigin(o); err != nil {
				return c, fmt.Errorf("MCP_ALLOWED_ORIGINS must list origins such as https://app.example.org, got %q", o)
			}
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}
	if c.Transport == "http" && c.AuthToken == "" && !Loopback(c.ListenAddr) {
		return c, fmt.Errorf("MCP_AUTH_TOKEN is required when LISTEN_ADDR is not a loopback address, got LISTEN_ADDR %q: anyone who can reach the port could call the tools", c.ListenAddr)
	}
	return c, nil
}

// Loopback reports whether a listen address only accepts connections from
// this machine: localhost or a loopback IP. An empty host listens on every
// interface, so it is not loopback.
func Loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
