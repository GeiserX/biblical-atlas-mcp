package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
)

// modernMeta is the per-request _meta a 2026-07-28 client sends.
const modernMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":"t","version":"1"}}`

// rpcAnswer is one JSON-RPC response.
type rpcAnswer struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// stdioExchange runs the real program on stdio, sends the lines and returns
// the answers by id once every request with an id has one.
func stdioExchange(t *testing.T, lines ...string) map[int]rpcAnswer {
	t.Helper()
	want := 0
	for _, l := range lines {
		if strings.Contains(l, `"id":`) {
			want++
		}
	}
	pr, pw := io.Pipe()
	var out, errb lockedBuffer
	done := make(chan int)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		done <- run(ctx, nil, pr, &out, &errb, env(map[string]string{"TRANSPORT": "stdio", "BIBLICAL_ATLAS_DATA_FILE": fixture}))
	}()
	if _, err := io.WriteString(pw, strings.Join(lines, "\n")+"\n"); err != nil {
		t.Fatal(err)
	}
	answers := map[int]rpcAnswer{}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		answers = map[int]rpcAnswer{}
		for _, l := range strings.Split(out.String(), "\n") {
			var a rpcAnswer
			if json.Unmarshal([]byte(l), &a) == nil && a.ID != 0 {
				answers[a.ID] = a
			}
		}
		if len(answers) >= want {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	pw.Close()
	cancel()
	<-done
	if len(answers) < want {
		t.Fatalf("got %d of %d answers: %s\nstderr: %s", len(answers), want, out.String(), errb.String())
	}
	return answers
}

func decode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
}

// TestProtocolModernStdio drives the 2026-07-28 stateless protocol over
// stdio: server/discover, a cacheable tools/list, a tools/call with
// structured content, and ping, which that revision removed.
func TestProtocolModernStdio(t *testing.T) {
	a := stdioExchange(t,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+modernMeta+`}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{`+modernMeta+`}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"query":"Timoteo"},`+modernMeta+`}}`,
		`{"jsonrpc":"2.0","id":4,"method":"ping","params":{`+modernMeta+`}}`,
	)

	var disc struct {
		SupportedVersions []string `json:"supportedVersions"`
		Instructions      string   `json:"instructions"`
		TTL               *int64   `json:"ttlMs"`
		CacheScope        string   `json:"cacheScope"`
		Capabilities      struct {
			Tools *struct{} `json:"tools"`
		} `json:"capabilities"`
		Meta map[string]struct {
			Name       string `json:"name"`
			Title      string `json:"title"`
			Version    string `json:"version"`
			WebsiteURL string `json:"websiteUrl"`
			Icons      []struct {
				Src string `json:"src"`
			} `json:"icons"`
		} `json:"_meta"`
	}
	if a[1].Error != nil {
		t.Fatalf("server/discover: %+v", a[1].Error)
	}
	decode(t, a[1].Result, &disc)
	for _, v := range []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"} {
		if !slices.Contains(disc.SupportedVersions, v) {
			t.Errorf("supportedVersions %v lacks %s", disc.SupportedVersions, v)
		}
	}
	info := disc.Meta["io.modelcontextprotocol/serverInfo"]
	if info.Name != "biblical-atlas-mcp" || info.Title != "Biblical Atlas" || info.WebsiteURL == "" || len(info.Icons) != 2 {
		t.Errorf("serverInfo %+v in %s", info, a[1].Result)
	}
	if disc.Capabilities.Tools == nil || !strings.Contains(disc.Instructions, "Spanish Bible atlas") {
		t.Errorf("discover result %s", a[1].Result)
	}
	if disc.TTL == nil || *disc.TTL != 3_600_000 || disc.CacheScope != "public" {
		t.Errorf("discover cache hints: %s", a[1].Result)
	}

	var list struct {
		ResultType string `json:"resultType"`
		TTL        *int64 `json:"ttlMs"`
		CacheScope string `json:"cacheScope"`
		Tools      []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	decode(t, a[2].Result, &list)
	if list.ResultType != "complete" || list.TTL == nil || *list.TTL != 3_600_000 || list.CacheScope != "public" || len(list.Tools) != 10 {
		t.Errorf("modern tools/list: %s", a[2].Result)
	}

	var res struct {
		ResultType        string          `json:"resultType"`
		IsError           bool            `json:"isError"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	decode(t, a[3].Result, &res)
	if res.ResultType != "complete" || res.IsError || !strings.Contains(string(res.StructuredContent), `"id":"timoteo"`) {
		t.Errorf("modern tools/call: %s", a[3].Result)
	}

	if a[4].Error == nil || a[4].Error.Code != -32601 {
		t.Errorf("modern ping: want -32601, got %+v %s", a[4].Error, a[4].Result)
	}
}

// TestProtocolLegacyStdio is what Claude Code, Claude Desktop, Cursor and
// VS Code send today: initialize, then plain requests with no _meta.
func TestProtocolLegacyStdio(t *testing.T) {
	a := stdioExchange(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	)
	var init struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name  string `json:"name"`
			Title string `json:"title"`
		} `json:"serverInfo"`
	}
	decode(t, a[1].Result, &init)
	if init.ProtocolVersion != "2025-11-25" || init.ServerInfo.Name != "biblical-atlas-mcp" || init.ServerInfo.Title != "Biblical Atlas" {
		t.Errorf("initialize: %s", a[1].Result)
	}
	var list struct {
		ResultType *string `json:"resultType"`
		Tools      []struct {
			Name        string         `json:"name"`
			Title       string         `json:"title"`
			Annotations map[string]any `json:"annotations"`
		} `json:"tools"`
	}
	decode(t, a[2].Result, &list)
	if len(list.Tools) != 10 || list.Tools[0].Name != "dataset_info" || list.Tools[0].Title == "" {
		t.Errorf("legacy tools/list: %s", a[2].Result)
	}
	if a[3].Error != nil {
		t.Errorf("legacy ping must still work: %+v", a[3].Error)
	}
}

// TestProtocolModernHTTP sends a 2026-07-28 tools/call through the whole
// HTTP stack (origin check, bearer token, in-flight cap, body cap), with the
// headers that revision requires, and checks a missing header is refused.
func TestProtocolModernHTTP(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: fixture})
	h := httpHandler(newServer(store), "s3cret", nil)
	post := func(body string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer s3cret")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	call := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"lookup_passage","arguments":{"reference":"Acts 16:1"},` + modernMeta + `}}`
	rec := post(call, map[string]string{"MCP-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/call", "Mcp-Name": "lookup_passage"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"resultType":"complete"`) || !strings.Contains(rec.Body.String(), `timoteo-se-une-a-pablo`) {
		t.Fatalf("modern tools/call over HTTP: %d %s", rec.Code, rec.Body.String())
	}
	rec = post(call, map[string]string{"MCP-Protocol-Version": "2026-07-28", "Mcp-Name": "lookup_passage"})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), fmt.Sprint(-32020)) {
		t.Fatalf("modern call without Mcp-Method: %d %s", rec.Code, rec.Body.String())
	}
	disc := `{"jsonrpc":"2.0","id":8,"method":"server/discover","params":{` + modernMeta + `}}`
	rec = post(disc, map[string]string{"MCP-Protocol-Version": "2026-07-28", "Mcp-Method": "server/discover"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"2026-07-28"`) {
		t.Fatalf("server/discover over HTTP: %d %s", rec.Code, rec.Body.String())
	}
}
