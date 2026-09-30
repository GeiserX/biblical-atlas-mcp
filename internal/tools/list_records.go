package tools

import (
	"fmt"
	"sort"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

var listableTypes = []atlas.Type{
	atlas.TypePerson, atlas.TypePlace, atlas.TypePeriod, atlas.TypeLetter, atlas.TypeJourney,
	atlas.TypeFind, atlas.TypeTour, atlas.TypeBook, atlas.TypeMonth, atlas.TypeCalendarTopic,
}

func listRecordsTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithString("type", mcp.Required(), mcp.Description("What to list. Use list_events for events."), mcp.Enum(typeNames(listableTypes)...)),
		mcp.WithString("kind", mcp.Description(`Required for person (an office, for example "high_priest" or "king") and place (a place kind, for example "rio" or "ciudad"). Optional for period (for example "rey", "potencia", "era"). An unknown value returns the values present.`)),
	}
	return Tool{
		Def: newTool("list_records", "List atlas records",
			`List the atlas's small collections in full: periods (reigns, empires, eras, high priests, governors), letters, journeys, archaeological finds, guided tours, Bible books, Hebrew months and calendar topics. Also lists places of one kind (for example every river) and people who held one office (for example every high priest). Use search when you have a name, and list_events for events.`,
			append(opts, pagingOptions(20, maxLimit)...)...),
		Handle: listRecords,
	}
}

func listRecords(c *Call) (*obj, error) {
	s := c.Snap
	t, err := recordType(c.Args, "type", listableTypes)
	if err != nil {
		return nil, err
	}
	kind, hasKind, err := str(c.Args, "kind", 60)
	if err != nil {
		return nil, err
	}
	switch t {
	case atlas.TypePerson, atlas.TypePlace:
		if !hasKind {
			return nil, fmt.Errorf("kind is required for type %s; values present: %s", t, joinList(kindsFor(s, t)))
		}
	case atlas.TypePeriod:
	default:
		if hasKind {
			return nil, fmt.Errorf("kind is only accepted for person, place and period, not for %s", t)
		}
	}
	if hasKind {
		if err := oneOf("kind", kind, kindsFor(s, t)); err != nil {
			return nil, err
		}
	}
	limit, offset, err := paging(c.Args, 20, maxLimit)
	if err != nil {
		return nil, err
	}

	var results []*obj
	var total int
	emit := func(n int, build func(i int) *obj) {
		total = n
		lo, hi := window(n, offset, limit)
		for i := lo; i < hi; i++ {
			results = append(results, build(i))
		}
	}
	switch t {
	case atlas.TypePerson:
		type holder struct {
			rec    *atlas.Rec
			office atlas.Office
			start  int
			dated  bool
		}
		var hs []holder
		for _, r := range s.Records(atlas.TypePerson) {
			p := r.Value.(*atlas.Person)
			found := false
			var best holder
			for _, of := range p.Offices {
				if of.Office != kind {
					continue
				}
				start, ok := of.Date.Start()
				if !found || (ok && (!best.dated || start < best.start)) {
					best = holder{rec: r, office: of, start: start, dated: ok}
					found = true
				}
			}
			if found {
				hs = append(hs, best)
			}
		}
		sort.SliceStable(hs, func(i, j int) bool {
			a, b := hs[i], hs[j]
			if a.dated != b.dated {
				return a.dated
			}
			if a.start != b.start {
				return a.start < b.start
			}
			if na, nb := atlas.Norm(a.rec.Name), atlas.Norm(b.rec.Name); na != nb {
				return na < nb
			}
			return a.rec.ID < b.rec.ID
		})
		emit(len(hs), func(i int) *obj { return item(s, hs[i].rec).set("office", officeObj(s, hs[i].office, maxItemSources)) })
	case atlas.TypePlace:
		var rs []*atlas.Rec
		for _, r := range s.Records(atlas.TypePlace) {
			if r.Value.(*atlas.Place).Kind == kind {
				rs = append(rs, r)
			}
		}
		emit(len(rs), func(i int) *obj {
			p := rs[i].Value.(*atlas.Place)
			return item(s, rs[i]).set("kind", p.Kind).floatp("lat", p.Lat).floatp("lon", p.Lon)
		})
	case atlas.TypePeriod:
		var rs []*atlas.Rec
		for _, r := range s.Records(atlas.TypePeriod) {
			if !hasKind || r.Value.(*atlas.Period).Kind == kind {
				rs = append(rs, r)
			}
		}
		emit(len(rs), func(i int) *obj {
			p := rs[i].Value.(*atlas.Period)
			o := item(s, rs[i]).set("kind", p.Kind)
			if p.Ruler != "" {
				o.set("ruler", ref(s, atlas.TypePerson, p.Ruler))
			}
			return o
		})
	case atlas.TypeBook:
		rs := s.Records(atlas.TypeBook)
		emit(len(rs), func(i int) *obj {
			b := rs[i].Value.(*atlas.Book)
			return item(s, rs[i]).keep("number", b.Num).set("abbreviation", b.Abbr).keep("chapters", b.Chapters).keep("fully_read", s.FullyRead[b.Slug])
		})
	default:
		rs := s.Records(t)
		emit(len(rs), func(i int) *obj { return item(s, rs[i]) })
	}
	return page(s, total, offset, limit, results), nil
}

func kindsFor(s *atlas.Snapshot, t atlas.Type) []string {
	switch t {
	case atlas.TypePerson:
		return s.OfficeKinds
	case atlas.TypePlace:
		return s.PlaceKinds
	case atlas.TypePeriod:
		return s.PeriodKinds
	}
	return nil
}

func joinList(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}
