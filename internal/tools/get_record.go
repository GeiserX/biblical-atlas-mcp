package tools

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	maxRecordItems = 20
	maxPlacePeople = 30
)

// maxRelations and maxRecordSources are how many relations and sources one
// get_record answer holds; the rest come with relations_offset and
// sources_offset. Variables so a test can lower them.
var (
	maxRelations     = 30
	maxRecordSources = 50
)

func getRecordTool() Tool {
	return Tool{
		Def: newTool("get_record", "Get one atlas record",
			`Get one atlas record in full: a person with family, relations, offices and events; a place with coordinates, candidate sites and what happened there; an event with its date, people, places and Bible passages; or a period, letter, journey with its stops, find, tour, Bible book, Hebrew month or calendar topic. Needs the type and the id from search. Returns the record's sources with their links, 50 at a time; a person's relations carry their first source and a count, and come 30 at a time.`,
			mcp.WithString("type", mcp.Required(), mcp.Description("Record type, from search."), mcp.Enum(typeNames(atlas.Types)...)),
			mcp.WithString("id", mcp.Required(), mcp.Description("Record id, from search. Case is ignored.")),
			mcp.WithInteger("relations_offset", mcp.Description("Person only: relations to skip. A person answer holds at most 30 relations; use next_relations_offset from the answer to read the rest."), mcp.DefaultNumber(0), mcp.Min(0)),
			mcp.WithInteger("sources_offset", mcp.Description("Sources to skip. An answer holds at most 50 of the record's sources; use next_sources_offset from the answer to read the rest."), mcp.DefaultNumber(0), mcp.Min(0)),
		),
		Handle: getRecord,
	}
}

func getRecord(c *Call) (*obj, error) {
	t, err := recordType(c.Args, "type", atlas.Types)
	if err != nil {
		return nil, err
	}
	id, _, err := recordID(c.Snap, c.Args, "id", t, true)
	if err != nil {
		return nil, err
	}
	relOffset, hasRelOffset, err := wholeNumber(c.Args, "relations_offset", 0, 1_000_000, 0)
	if err != nil {
		return nil, err
	}
	if hasRelOffset && t != atlas.TypePerson {
		return nil, errors.New("relations_offset is only accepted for type person")
	}
	srcOffset, _, err := wholeNumber(c.Args, "sources_offset", 0, 1_000_000, 0)
	if err != nil {
		return nil, err
	}
	s := c.Snap
	r := s.Rec(t, id)
	o := recRef(r)
	if r.Common != nil {
		o.set("summary", r.Common.Summary)
	}
	switch v := r.Value.(type) {
	case *atlas.Person:
		personView(s, o, v, relOffset)
	case *atlas.Place:
		placeView(s, o, v)
	case *atlas.Event:
		eventView(s, o, v)
	case *atlas.Period:
		periodView(s, o, v)
	case *atlas.Letter:
		letterView(s, o, v)
	case *atlas.Journey:
		journeyView(s, o, v)
	case *atlas.Find:
		findView(s, o, v)
	case *atlas.Tour:
		tourView(s, o, v)
	case *atlas.Book:
		bookView(s, o, v)
	case *atlas.Month:
		monthView(s, o, v)
	case *atlas.CalendarTopic:
		o.set("text", v.Text)
	}
	commonTail(s, o, r.Common, srcOffset)
	return o.set("dataset", s.Generated), nil
}

func commonTail(s *atlas.Snapshot, o *obj, cm *atlas.Common, srcOffset int) {
	if cm == nil {
		return
	}
	o.set("status", cm.Status).set("checked_on", cm.CheckedOn).set("reason", cm.Reason).
		set("links", links(cm.Links))
	all := sources(s, cm.Sources)
	lo, hi := window(len(all), srcOffset, maxRecordSources)
	o.set("sources", all[lo:hi])
	if hi-lo < len(all) {
		o.keep("sources_total", len(all))
	}
	if hi < len(all) {
		o.keep("next_sources_offset", hi)
	}
	o.set("not_claimed", cm.NotClaimed).set("note", cm.Note)
	var alts []*obj
	for _, a := range cm.Alternatives {
		alts = append(alts, newObj().set("date", date(a.Date)).set("note", a.Note).set("sources", sources(s, a.Sources)))
	}
	o.set("alternatives", alts)
	var hist []*obj
	for _, h := range cm.History {
		hist = append(hist, newObj().set("date", h.Date).set("change", h.Change).set("source", sourceObj(s, h.Source)))
	}
	o.set("history", hist)
}

// --- person ---------------------------------------------------------------

var relationOrder = []string{"kin", "same_as", "succeeds", "disciple_of", "accompanies", "tie", "appears_to", "born_in", "lived_in", "died_in"}

func relationRank(t string) int {
	for i, x := range relationOrder {
		if x == t {
			return i
		}
	}
	return len(relationOrder)
}

// personEdges lists every relation of a person, both directions, in the
// fixed order: by type, then by the other record's name.
func personEdges(s *atlas.Snapshot, id string) []atlas.RelEdge {
	edges := append(append([]atlas.RelEdge{}, s.RelOut[id]...), s.RelIn[id]...)
	sort.SliceStable(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if ra, rb := relationRank(a.Rel.Type), relationRank(b.Rel.Type); ra != rb {
			return ra < rb
		}
		if a.Rel.Type != b.Rel.Type {
			return a.Rel.Type < b.Rel.Type
		}
		na, nb := nameOf(s, a.OtherType, a.Other), nameOf(s, b.OtherType, b.Other)
		if na != nb {
			return na < nb
		}
		return a.Rel.Key < b.Rel.Key
	})
	return edges
}

// relationObj renders a relation as seen from one side. phrase is the
// wording the atlas shows on that side's card. word is the atlas's role word
// for the person the relation is stored as pointing at, so word_of says
// which side it describes: storedSide when the edge is stored on the side
// the answer looks from, otherSide when it is stored on the other one.
// maxSources = 0 keeps every source.
func relationObj(s *atlas.Snapshot, e atlas.RelEdge, o *obj, storedSide, otherSide string, maxSources int) *obj {
	r := e.Rel
	phrase, wordOf := r.InverseVerb, otherSide
	if e.Stored {
		phrase, wordOf = r.Verb, storedSide
	}
	o.set("phrase", phrase).set("type", r.Type).set("word", r.Word)
	if r.Word != "" {
		o.set("word_of", wordOf)
	}
	o.set("date", date(r.Date)).set("reference", r.Reference).flag("inferred", r.Inferred).
		set("status", r.Status).set("certainty", r.Certainty)
	srcs := sources(s, r.Sources)
	if maxSources > 0 && len(srcs) > maxSources {
		o.set("sources", srcs[:maxSources]).keep("sources_total", len(srcs))
	} else {
		o.set("sources", srcs)
	}
	return o
}

// officeObj renders an office. maxSources = 0 keeps every source.
func officeObj(s *atlas.Snapshot, of atlas.Office, maxSources int) *obj {
	o := newObj().set("office", of.Office).set("label", of.Label)
	if of.Place != "" {
		o.set("place", ref(s, atlas.TypePlace, of.Place))
	}
	o.set("date", date(of.Date)).flag("inferred", of.Inferred).set("status", of.Status)
	srcs := sources(s, of.Sources)
	if maxSources > 0 && len(srcs) > maxSources {
		return o.set("sources", srcs[:maxSources]).keep("sources_total", len(srcs))
	}
	return o.set("sources", srcs)
}

func personView(s *atlas.Snapshot, o *obj, p *atlas.Person, relOffset int) {
	var names []*obj
	for _, n := range p.Names {
		if n.Name == p.Name && n.Note == "" {
			continue
		}
		names = append(names, newObj().set("name", n.Name).set("note", n.Note))
	}
	o.set("other_names", names).set("date", date(p.Date)).set("disambiguation", p.Disambiguation).
		set("not_to_confuse_with", refs(s, atlas.TypePerson, p.NotToConfuse))

	groups := map[string][]string{}
	for _, e := range s.RelOut[p.ID] {
		if e.OtherType == atlas.TypePerson && e.Rel.Family != "" {
			groups[e.Rel.Family] = append(groups[e.Rel.Family], e.Other)
		}
	}
	for _, e := range s.RelIn[p.ID] {
		if e.Rel.InverseFamily != "" {
			groups[e.Rel.InverseFamily] = append(groups[e.Rel.InverseFamily], e.Owner)
		}
	}
	family := newObj()
	for _, g := range []string{"parents", "children", "spouses", "siblings"} {
		family.set(g, refs(s, atlas.TypePerson, groups[g]))
	}
	o.set("family", family)

	var edges []atlas.RelEdge
	for _, e := range personEdges(s, p.ID) {
		if s.Has(e.OtherType, e.Other) {
			edges = append(edges, e)
		}
	}
	lo, hi := window(len(edges), relOffset, maxRelations)
	var rels []*obj
	for _, e := range edges[lo:hi] {
		// word describes the other record when the relation is stored here
		// and this person when it is stored on the other one.
		rels = append(rels, relationObj(s, e, newObj().keep("other", ref(s, e.OtherType, e.Other)), "other", "self", 1))
	}
	o.set("relations", rels)
	if len(edges) > len(rels) {
		o.keep("relations_total", len(edges))
	}
	if hi < len(edges) {
		o.keep("next_relations_offset", hi)
	}

	var offices []*obj
	for _, of := range p.Offices {
		offices = append(offices, officeObj(s, of, 0))
	}
	o.set("offices", offices)

	evs := s.EventsByPerson[p.ID]
	var items []*obj
	for _, i := range evs {
		if len(items) == maxRecordItems {
			break
		}
		e := s.File.Events[i]
		it := itemOf(s, atlas.TypeEvent, e.ID)
		if role, ok := e.Roles[p.ID]; ok {
			it.set("role", role.Role)
		}
		items = append(items, it)
	}
	o.set("events", items)
	if len(evs) > 0 {
		o.keep("events_total", len(evs))
	}
	o.set("letters", roleRefs(s, s.LettersByPerson[p.ID]))
	o.set("journeys", roleRefs(s, s.JourneysByPerson[p.ID]))
}

func roleRefs(s *atlas.Snapshot, rs []atlas.RoleRef) []*obj {
	var out []*obj
	for _, r := range rs {
		if x := ref(s, r.Type, r.ID); x != nil {
			out = append(out, x.keep("as", r.As))
		}
	}
	return out
}

// --- place ----------------------------------------------------------------

func placeView(s *atlas.Snapshot, o *obj, p *atlas.Place) {
	var names []*obj
	for _, n := range p.Names {
		if n.Name == p.Name && n.Note == "" && n.From == nil && n.To == nil {
			continue
		}
		names = append(names, newObj().set("name", n.Name).intp("from", histYear(n.From)).intp("to", histYear(n.To)).set("note", n.Note))
	}
	o.set("other_names", names).set("kind", p.Kind).floatp("lat", p.Lat).floatp("lon", p.Lon).set("precision", p.Precision)
	if p.CoordSource != "" || p.CoordURL != "" {
		o.set("coord_source", newObj().set("id", p.CoordSource).set("url", p.CoordURL))
	}
	o.set("coord_note", p.CoordNote)
	var cands []*obj
	for _, cd := range p.Candidates {
		x := newObj().set("name", cd.Name).set("status", cd.Status)
		if g := cd.Geometry; g != nil {
			x.set("shape", g.Kind).floatp("lat", g.Lat).floatp("lon", g.Lon).floatp("radius_km", g.RadiusKm)
			if g.To != nil && g.To.Lat != nil && g.To.Lon != nil {
				x.set("to", newObj().floatp("lat", g.To.Lat).floatp("lon", g.To.Lon))
			}
		}
		cands = append(cands, x.set("reason", cd.Reason).set("note", cd.Note).set("sources", sources(s, cd.Sources)))
	}
	o.set("candidates", cands)

	groups := map[string][]string{}
	for _, e := range s.RelByPlace[p.ID] {
		switch e.Rel.Type {
		case "born_in":
			groups["born"] = append(groups["born"], e.Owner)
		case "lived_in":
			groups["lived"] = append(groups["lived"], e.Owner)
		case "died_in":
			groups["died"] = append(groups["died"], e.Owner)
		}
	}
	people := newObj()
	for _, g := range []string{"born", "lived", "died"} {
		all := refs(s, atlas.TypePerson, groups[g])
		if len(all) > maxPlacePeople {
			people.keep(g, all[:maxPlacePeople])
		} else {
			people.set(g, all)
		}
		if len(all) > 0 {
			people.keep(g+"_total", len(all))
		}
	}
	o.set("people", people)

	evs := s.EventsByPlace[p.ID]
	var items []*obj
	for _, i := range evs {
		if len(items) == maxRecordItems {
			break
		}
		e := s.File.Events[i]
		it := itemOf(s, atlas.TypeEvent, e.ID)
		it.flag("main", len(e.Places) > 0 && e.Places[0] == p.ID)
		items = append(items, it)
	}
	o.set("events", items)
	if len(evs) > 0 {
		o.keep("events_total", len(evs))
	}
	o.set("letters", roleRefs(s, s.LettersByPlace[p.ID]))
	var stops []*obj
	for _, st := range s.StopsByPlace[p.ID] {
		stops = append(stops, newObj().set("journey", ref(s, atlas.TypeJourney, st.Journey.ID)).keep("order", st.Stop.Order).
			set("date", date(st.Stop.Date)).set("reference", st.Stop.Reference))
	}
	o.set("journey_stops", stops)
	o.set("finds", refs(s, atlas.TypeFind, s.FindsByPlace[p.ID]))
	o.set("seat_of", refs(s, atlas.TypePeriod, s.SeatsByPlace[p.ID]))
}

// --- event ----------------------------------------------------------------

func passageObjs(s *atlas.Snapshot, ps []string) []*obj {
	var out []*obj
	for _, p := range ps {
		x := newObj().set("text", p)
		if rs, err := s.Books.ParseRefs(p); err == nil && len(rs) > 0 {
			x.set("url", atlas.PassageURL(rs))
		}
		out = append(out, x)
	}
	return out
}

func eventView(s *atlas.Snapshot, o *obj, e *atlas.Event) {
	o.set("date", date(e.Date)).set("places", refs(s, atlas.TypePlace, e.Places)).
		set("people", refs(s, atlas.TypePerson, e.People)).set("present", refs(s, atlas.TypePerson, e.Present))
	var roles []*obj
	for _, pid := range roleOrder(e) {
		r := e.Roles[pid]
		person := ref(s, atlas.TypePerson, pid)
		if person == nil {
			continue
		}
		x := newObj().keep("person", person).set("role", r.Role).set("date", date(r.Date))
		if r.Place != "" {
			x.set("place", ref(s, atlas.TypePlace, r.Place))
		}
		roles = append(roles, x)
	}
	o.set("roles", roles).set("passages", passageObjs(s, e.Passages)).set("kind", e.Kind)
	if n := e.NarrativeOrder; n != nil {
		x := newObj().set("series", n.Series).keep("order", n.Order)
		if n.After != "" {
			x.set("after", ref(s, atlas.TypeEvent, n.After))
		}
		o.set("narrative_order", x)
	}
	o.set("periods", refs(s, atlas.TypePeriod, s.PeriodsByEvent[e.ID]))
}

// roleOrder lists the people with a role in the order the event names them,
// then any others by id.
func roleOrder(e *atlas.Event) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range e.People {
		if _, ok := e.Roles[p]; ok && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	var rest []string
	for p := range e.Roles {
		if !seen[p] {
			rest = append(rest, p)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// --- period, letter, journey, find, tour ------------------------------------

func periodView(s *atlas.Snapshot, o *obj, p *atlas.Period) {
	o.set("kind", p.Kind).set("date", date(p.Date))
	if p.Ruler != "" {
		o.set("ruler", ref(s, atlas.TypePerson, p.Ruler))
	}
	o.set("office", p.Office).set("places", refs(s, atlas.TypePlace, p.Places)).
		set("events", refs(s, atlas.TypeEvent, p.Events)).intp("attested_from", histYear(p.AttestedFrom))
}

func contextObj(s *atlas.Snapshot, c *atlas.Context) *obj {
	if c == nil {
		return nil
	}
	return newObj().set("summary", c.Summary).set("sources", sources(s, c.Sources))
}

func letterView(s *atlas.Snapshot, o *obj, l *atlas.Letter) {
	if b := s.LetterBook(l); b != nil {
		o.set("book", ref(s, atlas.TypeBook, b.ID))
	}
	if l.Writer != "" {
		o.set("writer", ref(s, atlas.TypePerson, l.Writer))
	}
	o.set("reference", l.Reference).set("written_in", refs(s, atlas.TypePlace, l.WrittenIn)).set("date", date(l.Date))
	if rc := l.Recipients; rc != nil {
		o.set("recipients", newObj().set("text", rc.Text).set("places", refs(s, atlas.TypePlace, rc.Places)).set("people", refs(s, atlas.TypePerson, rc.People)))
	}
	o.set("people", refs(s, atlas.TypePerson, l.People)).set("carriers", refs(s, atlas.TypePerson, l.Carriers)).
		set("origin_context", contextObj(s, l.OriginContext)).set("destination_context", contextObj(s, l.DestinationContext))
}

func journeyView(s *atlas.Snapshot, o *obj, j *atlas.Journey) {
	if j.Traveller != "" {
		o.set("traveller", ref(s, atlas.TypePerson, j.Traveller))
	}
	o.set("reference", j.Reference).set("date", date(j.Date)).set("companions", refs(s, atlas.TypePerson, j.Companions))
	var stops []*obj
	for _, st := range j.Stops {
		x := newObj().keep("order", st.Order)
		if pl := ref(s, atlas.TypePlace, st.Place); pl != nil {
			p := s.File.Places[st.Place]
			x.keep("place", pl.floatp("lat", p.Lat).floatp("lon", p.Lon))
		}
		stops = append(stops, x.set("reference", st.Reference).set("date", date(st.Date)).set("note", st.Note).
			set("status", st.Status).set("sources", sources(s, st.Sources)))
	}
	o.set("stops", stops)
}

// selRef turns a site selection ("persona:pablo", "pasaje:hch-16") into a ref.
func selRef(s *atlas.Snapshot, sel string) *obj {
	prefix, id, ok := strings.Cut(sel, ":")
	if !ok {
		return nil
	}
	if prefix == "pasaje" {
		x := newObj().keep("type", "passage").keep("id", id)
		if b, ch := chapterFromPassageID(s, id); b != nil {
			x.keep("name", b.Name+" "+strconv.Itoa(ch)).keep("url", passageURL(b, ch)).keep("read_url", atlas.ChapterURL(b, ch))
		}
		return x
	}
	t, ok := atlas.TypeFromPrefix(prefix)
	if !ok {
		return nil
	}
	return ref(s, t, id)
}

// chapterFromPassageID reads "hch-16" (abbreviation) or "hechos-16" (slug).
func chapterFromPassageID(s *atlas.Snapshot, id string) (*atlas.Book, int) {
	i := strings.LastIndex(id, "-")
	if i <= 0 {
		return nil, 0
	}
	ch, err := strconv.Atoi(id[i+1:])
	if err != nil {
		return nil, 0
	}
	key := id[:i]
	for _, b := range s.File.Books {
		if (atlas.Norm(b.Abbr) == key || b.Slug == key) && ch >= 1 && ch <= b.Chapters {
			return b, ch
		}
	}
	return nil, 0
}

func findView(s *atlas.Snapshot, o *obj, h *atlas.Find) {
	if h.FoundAt != "" {
		o.set("found_at", ref(s, atlas.TypePlace, h.FoundAt))
	}
	var rel []*obj
	for _, sel := range h.RelatesTo {
		if x := selRef(s, sel); x != nil {
			rel = append(rel, x)
		}
	}
	o.set("relates_to", rel).set("object_date", date(h.ObjectDate)).set("identification", h.Identification)
}

func tourView(s *atlas.Snapshot, o *obj, t *atlas.Tour) {
	var stops []*obj
	for _, st := range t.Stops {
		x := newObj().set("about", selRef(s, st.Sel))
		if st.T != nil {
			x.keep("year", atlas.ToHistorical(int(math.Floor(*st.T))))
		}
		x.set("text", st.Text).set("passages", st.Passages)
		if q := st.Question; q != nil {
			x.set("question", newObj().set("text", q.Text).set("options", q.Options).set("answer", q.Answer).set("explanation", q.Explanation))
		}
		stops = append(stops, x.set("not_known", st.NotKnown))
	}
	o.set("stops", stops)
}

// --- book, month ----------------------------------------------------------

func bookView(s *atlas.Snapshot, o *obj, b *atlas.Book) {
	o.keep("number", b.Num).set("abbreviation", b.Abbr).keep("chapters", b.Chapters).set("verses_per_chapter", b.Verses).
		set("writer", b.Writer).set("written_in", b.Place).set("written", date(b.Date)).set("covers", date(b.Covers))
	var omitted []*obj
	chapters := make([]int, 0, len(b.Omitted))
	for k := range b.Omitted {
		if n, err := strconv.Atoi(k); err == nil {
			chapters = append(chapters, n)
		}
	}
	sort.Ints(chapters)
	for _, ch := range chapters {
		omitted = append(omitted, newObj().keep("chapter", ch).keep("verses", b.Omitted[strconv.Itoa(ch)]))
	}
	o.set("omitted_verses", omitted)
	if cv, ok := s.File.Coverage[b.Slug]; ok {
		o.set("coverage", newObj().keep("chapters", cv.Chapters).keep("chapters_read", cv.ChaptersRead).
			keep("verses", cv.Verses).keep("verses_read", cv.VersesRead))
	}
	o.keep("fully_read", s.FullyRead[b.Slug])
	if l := s.BookLetter(b); l != nil {
		o.set("letter", ref(s, atlas.TypeLetter, l.ID))
	}
	o.set("read_url", atlas.ChapterURL(b, 1))
}

func monthView(s *atlas.Snapshot, o *obj, m *atlas.Month) {
	o.keep("order", m.Order).set("other_names", m.OtherNames)
	var names []*obj
	for _, n := range m.Names {
		names = append(names, newObj().set("name", n.Name).intp("from", histYear(n.From)).intp("to", histYear(n.To)).
			set("note", n.Note).set("sources", sources(s, n.Sources)))
	}
	o.set("names_by_era", names).set("equivalent", m.Equivalent)
	var fests []*obj
	for _, f := range m.Festivals {
		fests = append(fests, newObj().set("name", f.Name).intp("from_day", f.FromDay).intp("to_day", f.ToDay).
			intp("instituted", histYear(f.Instituted)).set("sources", sources(s, f.Sources)))
	}
	o.set("festivals", fests).set("weather", m.Weather).set("field", m.Field)
}
