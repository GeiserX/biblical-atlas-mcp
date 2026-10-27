package tools

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// Output schemas. They describe the keys every answer of a tool can carry
// and the types of the ones a client is likely to read; keys not listed may
// appear (additional properties stay allowed) because a record carries
// whatever the atlas has for it. TestTools checks every case of the tool
// table against them (the test server adds output validation), and
// TestToolMetadata holds the positive control.

type schema = map[string]any

func object(props schema, required ...string) schema {
	o := schema{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

func arrayOf(items schema) schema { return schema{"type": "array", "items": items} }

func typed(t, description string) schema {
	s := schema{"type": t}
	if description != "" {
		s["description"] = description
	}
	return s
}

var (
	anyObject = schema{"type": "object"}
	strS      = typed("string", "")
	intS      = typed("integer", "")
	numS      = typed("number", "")
	boolS     = typed("boolean", "")

	// refSchema points at one record.
	refSchema = object(schema{
		"type": typed("string", "Record type."),
		"id":   typed("string", "Record id; pass it with type to get_record."),
		"name": typed("string", "Name as the atlas gives it, in Spanish."),
		"url":  typed("string", "The record's card on the atlas site."),
	}, "type", "id")

	sourceSchema = object(schema{
		"id": strS, "title": strS, "work": strS, "url": strS, "kind": strS,
	})

	dateSchema = object(schema{
		"text": typed("string", "The atlas's own rendering."),
		"from": typed("integer", "Signed historical year: -607 is 607 a.e.c."),
		"to":   typed("integer", "Signed historical year."),
		"kind": typed("string", `"narrativa" means placed only by the order of the story.`),
	})

	// itemProps are the keys of a record in a list.
	itemProps = schema{
		"type": strS, "id": strS, "name": strS, "url": strS,
		"date": dateSchema, "summary": strS, "disambiguation": strS, "status": strS,
		"sources":       arrayOf(sourceSchema),
		"sources_total": intS,
	}
)

func withProps(base schema, more schema) schema {
	out := schema{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range more {
		out[k] = v
	}
	return out
}

// item is a list entry, with the keys a tool adds to it.
func itemSchema(more schema) schema {
	return object(withProps(itemProps, more), "type", "id")
}

// pageSchema is the paging envelope around one page of results.
func pageSchema(item schema, more schema, required ...string) schema {
	props := withProps(schema{
		"total":       typed("integer", "Results in all pages."),
		"offset":      intS,
		"limit":       intS,
		"next_offset": typed("integer", "Offset of the next page; absent on the last page."),
		"results":     arrayOf(item),
		"dataset":     typed("string", "Build date of the data that answered."),
	}, more)
	return object(props, append([]string{"total", "offset", "limit", "results"}, required...)...)
}

// recordSchema is one record in full, as get_record returns it.
var recordSchema = object(withProps(itemProps, schema{
	"links":               arrayOf(sourceSchema),
	"dataset":             strS,
	"sources_total":       typed("integer", "All the record's sources, when they fill more than one answer."),
	"next_sources_offset": typed("integer", "Pass as sources_offset to read the next sources."),
}), "type", "id")

func outputSchema(s schema) mcp.ToolOption {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return mcp.WithRawOutputSchema(b)
}

// exclusiveMin is a JSON Schema exclusiveMinimum, which mcp-go has no
// option for.
func exclusiveMin(v float64) mcp.PropertyOption {
	return func(p map[string]any) { p["exclusiveMinimum"] = v }
}
