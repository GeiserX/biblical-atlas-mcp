package atlas

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Rec is the uniform view of one record of any type.
type Rec struct {
	Type           Type
	ID             string
	Name           string
	Date           *Date
	Common         *Common
	Disambiguation string
	Pos            int // position in file order within its type
	Value          any // *Person, *Place, *Event, ...

	texts         []string // normalised names searched
	summaryTokens []string
}

// RecKey names a record.
type RecKey struct {
	Type Type
	ID   string
}

// RelEdge is one relation seen from one person.
type RelEdge struct {
	Rel       *Relation
	Owner     string // the person the relation is stored on
	Other     string // the record on the far side
	OtherType Type   // person or place
	Stored    bool   // true when stored on the person the edge is listed for
}

// RoleRef is a record tied to a person or place in some capacity.
type RoleRef struct {
	Type Type
	ID   string
	As   string
}

// StopRef points at one stop of a journey.
type StopRef struct {
	Journey *Journey
	Stop    *Stop
}

// Life is what the events say about a person's birth and death.
type Life struct {
	Born, Died *Date
}

// LoadStats counts what the load had to skip.
type LoadStats struct {
	Dangling         int // references to records that do not exist
	BadCitations     int // citation fields that did not parse
	BadEventPassages int // event passages that did not parse
}

// Snapshot is one loaded data file with its indexes. It never changes after
// Build returns; a refresh builds a new one.
type Snapshot struct {
	Format    string
	Generated string
	ETag      string
	Source    string
	LoadedAt  time.Time
	File      *File
	Books     *BookTable
	Stats     LoadStats

	recs  map[Type]map[string]*Rec
	order map[Type][]*Rec

	SameName         map[string]int // normalised person name -> people with it
	RelOut           map[string][]RelEdge
	RelIn            map[string][]RelEdge
	RelByPlace       map[string][]RelEdge
	Adjacent         map[string][]RelEdge
	EventsByPerson   map[string][]int
	EventsByPlace    map[string][]int
	LettersByPlace   map[string][]RoleRef
	LettersByPerson  map[string][]RoleRef
	JourneysByPerson map[string][]RoleRef
	StopsByPlace     map[string][]StopRef
	FindsByPlace     map[string][]string
	SeatsByPlace     map[string][]string
	PeriodsByEvent   map[string][]string
	EventRanges      [][]Range
	CitedBy          map[Chapter][]RecKey
	SourceChapter    map[string]Chapter
	Lives            map[string]*Life
	FullyRead        map[string]bool
	OfficeKinds      []string
	PlaceKinds       []string
	PeriodKinds      []string
	EventKinds       []string
	RoleKinds        []string
}

// Rec returns the record of a type with an id, or nil.
func (s *Snapshot) Rec(t Type, id string) *Rec { return s.recs[t][id] }

// Has reports whether a record exists.
func (s *Snapshot) Has(t Type, id string) bool { return s.recs[t][id] != nil }

// Records lists every record of a type in file order.
func (s *Snapshot) Records(t Type) []*Rec { return s.order[t] }

// Build indexes a decoded file. It is the only place indexes are made.
func Build(f *File) *Snapshot {
	s := &Snapshot{
		Format: f.Format, Generated: f.Generated, File: f,
		recs: map[Type]map[string]*Rec{}, order: map[Type][]*Rec{},
		SameName: map[string]int{}, RelOut: map[string][]RelEdge{}, RelIn: map[string][]RelEdge{},
		RelByPlace: map[string][]RelEdge{}, Adjacent: map[string][]RelEdge{},
		EventsByPerson: map[string][]int{}, EventsByPlace: map[string][]int{},
		LettersByPlace: map[string][]RoleRef{}, LettersByPerson: map[string][]RoleRef{},
		JourneysByPerson: map[string][]RoleRef{}, StopsByPlace: map[string][]StopRef{},
		FindsByPlace: map[string][]string{}, SeatsByPlace: map[string][]string{},
		PeriodsByEvent: map[string][]string{}, CitedBy: map[Chapter][]RecKey{},
		SourceChapter: map[string]Chapter{}, Lives: map[string]*Life{}, FullyRead: map[string]bool{},
	}
	s.Books = newBookTable(f.Books)
	s.addRecords()
	s.indexRelations()
	s.indexEvents()
	s.indexOthers()
	s.indexPassages()
	for slug, c := range f.Coverage {
		if c.Complete() {
			s.FullyRead[slug] = true
		}
	}
	return s
}

func (s *Snapshot) add(r *Rec, texts ...string) {
	if s.recs[r.Type] == nil {
		s.recs[r.Type] = map[string]*Rec{}
	}
	if _, dup := s.recs[r.Type][r.ID]; dup {
		return
	}
	r.Pos = len(s.order[r.Type])
	for _, t := range texts {
		if n := Norm(t); n != "" {
			r.texts = append(r.texts, n)
		}
	}
	if r.Common != nil && r.Common.Summary != "" {
		r.summaryTokens = tokens(Norm(r.Common.Summary))
	}
	s.recs[r.Type][r.ID] = r
	s.order[r.Type] = append(s.order[r.Type], r)
}

func altNames(ns []AltName) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Name)
	}
	return out
}

func (s *Snapshot) addRecords() {
	f := s.File
	for _, id := range f.PeopleOrder {
		p := f.People[id]
		s.add(&Rec{Type: TypePerson, ID: id, Name: p.Name, Date: p.Date, Common: &p.Common, Disambiguation: p.Disambiguation, Value: p},
			append([]string{p.Name}, altNames(p.Names)...)...)
		s.SameName[Norm(p.Name)]++
	}
	for _, id := range f.PlaceOrder {
		p := f.Places[id]
		texts := append([]string{p.Name}, altNames(p.Names)...)
		for _, c := range p.Candidates {
			texts = append(texts, c.Name)
		}
		s.add(&Rec{Type: TypePlace, ID: id, Name: p.Name, Common: &p.Common, Value: p}, texts...)
	}
	for _, e := range f.Events {
		s.add(&Rec{Type: TypeEvent, ID: e.ID, Name: e.Title, Date: e.Date, Common: &e.Common, Value: e}, e.Title, e.Search)
	}
	for _, p := range f.Periods {
		s.add(&Rec{Type: TypePeriod, ID: p.ID, Name: p.Name, Date: p.Date, Common: &p.Common, Value: p}, p.Name, p.Search)
	}
	for _, l := range f.Letters {
		texts := []string{l.Book}
		if b := s.letterBook(l); b != nil {
			texts = append(append(texts, b.Abbr), b.Forms...)
		}
		s.add(&Rec{Type: TypeLetter, ID: l.ID, Name: l.Book, Date: l.Date, Common: &l.Common, Value: l}, texts...)
	}
	for _, j := range f.Journeys {
		s.add(&Rec{Type: TypeJourney, ID: j.ID, Name: j.Name, Date: j.Date, Common: &j.Common, Value: j}, j.Name)
	}
	for _, h := range f.Finds {
		s.add(&Rec{Type: TypeFind, ID: h.ID, Name: h.Name, Date: h.ObjectDate, Common: &h.Common, Value: h}, h.Name)
	}
	for _, t := range f.Tours {
		s.add(&Rec{Type: TypeTour, ID: t.ID, Name: t.Title, Common: &t.Common, Value: t}, t.Title)
	}
	for _, b := range f.Books {
		s.add(&Rec{Type: TypeBook, ID: b.ID, Name: b.Name, Date: b.Date, Common: &b.Common, Value: b},
			append([]string{b.Name, b.Abbr}, b.Forms...)...)
	}
	for _, m := range f.Months {
		texts := append(append([]string{m.Name}, m.OtherNames...), altNames(m.Names)...)
		s.add(&Rec{Type: TypeMonth, ID: m.ID, Name: m.Name, Common: &m.Common, Value: m}, texts...)
	}
	for _, t := range f.Topics {
		s.add(&Rec{Type: TypeCalendarTopic, ID: t.ID, Name: t.Title, Common: &t.Common, Value: t}, t.Title)
	}
}

// LetterBook finds the Bible book a letter is.
func (s *Snapshot) LetterBook(l *Letter) *Book { return s.letterBook(l) }

func (s *Snapshot) letterBook(l *Letter) *Book {
	for _, b := range s.File.Books {
		if b.Slug == l.ID {
			return b
		}
	}
	return s.Books.Lookup(l.Book)
}

// BookLetter finds the letter that is a book, if any.
func (s *Snapshot) BookLetter(b *Book) *Letter {
	for _, l := range s.File.Letters {
		if s.letterBook(l) == b {
			return l
		}
	}
	return nil
}

// ref checks a reference and counts it when dangling.
func (s *Snapshot) ref(t Type, id string) bool {
	if id == "" {
		return false
	}
	if s.Has(t, id) {
		return true
	}
	s.Stats.Dangling++
	return false
}

func (s *Snapshot) indexRelations() {
	f := s.File
	offices, seen := []string{}, map[string]bool{}
	for _, owner := range f.PeopleOrder {
		p := f.People[owner]
		for i := range p.Relations {
			r := &p.Relations[i]
			switch {
			case r.Person != "":
				if !s.ref(TypePerson, r.Person) {
					continue
				}
				s.RelOut[owner] = append(s.RelOut[owner], RelEdge{Rel: r, Owner: owner, Other: r.Person, OtherType: TypePerson, Stored: true})
				s.RelIn[r.Person] = append(s.RelIn[r.Person], RelEdge{Rel: r, Owner: owner, Other: owner, OtherType: TypePerson})
				s.Adjacent[owner] = append(s.Adjacent[owner], RelEdge{Rel: r, Owner: owner, Other: r.Person, OtherType: TypePerson, Stored: true})
				if r.Person != owner {
					s.Adjacent[r.Person] = append(s.Adjacent[r.Person], RelEdge{Rel: r, Owner: owner, Other: owner, OtherType: TypePerson})
				}
			case r.Place != "":
				if !s.ref(TypePlace, r.Place) {
					continue
				}
				s.RelOut[owner] = append(s.RelOut[owner], RelEdge{Rel: r, Owner: owner, Other: r.Place, OtherType: TypePlace, Stored: true})
				s.RelByPlace[r.Place] = append(s.RelByPlace[r.Place], RelEdge{Rel: r, Owner: owner, Other: owner, OtherType: TypePerson})
			}
		}
		for _, o := range p.Offices {
			if o.Office != "" && !seen[o.Office] {
				seen[o.Office] = true
				offices = append(offices, o.Office)
			}
			if o.Place != "" {
				s.ref(TypePlace, o.Place)
			}
		}
	}
	for id := range s.Adjacent {
		edges := s.Adjacent[id]
		sort.SliceStable(edges, func(i, j int) bool {
			if edges[i].Rel.Key != edges[j].Rel.Key {
				return edges[i].Rel.Key < edges[j].Rel.Key
			}
			return edges[i].Other < edges[j].Other
		})
	}
	sort.Strings(offices)
	s.OfficeKinds = offices
}

func (s *Snapshot) indexEvents() {
	kinds, roles := map[string]bool{}, map[string]bool{}
	for i, e := range s.File.Events {
		people := map[string]bool{}
		for _, lists := range [][]string{e.People, e.Present} {
			for _, p := range lists {
				if !people[p] && s.ref(TypePerson, p) {
					people[p] = true
					s.EventsByPerson[p] = append(s.EventsByPerson[p], i)
				}
			}
		}
		for p, r := range e.Roles {
			if r.Role != "" {
				roles[r.Role] = true
			}
			if !people[p] && s.ref(TypePerson, p) {
				people[p] = true
				s.EventsByPerson[p] = append(s.EventsByPerson[p], i)
			}
			if s.Has(TypePerson, p) {
				date := r.Date
				if date == nil {
					date = e.Date
				}
				life := s.Lives[p]
				if life == nil {
					life = &Life{}
					s.Lives[p] = life
				}
				switch r.Role {
				case "born":
					if life.Born == nil {
						life.Born = date
					}
				case "died":
					if life.Died == nil {
						life.Died = date
					}
				}
			}
		}
		placeSeen := map[string]bool{}
		for _, pl := range e.Places {
			if !placeSeen[pl] && s.ref(TypePlace, pl) {
				placeSeen[pl] = true
				s.EventsByPlace[pl] = append(s.EventsByPlace[pl], i)
			}
		}
		if e.Kind != "" {
			kinds[e.Kind] = true
		}
	}
	// Positions were appended in ascending order per event, but roles come
	// from a map; keep every list sorted.
	for _, m := range []map[string][]int{s.EventsByPerson, s.EventsByPlace} {
		for k := range m {
			sort.Ints(m[k])
		}
	}
	s.EventKinds = sortedKeys(kinds)
	s.RoleKinds = sortedKeys(roles)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Snapshot) indexOthers() {
	f := s.File
	placeKinds, periodKinds := map[string]bool{}, map[string]bool{}
	for _, id := range f.PlaceOrder {
		if k := f.Places[id].Kind; k != "" {
			placeKinds[k] = true
		}
	}
	for _, l := range f.Letters {
		for _, pl := range l.WrittenIn {
			if s.ref(TypePlace, pl) {
				s.LettersByPlace[pl] = appendRole(s.LettersByPlace[pl], RoleRef{TypeLetter, l.ID, "written_here"})
			}
		}
		if l.Writer != "" && s.ref(TypePerson, l.Writer) {
			s.LettersByPerson[l.Writer] = appendRole(s.LettersByPerson[l.Writer], RoleRef{TypeLetter, l.ID, "writer"})
		}
		if l.Recipients != nil {
			for _, pl := range l.Recipients.Places {
				if s.ref(TypePlace, pl) {
					s.LettersByPlace[pl] = appendRole(s.LettersByPlace[pl], RoleRef{TypeLetter, l.ID, "sent_here"})
				}
			}
			for _, p := range l.Recipients.People {
				if s.ref(TypePerson, p) {
					s.LettersByPerson[p] = appendRole(s.LettersByPerson[p], RoleRef{TypeLetter, l.ID, "recipient"})
				}
			}
		}
		for _, p := range l.Carriers {
			if s.ref(TypePerson, p) {
				s.LettersByPerson[p] = appendRole(s.LettersByPerson[p], RoleRef{TypeLetter, l.ID, "carrier"})
			}
		}
		for _, p := range l.People {
			if s.ref(TypePerson, p) {
				s.LettersByPerson[p] = appendRole(s.LettersByPerson[p], RoleRef{TypeLetter, l.ID, "named"})
			}
		}
	}
	for _, j := range f.Journeys {
		if j.Traveller != "" && s.ref(TypePerson, j.Traveller) {
			s.JourneysByPerson[j.Traveller] = appendRole(s.JourneysByPerson[j.Traveller], RoleRef{TypeJourney, j.ID, "traveller"})
		}
		for _, p := range j.Companions {
			if s.ref(TypePerson, p) {
				s.JourneysByPerson[p] = appendRole(s.JourneysByPerson[p], RoleRef{TypeJourney, j.ID, "companion"})
			}
		}
		for i := range j.Stops {
			st := &j.Stops[i]
			if s.ref(TypePlace, st.Place) {
				s.StopsByPlace[st.Place] = append(s.StopsByPlace[st.Place], StopRef{Journey: j, Stop: st})
			}
		}
	}
	for _, h := range f.Finds {
		if s.ref(TypePlace, h.FoundAt) {
			s.FindsByPlace[h.FoundAt] = append(s.FindsByPlace[h.FoundAt], h.ID)
		}
	}
	for _, p := range f.Periods {
		if p.Kind != "" {
			periodKinds[p.Kind] = true
		}
		if len(p.Places) > 0 && s.ref(TypePlace, p.Places[0]) {
			s.SeatsByPlace[p.Places[0]] = append(s.SeatsByPlace[p.Places[0]], p.ID)
		}
		for _, e := range p.Events {
			if s.ref(TypeEvent, e) {
				s.PeriodsByEvent[e] = append(s.PeriodsByEvent[e], p.ID)
			}
		}
		if p.Ruler != "" {
			s.ref(TypePerson, p.Ruler)
		}
	}
	s.PlaceKinds = sortedKeys(placeKinds)
	s.PeriodKinds = sortedKeys(periodKinds)
}

// appendRole adds a role unless the same record already holds it.
func appendRole(list []RoleRef, r RoleRef) []RoleRef {
	for _, x := range list {
		if x == r {
			return list
		}
	}
	return append(list, r)
}

func validAll(rs []Range) error {
	for _, r := range rs {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Snapshot) indexPassages() {
	f := s.File
	for id, src := range f.Sources {
		if src == nil {
			continue
		}
		if c, ok := s.Books.chapterOfURL(src.URL); ok {
			s.SourceChapter[id] = c
		}
	}
	s.EventRanges = make([][]Range, len(f.Events))
	for i, e := range f.Events {
		for _, p := range e.Passages {
			rs, err := s.Books.ParseRefs(p)
			if err == nil {
				err = validAll(rs)
			}
			if err != nil {
				s.Stats.BadEventPassages++
				continue
			}
			s.EventRanges[i] = append(s.EventRanges[i], rs...)
		}
	}
	cited := map[Chapter]map[RecKey]bool{}
	cite := func(k RecKey, c Chapter) {
		if cited[c] == nil {
			cited[c] = map[RecKey]bool{}
		}
		cited[c][k] = true
	}
	citeSources := func(k RecKey, ids []string) {
		for _, id := range ids {
			if c, ok := s.SourceChapter[id]; ok {
				cite(k, c)
			}
		}
	}
	citeText := func(k RecKey, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		rs, err := s.Books.ParseRefs(text)
		if err == nil {
			err = validAll(rs)
		}
		if err != nil {
			// A citation that does not parse, or names a chapter or verse its
			// book lacks, or ends before it starts, is counted and skipped.
			s.Stats.BadCitations++
			return
		}
		for _, r := range rs {
			for _, c := range r.Chapters() {
				cite(k, c)
			}
		}
	}
	for _, t := range Types {
		if t == TypeEvent {
			continue
		}
		for _, r := range s.order[t] {
			citeSources(RecKey{t, r.ID}, r.Common.Sources)
		}
	}
	for _, id := range f.PeopleOrder {
		p := f.People[id]
		k := RecKey{TypePerson, id}
		for _, o := range p.Offices {
			citeSources(k, o.Sources)
		}
		for _, r := range p.Relations {
			citeSources(k, r.Sources)
			citeText(k, r.Reference)
			if r.Person != "" && s.Has(TypePerson, r.Person) {
				other := RecKey{TypePerson, r.Person}
				citeText(other, r.Reference)
			}
		}
	}
	for _, l := range f.Letters {
		k := RecKey{TypeLetter, l.ID}
		citeText(k, l.Reference)
		if l.OriginContext != nil {
			citeSources(k, l.OriginContext.Sources)
		}
		if l.DestinationContext != nil {
			citeSources(k, l.DestinationContext.Sources)
		}
	}
	for _, j := range f.Journeys {
		k := RecKey{TypeJourney, j.ID}
		citeText(k, j.Reference)
		for _, st := range j.Stops {
			citeText(k, st.Reference)
			citeSources(k, st.Sources)
		}
	}
	for _, t := range f.Tours {
		k := RecKey{TypeTour, t.ID}
		for _, st := range t.Stops {
			for _, p := range st.Passages {
				citeText(k, p)
			}
		}
	}
	for c, set := range cited {
		keys := make([]RecKey, 0, len(set))
		for k := range set {
			keys = append(keys, k)
		}
		s.SortKeys(keys)
		s.CitedBy[c] = keys
	}
}

// Warnings lists what the load found wrong with the file as a whole. Today
// that is one case: the file has sources but none of them is a chapter link
// the server can read, so every chapter lookup through sources comes back
// empty. That happens when the atlas moves its links to a shape this server
// does not know yet.
func (s *Snapshot) Warnings() []string {
	if len(s.File.Sources) > 0 && len(s.SourceChapter) == 0 {
		return []string{fmt.Sprintf("none of the %d sources is a Bible chapter link this server can read, so cited_by and every chapter found through sources are empty; the data file may use a link shape this server version does not know", len(s.File.Sources))}
	}
	return nil
}

// SortKeys sorts record keys by type order, then name, then id.
func (s *Snapshot) SortKeys(keys []RecKey) {
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Type != b.Type {
			return a.Type.Rank() < b.Type.Rank()
		}
		na, nb := s.sortName(a), s.sortName(b)
		if na != nb {
			return na < nb
		}
		return a.ID < b.ID
	})
}

func (s *Snapshot) sortName(k RecKey) string {
	if r := s.Rec(k.Type, k.ID); r != nil {
		return Norm(r.Name)
	}
	return k.ID
}

// Hit is one search result.
type Hit struct {
	Rec     *Rec
	Score   int
	Matched string // "name" or "summary"
}

// Search scores every record of the wanted types against the query. An
// empty type set means every type.
func (s *Snapshot) Search(query string, types map[Type]bool) []Hit {
	nq := Norm(query)
	qWords := tokens(nq)
	var hits []Hit
	for _, t := range Types {
		if len(types) > 0 && !types[t] {
			continue
		}
		for _, r := range s.order[t] {
			best := 0
			for _, n := range r.texts {
				if sc := ScoreText(nq, n); sc > best {
					best = sc
				}
			}
			if best > 0 {
				hits = append(hits, Hit{Rec: r, Score: best, Matched: "name"})
				continue
			}
			if summaryMatches(qWords, r.summaryTokens) {
				hits = append(hits, Hit{Rec: r, Score: ScoreSummary, Matched: "summary"})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Rec.Type != b.Rec.Type {
			return a.Rec.Type.Rank() < b.Rec.Type.Rank()
		}
		na, nb := Norm(a.Rec.Name), Norm(b.Rec.Name)
		if na != nb {
			return na < nb
		}
		return a.Rec.ID < b.Rec.ID
	})
	return hits
}

// Suggest returns up to five records of a type whose id or name is close to
// an id that did not resolve.
func (s *Snapshot) Suggest(t Type, id string) []*Rec {
	id = strings.ToLower(strings.TrimSpace(id))
	q := Norm(strings.ReplaceAll(id, "-", " "))
	type cand struct {
		r     *Rec
		score int
	}
	var cands []cand
	for _, r := range s.order[t] {
		best := 0
		for _, n := range r.texts {
			best = max(best, ScoreText(q, n))
			if n != "" && strings.HasPrefix(q, n+" ") {
				best = max(best, 55)
			}
		}
		if d := levenshtein(id, r.ID); d <= 3 {
			best = max(best, 90-10*d)
		}
		if strings.HasPrefix(r.ID, id+"-") || strings.HasPrefix(id, r.ID+"-") {
			best = max(best, 70)
		}
		if best > 0 {
			cands = append(cands, cand{r, best})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].r.ID < cands[j].r.ID
	})
	var out []*Rec
	for i := 0; i < len(cands) && i < 5; i++ {
		out = append(out, cands[i].r)
	}
	return out
}
