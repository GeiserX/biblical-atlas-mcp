package tools

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

func listEventsTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithInteger("from_year", mcp.Description("Signed historical year: -607 is 607 a.e.c. (BCE), 33 is 33 e.c. (CE). No year 0."), mcp.Min(MinYear), mcp.Max(MaxYear)),
		mcp.WithInteger("to_year", mcp.Description("End of the range, same convention. Defaults to from_year. Needs from_year."), mcp.Min(MinYear), mcp.Max(MaxYear)),
		mcp.WithString("person", mcp.Description("Person id from search.")),
		mcp.WithString("place", mcp.Description("Place id from search.")),
		mcp.WithString("role", mcp.Description(`The person's role in the event, for example "born", "died", "spoke". Needs person.`)),
		mcp.WithString("kind", mcp.Description(`Kind of event, for example "speech", "death", "birth", "writing".`)),
		mcp.WithString("book", mcp.Description(`Bible book: name, abbreviation or slug ("Rut", "Hch", "1-samuel"). Matches the events of the book's own story, marked in_story: true (listed first, in story order, when no year is given), and also the events that only cite a passage of the book.`)),
		mcp.WithBoolean("anchored_only", mcp.Description(`Leave out events placed only by the order of the story (date kind "narrativa").`), mcp.DefaultBool(false)),
	}
	return Tool{
		Def: newTool("list_events", "List events",
			`List events in date order, filtered by any mix of year or year range, person, place, kind of event and Bible book. Answers "what happened in 607 a.e.c.", "what happened at Jerusalén", "what did Pablo take part in" and "which events does the book of Rut tell or cite" (in_story marks the ones its own story tells). Years are signed: -607 is 607 a.e.c., 33 is 33 e.c. At least one filter is required.`,
			append(opts, pagingOptions(20, maxLimit)...)...),
		Handle: listEvents,
	}
}

func listEvents(c *Call) (*obj, error) {
	s := c.Snap
	from, hasFrom, err := year(c.Args, "from_year")
	if err != nil {
		return nil, err
	}
	to, hasTo, err := year(c.Args, "to_year")
	if err != nil {
		return nil, err
	}
	if hasTo && !hasFrom {
		return nil, errors.New("to_year needs from_year")
	}
	if !hasTo {
		to = from
	}
	if hasFrom && to < from {
		return nil, fmt.Errorf("to_year must not be before from_year, got %d to %d", from, to)
	}
	person, hasPerson, err := recordID(s, c.Args, "person", atlas.TypePerson, false)
	if err != nil {
		return nil, err
	}
	place, hasPlace, err := recordID(s, c.Args, "place", atlas.TypePlace, false)
	if err != nil {
		return nil, err
	}
	role, hasRole, err := str(c.Args, "role", 40)
	if err != nil {
		return nil, err
	}
	if hasRole {
		if !hasPerson {
			return nil, errors.New("role needs person")
		}
		if err := oneOf("role", role, s.RoleKinds); err != nil {
			return nil, err
		}
	}
	kind, hasKind, err := str(c.Args, "kind", 40)
	if err != nil {
		return nil, err
	}
	if hasKind {
		if err := oneOf("kind", kind, s.EventKinds); err != nil {
			return nil, err
		}
	}
	bookArg, hasBook, err := str(c.Args, "book", 60)
	if err != nil {
		return nil, err
	}
	var book *atlas.Book
	if hasBook {
		if book = s.Books.Lookup(bookArg); book == nil {
			msg := fmt.Sprintf("book %q is not a Bible book the atlas knows", bookArg)
			if sugg := s.Books.Suggest(bookArg); len(sugg) > 0 {
				msg += ". Close names: " + strings.Join(sugg, ", ")
			}
			return nil, errors.New(msg)
		}
	}
	anchored, err := boolean(c.Args, "anchored_only", false)
	if err != nil {
		return nil, err
	}
	if !hasFrom && !hasPerson && !hasPlace && !hasKind && !hasBook {
		return nil, errors.New("give at least one filter: from_year, person, place, kind or book")
	}
	limit, offset, err := paging(c.Args, 20, maxLimit)
	if err != nil {
		return nil, err
	}

	a, b := atlas.ToAstronomical(from), atlas.ToAstronomical(to)
	var candidates []int
	switch {
	case hasPerson:
		candidates = s.EventsByPerson[person]
	case hasPlace:
		candidates = s.EventsByPlace[place]
	default:
		candidates = make([]int, len(s.File.Events))
		for i := range candidates {
			candidates[i] = i
		}
	}
	var matched []int
	for _, i := range candidates {
		e := s.File.Events[i]
		if hasFrom && !e.Date.Covers(a, b) {
			continue
		}
		if hasPlace && !contains(e.Places, place) {
			continue
		}
		if hasRole && e.Roles[person].Role != role {
			continue
		}
		if hasKind && e.Kind != kind {
			continue
		}
		if anchored && e.Date != nil && e.Date.Kind == "narrativa" {
			continue
		}
		if book != nil && !inBook(s, i, book) {
			continue
		}
		matched = append(matched, i)
	}
	if book != nil && !hasFrom {
		sortByStory(s, matched, book)
	}
	lo, hi := window(len(matched), offset, limit)
	var results []*obj
	for _, i := range matched[lo:hi] {
		e := s.File.Events[i]
		o := itemOf(s, atlas.TypeEvent, e.ID).set("places", refs(s, atlas.TypePlace, e.Places))
		if n := len(uniq(e.People)); n > 0 {
			o.keep("people_total", n)
		}
		o.set("passages", e.Passages).set("kind", e.Kind)
		if book != nil {
			o.flag("in_story", inStory(e, book))
		}
		if hasPerson {
			o.set("role", e.Roles[person].Role).flag("present", contains(e.Present, person))
		}
		if hasPlace {
			o.flag("main", len(e.Places) > 0 && e.Places[0] == place)
		}
		results = append(results, o)
	}
	return page(s, len(matched), offset, limit, results), nil
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// inStory reports whether the event belongs to the book's narrative series.
// A series named "<slug>-<more>" (for example "jueces-apendice") counts.
func inStory(e *atlas.Event, b *atlas.Book) bool {
	n := e.NarrativeOrder
	return n != nil && (n.Series == b.Slug || strings.HasPrefix(n.Series, b.Slug+"-"))
}

// inBook reports whether the event is part of the book's story or cites a
// passage of the book.
func inBook(s *atlas.Snapshot, i int, b *atlas.Book) bool {
	if inStory(s.File.Events[i], b) {
		return true
	}
	for _, r := range s.EventRanges[i] {
		if r.Book != nil && r.Book.Num == b.Num {
			return true
		}
	}
	return false
}

// sortByStory puts the book's own series first in story order, then the
// events that only cite the book, in file order.
func sortByStory(s *atlas.Snapshot, idx []int, b *atlas.Book) {
	sort.SliceStable(idx, func(x, y int) bool {
		ex, ey := s.File.Events[idx[x]], s.File.Events[idx[y]]
		sx, sy := inStory(ex, b), inStory(ey, b)
		if sx != sy {
			return sx
		}
		if sx && ex.NarrativeOrder.Series != ey.NarrativeOrder.Series {
			return ex.NarrativeOrder.Series == b.Slug
		}
		if sx && ex.NarrativeOrder.Order != ey.NarrativeOrder.Order {
			return ex.NarrativeOrder.Order < ey.NarrativeOrder.Order
		}
		return idx[x] < idx[y]
	})
}
