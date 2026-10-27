package tools

import (
	"fmt"
	"strings"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

// maxSuggestions is how many did_you_mean records a search with no hit
// carries.
const maxSuggestions = 5

func searchTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithString("query", mcp.Required(), mcp.Description(`Name to look for, in Spanish, for example "Pablo", "Jerusalén", "Diluvio". 1 to 100 characters.`), mcp.MinLength(1), mcp.MaxLength(100)),
		mcp.WithArray("types", mcp.Description("Only these record types. Default: every type."), mcp.WithStringEnumItems(typeNames(atlas.Types)), mcp.UniqueItems(true)),
	}
	return Tool{
		Def: newTool("search", "Search the atlas",
			`Find people, places, events, periods, letters, journeys, archaeological finds, tours, Bible books and Hebrew months in the atlas by name. Use this first to get the type and id that every other tool needs. Matching ignores accents and case. Names are Spanish ("Pablo", "Jerusalén", "Diluvio"). Homonyms come back as separate results, each with a disambiguation line. A gate, tower, pool or other site inside a place finds that place, with matched "part" and the part named. A Bible book is found by its Spanish or English name ("Acts" finds Hechos), and a query that reads as a reference ("Acts 16:1") also returns it as passage, to pass to lookup_passage. With no match, did_you_mean lists records with close names.`,
			append(append(opts, pagingOptions(10, 50)...), outputSchema(pageSchema(itemSchema(schema{
				"score":           intS,
				"matched":         schema{"type": "string", "enum": []string{"name", "part", "summary", "book"}},
				"part":            object(schema{"name": strS, "label": strS, "of": refSchema, "near": boolS}, "name"),
				"same_name_count": intS,
			}), schema{
				"did_you_mean": typed("array", "Records with a name close to a query that found nothing."),
				"passage":      typed("string", "The query read as a Bible reference; pass it to lookup_passage."),
			})))...),
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
	passage := ""
	if len(types) == 0 || types[atlas.TypeBook] {
		var book *atlas.Book
		book, passage = bookInQuery(s, q)
		if r := bookRec(s, book); r != nil && !hasRec(hits, r) {
			hits = append([]atlas.Hit{{Rec: r, Score: 100, Matched: "book"}}, hits...)
		}
	}
	lo, hi := window(len(hits), offset, limit)
	var results []*obj
	for _, h := range hits[lo:hi] {
		o := item(s, h.Rec).keep("score", h.Score).keep("matched", h.Matched)
		if h.Part != nil {
			o.keep("part", partMatch(s, h))
		}
		if h.Rec.Type == atlas.TypePerson {
			if n := s.SameName[atlas.Norm(h.Rec.Name)]; n > 1 {
				o.keep("same_name_count", n)
			}
		}
		results = append(results, o)
	}
	out := page(s, len(hits), offset, limit, results).set("passage", passage)
	if len(hits) == 0 {
		out.set("did_you_mean", suggest(s, q, types))
	}
	return out, nil
}

// bookInQuery reads the query as a Bible book or reference, Spanish or
// English. A reference with a chapter also comes back as passage. A bare
// book form needs four letters, so abbreviations such as "Is" or "Ro" do
// not push a book in front of Isaac or Rode.
func bookInQuery(s *atlas.Snapshot, q string) (*atlas.Book, string) {
	if r, err := s.Books.ParseOne(q); err == nil && r.Validate() == nil && !r.WholeBook {
		return r.Book, r.Text()
	}
	if len([]rune(atlas.BookKey(q))) >= 4 {
		return s.Books.Lookup(q), ""
	}
	return nil, ""
}

func bookRec(s *atlas.Snapshot, b *atlas.Book) *atlas.Rec {
	if b == nil {
		return nil
	}
	return s.Rec(atlas.TypeBook, b.Slug)
}

func hasRec(hits []atlas.Hit, r *atlas.Rec) bool {
	for _, h := range hits {
		if h.Rec == r {
			return true
		}
	}
	return false
}

// suggest finds records whose id or name is close to a query that matched
// nothing, taking the best of each type in turn.
func suggest(s *atlas.Snapshot, q string, types map[atlas.Type]bool) []*obj {
	slug := strings.Join(strings.Fields(atlas.Norm(q)), "-")
	var perType [][]*atlas.Rec
	for _, t := range atlas.Types {
		if len(types) > 0 && !types[t] {
			continue
		}
		if recs := s.Suggest(t, slug); len(recs) > 0 {
			perType = append(perType, recs)
		}
	}
	var out []*obj
	for i := 0; len(out) < maxSuggestions; i++ {
		added := false
		for _, recs := range perType {
			if i < len(recs) && len(out) < maxSuggestions {
				out = append(out, recRef(recs[i]))
				added = true
			}
		}
		if !added {
			break
		}
	}
	return out
}

// partMatch names the part a hit came through: "Puerta de las Ovejas, en
// Jerusalén", or "junto a" for a part beside the place, as the site words it.
func partMatch(s *atlas.Snapshot, h atlas.Hit) *obj {
	parent := s.Rec(atlas.TypePlace, h.PartOf)
	where := "en"
	if h.Part.Near {
		where = "junto a"
	}
	o := newObj().keep("name", h.Part.Name)
	if parent != nil {
		o.keep("label", h.Part.Name+", "+where+" "+parent.Name).set("of", recRef(parent))
	}
	return o.flag("near", h.Part.Near)
}
