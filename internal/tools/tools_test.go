package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/server"
)

var (
	testOnce   sync.Once
	testServer *server.MCPServer
)

func mcpServer(t *testing.T) *server.MCPServer {
	t.Helper()
	testOnce.Do(func() {
		store := atlas.NewStore(atlas.Options{File: "../atlas/testdata/data.json"})
		if _, err := store.Snapshot(context.Background()); err != nil {
			panic(err)
		}
		testServer = server.NewMCPServer("test", "0", server.WithToolCapabilities(false), server.WithInstructions(Instructions))
		Register(testServer, store)
	})
	return testServer
}

// call sends a tools/call message through the server, so schemas and
// registration are part of every test.
func call(t *testing.T, name string, args map[string]any) (string, bool) {
	t.Helper()
	return callOn(t, mcpServer(t), name, args)
}

func callOn(t *testing.T, srv *server.MCPServer, name string, args map[string]any) (string, bool) {
	t.Helper()
	msg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	resp := srv.HandleMessage(context.Background(), msg)
	raw, _ := json.Marshal(resp)
	var out struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("bad response %s: %v", raw, err)
	}
	if out.Error != nil {
		t.Fatalf("JSON-RPC error for %s: %s", name, out.Error.Message)
	}
	if len(out.Result.Content) != 1 {
		t.Fatalf("want one content item, got %s", raw)
	}
	return out.Result.Content[0].Text, out.Result.IsError
}

type toolCase struct {
	name    string
	tool    string
	args    map[string]any
	want    []string // substrings of the compact JSON answer
	notWant []string
	wantErr string // substring of the error text; empty for a normal answer
}

var cases = []toolCase{
	// search
	{name: "search homonyms", tool: "search", args: map[string]any{"query": "Santiago", "types": []any{"person"}},
		want: []string{`"total":2`, `"results":[{"type":"person","id":"santiago-hijo-de-alfeo"`, `"same_name_count":2`, `"disambiguation":"El apóstol hijo de Alfeo (prueba)."`, `"dataset":"2026-09-30"`}},
	{name: "search colliding ids", tool: "search", args: map[string]any{"query": "ADÁN"},
		want: []string{`"results":[{"type":"person","id":"adan","name":"Adán","url":"https://biblical-atlas.geiser.cloud/#sel=persona:adan"`, `{"type":"place","id":"adan"`, `"score":100,"matched":"name"`}},
	{name: "search summary tier", tool: "search", args: map[string]any{"query": "apostol viajo"},
		want: []string{`"total":1`, `"id":"pablo"`, `"score":20,"matched":"summary"`}},
	{name: "search paging", tool: "search", args: map[string]any{"query": "a", "limit": 1.0, "offset": 1.0},
		want: []string{`"offset":1,"limit":1,"next_offset":2`}},
	{name: "search item sources", tool: "search", args: map[string]any{"query": "Pablo", "types": []any{"person"}},
		// An item shows the first of its 4 sources (links first), and a URL
		// shared by two ids counts once.
		want:    []string{`"sources":[{"title":"Perspicacia: «Pablo»","url":"https://wol.jw.org/es/wol/d/r4/lp-s/1200003406","kind":"perspicacia"}],"sources_total":4`},
		notWant: []string{`"id":"hechos-9"`, `hechos-9-copia`}},
	{name: "search no match", tool: "search", args: map[string]any{"query": "zzzz"},
		want: []string{`{"total":0,"offset":0,"limit":10,"results":[]`}},
	{name: "search missing query", tool: "search", args: map[string]any{}, wantErr: "query is required"},
	{name: "search long query", tool: "search", args: map[string]any{"query": strings.Repeat("a", 101)}, wantErr: "query must be at most 100 characters, got 101"},
	{name: "search bad type", tool: "search", args: map[string]any{"query": "a", "types": []any{"people"}}, wantErr: `types must contain only person, place`},
	{name: "search limit too high", tool: "search", args: map[string]any{"query": "a", "limit": 51.0}, wantErr: "limit must be a whole number from 1 to 50, got 51"},
	{name: "search fractional limit", tool: "search", args: map[string]any{"query": "a", "limit": 2.5}, wantErr: "limit must be a whole number from 1 to 50, got 2.5"},
	{name: "search negative offset", tool: "search", args: map[string]any{"query": "a", "offset": -1.0}, wantErr: "offset must be a whole number"},

	// get_record
	{name: "person family both ways", tool: "get_record", args: map[string]any{"type": "person", "id": "eunice"},
		want: []string{`"family":{"parents":[{"type":"person","id":"loida"`, `"children":[{"type":"person","id":"timoteo"`,
			// Stored on eunice, word names loida; stored on timoteo, word names eunice.
			`"phrase":"su madre","type":"kin","word":"mother","word_of":"other"`, `"phrase":"Timoteo es su hijo","type":"kin","word":"mother","word_of":"self"`,
			`"sources":[{"id":"it-eunice","title":"Eunice","work":"Obra de prueba","url":"https://wol.jw.org/es/wol/d/r4/lp-s/1200001421"}],"sources_total":2}`,
			`"letters":[{"type":"letter","id":"2-timoteo","name":"2 Timoteo","url":"https://biblical-atlas.geiser.cloud/#sel=carta:2-timoteo","as":"named"}]`}},
	{name: "person relations order", tool: "get_record", args: map[string]any{"type": "person", "id": "timoteo"},
		want: []string{`"relations":[{"other":{"type":"person","id":"eunice"`, `"type":"kin","word":"mother"`, `"journeys":[{"type":"journey","id":"primer-viaje"`, `"as":"companion"`, `"events_total":1`, `"as":"recipient"`}},
	{name: "type names are exact", tool: "get_record", args: map[string]any{"type": "PERSON", "id": "PABLO"},
		wantErr: "type must be one of"},
	{name: "person case-insensitive id", tool: "get_record", args: map[string]any{"type": "person", "id": "PABLO"},
		want: []string{`"id":"pablo"`, `"date":{"text":"c. 33-65 e.c.","from":33,"to":65,"approx":true,"kind":"anclada","precision":"rango"}`,
			`"other":{"type":"place","id":"tarso"`, `"phrase":"nació aquí"`, `"status":"pendiente"`, `"inferred":true`,
			`"history":[{"date":"2026-09-29","change":"Cambio de prueba.","source":{"id":"hechos-9"`, `"sources":[{"id":"it-pablo"`},
		notWant: []string{`"eden"`, `it-fantasma`, `hechos-9-copia`, `next_relations_offset`}},
	{name: "relations offset only for people", tool: "get_record", args: map[string]any{"type": "place", "id": "jerusalen", "relations_offset": 1.0},
		wantErr: "relations_offset is only accepted for type person"},
	{name: "person with office", tool: "get_record", args: map[string]any{"type": "person", "id": "anas"},
		want: []string{`"offices":[{"office":"high_priest","label":"sumo sacerdote","place":{"type":"place","id":"jerusalen"`, `"from":6,"to":15`}},
	{name: "person homonym warning", tool: "get_record", args: map[string]any{"type": "person", "id": "santiago-padre-de-tadeo"},
		want: []string{`"not_to_confuse_with":[{"type":"person","id":"santiago-hijo-de-alfeo"`}},
	{name: "place with point", tool: "get_record", args: map[string]any{"type": "place", "id": "jerusalen"},
		want: []string{`"kind":"ciudad","lat":31.776667,"lon":35.234167,"precision":"punto"`, `"coord_source":{"id":"openbible:a0002"`,
			`{"name":"Salem","from":-1933,"to":-1933,"note":"Nombre antiguo (prueba)."}`, `"people":{"lived":[{"type":"person","id":"pablo"`, `"lived_total":1`,
			`"events_total":3`, `"main":true`, `"journey_stops":[{"journey":{"type":"journey","id":"primer-viaje"`, `"finds":[{"type":"find","id":"osario-de-caifas"`,
			`"seat_of":[{"type":"period","id":"sumo-sacerdocio-de-anas"`}},
	{name: "place without point", tool: "get_record", args: map[string]any{"type": "place", "id": "acsaf"},
		want:    []string{`"candidates":[{"name":"Primer sitio de prueba","status":"alternativa","shape":"punto","lat":32.872761,"lon":35.15133`, `"radius_km":5`},
		notWant: []string{`"lat":null`}},
	{name: "place colliding id", tool: "get_record", args: map[string]any{"type": "place", "id": "adan"},
		want: []string{`"type":"place","id":"adan"`, `"url":"https://biblical-atlas.geiser.cloud/#sel=lugar:adan"`}},
	{name: "event with roles", tool: "get_record", args: map[string]any{"type": "event", "id": "creacion-de-adan"},
		want: []string{`"date":{"text":"4026 a.e.c.","from":-4026,"to":-4026`, `"roles":[{"person":{"type":"person","id":"adan"`, `"role":"born"`,
			`"passages":[{"text":"Gé 2:7, 8","url":"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/G%C3%A9nesis/2/#v1002007-v1002008"}]`, `"kind":"birth"`, `"narrative_order":{"series":"genesis","order":1}`}},
	{name: "event with alternatives", tool: "get_record", args: map[string]any{"type": "event", "id": "arresto-de-pablo-en-jerusalen"},
		want: []string{`"month":"sivan","season":"primavera"`, `"alternatives":[{"date":{"text":"57 e.c.","from":57,"to":57,"kind":"anclada","precision":"año","chronology":"secular"}`,
			`"periods":[{"type":"period","id":"potencia-romana"`}},
	{name: "event with present", tool: "get_record", args: map[string]any{"type": "event", "id": "timoteo-se-une-a-pablo"},
		want: []string{`"present":[{"type":"person","id":"eunice"`, `"kind":"narrativa"`}},
	{name: "event open date", tool: "get_record", args: map[string]any{"type": "event", "id": "loida-ensena-a-eunice"},
		want: []string{`"date":{"text":"antes de 49 e.c.","to":48,`}},
	{name: "period", tool: "get_record", args: map[string]any{"type": "period", "id": "sumo-sacerdocio-de-anas"},
		want: []string{`"ruler":{"type":"person","id":"anas"`, `"office":"high_priest"`, `"attested_from":6`}},
	{name: "period across eras", tool: "get_record", args: map[string]any{"type": "period", "id": "potencia-romana"},
		want: []string{`"from":-63,"to":476`, `"events":[{"type":"event","id":"arresto-de-pablo-en-jerusalen"`}},
	{name: "letter", tool: "get_record", args: map[string]any{"type": "letter", "id": "2-timoteo"},
		want: []string{`"book":{"type":"book","id":"2-timoteo"`, `"writer":{"type":"person","id":"pablo"`, `"written_in":[{"type":"place","id":"roma"`,
			`"recipients":{"text":"Timoteo","people":[{"type":"person","id":"timoteo"`, `"origin_context":{"summary":"Contexto de prueba."`}},
	{name: "journey", tool: "get_record", args: map[string]any{"type": "journey", "id": "primer-viaje"},
		want: []string{`"traveller":{"type":"person","id":"pablo"`, `"stops":[{"order":1,"place":{"type":"place","id":"jerusalen","name":"Jerusalén","url":"https://biblical-atlas.geiser.cloud/#sel=lugar:jerusalen","lat":31.776667,"lon":35.234167}`}},
	{name: "find", tool: "get_record", args: map[string]any{"type": "find", "id": "osario-de-caifas"},
		want:    []string{`"found_at":{"type":"place","id":"jerusalen"`, `"relates_to":[{"type":"person","id":"anas"`, `"object_date":{"text":"c. 30 e.c."`, `"identification":"probable"`},
		notWant: []string{"no-existe"}},
	{name: "tour", tool: "get_record", args: map[string]any{"type": "tour", "id": "cartas-y-ciudades"},
		want: []string{`"about":{"type":"event","id":"formacion-de-pablo"`, `"year":-2`, `"about":{"type":"passage","id":"hch-16","name":"Hechos 16","url":"https://biblical-atlas.geiser.cloud/#sel=pasaje:hch-16","read_url":"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/hechos/16/"`,
			`"question":{"text":"¿Desde qué ciudad? (prueba)","options":["Roma","Atenas"],"answer":"Roma"`, `"not_known":"Dato de prueba que no se sabe."`}},
	{name: "book not read", tool: "get_record", args: map[string]any{"type": "book", "id": "hechos"},
		want:    []string{`"number":44,"abbreviation":"Hch","chapters":28`, `"omitted_verses":[{"chapter":8,"verses":[37]}`, `"fully_read":false`, `"read_url":"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/hechos/1/"`},
		notWant: []string{`"coverage"`}},
	{name: "book read", tool: "get_record", args: map[string]any{"type": "book", "id": "genesis"},
		want: []string{`"coverage":{"chapters":50,"chapters_read":50,"verses":1533,"verses_read":1533}`, `"fully_read":true`, `"covers":{"text":"hasta 1657 a.e.c.","to":-1657`}},
	{name: "book that is a letter", tool: "get_record", args: map[string]any{"type": "book", "id": "2-timoteo"},
		want: []string{`"letter":{"type":"letter","id":"2-timoteo"`}},
	{name: "month", tool: "get_record", args: map[string]any{"type": "month", "id": "nisan"},
		want:    []string{`"type":"month","id":"nisan","name":"Nisán"`, `"other_names":["Abib"]`, `{"name":"Abib","to":-537`, `"festivals":[{"name":"Pascua","from_day":14,"to_day":14,"instituted":-1513`},
		notWant: []string{`"url":"https://biblical-atlas.geiser.cloud/#sel=`}},
	{name: "calendar topic", tool: "get_record", args: map[string]any{"type": "calendar_topic", "id": "mes-lunar"},
		want: []string{`"text":"Texto de prueba sobre el mes lunar."`}},
	{name: "unknown id suggests", tool: "get_record", args: map[string]any{"type": "person", "id": "pablo-apostol"},
		wantErr: `no person with id "pablo-apostol". Did you mean: pablo (Pablo)?`},
	{name: "unknown id without suggestion", tool: "get_record", args: map[string]any{"type": "letter", "id": "qqqqqqqqqq"},
		wantErr: `no letter with id "qqqqqqqqqq". Find ids with search`},
	{name: "missing id", tool: "get_record", args: map[string]any{"type": "person"}, wantErr: "id is required"},

	// list_records
	{name: "people by office", tool: "list_records", args: map[string]any{"type": "person", "kind": "high_priest"},
		want: []string{`"total":2`, `"results":[{"type":"person","id":"anas"`, `"office":{"office":"high_priest"`, `{"type":"person","id":"santiago-padre-de-tadeo"`,
			// The office in a list item carries its first source only.
			`"status":"verified","sources":[{"id":"it-anas","title":"Anás","work":"Obra de prueba","url":"https://wol.jw.org/es/wol/d/r4/lp-s/1200000291"}],"sources_total":2}`}},
	{name: "places by kind", tool: "list_records", args: map[string]any{"type": "place", "kind": "ciudad", "limit": 2.0},
		want: []string{`"total":6,"offset":0,"limit":2,"next_offset":2,"results":[{"type":"place","id":"acsaf"`, `{"type":"place","id":"adan"`}},
	{name: "periods by kind", tool: "list_records", args: map[string]any{"type": "period", "kind": "potencia"},
		want: []string{`"total":1`, `"id":"potencia-romana"`, `"kind":"potencia"`}},
	{name: "all periods", tool: "list_records", args: map[string]any{"type": "period"},
		want: []string{`"total":2`, `"ruler":{"type":"person","id":"anas"`}},
	{name: "exact last page", tool: "list_records", args: map[string]any{"type": "period", "limit": 2.0},
		want: []string{`"total":2,"offset":0,"limit":2,"results"`}, notWant: []string{"next_offset"}},
	{name: "page before the last", tool: "list_records", args: map[string]any{"type": "period", "limit": 1.0},
		want: []string{`"total":2,"offset":0,"limit":1,"next_offset":1`}},
	{name: "limit above 50", tool: "list_records", args: map[string]any{"type": "period", "limit": 51.0}, wantErr: "limit must be a whole number from 1 to 50, got 51"},
	{name: "books", tool: "list_records", args: map[string]any{"type": "book"},
		want: []string{`"total":5`, `"number":1,"abbreviation":"Gé","chapters":50,"fully_read":true`}},
	{name: "months", tool: "list_records", args: map[string]any{"type": "month"}, want: []string{`"total":2`}},
	{name: "topics", tool: "list_records", args: map[string]any{"type": "calendar_topic"}, want: []string{`"total":1`, `"id":"mes-lunar"`}},
	{name: "letters", tool: "list_records", args: map[string]any{"type": "letter"}, want: []string{`"total":1`}},
	{name: "journeys", tool: "list_records", args: map[string]any{"type": "journey"}, want: []string{`"total":1`}},
	{name: "finds", tool: "list_records", args: map[string]any{"type": "find"}, want: []string{`"total":1`}},
	{name: "tours", tool: "list_records", args: map[string]any{"type": "tour"}, want: []string{`"total":1`}},
	{name: "events are not listed here", tool: "list_records", args: map[string]any{"type": "event"}, wantErr: "type must be one of person, place, period"},
	{name: "person needs kind", tool: "list_records", args: map[string]any{"type": "person"}, wantErr: "kind is required for type person; values present: high_priest"},
	{name: "unknown kind lists values", tool: "list_records", args: map[string]any{"type": "place", "kind": "volcan"}, wantErr: `kind "volcan" is not in the atlas; values present: ciudad, region`},
	{name: "kind rejected for letters", tool: "list_records", args: map[string]any{"type": "letter", "kind": "x"}, wantErr: "kind is only accepted for person, place and period"},

	// list_events
	{name: "events in a year, open start included", tool: "list_events", args: map[string]any{"from_year": -4026.0},
		want: []string{`"total":2`, `"results":[{"type":"event","id":"creacion-de-adan"`, `"id":"loida-ensena-a-eunice"`}},
	{name: "events across the era boundary", tool: "list_events", args: map[string]any{"from_year": -2.0, "to_year": 1.0},
		want: []string{`"total":2`, `"results":[{"type":"event","id":"formacion-de-pablo"`, `"id":"loida-ensena-a-eunice"`}},
	{name: "events of a person", tool: "list_events", args: map[string]any{"person": "pablo"},
		want: []string{`"total":5`, `"people_total":1`, `"passages":["Hch 22:3"]`}},
	{name: "events of a person with a role", tool: "list_events", args: map[string]any{"person": "pablo", "role": "spoke"},
		want: []string{`"total":1`, `"role":"spoke"`}},
	{name: "present flag", tool: "list_events", args: map[string]any{"person": "eunice"},
		want: []string{`"total":2`, `"present":true`}},
	{name: "events at a place", tool: "list_events", args: map[string]any{"place": "jerusalen"},
		want: []string{`"total":3`, `"main":true`}},
	{name: "events by kind", tool: "list_events", args: map[string]any{"kind": "speech"},
		want: []string{`"total":1`, `"kind":"speech"`}},
	{name: "events of a book in story order", tool: "list_events", args: map[string]any{"book": "Hch"},
		want: []string{`"total":5`, `"results":[{"type":"event","id":"primer-viaje-misional"`, `"in_story":true`}},
	{name: "events that only cite a book", tool: "list_events", args: map[string]any{"book": "2Ti"},
		want: []string{`"total":1`, `"id":"loida-ensena-a-eunice"`}, notWant: []string{`"in_story"`}},
	{name: "anchored only", tool: "list_events", args: map[string]any{"person": "pablo", "anchored_only": true},
		want: []string{`"total":4`}, notWant: []string{"timoteo-se-une-a-pablo"}},
	{name: "no filter", tool: "list_events", args: map[string]any{}, wantErr: "give at least one filter"},
	{name: "to_year alone", tool: "list_events", args: map[string]any{"to_year": 5.0}, wantErr: "to_year needs from_year"},
	{name: "year zero", tool: "list_events", args: map[string]any{"from_year": 0.0}, wantErr: "from_year must be a whole number from -4100 to 500 and not 0"},
	{name: "range backwards", tool: "list_events", args: map[string]any{"from_year": 10.0, "to_year": 5.0}, wantErr: "to_year must not be before from_year"},
	{name: "role needs person", tool: "list_events", args: map[string]any{"role": "born", "kind": "birth"}, wantErr: "role needs person"},
	{name: "unknown role", tool: "list_events", args: map[string]any{"person": "pablo", "role": "sang"}, wantErr: `role "sang" is not in the atlas; values present: born, died, spoke`},
	{name: "unknown kind", tool: "list_events", args: map[string]any{"kind": "war"}, wantErr: `kind "war" is not in the atlas`},
	{name: "unknown book", tool: "list_events", args: map[string]any{"book": "Hechs"}, wantErr: `book "Hechs" is not a Bible book the atlas knows. Close names: Hechos`},
	{name: "unknown person", tool: "list_events", args: map[string]any{"person": "pablito"}, wantErr: `no person with id "pablito". Did you mean: pablo (Pablo)`},

	// people_in_year
	{name: "dated", tool: "people_in_year", args: map[string]any{"year": 50.0},
		want: []string{`"total":1`, `"id":"pablo"`, `"basis":"dated"`, `"year":50`, `"periods":[{"type":"period","id":"potencia-romana"`}},
	{name: "office and event", tool: "people_in_year", args: map[string]any{"year": 10.0},
		want: []string{`"total":4`, `"results":[{"type":"person","id":"anas"`, `"basis":"office"`, `"basis":"event","evidence":{"type":"event","id":"loida-ensena-a-eunice"`,
			`"basis":"event","evidence":{"type":"event","id":"formacion-de-pablo"`}},
	{name: "birth and death", tool: "people_in_year", args: map[string]any{"year": -3500.0},
		want: []string{`"id":"adan"`, `"basis":"birth_and_death","evidence":{"born":{"text":"4026 a.e.c."`, `"died":{"text":"3096 a.e.c."`}},
	{name: "birth and death beats the atlas date", tool: "people_in_year", args: map[string]any{"year": -3150.0},
		want: []string{`"id":"adan"`, `"basis":"birth_and_death"`}, notWant: []string{`"basis":"dated"`}},
	{name: "nobody", tool: "people_in_year", args: map[string]any{"year": 300.0},
		want: []string{`"total":0,"offset":0,"limit":20,"results":[]`}},
	{name: "missing year", tool: "people_in_year", args: map[string]any{}, wantErr: "year is required"},
	{name: "year out of range", tool: "people_in_year", args: map[string]any{"year": -5000.0}, wantErr: "year must be a whole number from -4100 to 500 and not 0"},
	{name: "fractional year", tool: "people_in_year", args: map[string]any{"year": 33.5}, wantErr: "got 33.5"},

	// lookup_passage
	{name: "verse", tool: "lookup_passage", args: map[string]any{"reference": "Hch 16:1"},
		want: []string{`"total":1`, `"id":"timoteo-se-une-a-pablo"`, `"passage":{"book":{"type":"book","id":"hechos"`, `"chapter":16,"verse":1,"to_chapter":16,"to_verse":1,"text":"Hch 16:1","url":"https://biblical-atlas.geiser.cloud/#sel=pasaje:hch-16","read_url":"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/hechos/16/#v44016001"`,
			`"fully_read":false,"notice":"The atlas has not yet read this book verse by verse.`, `"people":[{"type":"person","id":"eunice"`, `"people_total":3`,
			`"cited_by":[{"type":"person","id":"eunice"`, `{"type":"tour","id":"cartas-y-ciudades"`, `"cited_by_total":4`}},
	{name: "read book", tool: "lookup_passage", args: map[string]any{"reference": "Gé 2:8"},
		want: []string{`"id":"creacion-de-adan"`, `"fully_read":true`}, notWant: []string{`"notice"`}},
	{name: "chapter span", tool: "lookup_passage", args: map[string]any{"reference": "Hechos 14"},
		want: []string{`"total":2`, `"id":"primer-viaje-misional"`, `"id":"pablo-habla-en-listra"`}},
	{name: "gap in an event passage", tool: "lookup_passage", args: map[string]any{"reference": "Hch 14:12"},
		want: []string{`"total":1`, `"id":"primer-viaje-misional"`}, notWant: []string{`pablo-habla-en-listra`}},
	{name: "verse named after a gap", tool: "lookup_passage", args: map[string]any{"reference": "Hch 14:18"},
		want: []string{`"total":2`, `"id":"pablo-habla-en-listra"`}},
	{name: "gap in the reference", tool: "lookup_passage", args: map[string]any{"reference": "Hch 16:1-3, 6"},
		wantErr: `reference must name one passage with no gaps, got 2 in "Hch 16:1-3, 6" (Hch 16:1-3; Hch 16:6)`},
	{name: "comma between chapters", tool: "lookup_passage", args: map[string]any{"reference": "Hch 3, 16"},
		wantErr: `write "Hch 3:16" for chapter 3, verse 16, or "Hch 3-16" for chapters 3 to 16`},
	{name: "bookless continuation", tool: "lookup_passage", args: map[string]any{"reference": "Hch 22:1"},
		want: []string{`"total":1`, `"id":"arresto-de-pablo-en-jerusalen"`}},
	{name: "whole book", tool: "lookup_passage", args: map[string]any{"reference": "Rut"},
		want: []string{`"total":1`, `"text":"Rut","read_url":"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/rut/1/"`}, notWant: []string{`#sel=pasaje`}},
	{name: "nothing in a single-chapter book", tool: "lookup_passage", args: map[string]any{"reference": "Flm 5"},
		want: []string{`"total":0`, `"text":"Flm 5"`, `"notice"`}},
	{name: "unknown book", tool: "lookup_passage", args: map[string]any{"reference": "Hechs 16"}, wantErr: `unknown Bible book "Hechs". Close book names: Hechos`},
	{name: "chapter too high", tool: "lookup_passage", args: map[string]any{"reference": "Hch 29"}, wantErr: "Hechos has 28 chapters, got chapter 29"},
	{name: "verse too high", tool: "lookup_passage", args: map[string]any{"reference": "Hch 16:41"}, wantErr: "Hechos 16 has 40 verses, got verse 41"},
	{name: "two references", tool: "lookup_passage", args: map[string]any{"reference": "Hch 16:1; 17:1"}, wantErr: "reference must name one passage"},
	{name: "empty reference", tool: "lookup_passage", args: map[string]any{"reference": "  "}, wantErr: "reference is required"},

	// find_connection
	{name: "chain across stored sides", tool: "find_connection", args: map[string]any{"from": "loida", "to": "pablo"},
		want: []string{`"connected":true`, `"steps":[{"from":{"type":"person","id":"loida"`, `"phrase":"Eunice es su hija","type":"kin","word":"mother","word_of":"from"`, `"phrase":"Timoteo es su hijo"`, `"phrase":"estuvo con Pablo"`, `"shared_events_total":0`}},
	{name: "direct tie with shared events", tool: "find_connection", args: map[string]any{"from": "pablo", "to": "timoteo", "max_steps": 1.0},
		want: []string{`"connected":true`, `"phrase":"Timoteo estuvo con Pablo"`, `"reference":"Hch 16:3; 2Ti 1:2"`, `"shared_events":[{"type":"event","id":"timoteo-se-une-a-pablo"`, `"shared_events_total":1`}},
	{name: "too far", tool: "find_connection", args: map[string]any{"from": "loida", "to": "pablo", "max_steps": 2.0},
		want: []string{`"connected":false`}, notWant: []string{`"steps"`}},
	{name: "not connected", tool: "find_connection", args: map[string]any{"from": "adan", "to": "pablo"},
		want: []string{`"connected":false`}},
	{name: "same person", tool: "find_connection", args: map[string]any{"from": "pablo", "to": "pablo"}, wantErr: "two different people"},
	{name: "unknown person", tool: "find_connection", args: map[string]any{"from": "pablo", "to": "timotteo"}, wantErr: `Did you mean: timoteo (Timoteo)`},
	{name: "max steps too high", tool: "find_connection", args: map[string]any{"from": "pablo", "to": "timoteo", "max_steps": 13.0}, wantErr: "max_steps must be a whole number from 1 to 12, got 13"},

	// places_near
	{name: "near a place", tool: "places_near", args: map[string]any{"place": "jerusalen"},
		want: []string{`"total":1`, `"id":"adan"`, `"distance_km":46.`, `"origin":{"lat":31.776667,"lon":35.234167,"place":{"type":"place","id":"jerusalen"`, `"without_coordinates":1`}},
	{name: "near a point by kind", tool: "places_near", args: map[string]any{"lat": 32.8, "lon": 35.5, "radius_km": 200.0, "kind": "region"},
		want: []string{`"total":1`, `"id":"galilea"`, `"kind":"region"`}},
	{name: "place without a point", tool: "places_near", args: map[string]any{"place": "acsaf"},
		wantErr: `place "acsaf" has no known point. Candidate sites: Primer sitio de prueba (lat 32.872761, lon 35.15133)`},
	{name: "place and point", tool: "places_near", args: map[string]any{"place": "jerusalen", "lat": 1.0, "lon": 1.0}, wantErr: "not both"},
	{name: "lat without lon", tool: "places_near", args: map[string]any{"lat": 1.0}, wantErr: "lat and lon go together"},
	{name: "no origin", tool: "places_near", args: map[string]any{}, wantErr: "give place, or lat and lon"},
	{name: "zero radius", tool: "places_near", args: map[string]any{"place": "jerusalen", "radius_km": 0.0}, wantErr: "radius_km must be a number above 0 and at most 2000, got 0"},
	{name: "huge radius", tool: "places_near", args: map[string]any{"place": "jerusalen", "radius_km": 3000.0}, wantErr: "got 3000"},
	{name: "bad latitude", tool: "places_near", args: map[string]any{"lat": 91.0, "lon": 0.0}, wantErr: "lat must be a number from -90 to 90, got 91"},

	// dataset_info
	{name: "dataset info", tool: "dataset_info", args: map[string]any{},
		want:    []string{`"format":"biblical-atlas/v0","generated":"2026-09-30"`, `"stale":false`, `"people":8`, `"books_fully_read":["genesis","rut"]`, `"books_not_yet_read":["hechos","2-timoteo","filemon"]`, `"site":"https://biblical-atlas.geiser.cloud/"`, `"chapter_sources":10`},
		notWant: []string{`"refresh_error"`, `"warnings"`}},
}

func TestTools(t *testing.T) {
	for _, c := range cases {
		t.Run(c.tool+"/"+c.name, func(t *testing.T) {
			text, isErr := call(t, c.tool, c.args)
			if c.wantErr != "" {
				if !isErr || !strings.Contains(text, c.wantErr) {
					t.Fatalf("want error containing %q, got isError=%v %s", c.wantErr, isErr, text)
				}
				return
			}
			if isErr {
				t.Fatalf("unexpected error: %s", text)
			}
			for _, w := range c.want {
				if !strings.Contains(text, w) {
					t.Errorf("missing %s\nin %s", w, text)
				}
			}
			for _, w := range c.notWant {
				if strings.Contains(text, w) {
					t.Errorf("unexpected %s\nin %s", w, text)
				}
			}
			if !json.Valid([]byte(text)) {
				t.Fatalf("not JSON: %s", text)
			}
			if strings.Contains(text, "null") {
				t.Errorf("answer holds a null: %s", text)
			}
		})
	}
}

// TestEveryToolHasACase fails for a registered tool with no successful case
// or no error case (dataset_info takes no input, so it has no error case).
func TestEveryToolHasACase(t *testing.T) {
	ok, bad := map[string]bool{}, map[string]bool{}
	for _, c := range cases {
		if c.wantErr == "" {
			ok[c.tool] = true
		} else {
			bad[c.tool] = true
		}
	}
	for _, tool := range All() {
		name := tool.Def.Name
		if !ok[name] {
			t.Errorf("tool %s has no successful case", name)
		}
		if !bad[name] && name != "dataset_info" {
			t.Errorf("tool %s has no error case", name)
		}
	}
	if len(All()) != 9 {
		t.Errorf("want 9 tools, got %d", len(All()))
	}
}

func TestAnnotations(t *testing.T) {
	for _, tool := range All() {
		a := tool.Def.Annotations
		if a.Title == "" || a.ReadOnlyHint == nil || !*a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint ||
			a.IdempotentHint == nil || !*a.IdempotentHint || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("tool %s: annotations %+v", tool.Def.Name, a)
		}
		if tool.Def.Description == "" {
			t.Errorf("tool %s has no description", tool.Def.Name)
		}
	}
}

// TestLinks checks every URL any tool answer holds.
func TestLinks(t *testing.T) {
	allowed := []string{SiteBase, "https://www.jw.org/", "https://wol.jw.org/", "https://www.openbible.info/"}
	seen := 0
	for _, c := range cases {
		if c.wantErr != "" {
			continue
		}
		text, _ := call(t, c.tool, c.args)
		var v any
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			t.Fatal(err)
		}
		walkURLs(v, func(key, u string) {
			seen++
			for _, a := range allowed {
				if strings.HasPrefix(u, a) {
					return
				}
			}
			t.Errorf("%s: %s = %s is outside the allowed hosts", c.name, key, u)
		})
	}
	if seen < 100 {
		t.Fatalf("only %d URLs checked; the walk is broken", seen)
	}
}

func walkURLs(v any, f func(key, u string)) {
	switch x := v.(type) {
	case map[string]any:
		for k, y := range x {
			if s, ok := y.(string); ok && (k == "url" || strings.HasSuffix(k, "_url") || k == "site" || k == "source") && strings.HasPrefix(s, "http") {
				f(k, s)
			}
			walkURLs(y, f)
		}
	case []any:
		for _, y := range x {
			walkURLs(y, f)
		}
	}
}

// TestJWOrgLinks runs the tools on the fixture copy whose chapter sources
// link to jw.org instead of wol.jw.org: the same passages are found and
// cited, and dataset_info counts the same chapter sources.
func TestJWOrgLinks(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: "../atlas/testdata/jworg.json"})
	if _, err := store.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("test", "0", server.WithToolCapabilities(false))
	Register(s, store)
	for _, c := range []struct {
		tool string
		args map[string]any
		want []string
	}{
		{"lookup_passage", map[string]any{"reference": "Hch 16:1"}, []string{`"id":"timoteo-se-une-a-pablo"`, `"cited_by":[{"type":"person","id":"eunice"`, `"cited_by_total":4`}},
		{"lookup_passage", map[string]any{"reference": "Hch 9"}, []string{`"cited_by":[`}},
		{"dataset_info", map[string]any{}, []string{`"chapter_sources":10`}},
	} {
		text, isErr := callOn(t, s, c.tool, c.args)
		if isErr {
			t.Fatalf("%s %v: %s", c.tool, c.args, text)
		}
		for _, w := range c.want {
			if !strings.Contains(text, w) {
				t.Errorf("%s %v: want %s in %s", c.tool, c.args, w, text)
			}
		}
	}
}

func TestDataUnavailable(t *testing.T) {
	store := atlas.NewStore(atlas.Options{File: "does-not-exist.json"})
	s := server.NewMCPServer("test", "0")
	Register(s, store)
	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"dataset_info","arguments":{}}}`)
	raw, _ := json.Marshal(s.HandleMessage(context.Background(), msg))
	if !strings.Contains(string(raw), `"isError":true`) || !strings.Contains(string(raw), "does-not-exist.json") {
		t.Fatalf("got %s", raw)
	}
}

func ExampleInstructions() {
	fmt.Println(strings.Split(Instructions, "\n")[0])
	// Output: This server answers from biblical-atlas, a Spanish Bible atlas (https://biblical-atlas.geiser.cloud/).
}

// TestRelationsPage lowers the relation cap to page a person's relations.
func TestRelationsPage(t *testing.T) {
	old := maxRelations
	maxRelations = 1
	t.Cleanup(func() { maxRelations = old })
	first, isErr := call(t, "get_record", map[string]any{"type": "person", "id": "eunice"})
	if isErr || !strings.Contains(first, `"relations":[{"other":{"type":"person","id":"loida"`) ||
		!strings.Contains(first, `"relations_total":2,"next_relations_offset":1`) {
		t.Fatalf("first page: %s", first)
	}
	second, isErr := call(t, "get_record", map[string]any{"type": "person", "id": "eunice", "relations_offset": 1.0})
	if isErr || !strings.Contains(second, `"relations":[{"other":{"type":"person","id":"timoteo"`) ||
		!strings.Contains(second, `"relations_total":2`) || strings.Contains(second, "next_relations_offset") {
		t.Fatalf("second page: %s", second)
	}
}

// TestSourcesPage lowers the source cap to page a record's sources.
func TestSourcesPage(t *testing.T) {
	old := maxRecordSources
	maxRecordSources = 2
	t.Cleanup(func() { maxRecordSources = old })
	first, isErr := call(t, "get_record", map[string]any{"type": "person", "id": "pablo"})
	if isErr || !strings.Contains(first, `"sources":[{"id":"it-pablo"`) || !strings.Contains(first, `"sources_total":4,"next_sources_offset":2`) {
		t.Fatalf("first page: %s", first)
	}
	second, isErr := call(t, "get_record", map[string]any{"type": "person", "id": "pablo", "sources_offset": 2.0})
	if isErr || !strings.Contains(second, `"sources":[{"id":"hechos-22"`) || !strings.Contains(second, `"sources_total":4`) {
		t.Fatalf("second page: %s", second)
	}
	if strings.Contains(second, "next_sources_offset") {
		t.Fatalf("second page is the last: %s", second)
	}
}
