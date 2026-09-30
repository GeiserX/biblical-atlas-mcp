package tools

import (
	"errors"
	"strings"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	maxPassageRefs = 30
	notYetRead     = "The atlas has not yet read this book verse by verse. Absence of an event here does not mean the passage has none."
)

func lookupPassageTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithString("reference", mcp.Required(), mcp.Description(`One Spanish Bible reference: "Hch 16:1", "Hechos 16", "2 Reyes 3", "Gé 2:7, 8", "Hch 13:1–14:28" or a book alone, "Rut".`), mcp.MinLength(1), mcp.MaxLength(80)),
	}
	return Tool{
		Def: newTool("lookup_passage", "Look up a Bible passage",
			`What the atlas has on a Bible passage: the events whose passages overlap it, the people and places in those events, and the letters, journey stops and other records that cite the chapter. Takes a Spanish reference such as "Hch 16:1", "Hechos 16", "2 Reyes 3", "Gé 2:7, 8", "Hch 13:1–14:28" or just a book, "Rut". Returns the link to read the chapter on wol.jw.org.`,
			append(opts, pagingOptions(20, maxLimit)...)...),
		Handle: lookupPassage,
	}
}

func lookupPassage(c *Call) (*obj, error) {
	s := c.Snap
	text, err := requiredStr(c.Args, "reference", 80)
	if err != nil {
		return nil, err
	}
	limit, offset, err := paging(c.Args, 20, maxLimit)
	if err != nil {
		return nil, err
	}
	r, err := s.Books.ParseOne(text)
	if err == nil {
		err = r.Validate()
	}
	if err != nil {
		var re *atlas.RefError
		if errors.As(err, &re) && len(re.Suggestions) > 0 {
			return nil, errors.New(re.Msg + ". Close book names: " + strings.Join(re.Suggestions, ", "))
		}
		return nil, err
	}
	b := r.Book

	var matched []int
	for i, ranges := range s.EventRanges {
		for _, er := range ranges {
			if er.Overlaps(r) {
				matched = append(matched, i)
				break
			}
		}
	}
	people, places := map[string]int{}, map[string]int{}
	for _, i := range matched {
		e := s.File.Events[i]
		for _, p := range uniq(e.People) {
			if s.Has(atlas.TypePerson, p) {
				people[p]++
			}
		}
		for _, p := range uniq(e.Places) {
			if s.Has(atlas.TypePlace, p) {
				places[p]++
			}
		}
	}
	lo, hi := window(len(matched), offset, limit)
	var results []*obj
	for _, i := range matched[lo:hi] {
		e := s.File.Events[i]
		results = append(results, itemOf(s, atlas.TypeEvent, e.ID).set("places", refs(s, atlas.TypePlace, e.Places)).set("passages", e.Passages))
	}

	passage := newObj().keep("book", ref(s, atlas.TypeBook, b.ID))
	if !r.WholeBook {
		passage.keep("chapter", r.C1)
		if r.HasVerses {
			passage.keep("verse", r.V1)
		}
		if r.C2 != r.C1 || r.HasVerses {
			passage.keep("to_chapter", r.C2)
		}
		if r.HasVerses {
			passage.keep("to_verse", r.V2)
		}
	}
	passage.keep("text", r.Text())
	if !r.WholeBook && r.C1 == r.C2 {
		passage.keep("url", passageURL(b, r.C1))
	}
	passage.keep("read_url", atlas.ChapterURL(b.Num, r.C1))

	out := page(s, len(matched), offset, limit, results).keep("passage", passage)
	fully := s.FullyRead[b.Slug]
	out.keep("fully_read", fully)
	if !fully {
		out.keep("notice", notYetRead)
	}
	addCounted(s, out, "people", atlas.TypePerson, people)
	addCounted(s, out, "places", atlas.TypePlace, places)

	seen := map[atlas.RecKey]bool{}
	var cited []atlas.RecKey
	for _, ch := range r.Chapters() {
		for _, k := range s.CitedBy[ch] {
			if !seen[k] {
				seen[k] = true
				cited = append(cited, k)
			}
		}
	}
	s.SortKeys(cited)
	var citedRefs []*obj
	for _, k := range cited {
		if len(citedRefs) == maxPassageRefs {
			break
		}
		if x := ref(s, k.Type, k.ID); x != nil {
			citedRefs = append(citedRefs, x)
		}
	}
	out.set("cited_by", citedRefs)
	if len(cited) > 0 {
		out.keep("cited_by_total", len(cited))
	}
	return out, nil
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// addCounted adds refs ordered by how many matching events name them.
func addCounted(s *atlas.Snapshot, o *obj, key string, t atlas.Type, counts map[string]int) {
	ids := sortedCounts(s, t, counts)
	var list []*obj
	for _, id := range ids {
		if len(list) == maxPassageRefs {
			break
		}
		list = append(list, ref(s, t, id))
	}
	o.set(key, list)
	if len(ids) > 0 {
		o.keep(key+"_total", len(ids))
	}
}
