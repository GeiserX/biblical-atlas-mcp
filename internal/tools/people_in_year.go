package tools

import (
	"errors"
	"sort"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

func peopleInYearTool() Tool {
	opts := []mcp.ToolOption{
		mcp.WithInteger("year", mcp.Required(), mcp.Description("Signed historical year: -607 is 607 a.e.c. (BCE), 33 is 33 e.c. (CE). No year 0."), mcp.Min(MinYear), mcp.Max(MaxYear)),
	}
	return Tool{
		Def: newTool("people_in_year", "People alive or active in a year",
			`Who was alive, ruling or active in a given year, and which reigns, empires and eras covered it. Each person comes with the basis of the claim: "birth_and_death" (dated birth and death events span the year), "dated" (the atlas's date for the person covers the year; that date is the person's known activity, which may be the whole life, a reign or a ministry), "office" (an office held that year) or "event" (an event dated to that year). Only "birth_and_death" bounds the life; the others show the person was alive and active that year. Years are signed: -607 is 607 a.e.c.`,
			append(opts, pagingOptions(20, maxLimit)...)...),
		Handle: peopleInYear,
	}
}

var basisRank = map[string]int{"birth_and_death": 0, "dated": 1, "office": 2, "event": 3}

func peopleInYear(c *Call) (*obj, error) {
	s := c.Snap
	y, ok, err := year(c.Args, "year")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("year is required (signed: -607 is 607 a.e.c.)")
	}
	limit, offset, err := paging(c.Args, 20, maxLimit)
	if err != nil {
		return nil, err
	}
	a := atlas.ToAstronomical(y)
	type hit struct {
		rec      *atlas.Rec
		basis    string
		evidence any
	}
	var hits []hit
	for _, r := range s.Records(atlas.TypePerson) {
		p := r.Value.(*atlas.Person)
		if life := s.Lives[p.ID]; life != nil && life.Born != nil && life.Died != nil {
			b, okB := life.Born.Start()
			d, okD := life.Died.End()
			if okB && okD && b <= a && a <= d {
				hits = append(hits, hit{r, "birth_and_death", newObj().set("born", date(life.Born)).set("died", date(life.Died))})
				continue
			}
		}
		if p.Date.Covers(a, a) {
			hits = append(hits, hit{r, "dated", date(p.Date)})
			continue
		}
		var office *atlas.Office
		for i := range p.Offices {
			if p.Offices[i].Date.Covers(a, a) {
				office = &p.Offices[i]
				break
			}
		}
		if office != nil {
			hits = append(hits, hit{r, "office", officeObj(s, *office, maxItemSources)})
			continue
		}
		for _, i := range s.EventsByPerson[p.ID] {
			e := s.File.Events[i]
			if e.Date != nil && e.Date.Kind != "narrativa" && e.Date.Covers(a, a) {
				hits = append(hits, hit{r, "event", ref(s, atlas.TypeEvent, e.ID).set("date", date(e.Date))})
				break
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		x, z := hits[i], hits[j]
		if basisRank[x.basis] != basisRank[z.basis] {
			return basisRank[x.basis] < basisRank[z.basis]
		}
		if nx, nz := atlas.Norm(x.rec.Name), atlas.Norm(z.rec.Name); nx != nz {
			return nx < nz
		}
		return x.rec.ID < z.rec.ID
	})
	lo, hi := window(len(hits), offset, limit)
	var results []*obj
	for _, h := range hits[lo:hi] {
		results = append(results, item(s, h.rec).keep("basis", h.basis).keep("evidence", h.evidence))
	}
	var periods []*obj
	for _, r := range s.Records(atlas.TypePeriod) {
		if r.Date.Covers(a, a) {
			periods = append(periods, item(s, r))
		}
	}
	return page(s, len(hits), offset, limit, results).keep("year", y).set("periods", periods), nil
}
