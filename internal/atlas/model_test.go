package atlas

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeFixture(t *testing.T) {
	f, err := Decode(readFixture(t, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	if f.Format != "biblical-atlas/v0" || f.Generated != "2026-09-30" {
		t.Fatalf("format %q generated %q", f.Format, f.Generated)
	}
	if len(f.People) != 8 || len(f.Places) != 7 || len(f.Events) != 9 || len(f.Books) != 5 || len(f.Months) != 2 || len(f.Topics) != 1 {
		t.Fatalf("counts: %d people, %d places, %d events, %d books, %d months, %d topics",
			len(f.People), len(f.Places), len(f.Events), len(f.Books), len(f.Months), len(f.Topics))
	}
	if f.PlaceOrder[0] != "acsaf" || f.PeopleOrder[len(f.PeopleOrder)-1] != "timoteo" {
		t.Fatalf("file order lost: %v %v", f.PlaceOrder, f.PeopleOrder)
	}
	// A relation with only the old Spanish fields falls back to them.
	var rel *Relation
	for i, r := range f.People["pablo"].Relations {
		if r.Person == "santiago-hijo-de-alfeo" {
			rel = &f.People["pablo"].Relations[i]
		}
	}
	if rel == nil || rel.Type != "accompanies" || rel.Word != "compañero" || rel.Status != "pendiente" || rel.CheckedOn != "2026-09-20" {
		t.Fatalf("Spanish fallback: %+v", rel)
	}
	// English fields win over Spanish ones when both are present.
	r := f.People["eunice"].Relations[0]
	if r.Type != "kin" || r.Word != "mother" {
		t.Fatalf("English first: %+v", r)
	}
}

// Both ids of the one format load; any other id is refused with the upgrade error.
func TestDecodeFormatIDs(t *testing.T) {
	valid := string(readFixture(t, "data.json"))
	for _, c := range []struct{ id, want string }{
		{"biblical-atlas/v0", ""},
		{"biblical-earth/v0", ""},
		{"biblical-atlas/v1", `unsupported data format "biblical-atlas/v1": this server reads biblical-atlas/v0. Upgrade biblical-atlas-mcp.`},
	} {
		t.Run(c.id, func(t *testing.T) {
			body := strings.Replace(valid, `"formato": "biblical-atlas/v0"`, `"formato": "`+c.id+`"`, 1)
			if body == valid && c.id != "biblical-atlas/v0" {
				t.Fatal("fixture format not replaced")
			}
			f, err := Decode([]byte(body))
			switch {
			case c.want == "" && (err != nil || f.Format != c.id):
				t.Fatalf("got %v, want format %q loaded", err, c.id)
			case c.want != "" && (err == nil || err.Error() != c.want):
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
}

// A repeated event id keeps the first entry only, so every position the
// indexes hold names the same event as the record for its id.
func TestDecodeDropsRepeatedIDs(t *testing.T) {
	var top map[string]any
	if err := json.Unmarshal(readFixture(t, "data.json"), &top); err != nil {
		t.Fatal(err)
	}
	events := top["eventos"].([]any)
	top["eventos"] = append(events, map[string]any{
		"id": "creacion-de-adan", "titulo": "Otra creación", "personas": []any{"timoteo"},
		"lugares": []any{"listra"}, "pasajes": []any{"Hch 16:1"},
	})
	body, err := json.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Events) != len(events) {
		t.Fatalf("%d events, want %d", len(f.Events), len(events))
	}
	s := Build(f)
	for i, e := range f.Events {
		if r := s.Rec(TypeEvent, e.ID); r == nil || r.Value != e {
			t.Fatalf("position %d (%s) is not the event its record holds", i, e.ID)
		}
	}
	for _, i := range s.EventsByPerson["timoteo"] {
		if f.Events[i].ID == "creacion-de-adan" {
			t.Fatal("the repeated entry was indexed")
		}
	}
}

func TestDecodeEnglishOnly(t *testing.T) {
	f, err := Decode(readFixture(t, "english-only.json"))
	if err != nil {
		t.Fatal(err)
	}
	r := f.People["timoteo"].Relations[2]
	if r.Tipo != "" || r.Relacion != "" || r.Type != "kin" || r.Word != "mother" {
		t.Fatalf("relation: %+v", r)
	}
	for _, r := range f.People["pablo"].Relations {
		if r.Person == "santiago-hijo-de-alfeo" && (r.Type != "accompanies" || r.Status != "pending" || r.CheckedOn != "2026-09-20") {
			t.Fatalf("relation: %+v", r)
		}
	}
	s := Build(f)
	if len(s.RelIn["pablo"]) != 1 || s.RelIn["pablo"][0].Owner != "timoteo" {
		t.Fatalf("reverse index: %+v", s.RelIn["pablo"])
	}
}

func TestDecodeRejects(t *testing.T) {
	valid := string(readFixture(t, "data.json"))
	cases := []struct {
		name, body, want string
	}{
		{"unknown format", string(readFixture(t, "unknown-format.json")),
			`unsupported data format "biblical-atlas/v9": this server reads biblical-atlas/v0. Upgrade biblical-atlas-mcp.`},
		{"missing format", `{"personas":{}}`, `unsupported data format "": this server reads biblical-atlas/v0. Upgrade biblical-atlas-mcp.`},
		{"html", "<!doctype html><html><body>hola</body></html>", "data file is not a JSON object"},
		{"array", "[1,2]", "data file is not a JSON object"},
		{"empty", "", "data file is not a JSON object"},
		{"truncated", valid[:len(valid)/2], "data file is not valid JSON"},
		{"no people", strings.Replace(valid, `"personas": {`, `"personas_old": {`, 1), `data file has no "personas" object`},
		{"empty events", `{"formato":"biblical-atlas/v0","personas":{"a":{}},"lugares":{"b":{}},"fuentes":{"c":{}},"eventos":[],"libros":[{}]}`, `data file has no "eventos" array`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode([]byte(c.body))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
}

func TestIndexes(t *testing.T) {
	s := fixture(t)
	if s.Stats.Dangling != 1 || s.Stats.BadEventPassages != 0 || s.Stats.BadCitations != 0 {
		t.Fatalf("stats: %+v", s.Stats)
	}
	// Relations stored on the other person show up in the reverse index.
	in := s.RelIn["eunice"]
	if len(in) != 1 || in[0].Owner != "timoteo" || in[0].Stored {
		t.Fatalf("RelIn eunice: %+v", in)
	}
	if len(s.RelByPlace["listra"]) != 2 {
		t.Fatalf("RelByPlace listra: %+v", s.RelByPlace["listra"])
	}
	// Adjacency is sorted by relation key.
	adj := s.Adjacent["pablo"]
	if len(adj) != 2 || adj[0].Other != "santiago-hijo-de-alfeo" || adj[1].Other != "timoteo" {
		t.Fatalf("Adjacent pablo: %+v", adj)
	}
	if got := s.EventsByPerson["eunice"]; len(got) != 2 || got[0] != 4 || got[1] != 7 {
		t.Fatalf("EventsByPerson eunice: %v", got)
	}
	if got := s.EventsByPlace["jerusalen"]; len(got) != 3 {
		t.Fatalf("EventsByPlace jerusalen: %v", got)
	}
	life := s.Lives["adan"]
	if life == nil || *life.Born.From != -4025 || *life.Died.From != -3095 {
		t.Fatalf("life of adan: %+v", life)
	}
	cited := s.CitedBy[Chapter{44, 16}]
	want := []RecKey{{TypePerson, "eunice"}, {TypePerson, "pablo"}, {TypePerson, "timoteo"}, {TypeTour, "cartas-y-ciudades"}}
	if len(cited) != len(want) {
		t.Fatalf("cited by Hch 16: %v", cited)
	}
	for i := range want {
		if cited[i] != want[i] {
			t.Fatalf("cited by Hch 16: %v", cited)
		}
	}
	if !s.FullyRead["genesis"] || !s.FullyRead["rut"] || s.FullyRead["hechos"] {
		t.Fatalf("fully read: %v", s.FullyRead)
	}
	if strings.Join(s.RoleKinds, ",") != "born,died,spoke" || strings.Join(s.EventKinds, ",") != "birth,death,speech" {
		t.Fatalf("kinds: %v %v", s.RoleKinds, s.EventKinds)
	}
	if len(s.LettersByPerson["eunice"]) != 1 || s.LettersByPerson["eunice"][0].As != "named" {
		t.Fatalf("letters by eunice: %v", s.LettersByPerson["eunice"])
	}
	if s.Rec(TypePerson, "adan") == nil || s.Rec(TypePlace, "adan") == nil {
		t.Fatal("colliding ids must both exist")
	}
}

// TestMalformedEntriesDoNotPanic: a null book and reversed ranges in every
// citation field decode and build; the bad citations are counted.
func TestMalformedEntriesDoNotPanic(t *testing.T) {
	body := string(readFixture(t, "data.json"))
	for _, r := range []struct{ old, new string }{
		{`"libros": [`, `"libros": [null, `},
		{`"referencia": "Hch 13:1–14:28"`, `"referencia": "Hch 16-3"`},
		{`"referencia": "Hch 14:6-20"`, `"referencia": "Hch 16:5-14:2"`},
		{`"reference": "2Ti 1:5"}`, `"reference": "Gé 9-2"}`},
		{`"pasajes": ["Gé 5:5"]`, `"pasajes": ["Gé 5-3"]`},
	} {
		if strings.Count(body, r.old) != 1 {
			t.Fatalf("fixture changed: %q found %d times", r.old, strings.Count(body, r.old))
		}
		body = strings.Replace(body, r.old, r.new, 1)
	}
	f, err := Decode([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Books) != 5 {
		t.Fatalf("null book kept: %d books", len(f.Books))
	}
	s := Build(f)
	// The relation's reference is read once for each of its two people.
	if s.Stats.BadCitations != 4 || s.Stats.BadEventPassages != 1 {
		t.Fatalf("bad citations: %+v", s.Stats)
	}
}
