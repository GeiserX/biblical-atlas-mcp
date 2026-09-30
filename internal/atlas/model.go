package atlas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// KnownFormats lists the data formats this server reads, current id first.
// "biblical-earth/v0" is the same format under the id older atlas builds
// write. Adding a format is a one-line change here plus whatever the decoder
// needs.
var KnownFormats = []string{"biblical-atlas/v0", "biblical-earth/v0"}

// Type is a record type in the tool vocabulary.
type Type string

// The record types, in the fixed order results are sorted by.
const (
	TypePerson        Type = "person"
	TypePlace         Type = "place"
	TypeEvent         Type = "event"
	TypePeriod        Type = "period"
	TypeLetter        Type = "letter"
	TypeJourney       Type = "journey"
	TypeFind          Type = "find"
	TypeTour          Type = "tour"
	TypeBook          Type = "book"
	TypeMonth         Type = "month"
	TypeCalendarTopic Type = "calendar_topic"
)

// Types is every record type in the fixed order.
var Types = []Type{TypePerson, TypePlace, TypeEvent, TypePeriod, TypeLetter, TypeJourney, TypeFind, TypeTour, TypeBook, TypeMonth, TypeCalendarTopic}

var sitePrefix = map[Type]string{
	TypePerson: "persona", TypePlace: "lugar", TypeEvent: "evento", TypePeriod: "periodo",
	TypeLetter: "carta", TypeJourney: "viaje", TypeFind: "hallazgo", TypeTour: "recorrido", TypeBook: "libro",
}

// Prefix is the site's selection prefix for the type, or "" when the site
// has no card for it.
func (t Type) Prefix() string { return sitePrefix[t] }

// Rank is the position of the type in the fixed order.
func (t Type) Rank() int {
	for i, x := range Types {
		if x == t {
			return i
		}
	}
	return len(Types)
}

// TypeFromPrefix maps a site prefix ("persona") to a type.
func TypeFromPrefix(p string) (Type, bool) {
	for t, x := range sitePrefix {
		if x == p {
			return t, true
		}
	}
	return "", false
}

// ParseType checks a type name from tool input.
func ParseType(s string) (Type, bool) {
	for _, t := range Types {
		if string(t) == s {
			return t, true
		}
	}
	return "", false
}

// Date is the atlas's date object. Years are astronomical.
type Date struct {
	From       *int        `json:"desde"`
	To         *int        `json:"hasta"`
	Precision  string      `json:"precision"`
	Approx     bool        `json:"aprox"`
	Kind       string      `json:"tipo"`
	Chronology string      `json:"cronologia"`
	Text       string      `json:"texto"`
	Note       string      `json:"nota"`
	Detail     *DateDetail `json:"detalle"`
}

// DateDetail holds the month, day and season of a date.
type DateDetail struct {
	Month  string `json:"mes"`
	Day    int    `json:"dia"`
	Season string `json:"estacion"`
}

// Link is an entry of "enlaces".
type Link struct {
	Title string `json:"titulo"`
	URL   string `json:"url"`
	Kind  string `json:"tipo"`
}

// Alternative is another dating of the record.
type Alternative struct {
	Date    *Date    `json:"fecha"`
	Note    string   `json:"nota"`
	Sources []string `json:"fuentes"`
}

// Change is an entry of "historial".
type Change struct {
	Date   string `json:"fecha"`
	Change string `json:"cambio"`
	Source string `json:"fuente"`
}

// Common holds the fields every record kind may carry.
type Common struct {
	ID           string        `json:"id"`
	Summary      string        `json:"resumen"`
	Reason       string        `json:"razon"`
	Sources      []string      `json:"fuentes"`
	Links        []Link        `json:"enlaces"`
	Status       string        `json:"estado"`
	CheckedOn    string        `json:"consultado"`
	NotClaimed   []string      `json:"no_afirmamos"`
	Note         string        `json:"nota"`
	Alternatives []Alternative `json:"alternativas"`
	History      []Change      `json:"historial"`
	Search       string        `json:"buscar"`
}

// AltName is another name of a person, place or month.
type AltName struct {
	Name    string   `json:"nombre"`
	Note    string   `json:"nota"`
	From    *int     `json:"desde"`
	To      *int     `json:"hasta"`
	Sources []string `json:"fuentes"`
}

// Relation is a tie stored on one person, pointing at a person or a place.
// The atlas carries old Spanish fields and newer English ones; normalise
// reads the English one first.
type Relation struct {
	Type          string   `json:"type"`
	Tipo          string   `json:"tipo"`
	Word          string   `json:"word"`
	Relacion      string   `json:"relacion"`
	Status        string   `json:"status"`
	Estado        string   `json:"estado"`
	CheckedOn     string   `json:"checked_on"`
	Consultado    string   `json:"consultado"`
	Person        string   `json:"persona"`
	Place         string   `json:"lugar"`
	Family        string   `json:"family"`
	InverseFamily string   `json:"inverse_family"`
	Verb          string   `json:"verb"`
	InverseVerb   string   `json:"inverse_verb"`
	Key           string   `json:"key"`
	Reference     string   `json:"reference"`
	Inferred      bool     `json:"deducido"`
	Certainty     string   `json:"certainty"`
	Date          *Date    `json:"fecha"`
	Sources       []string `json:"fuentes"`
	Reason        string   `json:"razon"`
}

var spanishRelationType = map[string]string{
	"pariente": "kin", "vivio_en": "lived_in", "murio_en": "died_in", "nacio_en": "born_in",
	"sucede_a": "succeeds", "mismo_que": "same_as", "se_aparece_a": "appears_to", "acompana": "accompanies",
}

func (r *Relation) normalise() {
	if r.Type == "" {
		if t, ok := spanishRelationType[r.Tipo]; ok {
			r.Type = t
		} else {
			r.Type = r.Tipo
		}
	}
	if r.Word == "" {
		r.Word = r.Relacion
	}
	if r.Status == "" {
		r.Status = r.Estado
	}
	if r.CheckedOn == "" {
		r.CheckedOn = r.Consultado
	}
}

// Office is an office a person held.
type Office struct {
	Office    string   `json:"office"`
	Label     string   `json:"es"`
	Place     string   `json:"place"`
	Date      *Date    `json:"date"`
	Inferred  bool     `json:"inferred"`
	Status    string   `json:"status"`
	Sources   []string `json:"sources"`
	Reason    string   `json:"reason"`
	CheckedOn string   `json:"checked_on"`
}

// Person is an entry of "personas".
type Person struct {
	Common
	Name           string     `json:"nombre"`
	Names          []AltName  `json:"nombres"`
	Date           *Date      `json:"fecha"`
	Disambiguation string     `json:"desambiguacion"`
	NotToConfuse   []string   `json:"no_confundir_con"`
	Relations      []Relation `json:"relaciones"`
	Offices        []Office   `json:"offices"`
}

// LatLon is a point.
type LatLon struct {
	Lat *float64 `json:"lat"`
	Lon *float64 `json:"lon"`
}

// Geometry is the shape of a candidate site.
type Geometry struct {
	Kind     string   `json:"tipo"`
	Lat      *float64 `json:"lat"`
	Lon      *float64 `json:"lon"`
	RadiusKm *float64 `json:"radio_km"`
	To       *LatLon  `json:"hasta"`
}

// Candidate is a proposed site for a place.
type Candidate struct {
	Name     string    `json:"nombre"`
	Status   string    `json:"estado"`
	Geometry *Geometry `json:"geometria"`
	Reason   string    `json:"razon"`
	Note     string    `json:"nota"`
	Sources  []string  `json:"fuentes"`
}

// Place is an entry of "lugares".
type Place struct {
	Common
	Name        string      `json:"nombre"`
	Names       []AltName   `json:"nombres"`
	Kind        string      `json:"tipo"`
	Lat         *float64    `json:"lat"`
	Lon         *float64    `json:"lon"`
	Precision   string      `json:"precision"`
	CoordSource string      `json:"coord_fuente"`
	CoordURL    string      `json:"coord_url"`
	CoordNote   string      `json:"coord_nota"`
	Candidates  []Candidate `json:"candidatos"`
}

// HasPoint reports whether the place has a known point.
func (p *Place) HasPoint() bool { return p.Lat != nil && p.Lon != nil }

// Role is what one person does in an event.
type Role struct {
	Role  string `json:"role"`
	Date  *Date  `json:"date"`
	Place string `json:"place"`
}

// NarrativeOrder places an event in the order of a book's story.
type NarrativeOrder struct {
	Series string  `json:"serie"`
	Order  float64 `json:"orden"`
	After  string  `json:"tras"`
}

// Event is an entry of "eventos".
type Event struct {
	Common
	Title          string          `json:"titulo"`
	Date           *Date           `json:"fecha"`
	Places         []string        `json:"lugares"`
	People         []string        `json:"personas"`
	Present        []string        `json:"presentes"`
	Roles          map[string]Role `json:"roles"`
	Passages       []string        `json:"pasajes"`
	Kind           string          `json:"type"`
	NarrativeOrder *NarrativeOrder `json:"orden_relato"`
}

// Period is an entry of "periodos".
type Period struct {
	Common
	Name         string   `json:"nombre"`
	Kind         string   `json:"tipo"`
	Date         *Date    `json:"fecha"`
	Ruler        string   `json:"persona"`
	Office       string   `json:"office"`
	Places       []string `json:"lugares"`
	Events       []string `json:"sucesos"`
	AttestedFrom *int     `json:"consta_desde"`
}

// Recipients says who a letter was written to.
type Recipients struct {
	Text   string   `json:"texto"`
	Places []string `json:"lugares"`
	People []string `json:"personas"`
}

// Context is a letter's origin or destination context.
type Context struct {
	Summary string   `json:"resumen"`
	Sources []string `json:"fuentes"`
}

// Letter is an entry of "cartas".
type Letter struct {
	Common
	Book               string      `json:"libro"`
	Writer             string      `json:"escritor"`
	Reference          string      `json:"referencia"`
	WrittenIn          []string    `json:"escrita_en"`
	Date               *Date       `json:"fecha"`
	Recipients         *Recipients `json:"destinatarios"`
	People             []string    `json:"personas"`
	Carriers           []string    `json:"portadores"`
	OriginContext      *Context    `json:"contexto_origen"`
	DestinationContext *Context    `json:"contexto_destino"`
}

// Stop is one stop of a journey.
type Stop struct {
	Order     int      `json:"orden"`
	Place     string   `json:"lugar"`
	Reference string   `json:"referencia"`
	Date      *Date    `json:"fecha"`
	Note      string   `json:"nota"`
	Status    string   `json:"estado"`
	Sources   []string `json:"fuentes"`
	Reason    string   `json:"razon"`
}

// Journey is an entry of "viajes".
type Journey struct {
	Common
	Name       string   `json:"nombre"`
	Traveller  string   `json:"persona"`
	Companions []string `json:"companeros"`
	Reference  string   `json:"referencia"`
	Date       *Date    `json:"fecha"`
	Stops      []Stop   `json:"paradas"`
}

// Find is an entry of "hallazgos".
type Find struct {
	Common
	Name           string   `json:"nombre"`
	FoundAt        string   `json:"lugar_hallazgo"`
	RelatesTo      []string `json:"relaciona"`
	ObjectDate     *Date    `json:"fecha_objeto"`
	Identification string   `json:"identificacion"`
}

// Question is a tour stop's quiz.
type Question struct {
	Text        string   `json:"texto"`
	Options     []string `json:"opciones"`
	Answer      string   `json:"respuesta"`
	Explanation string   `json:"explicacion"`
}

// TourStop is one stop of a guided tour.
type TourStop struct {
	Sel      string    `json:"sel"`
	T        *float64  `json:"t"`
	Text     string    `json:"texto"`
	Passages []string  `json:"pasajes"`
	Question *Question `json:"pregunta"`
	NotKnown string    `json:"no_sabemos"`
}

// Tour is an entry of "recorridos".
type Tour struct {
	Common
	Title string     `json:"titulo"`
	Stops []TourStop `json:"paradas"`
}

// Book is an entry of "libros". Its id is the slug.
type Book struct {
	Common
	Slug     string           `json:"slug"`
	Num      int              `json:"num"`
	Name     string           `json:"nombre"`
	Abbr     string           `json:"abr"`
	Forms    []string         `json:"formas"`
	Spoken   []string         `json:"habladas"`
	Chapters int              `json:"capitulos"`
	Verses   []int            `json:"versiculos"`
	Omitted  map[string][]int `json:"omitidos"`
	Writer   string           `json:"escritor"`
	Place    string           `json:"lugar"`
	Date     *Date            `json:"fecha"`
	Covers   *Date            `json:"abarca"`
}

// Festival is a feast in a month.
type Festival struct {
	Name       string   `json:"nombre"`
	FromDay    *int     `json:"desde"`
	ToDay      *int     `json:"hasta"`
	Instituted *int     `json:"instituida"`
	Sources    []string `json:"fuentes"`
}

// Month is an entry of "calendario.meses".
type Month struct {
	Common
	Name       string     `json:"nombre"`
	Order      int        `json:"orden"`
	OtherNames []string   `json:"otros_nombres"`
	Names      []AltName  `json:"nombres"`
	Equivalent string     `json:"equivale"`
	Festivals  []Festival `json:"fiestas"`
	Weather    string     `json:"clima"`
	Field      string     `json:"campo"`
}

// CalendarTopic is an entry of "calendario.explicacion".
type CalendarTopic struct {
	Common
	Title string `json:"titulo"`
	Text  string `json:"texto"`
}

// Source is an entry of "fuentes".
type Source struct {
	Title string `json:"titulo"`
	Work  string `json:"obra"`
	URL   string `json:"url"`
}

// Coverage says how much of a book the atlas has read verse by verse.
type Coverage struct {
	Chapters     int `json:"capitulos"`
	ChaptersRead int `json:"completos"`
	Verses       int `json:"versiculos"`
	VersesRead   int `json:"leidos"`
}

// Complete reports whether every chapter and verse has been read.
func (c Coverage) Complete() bool {
	return c.Chapters > 0 && c.ChaptersRead >= c.Chapters && c.VersesRead >= c.Verses
}

// File is the decoded data file.
type File struct {
	Format      string
	Generated   string
	Sources     map[string]*Source
	Books       []*Book
	Months      []*Month
	Topics      []*CalendarTopic
	Places      map[string]*Place
	PlaceOrder  []string
	People      map[string]*Person
	PeopleOrder []string
	Journeys    []*Journey
	Letters     []*Letter
	Events      []*Event
	Periods     []*Period
	Finds       []*Find
	Tours       []*Tour
	Coverage    map[string]Coverage
}

// FormatError is returned for a data file in a format this server does not read.
type FormatError struct{ Got string }

func (e *FormatError) Error() string {
	return fmt.Sprintf("unsupported data format %q: this server reads %s. Upgrade biblical-atlas-mcp.", e.Got, KnownFormats[0])
}

// Decode parses and validates a data file.
func Decode(body []byte) (*File, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("data file is not a JSON object")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return nil, fmt.Errorf("data file is not valid JSON: %w", err)
	}
	f := &File{}
	if raw, ok := top["formato"]; ok {
		if err := json.Unmarshal(raw, &f.Format); err != nil {
			return nil, &FormatError{Got: string(raw)}
		}
	}
	known := false
	for _, k := range KnownFormats {
		if f.Format == k {
			known = true
		}
	}
	if !known {
		return nil, &FormatError{Got: f.Format}
	}
	for _, k := range []string{"personas", "lugares", "fuentes"} {
		if !nonEmpty(top[k], '{') {
			return nil, fmt.Errorf("data file has no %q object", k)
		}
	}
	for _, k := range []string{"eventos", "libros"} {
		if !nonEmpty(top[k], '[') {
			return nil, fmt.Errorf("data file has no %q array", k)
		}
	}
	var err error
	if raw, ok := top["generado"]; ok {
		_ = json.Unmarshal(raw, &f.Generated)
	}
	if f.Sources, _, err = decodeOrdered[Source](top["fuentes"]); err != nil {
		return nil, fmt.Errorf("fuentes: %w", err)
	}
	if f.People, f.PeopleOrder, err = decodeOrdered[Person](top["personas"]); err != nil {
		return nil, fmt.Errorf("personas: %w", err)
	}
	if f.Places, f.PlaceOrder, err = decodeOrdered[Place](top["lugares"]); err != nil {
		return nil, fmt.Errorf("lugares: %w", err)
	}
	for _, x := range []struct {
		key string
		dst any
	}{
		{"libros", &f.Books}, {"viajes", &f.Journeys}, {"cartas", &f.Letters}, {"eventos", &f.Events},
		{"periodos", &f.Periods}, {"hallazgos", &f.Finds}, {"recorridos", &f.Tours}, {"cobertura", &f.Coverage},
	} {
		raw, ok := top[x.key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, x.dst); err != nil {
			return nil, fmt.Errorf("%s: %w", x.key, err)
		}
	}
	if raw, ok := top["calendario"]; ok {
		var cal struct {
			Months []*Month         `json:"meses"`
			Topics []*CalendarTopic `json:"explicacion"`
		}
		if err := json.Unmarshal(raw, &cal); err != nil {
			return nil, fmt.Errorf("calendario: %w", err)
		}
		f.Months, f.Topics = cal.Months, cal.Topics
	}
	for id, p := range f.People {
		if p.ID == "" {
			p.ID = id
		}
		for i := range p.Relations {
			p.Relations[i].normalise()
		}
	}
	for id, p := range f.Places {
		if p.ID == "" {
			p.ID = id
		}
	}
	for _, b := range f.Books {
		if b == nil {
			continue
		}
		if b.ID == "" {
			b.ID = b.Slug
		}
	}
	f.Books = compact(f.Books)
	f.Events = compact(f.Events)
	f.Periods = compact(f.Periods)
	f.Letters = compact(f.Letters)
	f.Journeys = compact(f.Journeys)
	f.Finds = compact(f.Finds)
	f.Tours = compact(f.Tours)
	f.Months = compact(f.Months)
	f.Topics = compact(f.Topics)
	return f, nil
}

// compact drops null entries and entries without an id.
func compact[T any, P interface {
	*T
	id() string
}](in []P) []P {
	out := in[:0]
	for _, x := range in {
		if x != nil && x.id() != "" {
			out = append(out, x)
		}
	}
	return out
}

func (c *Common) id() string { return c.ID }

func nonEmpty(raw json.RawMessage, open byte) bool {
	t := bytes.TrimSpace(raw)
	if len(t) < 2 || t[0] != open {
		return false
	}
	inner := bytes.TrimSpace(t[1 : len(t)-1])
	return len(inner) > 0
}

// decodeOrdered decodes a JSON object of records and keeps the key order.
func decodeOrdered[T any](raw json.RawMessage) (map[string]*T, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, errors.New("not an object")
	}
	out := map[string]*T{}
	var order []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, _ := tok.(string)
		v := new(T)
		if err := dec.Decode(v); err != nil {
			return nil, nil, fmt.Errorf("%q: %w", key, err)
		}
		if _, seen := out[key]; !seen {
			order = append(order, key)
		}
		out[key] = v
	}
	return out, order, nil
}
