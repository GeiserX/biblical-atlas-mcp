// Command biblical-atlas-mcp is an MCP server for the biblical-atlas atlas.
package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/geiserx/biblical-atlas-mcp/internal/config"
	"github.com/geiserx/biblical-atlas-mcp/internal/tools"
	"github.com/geiserx/biblical-atlas-mcp/version"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv)
	stop()
	os.Exit(code)
}

// run holds the whole program so tests can call it directly.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) > 0 {
		switch {
		case len(args) == 1 && (args[0] == "--version" || args[0] == "-v"):
			fmt.Fprintln(stdout, version.String())
			return 0
		case len(args) == 1 && (args[0] == "--help" || args[0] == "-h"):
			fmt.Fprintf(stdout, "biblical-atlas-mcp %s\nAn MCP server for the biblical-atlas Bible atlas.\n\n%s", version.String(), config.Help)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown argument %q; configuration is by environment variables, see --help\n", strings.Join(args, " "))
			return 2
		}
	}
	logger := log.New(stderr, "biblical-atlas-mcp: ", log.LstdFlags)
	cfg, err := config.Load(getenv)
	if err != nil {
		logger.Print(err)
		return 2
	}
	store := atlas.NewStore(atlas.Options{
		URL:       cfg.DataURL,
		File:      cfg.DataFile,
		CacheDir:  cfg.CacheDir,
		TTL:       cfg.Refresh,
		Timeout:   cfg.Timeout,
		UserAgent: "biblical-atlas-mcp/" + version.Version,
		Logf:      logger.Printf,
	})
	store.Start()
	s := newServer(store)

	if cfg.Transport == "stdio" {
		logger.Printf("%s on stdio, data from %s", version.String(), store.LogSource())
		stdio := server.NewStdioServer(s)
		stdio.SetErrorLogger(logger)
		if err := stdio.Listen(ctx, stdin, stdout); err != nil && !errors.Is(err, context.Canceled) {
			logger.Printf("stdio server: %v", err)
			return 1
		}
		return 0
	}

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Printf("listen on %s: %v", cfg.ListenAddr, err)
		return 1
	}
	srv := &http.Server{
		Handler:           httpHandler(s, cfg.AuthToken, cfg.AllowedOrigins),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// A tool call can wait for the first download, so the write limit
		// sits above the download timeout.
		WriteTimeout: cfg.Timeout + 30*time.Second,
		IdleTimeout:  2 * time.Minute,
	}
	logger.Printf("%s listening on http://%s/mcp, data from %s", version.String(), ln.Addr(), store.LogSource())
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		logger.Printf("http server: %v", err)
		return 1
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Printf("http shutdown: %v", err)
	}
	return 0
}

func newServer(store *atlas.Store) *server.MCPServer {
	s := server.NewMCPServer("biblical-atlas-mcp", version.Version, tools.ServerOptions()...)
	tools.Register(s, store)
	return s
}

// maxBody caps one request to /mcp. A tool call is a few hundred bytes.
const maxBody = 1 << 20

// maxInFlight caps the /mcp requests served at once. A search or a long
// find_connection takes milliseconds, so a full house means a flood, and the
// rest get 503 instead of queuing.
const maxInFlight = 32

// httpHandler serves /mcp (stateless streamable HTTP with no GET event
// stream) and /healthz (no auth, no data access). /mcp refuses cross-origin
// browser requests from origins not listed, checks the bearer token when one
// is set, caps concurrent requests and caps the body.
func httpHandler(s *server.MCPServer, token string, origins []string) http.Handler {
	var mcpHandler http.Handler = server.NewStreamableHTTPServer(s,
		server.WithStateLess(true), server.WithDisableStreaming(true))
	mcpHandler = limitBody(mcpHandler, maxBody)
	mcpHandler = limitInFlight(mcpHandler, maxInFlight)
	if token != "" {
		mcpHandler = bearerAuth(mcpHandler, token)
	}
	// Origins are checked by config.Load, so an error here cannot happen.
	cop := http.NewCrossOriginProtection()
	for _, o := range origins {
		_ = cop.AddTrustedOrigin(o)
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "cross-origin request refused; list the origin in MCP_ALLOWED_ORIGINS to allow it", http.StatusForbidden)
	}))
	mcpHandler = cop.Handler(mcpHandler)
	// Outermost: a CORS preflight carries no token, so it is answered
	// before bearerAuth sees it.
	mcpHandler = allowOrigins(mcpHandler, origins)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok")
	})
	return mux
}

// limitBody answers 413 to a body over limit bytes and hands the rest on.
// corsHeaders are the request headers a browser page may send to /mcp.
const corsHeaders = "Authorization, Content-Type, Accept, MCP-Protocol-Version, Mcp-Method, Mcp-Name"

// allowOrigins gives the origins in MCP_ALLOWED_ORIGINS the CORS answers a
// browser needs to call /mcp: a preflight reply, and Access-Control-Allow-Origin
// on every response. Other origins get no CORS headers, so the browser keeps
// blocking them even where CrossOriginProtection lets a request through.
func allowOrigins(next http.Handler, origins []string) http.Handler {
	if len(origins) == 0 {
		return next
	}
	listed := make(map[string]bool, len(origins))
	for _, o := range origins {
		listed[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !listed[origin] {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Add("Vary", "Origin")
		h.Set("Access-Control-Allow-Origin", origin)
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "POST")
			h.Set("Access-Control-Allow-Headers", corsHeaders)
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func limitBody(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Body != http.NoBody {
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
			if err != nil {
				var tooBig *http.MaxBytesError
				if errors.As(err, &tooBig) {
					http.Error(w, fmt.Sprintf("request body is larger than %d bytes", limit), http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "cannot read the request body", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		next.ServeHTTP(w, r)
	})
}

// limitInFlight answers 503 when n requests are already being served.
func limitInFlight(next http.Handler, n int) http.Handler {
	slots := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many requests in flight; retry shortly", http.StatusServiceUnavailable)
		}
	})
}

// bearerAuth wants "Authorization: Bearer <token>". The scheme is matched
// without regard to case, as RFC 9110 says; the token is compared in
// constant time.
func bearerAuth(next http.Handler, token string) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scheme, got, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
