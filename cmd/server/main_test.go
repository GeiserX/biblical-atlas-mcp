package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/geiserx/biblical-atlas-mcp/version"
)

const fixture = "../../internal/atlas/testdata/data.json"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func runArgs(t *testing.T, args []string, m map[string]string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(context.Background(), args, strings.NewReader(""), &out, &errb, env(m))
	return code, out.String(), errb.String()
}

func TestFlags(t *testing.T) {
	if code, out, _ := runArgs(t, []string{"--version"}, nil); code != 0 || strings.TrimSpace(out) != version.String() {
		t.Fatalf("--version: %d %q", code, out)
	}
	if code, out, _ := runArgs(t, []string{"-v"}, nil); code != 0 || !strings.Contains(out, version.Version) {
		t.Fatalf("-v: %d %q", code, out)
	}
	if code, out, _ := runArgs(t, []string{"--help"}, nil); code != 0 || !strings.Contains(out, "BIBLICAL_ATLAS_DATA_URL") || !strings.Contains(out, "MCP_AUTH_TOKEN") {
		t.Fatalf("--help: %d %q", code, out)
	}
	if code, _, errs := runArgs(t, []string{"--frobnicate"}, nil); code != 2 || !strings.Contains(errs, "unknown argument") {
		t.Fatalf("unknown argument: %d %q", code, errs)
	}
	if code, _, _ := runArgs(t, []string{"--version", "extra"}, nil); code != 2 {
		t.Fatalf("two arguments: %d", code)
	}
}

func TestBadConfigExits2(t *testing.T) {
	code, _, errs := runArgs(t, nil, map[string]string{"BIBLICAL_ATLAS_REFRESH": "5s"})
	if code != 2 || !strings.Contains(errs, "BIBLICAL_ATLAS_REFRESH") {
		t.Fatalf("got %d %q", code, errs)
	}
}

func TestStdio(t *testing.T) {
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"query":"Timoteo"}}}`,
	}, "\n") + "\n"
	pr, pw := io.Pipe()
	var out, errb lockedBuffer
	done := make(chan int)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- run(ctx, nil, pr, &out, &errb, env(map[string]string{"TRANSPORT": "stdio", "BIBLICAL_ATLAS_DATA_FILE": fixture}))
	}()
	if _, err := io.WriteString(pw, in); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(out.String(), `"id":2`) {
		time.Sleep(10 * time.Millisecond)
	}
	pw.Close()
	cancel()
	<-done
	got := out.String()
	if !strings.Contains(got, `"instructions"`) || !strings.Contains(got, `timoteo`) {
		t.Fatalf("stdio output: %s\nstderr: %s", got, errb.String())
	}
	if strings.Contains(errb.String(), `"jsonrpc"`) {
		t.Fatal("protocol output leaked to stderr")
	}
}

// lockedBuffer is a buffer the server goroutine writes while the test reads.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestHTTPServesHealthz(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int)
	var errb bytes.Buffer
	go func() {
		done <- run(ctx, nil, strings.NewReader(""), io.Discard, &errb, env(map[string]string{
			"LISTEN_ADDR": addr, "BIBLICAL_ATLAS_DATA_FILE": fixture, "MCP_AUTH_TOKEN": "s3cret",
		}))
	}()
	var resp *http.Response
	var err error
	for i := 0; i < 100; i++ {
		resp, err = http.Get(fmt.Sprintf("http://%s/healthz", addr))
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "ok" {
		t.Fatalf("healthz: %d %q", resp.StatusCode, body)
	}
	resp, err = http.Post(fmt.Sprintf("http://%s/mcp", addr), "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("mcp without token: %d", resp.StatusCode)
	}
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("exit code %d, stderr %s", code, errb.String())
	}
}

func TestBearerAuth(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: fixture})
	h := httpHandler(newServer(store), "s3cret", nil)
	initMsg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`
	for _, c := range []struct {
		name, auth string
		want       int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "Bearer nope", http.StatusUnauthorized},
		{"no scheme", "s3cret", http.StatusUnauthorized},
		{"other scheme", "Basic s3cret", http.StatusUnauthorized},
		{"empty token", "Bearer ", http.StatusUnauthorized},
		{"token with a tail", "Bearer s3cretX", http.StatusUnauthorized},
		{"right", "Bearer s3cret", http.StatusOK},
		{"scheme in lower case", "bearer s3cret", http.StatusOK},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(initMsg))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("got %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
			if c.want == http.StatusOK && !strings.Contains(rec.Body.String(), "biblical-atlas-mcp") {
				t.Fatalf("initialize answer: %s", rec.Body.String())
			}
		})
	}
	// Health needs no token.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://localhost/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz with auth on: %d", rec.Code)
	}
}

func TestHTTPWithoutToken(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: fixture})
	h := httpHandler(newServer(store), "", nil)
	req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lookup_passage") {
		t.Fatalf("tools/list: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPLimits(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: fixture})
	h := httpHandler(newServer(store), "", nil)
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	// A body over the cap is refused before it reaches the MCP handler.
	if rec := post(`{"x":"` + strings.Repeat("a", maxBody) + `"}`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: %d %s", rec.Code, rec.Body.String())
	}
	// A body just under it still works.
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/list","pad":"` + strings.Repeat(" ", maxBody-200) + `"}`
	if rec := post(msg); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lookup_passage") {
		t.Fatalf("body under the cap: %d", rec.Code)
	}
	// The server is stateless and sends no notifications, so GET opens no stream.
	// The context ends a stream that should not exist, so a regression fails
	// instead of hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/mcp", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /mcp: %d", rec.Code)
	}
}

// TestNonLoopbackNeedsToken: the server refuses to listen beyond this
// machine with no token, and says why.
func TestNonLoopbackNeedsToken(t *testing.T) {
	// A server that wrongly starts stops after the timeout, so the test
	// fails fast instead of hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out, errb bytes.Buffer
	code := run(ctx, nil, strings.NewReader(""), &out, &errb, env(map[string]string{"LISTEN_ADDR": "0.0.0.0:0", "BIBLICAL_ATLAS_DATA_FILE": fixture}))
	if errs := errb.String(); code != 2 || !strings.Contains(errs, "MCP_AUTH_TOKEN is required") {
		t.Fatalf("got %d %q", code, errs)
	}
}

// TestOrigin: a browser page on another origin cannot call /mcp, a
// non-browser client and a listed origin can.
func TestOrigin(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: fixture})
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	post := func(h http.Handler, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/mcp", strings.NewReader(msg))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	h := httpHandler(newServer(store), "", nil)
	for _, c := range []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"no browser headers", nil, http.StatusOK},
		{"cross-site fetch", map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"old browser, foreign origin", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"same origin", map[string]string{"Origin": "http://localhost:8080", "Sec-Fetch-Site": "same-origin"}, http.StatusOK},
	} {
		t.Run(c.name, func(t *testing.T) {
			if rec := post(h, c.headers); rec.Code != c.want {
				t.Fatalf("got %d, want %d: %s", rec.Code, c.want, rec.Body.String())
			}
		})
	}
	allowed := httpHandler(newServer(store), "", []string{"https://app.example"})
	if rec := post(allowed, map[string]string{"Origin": "https://app.example", "Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusOK {
		t.Fatalf("listed origin: %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(allowed, map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusForbidden {
		t.Fatalf("unlisted origin with a list set: %d", rec.Code)
	}

	// A browser needs CORS headers as well: a preflight answer (which
	// carries no token) and Access-Control-Allow-Origin on the reply.
	withToken := httpHandler(newServer(store), "s3cret", []string{"https://app.example"})
	preflight := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, "http://localhost:8080/mcp", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type, mcp-method")
		rec := httptest.NewRecorder()
		withToken.ServeHTTP(rec, req)
		return rec
	}
	rec := preflight("https://app.example")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example" ||
		!strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") || !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Mcp-Method") {
		t.Fatalf("listed preflight: %d %v", rec.Code, rec.Header())
	}
	if rec := preflight("https://evil.example"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unlisted preflight got CORS headers: %v", rec.Header())
	}
	rec = post(withToken, map[string]string{"Origin": "https://app.example", "Sec-Fetch-Site": "cross-site", "Authorization": "Bearer s3cret"})
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("listed origin reply: %d %v", rec.Code, rec.Header())
	}
	if rec := post(withToken, map[string]string{"Origin": "https://app.example", "Sec-Fetch-Site": "cross-site"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("listed origin without the token: %d", rec.Code)
	}
}

// TestInFlightLimit: with every slot busy, the next request gets 503 at
// once instead of queuing.
func TestInFlightLimit(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	h := limitInFlight(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
	}), 1)
	first := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/mcp", nil))
		close(first)
	}()
	<-started
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("full house: %d", rec.Code)
	}
	close(release)
	<-first
	// The slot is free again.
	done := make(chan struct{})
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
		close(done)
	}()
	<-started
	<-done
}
