package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/geiserx/biblical-atlas-mcp/internal/config"
	"github.com/mark3labs/mcp-go/server"
)

// TestLiveTools calls every tool against the real data file with ids taken
// from the file itself. It runs only with BIBLICAL_ATLAS_LIVE=1.
func TestLiveTools(t *testing.T) {
	if os.Getenv("BIBLICAL_ATLAS_LIVE") != "1" {
		t.Skip("set BIBLICAL_ATLAS_LIVE=1 to download the real data file")
	}
	store := atlas.NewStore(atlas.Options{URL: config.DefaultDataURL, Timeout: 2 * time.Minute, UserAgent: "biblical-atlas-mcp-live-test"})
	snap, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("live", "0")
	Register(s, store)

	person := snap.Records(atlas.TypePerson)[0]
	var other, place string
	if adj := snap.Adjacent[person.ID]; len(adj) > 0 {
		other = adj[0].Other
	} else {
		other = snap.Records(atlas.TypePerson)[1].ID
	}
	for _, r := range snap.Records(atlas.TypePlace) {
		if r.Value.(*atlas.Place).HasPoint() {
			place = r.ID
			break
		}
	}
	book := snap.File.Books[0]
	calls := []struct {
		tool string
		args map[string]any
	}{
		{"search", map[string]any{"query": person.Name}},
		{"get_record", map[string]any{"type": "person", "id": person.ID}},
		{"get_record", map[string]any{"type": "place", "id": place}},
		{"get_record", map[string]any{"type": "event", "id": snap.File.Events[0].ID}},
		{"get_record", map[string]any{"type": "book", "id": book.Slug}},
		{"list_records", map[string]any{"type": "period"}},
		{"list_events", map[string]any{"person": person.ID}},
		{"people_in_year", map[string]any{"year": -607.0}},
		{"lookup_passage", map[string]any{"reference": book.Abbr + " 1"}},
		{"find_connection", map[string]any{"from": person.ID, "to": other}},
		{"places_near", map[string]any{"place": place}},
		{"dataset_info", map[string]any{}},
	}
	for _, c := range calls {
		msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": c.tool, "arguments": c.args}})
		raw, _ := json.Marshal(s.HandleMessage(context.Background(), msg))
		var out struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || out.Result.IsError {
			t.Errorf("%s %v: %s", c.tool, c.args, raw)
		}
	}
}

// Answer budgets. Claude Code refuses a tool answer above 25,000 tokens by
// default, and this JSON (Spanish text, ids, many URLs) runs near 3 bytes a
// token, so no answer may pass 70 KB. Answers with default arguments keep
// more room, 60 KB. The largest today are near 60 KB (a place's full source
// list) and 66 KB (a whole book in lookup_passage with limit 50).
const (
	budgetDefault = 60_000
	budgetMax     = 70_000
)

// TestLiveAnswerSizes calls every tool over the whole real file, with the
// default arguments and with the largest page, and fails when an answer
// passes its budget. It runs only with BIBLICAL_ATLAS_LIVE=1.
func TestLiveAnswerSizes(t *testing.T) {
	if os.Getenv("BIBLICAL_ATLAS_LIVE") != "1" {
		t.Skip("set BIBLICAL_ATLAS_LIVE=1 to download the real data file")
	}
	store := atlas.NewStore(atlas.Options{URL: config.DefaultDataURL, Timeout: 2 * time.Minute, UserAgent: "biblical-atlas-mcp-live-test"})
	snap, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("live", "0")
	Register(s, store)

	type probe struct {
		tool  string
		args  map[string]any
		paged bool
	}
	var probes []probe
	add := func(tool string, paged bool, args map[string]any) { probes = append(probes, probe{tool, args, paged}) }
	for _, typ := range atlas.Types {
		for _, r := range snap.Records(typ) {
			add("get_record", false, map[string]any{"type": string(typ), "id": r.ID})
		}
	}
	for _, r := range snap.Records(atlas.TypePerson) {
		add("list_events", true, map[string]any{"person": r.ID})
	}
	for _, r := range snap.Records(atlas.TypePlace) {
		add("list_events", true, map[string]any{"place": r.ID})
		if r.Value.(*atlas.Place).HasPoint() {
			add("places_near", true, map[string]any{"place": r.ID})
		}
	}
	for _, b := range snap.File.Books {
		add("list_events", true, map[string]any{"book": b.Slug})
		add("lookup_passage", true, map[string]any{"reference": b.Abbr})
		for c := 1; c <= b.Chapters; c++ {
			add("lookup_passage", true, map[string]any{"reference": fmt.Sprintf("%s %d", b.Abbr, c)})
		}
	}
	for y := MinYear; y <= MaxYear; y += 10 {
		if y != 0 {
			add("people_in_year", true, map[string]any{"year": float64(y)})
			add("list_events", true, map[string]any{"from_year": float64(y), "to_year": float64(min(y+9, MaxYear))})
		}
	}
	for _, typ := range listableTypes {
		switch typ {
		case atlas.TypePerson:
			for _, k := range snap.OfficeKinds {
				add("list_records", true, map[string]any{"type": "person", "kind": k})
			}
		case atlas.TypePlace:
			for _, k := range snap.PlaceKinds {
				add("list_records", true, map[string]any{"type": "place", "kind": k})
			}
		default:
			add("list_records", true, map[string]any{"type": string(typ)})
		}
	}
	for _, q := range []string{"a", "e", "jerusalen", "rey", "hijo"} {
		add("search", true, map[string]any{"query": q})
	}

	biggest := map[string]int{}
	check := func(p probe, args map[string]any, budget int, label string) {
		msg, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": p.tool, "arguments": args}})
		raw, _ := json.Marshal(s.HandleMessage(context.Background(), msg))
		var out struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &out); err != nil || len(out.Result.Content) != 1 {
			t.Errorf("%s %v: %s", p.tool, args, raw)
			return
		}
		if out.Result.IsError {
			t.Errorf("%s %v: %s", p.tool, args, out.Result.Content[0].Text)
			return
		}
		n := len(out.Result.Content[0].Text)
		if key := p.tool + " " + label; n > biggest[key] {
			biggest[key] = n
		}
		if n > budget {
			t.Errorf("%s %v (%s): %d bytes, budget %d", p.tool, args, label, n, budget)
		}
	}
	for _, p := range probes {
		check(p, p.args, budgetDefault, "default")
		if p.paged {
			args := map[string]any{"limit": float64(maxLimit)}
			if p.tool == "search" {
				args["limit"] = 50.0
			}
			for k, v := range p.args {
				args[k] = v
			}
			check(p, args, budgetMax, "largest page")
		}
	}
	for k, v := range biggest {
		t.Logf("largest %s answer: %d bytes", k, v)
	}
	t.Logf("%d probes", len(probes))
}
