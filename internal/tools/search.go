package tools

import (
	"fmt"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

func searchTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithString("query", mcp.Required(), mcp.Description(`Name to look for, in Spanish, for example "Pablo", "Jerusalén", "Diluvio". 1 to 100 characters.`), mcp.MinLength(1), mcp.MaxLength(100)),
		mcp.WithArray("types", mcp.Description("Only these record types. Default: every type."), mcp.WithStringEnumItems(typeNames(atlas.Types))),
	}
	return Tool{
		Def: newTool("search", "Search the atlas",
			`Find people, places, events, periods, letters, journeys, archaeological finds, tours, Bible books and Hebrew months in the atlas by name. Use this first to get the type and id that every other tool needs. Matching ignores accents and case. Names are Spanish ("Pablo", "Jerusalén", "Diluvio"). Homonyms come back as separate results, each with a disambiguation line.`,
			append(opts, pagingOptions(10, 50)...)...),
		Handle: search,
	}
}

func search(c *Call) (*obj, error) {
	q, err := requiredStr(c.Args, "query", 100)
	if err != nil {
		return nil, err
	}
	types := map[atlas.Type]bool{}
	if raw, ok := c.Args["types"]; ok && raw != nil {
		list, isList := raw.([]any)
		if !isList {
			return nil, fmt.Errorf("types must be a list of type names, got %s", fmtNum(raw))
		}
		for _, x := range list {
			name, _ := x.(string)
			t, ok := atlas.ParseType(name)
			if !ok {
				return nil, fmt.Errorf("types must contain only %s, got %s", joinTypes(atlas.Types), fmtNum(x))
			}
			types[t] = true
		}
	}
	limit, offset, err := paging(c.Args, 10, 50)
	if err != nil {
		return nil, err
	}
	s := c.Snap
	hits := s.Search(q, types)
	lo, hi := window(len(hits), offset, limit)
	var results []*obj
	for _, h := range hits[lo:hi] {
		o := item(s, h.Rec).keep("score", h.Score).keep("matched", h.Matched)
		if h.Rec.Type == atlas.TypePerson {
			if n := s.SameName[atlas.Norm(h.Rec.Name)]; n > 1 {
				o.keep("same_name_count", n)
			}
		}
		results = append(results, o)
	}
	return page(s, len(hits), offset, limit, results), nil
}
