package tools

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
)

// SiteBase is the atlas site. Card links always point here, whatever data
// URL the server reads.
const SiteBase = "https://biblical-atlas.geiser.cloud/"

// obj is a JSON object that keeps key order and leaves out empty values.
type obj struct {
	keys []string
	vals []any
}

func newObj() *obj { return &obj{} }

// set adds a key unless the value is empty: nil, "", an empty list or map,
// a nil pointer or an empty object. Numbers and booleans are always kept.
func (o *obj) set(k string, v any) *obj {
	if isEmpty(v) {
		return o
	}
	return o.keep(k, v)
}

// keep adds a key whatever its value.
func (o *obj) keep(k string, v any) *obj {
	for i, x := range o.keys {
		if x == k {
			o.vals[i] = v
			return o
		}
	}
	o.keys = append(o.keys, k)
	o.vals = append(o.vals, v)
	return o
}

// flag adds a boolean key only when it is true.
func (o *obj) flag(k string, b bool) *obj {
	if b {
		o.keep(k, true)
	}
	return o
}

// intp adds a key for a non-nil int pointer.
func (o *obj) intp(k string, p *int) *obj {
	if p != nil {
		o.keep(k, *p)
	}
	return o
}

// floatp adds a key for a non-nil float pointer.
func (o *obj) floatp(k string, p *float64) *obj {
	if p != nil {
		o.keep(k, *p)
	}
	return o
}

func (o *obj) empty() bool { return o == nil || len(o.keys) == 0 }

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case *obj:
		return x.empty()
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		return rv.IsNil()
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0
	}
	return false
}

// MarshalJSON writes the keys in insertion order.
func (o *obj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		vb, err := marshal(o.vals[i])
		if err != nil {
			return nil, err
		}
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// marshal encodes compact JSON without HTML escaping, so «», & and < stay
// readable.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// cardURL is the site's stable address for a record, or "" for a type the
// site has no card for.
func cardURL(t atlas.Type, id string) string {
	p := t.Prefix()
	if p == "" {
		return ""
	}
	return SiteBase + "#sel=" + p + ":" + id
}

// passageURL is the site's card for one chapter.
func passageURL(b *atlas.Book, chapter int) string {
	return SiteBase + "#sel=pasaje:" + passageID(b, chapter)
}

func passageID(b *atlas.Book, chapter int) string {
	return atlas.Norm(b.Abbr) + "-" + itoa(chapter)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// ref points at a record. It is nil when the record does not exist, so a
// dangling reference drops out of the output.
func ref(s *atlas.Snapshot, t atlas.Type, id string) *obj {
	r := s.Rec(t, id)
	if r == nil {
		return nil
	}
	return recRef(r)
}

func recRef(r *atlas.Rec) *obj {
	return newObj().keep("type", string(r.Type)).keep("id", r.ID).set("name", r.Name).set("url", cardURL(r.Type, r.ID))
}

// refs maps ids to refs, dropping the ones that do not exist.
func refs(s *atlas.Snapshot, t atlas.Type, ids []string) []*obj {
	var out []*obj
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if r := ref(s, t, id); r != nil {
			out = append(out, r)
		}
	}
	return out
}

func histYear(p *int) *int {
	if p == nil {
		return nil
	}
	h := atlas.ToHistorical(*p)
	return &h
}

// date renders an atlas date with historical years.
func date(d *atlas.Date) *obj {
	if d == nil {
		return nil
	}
	o := newObj().set("text", d.Text).intp("from", histYear(d.From)).intp("to", histYear(d.To)).
		flag("approx", d.Approx).set("kind", d.Kind).set("precision", d.Precision)
	if d.Detail != nil {
		o.set("month", d.Detail.Month)
		if d.Detail.Day != 0 {
			o.keep("day", d.Detail.Day)
		}
		o.set("season", d.Detail.Season)
	}
	o.set("note", d.Note)
	if d.Chronology != "" && d.Chronology != "tnm" {
		o.keep("chronology", d.Chronology)
	}
	if o.empty() {
		return nil
	}
	return o
}

func sourceObj(s *atlas.Snapshot, id string) *obj {
	src := s.File.Sources[id]
	if src == nil {
		return nil
	}
	return newObj().keep("id", id).set("title", src.Title).set("work", src.Work).set("url", src.URL)
}

// sources resolves source ids. Ids with the same URL collapse into the
// first; ids missing from "fuentes" are skipped.
func sources(s *atlas.Snapshot, ids []string) []*obj {
	var out []*obj
	seen := map[string]bool{}
	for _, id := range ids {
		o := sourceObj(s, id)
		if o == nil {
			continue
		}
		u := s.File.Sources[id].URL
		key := u
		if key == "" {
			key = "id:" + id
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, o)
	}
	return out
}

func links(ls []atlas.Link) []*obj {
	var out []*obj
	for _, l := range ls {
		out = append(out, newObj().set("title", l.Title).set("url", l.URL).set("kind", l.Kind))
	}
	return out
}

// maxItemSources is how many sources a list item carries; the full record
// has them all.
const maxItemSources = 1

// itemSources lists a record's links first, then its sources, without
// repeating a URL. It returns the first maxItemSources and the full count.
func itemSources(s *atlas.Snapshot, c *atlas.Common) ([]*obj, int) {
	if c == nil {
		return nil, 0
	}
	var all []*obj
	seen := map[string]bool{}
	for _, l := range c.Links {
		if l.URL == "" || seen[l.URL] {
			continue
		}
		seen[l.URL] = true
		all = append(all, newObj().set("title", l.Title).set("url", l.URL).set("kind", l.Kind))
	}
	for _, id := range c.Sources {
		src := s.File.Sources[id]
		if src == nil || (src.URL != "" && seen[src.URL]) {
			continue
		}
		if src.URL != "" {
			seen[src.URL] = true
		}
		all = append(all, sourceObj(s, id))
	}
	if len(all) > maxItemSources {
		return all[:maxItemSources], len(all)
	}
	return all, len(all)
}

// item is the short form of a record used in lists.
func item(s *atlas.Snapshot, r *atlas.Rec) *obj {
	o := recRef(r).set("date", date(r.Date))
	if r.Common != nil {
		o.set("summary", r.Common.Summary)
	}
	o.set("disambiguation", r.Disambiguation)
	if r.Common != nil {
		o.set("status", r.Common.Status)
	}
	srcs, total := itemSources(s, r.Common)
	o.set("sources", srcs)
	if total > 0 {
		o.keep("sources_total", total)
	}
	return o
}

func itemOf(s *atlas.Snapshot, t atlas.Type, id string) *obj {
	r := s.Rec(t, id)
	if r == nil {
		return nil
	}
	return item(s, r)
}

// page builds the paging object around one page of results.
func page(s *atlas.Snapshot, total, offset, limit int, results []*obj) *obj {
	if results == nil {
		results = []*obj{}
	}
	o := newObj().keep("total", total).keep("offset", offset).keep("limit", limit)
	if offset+limit < total {
		o.keep("next_offset", offset+limit)
	}
	o.keep("results", results)
	o.set("dataset", s.Generated)
	return o
}

// window returns the [offset, offset+limit) slice bounds for n items.
func window(n, offset, limit int) (int, int) {
	lo := min(offset, n)
	hi := min(offset+limit, n)
	return lo, hi
}

// sortedCounts orders ids by count descending, then by the record's name.
func sortedCounts(s *atlas.Snapshot, t atlas.Type, counts map[string]int) []string {
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
		if counts[a] != counts[b] {
			return counts[a] > counts[b]
		}
		na, nb := nameOf(s, t, a), nameOf(s, t, b)
		if na != nb {
			return na < nb
		}
		return a < b
	})
	return ids
}

func nameOf(s *atlas.Snapshot, t atlas.Type, id string) string {
	if r := s.Rec(t, id); r != nil {
		return atlas.Norm(r.Name)
	}
	return id
}
