package atlas

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// studyBible is the jw.org study Bible; a chapter is <book>/<chapter>/.
const studyBible = "https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/"

// ChapterURL is the jw.org study Bible link to one chapter. jw.org writes
// the book name with hyphens: an accented name as it is, escaped
// ("G%C3%A9nesis", "1-Cr%C3%B3nicas"); a plain one in lower case ("1-reyes").
func ChapterURL(b *Book, chapter int) string {
	name := strings.Join(strings.Fields(b.Name), "-")
	ascii := true
	for i := 0; i < len(name); i++ {
		if name[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		name = strings.ToLower(name)
	} else {
		name = url.PathEscape(name)
	}
	return studyBible + name + "/" + strconv.Itoa(chapter) + "/"
}

// PassageURL is the link to read a citation, as the site writes it: the
// chapter of its first range with the anchor jw.org highlights,
// #v<book><ccc><vvv> for one verse or #v<start>-v<end> for a span. Later
// ranges in the same chapter, the items of a list, extend the span to their
// last verse. A passage that crosses into another chapter opens its first
// chapter to that chapter's last verse, because jw.org highlights nothing
// across chapters. A whole chapter has no anchor.
func PassageURL(rs []Range) string {
	if len(rs) == 0 || rs[0].Book == nil {
		return ""
	}
	r := rs[0]
	b := r.Book
	u := ChapterURL(b, r.C1)
	if !r.HasVerses || r.V1 < 1 {
		return u
	}
	last := 0
	if r.C1 <= len(b.Verses) {
		last = b.Verses[r.C1-1]
	}
	end := r.V2
	if r.C2 > r.C1 {
		end = last
	}
	for _, x := range rs[1:] {
		if x.Book == b && x.HasVerses && x.C1 == r.C1 && x.C2 == r.C1 && r.C2 == r.C1 {
			end = max(end, x.V2)
		}
	}
	if end < r.V1 {
		end = r.V1
	}
	if r.V1 == 1 && last > 0 && end >= last {
		return u
	}
	anchor := func(v int) string { return fmt.Sprintf("v%d%03d%03d", b.Num, r.C1, v) }
	if end > r.V1 {
		return u + "#" + anchor(r.V1) + "-" + anchor(end)
	}
	return u + "#" + anchor(r.V1)
}

// Chapter names one chapter of one book.
type Chapter struct {
	Book    int // libros[].num
	Chapter int
}

// Range is a passage from (C1, V1) to (C2, V2) inside one book. V1 = 0 means
// the start of chapter C1; V2 is always a real verse, the chapter's last
// verse when the passage ends with a whole chapter.
type Range struct {
	Book           *Book
	C1, V1, C2, V2 int
	// Shape of the reference as written, used to print it back.
	WholeBook bool
	HasVerses bool
}

// Overlaps reports whether two ranges share at least one verse.
func (r Range) Overlaps(o Range) bool {
	if r.Book == nil || o.Book == nil || r.Book.Num != o.Book.Num {
		return false
	}
	return !less(r.C2, r.V2, o.C1, o.V1) && !less(o.C2, o.V2, r.C1, r.V1)
}

// Chapters lists every chapter the range touches. A range that ends before
// it starts touches none.
func (r Range) Chapters() []Chapter {
	if r.Book == nil || r.C2 < r.C1 {
		return nil
	}
	out := make([]Chapter, 0, r.C2-r.C1+1)
	for c := r.C1; c <= r.C2; c++ {
		out = append(out, Chapter{Book: r.Book.Num, Chapter: c})
	}
	return out
}

// Text prints the range with the atlas's own abbreviation: "Rut",
// "Hch 16", "Gé 5-7", "Hch 16:1", "Gé 2:7-8", "Hch 13:1–14:28".
func (r Range) Text() string {
	b := r.Book
	switch {
	case r.WholeBook:
		return b.Abbr
	case !r.HasVerses && r.C1 == r.C2:
		return fmt.Sprintf("%s %d", b.Abbr, r.C1)
	case !r.HasVerses:
		return fmt.Sprintf("%s %d-%d", b.Abbr, r.C1, r.C2)
	case b.Chapters == 1 && r.V1 == r.V2:
		return fmt.Sprintf("%s %d", b.Abbr, r.V1)
	case b.Chapters == 1:
		return fmt.Sprintf("%s %d-%d", b.Abbr, r.V1, r.V2)
	case r.C1 == r.C2 && r.V1 == r.V2:
		return fmt.Sprintf("%s %d:%d", b.Abbr, r.C1, r.V1)
	case r.C1 == r.C2:
		return fmt.Sprintf("%s %d:%d-%d", b.Abbr, r.C1, r.V1, r.V2)
	}
	return fmt.Sprintf("%s %d:%d–%d:%d", b.Abbr, r.C1, r.V1, r.C2, r.V2)
}

func less(c1, v1, c2, v2 int) bool {
	return c1 < c2 || (c1 == c2 && v1 < v2)
}

// BookTable maps every written form of a book name to the book. It is built
// from the loaded file; the server hard-codes no book name.
type BookTable struct {
	byKey map[string]*Book
	books []*Book
}

// BookKey is the lookup key: normalised, with white space, dots and hyphens
// removed, so "2 Reyes", "2Re" and "2-reyes" meet.
func BookKey(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '.' || r == '-' {
			return -1
		}
		return r
	}, Norm(s))
}

func newBookTable(books []*Book) *BookTable {
	t := &BookTable{byKey: map[string]*Book{}}
	for _, b := range books {
		t.books = append(t.books, b)
		for _, k := range b.keys() {
			if _, taken := t.byKey[k]; !taken && k != "" {
				t.byKey[k] = b
			}
		}
	}
	return t
}

func (b *Book) keys() []string {
	keys := []string{BookKey(b.Abbr), BookKey(b.Name), BookKey(b.Slug)}
	for _, f := range b.Forms {
		keys = append(keys, BookKey(f))
	}
	for _, f := range b.Spoken {
		keys = append(keys, BookKey(f))
	}
	return keys
}

// Lookup finds a book by any written form: name, abbreviation, slug or form.
func (t *BookTable) Lookup(s string) *Book {
	if t == nil {
		return nil
	}
	return t.byKey[BookKey(s)]
}

// Suggest returns up to five book names close to s, nearest first.
func (t *BookTable) Suggest(s string) []string {
	key := BookKey(s)
	type cand struct {
		b *Book
		d int
	}
	var cands []cand
	for _, b := range t.books {
		best := -1
		for _, k := range b.keys() {
			if k == "" {
				continue
			}
			d := levenshtein(key, k)
			if key != "" && (strings.HasPrefix(k, key) || strings.HasPrefix(key, k)) {
				d = min(d, 1)
			}
			if best < 0 || d < best {
				best = d
			}
		}
		if best >= 0 && best <= max(2, len([]rune(key))/3) {
			cands = append(cands, cand{b, best})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].d != cands[j].d {
			return cands[i].d < cands[j].d
		}
		return cands[i].b.Num < cands[j].b.Num
	})
	var out []string
	for i := 0; i < len(cands) && i < 5; i++ {
		out = append(out, cands[i].b.Name)
	}
	return out
}

// RefError says which part of a reference failed.
type RefError struct {
	Msg         string
	Suggestions []string // close book names, for an unknown book
}

func (e *RefError) Error() string { return e.Msg }

var (
	reBookHead = regexp.MustCompile(`^(?:[123][\s-]*)?\p{L}+`)
	reBookWord = regexp.MustCompile(`^[\s-]+\p{L}+`)
	reNumber   = regexp.MustCompile(`^\d+`)
)

// ParseRefs parses a citation that may hold several parts separated by ";".
// A part with no book continues in the book of the part before it. Each
// comma item of a part gives its own range, so "Gé 6:1, 2, 4" never covers
// verse 3; items that touch, such as "Gé 2:7, 8", join into one range.
func (t *BookTable) ParseRefs(s string) ([]Range, error) {
	out, _, err := t.parse(s)
	return out, err
}

func (t *BookTable) parse(s string) ([]Range, string, error) {
	var out []Range
	var book *Book
	listHint := ""
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		rs, b, hint, err := t.parsePart(part, book)
		if err != nil {
			return out, "", err
		}
		book = b
		if listHint == "" {
			listHint = hint
		}
		out = append(out, rs...)
	}
	if len(out) == 0 {
		return nil, "", &RefError{Msg: fmt.Sprintf("no reference found in %q", s)}
	}
	return out, listHint, nil
}

// ParseOne parses exactly one reference: one passage with no gaps.
func (t *BookTable) ParseOne(s string) (Range, error) {
	rs, listHint, err := t.parse(s)
	if err != nil {
		return Range{}, err
	}
	if listHint != "" {
		return Range{}, &RefError{Msg: fmt.Sprintf("%q separates two chapters with a comma; %s", s, listHint)}
	}
	if len(rs) != 1 {
		texts := make([]string, len(rs))
		for i, r := range rs {
			texts[i] = r.Text()
		}
		return Range{}, &RefError{Msg: fmt.Sprintf("reference must name one passage with no gaps, got %d in %q (%s); ask for each one separately", len(rs), s, strings.Join(texts, "; "))}
	}
	return rs[0], nil
}

// splitBook takes the book name off the front of a part: the longest run of
// leading words the table knows, so a name of any length works. When no run
// is known it returns every leading word as the unknown name.
func (t *BookTable) splitBook(part string) (name, rest string, found bool) {
	head := reBookHead.FindString(part)
	if head == "" {
		return "", part, false
	}
	ends := []int{len(head)}
	for pos := len(head); ; {
		w := reBookWord.FindString(part[pos:])
		if w == "" {
			break
		}
		pos += len(w)
		ends = append(ends, pos)
	}
	for i := len(ends) - 1; i >= 0; i-- {
		if t.Lookup(part[:ends[i]]) != nil {
			return part[:ends[i]], part[ends[i]:], true
		}
	}
	last := ends[len(ends)-1]
	return part[:last], part[last:], true
}

func (t *BookTable) parsePart(part string, prev *Book) ([]Range, *Book, string, error) {
	book := prev
	rest := part
	if name, r, ok := t.splitBook(part); ok {
		book = t.Lookup(name)
		if book == nil {
			return nil, nil, "", &RefError{
				Msg:         fmt.Sprintf("unknown Bible book %q", strings.TrimSpace(name)),
				Suggestions: t.Suggest(name),
			}
		}
		rest = strings.TrimPrefix(strings.TrimSpace(r), ".")
	} else if book == nil {
		return nil, nil, "", &RefError{Msg: fmt.Sprintf("reference %q does not start with a Bible book", part)}
	}
	rs, hint, err := parseNumbers(book, strings.TrimSpace(rest))
	if err != nil {
		return nil, nil, "", err
	}
	return rs, book, hint, nil
}

// numItem is one comma item: a[:b][-c[:d]].
type numItem struct {
	a, b, c, d          int
	hasB, hasDash, hasD bool
}

func lastVerse(b *Book, chapter int) int {
	if chapter >= 1 && chapter <= len(b.Verses) {
		return b.Verses[chapter-1]
	}
	return 999
}

// parseNumbers reads the chapter and verse part of one reference. Every comma
// item becomes a range; items that touch or overlap the one before join it.
// listHint is set when a comma separates two bare chapter numbers
// ("Jn 3, 16"), which a caller may mean as chapter and verse; it says how to
// write either reading.
func parseNumbers(book *Book, s string) (out []Range, listHint string, err error) {
	if s == "" {
		last := max(book.Chapters, 1)
		return []Range{{Book: book, C1: 1, V1: 0, C2: last, V2: lastVerse(book, last), WholeBook: true}}, "", nil
	}
	items, err := scanItems(s)
	if err != nil {
		return nil, "", &RefError{Msg: fmt.Sprintf("cannot read chapter and verse %q in %s: %v", s, book.Name, err)}
	}
	singleChapter := book.Chapters == 1 && !items[0].hasB
	chapter := 0   // chapter the bare numbers after a verse belong to
	var bare []int // bare chapter numbers, in order
	for i, it := range items {
		r := Range{Book: book}
		switch {
		case singleChapter:
			end := it.a
			if it.hasDash {
				end = it.c
			}
			r.C1, r.V1, r.C2, r.V2, r.HasVerses = 1, it.a, 1, end, true
		case it.hasB:
			chapter = it.a
			r.C1, r.V1, r.HasVerses = it.a, it.b, true
			switch {
			case it.hasDash && it.hasD:
				r.C2, r.V2 = it.c, it.d
				chapter = it.c
			case it.hasDash:
				r.C2, r.V2 = it.a, it.c
			default:
				r.C2, r.V2 = it.a, it.b
			}
		case chapter > 0 && i > 0:
			// A bare number after a chapter:verse item is another verse of that chapter.
			end := it.a
			if it.hasDash {
				end = it.c
			}
			r.C1, r.V1, r.C2, r.V2, r.HasVerses = chapter, it.a, chapter, end, true
		default:
			// Chapters only: "5", "5-7", or "5-6:3".
			bare = append(bare, it.a)
			r.C1 = it.a
			switch {
			case it.hasDash && it.hasD:
				r.C2, r.V2, r.HasVerses = it.c, it.d, true
			case it.hasDash:
				r.C2, r.V2 = it.c, lastVerse(book, it.c)
			default:
				r.C2, r.V2 = it.a, lastVerse(book, it.a)
			}
		}
		if n := len(out); n > 0 && touches(out[n-1], r) {
			prev := &out[n-1]
			if less(prev.C2, prev.V2, r.C2, r.V2) {
				prev.C2, prev.V2 = r.C2, r.V2
			}
			prev.HasVerses = prev.HasVerses || r.HasVerses
			continue
		}
		out = append(out, r)
	}
	for i := range out {
		r := &out[i]
		if r.HasVerses && r.V1 == 0 {
			// Mixed "5-6:3": the start is a whole chapter, print it as verse 1.
			r.V1 = 1
		}
		if !r.HasVerses {
			r.V1 = 0
		}
	}
	if len(bare) > 1 {
		listHint = fmt.Sprintf("write \"%[1]s %[2]d:%[3]d\" for chapter %[2]d, verse %[3]d, or \"%[1]s %[2]d-%[3]d\" for chapters %[2]d to %[3]d", book.Abbr, bare[0], bare[1])
	}
	return out, listHint, nil
}

// touches reports whether next, written after prev, starts inside prev or
// on the verse right after it.
func touches(prev, next Range) bool {
	if less(next.C1, next.V1, prev.C1, prev.V1) {
		return false
	}
	c, v := prev.C2, prev.V2+1
	if prev.V2 >= lastVerse(prev.Book, prev.C2) {
		c, v = prev.C2+1, 1
	}
	return !less(c, v, next.C1, next.V1)
}

func scanItems(s string) ([]numItem, error) {
	var items []numItem
	for _, raw := range strings.Split(s, ",") {
		p := strings.TrimSpace(raw)
		if p == "" {
			return nil, fmt.Errorf("empty item")
		}
		var it numItem
		var ok bool
		if it.a, p, ok = readInt(p); !ok {
			return nil, fmt.Errorf("expected a number at %q", p)
		}
		if strings.HasPrefix(p, ":") {
			if it.b, p, ok = readInt(strings.TrimSpace(p[1:])); !ok {
				return nil, fmt.Errorf("expected a verse after ':'")
			}
			it.hasB = true
		}
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "-") || strings.HasPrefix(p, "–") || strings.HasPrefix(p, "—") {
			_, size := utf8.DecodeRuneInString(p)
			if it.c, p, ok = readInt(strings.TrimSpace(p[size:])); !ok {
				return nil, fmt.Errorf("expected a number after the dash")
			}
			it.hasDash = true
			if strings.HasPrefix(p, ":") {
				if it.d, p, ok = readInt(strings.TrimSpace(p[1:])); !ok {
					return nil, fmt.Errorf("expected a verse after ':'")
				}
				it.hasD = true
			}
		}
		if strings.TrimSpace(p) != "" {
			return nil, fmt.Errorf("unexpected text %q", strings.TrimSpace(p))
		}
		items = append(items, it)
	}
	return items, nil
}

func readInt(s string) (int, string, bool) {
	m := reNumber.FindString(s)
	if m == "" || len(m) > 4 {
		return 0, s, false
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return 0, s, false
	}
	return n, strings.TrimSpace(s[len(m):]), true
}

// Validate checks that the range's chapters and verses exist in its book.
func (r Range) Validate() error {
	b := r.Book
	for _, c := range []int{r.C1, r.C2} {
		if c < 1 || c > b.Chapters {
			return &RefError{Msg: fmt.Sprintf("%s has %d chapters, got chapter %d", b.Name, b.Chapters, c)}
		}
	}
	if r.HasVerses {
		for _, cv := range [][2]int{{r.C1, r.V1}, {r.C2, r.V2}} {
			last := lastVerse(b, cv[0])
			if cv[1] < 1 || cv[1] > last {
				return &RefError{Msg: fmt.Sprintf("%s %d has %d verses, got verse %d", b.Name, cv[0], last, cv[1])}
			}
		}
	}
	if less(r.C2, r.V2, r.C1, r.V1) {
		return &RefError{Msg: fmt.Sprintf("the passage %s ends before it starts", r.Text())}
	}
	return nil
}

var (
	reWolChapter = regexp.MustCompile(`/wol/b/r4/lp-s/nwt(?:sty)?/(\d+)/(\d+)`)
	reJWChapter  = regexp.MustCompile(`^https://www\.jw\.org/es/biblioteca/biblia/(?:biblia-estudio|nwt)/libros/([^/?#]+)/(\d+)/?(?:[?#]|$)`)
)

// chapterOfURL reads (book number, chapter) from a chapter link in either
// shape the atlas uses: wol.jw.org, which names the book by number, or the
// jw.org Bible, which names it by its hyphenated name ("G%C3%A9nesis",
// "el-cantar-de-los-cantares").
func (t *BookTable) chapterOfURL(u string) (Chapter, bool) {
	if m := reWolChapter.FindStringSubmatch(u); m != nil {
		b, _ := strconv.Atoi(m[1])
		c, _ := strconv.Atoi(m[2])
		return Chapter{Book: b, Chapter: c}, true
	}
	m := reJWChapter.FindStringSubmatch(u)
	if m == nil {
		return Chapter{}, false
	}
	name, err := url.PathUnescape(m[1])
	if err != nil {
		return Chapter{}, false
	}
	b := t.Lookup(name)
	if b == nil {
		return Chapter{}, false
	}
	c, _ := strconv.Atoi(m[2])
	return Chapter{Book: b.Num, Chapter: c}, true
}
