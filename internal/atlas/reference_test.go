package atlas

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// withLongBook adds a book whose name has five words, longer than any
// fixed word count would allow.
func withLongBook(t *testing.T) *Snapshot {
	t.Helper()
	s := fixture(t)
	books := append([]*Book{}, s.Books.books...)
	books = append(books, &Book{Num: 99, Slug: "cinco", Abbr: "Cin", Name: "El Libro de las Cinco Palabras", Chapters: 3, Verses: []int{3, 3, 3}})
	s.Books = newBookTable(books)
	return s
}

func TestParseReferences(t *testing.T) {
	s := withLongBook(t)
	cases := []struct {
		in             string
		book           int
		c1, v1, c2, v2 int
		text           string
	}{
		{"Hch 16:1", 44, 16, 1, 16, 1, "Hch 16:1"},
		{"Hechos 16", 44, 16, 0, 16, 40, "Hch 16"},
		{"hch. 16", 44, 16, 0, 16, 40, "Hch 16"},
		{"HCH 16:1-3", 44, 16, 1, 16, 3, "Hch 16:1-3"},
		{"Gé 2:7, 8", 1, 2, 7, 2, 8, "Gé 2:7-8"},
		{"Ge 5-7", 1, 5, 0, 7, 24, "Gé 5-7"},
		{"Génesis 50", 1, 50, 0, 50, 26, "Gé 50"},
		{"Hch 13:1–14:28", 44, 13, 1, 14, 28, "Hch 13:1–14:28"},
		{"Hch 13:1-14:28", 44, 13, 1, 14, 28, "Hch 13:1–14:28"},
		{"Rut", 8, 1, 0, 4, 22, "Rut"},
		{"Flm 5", 57, 1, 5, 1, 5, "Flm 5"},
		{"Flm 1, 2", 57, 1, 1, 1, 2, "Flm 1-2"},
		{"Gé 2:7-9, 8", 1, 2, 7, 2, 9, "Gé 2:7-9"},
		{"Gé 2:25, 3:1", 1, 2, 25, 3, 1, "Gé 2:25–3:1"},
		{"El Libro de las Cinco Palabras 2", 99, 2, 0, 2, 3, "Cin 2"},
		{"2 Timoteo 1:5", 55, 1, 5, 1, 5, "2Ti 1:5"},
		{"2Ti 1", 55, 1, 0, 1, 18, "2Ti 1"},
		{"2-timoteo 4:6-8", 55, 4, 6, 4, 8, "2Ti 4:6-8"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, err := s.Books.ParseOne(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Validate(); err != nil {
				t.Fatal(err)
			}
			if r.Book.Num != c.book || r.C1 != c.c1 || r.V1 != c.v1 || r.C2 != c.c2 || r.V2 != c.v2 {
				t.Fatalf("got %d %d:%d-%d:%d", r.Book.Num, r.C1, r.V1, r.C2, r.V2)
			}
			if r.Text() != c.text {
				t.Fatalf("text %q, want %q", r.Text(), c.text)
			}
		})
	}
}

func TestParseCitationContinuation(t *testing.T) {
	s := fixture(t)
	rs, err := s.Books.ParseRefs("Hch 21:27-36; 22:1; Gé 2:7")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 3 || rs[1].Book.Num != 44 || rs[1].C1 != 22 || rs[1].V1 != 1 || rs[2].Book.Num != 1 {
		t.Fatalf("got %+v", rs)
	}
	if _, err := s.Books.ParseOne("Hch 21:27; 22:1"); err == nil || !strings.Contains(err.Error(), "one passage") {
		t.Fatalf("two parts accepted: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	s := fixture(t)
	cases := []struct{ in, want string }{
		{"Hechs 16", `unknown Bible book "Hechs"`},
		{"Hch 29", "Hechos has 28 chapters, got chapter 29"},
		{"Hch 16:41", "Hechos 16 has 40 verses, got verse 41"},
		{"Hch 16:0", "verse 0"},
		{"16:1", "does not start with a Bible book"},
		{"Hch 16:x", "cannot read chapter and verse"},
		{"Hch 16:5-2", "ends before it starts"},
		{"Hch 16:1-3, 6", `must name one passage with no gaps, got 2 in "Hch 16:1-3, 6" (Hch 16:1-3; Hch 16:6)`},
		{"Hechos 3, 16", `"Hechos 3, 16" separates two chapters with a comma; write "Hch 3:16" for chapter 3, verse 16, or "Hch 3-16" for chapters 3 to 16`},
		{"Hechos 3,16", `write "Hch 3:16"`},
		{"Flm 1, 2, 10-12", "got 2"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r, err := s.Books.ParseOne(c.in)
			if err == nil {
				err = r.Validate()
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want %q", err, c.want)
			}
		})
	}
	_, err := s.Books.ParseOne("Hechs 16")
	var re *RefError
	if !errors.As(err, &re) || len(re.Suggestions) == 0 || re.Suggestions[0] != "Hechos" {
		t.Fatalf("suggestions = %+v", re)
	}
}

func TestOverlaps(t *testing.T) {
	s := fixture(t)
	parse := func(x string) Range {
		r, err := s.Books.ParseOne(x)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	cases := []struct {
		a, b string
		want bool
	}{
		{"Hch 16", "Hch 16:1", true},
		{"Hch 16:1-3", "Hch 16:3", true},
		{"Hch 16:1-3", "Hch 16:4", false},
		{"Hch 13:1–14:28", "Hch 14", true},
		{"Hch 13:1–14:28", "Hch 15:1", false},
		{"Hechos", "Hch 28:31", true},
		{"Hch 16", "Gé 16", false},
	}
	for _, c := range cases {
		if got := parse(c.a).Overlaps(parse(c.b)); got != c.want {
			t.Errorf("%s overlaps %s = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// jw is the study Bible prefix, written out so the tests do not trust the
// constant they check.
const jw = "https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/"

// TestChapterOfURL reads a chapter from both link shapes the atlas has used:
// wol.jw.org with the book number, and the jw.org study Bible with the book
// name.
func TestChapterOfURL(t *testing.T) {
	s := fixture(t)
	for u, want := range map[string]Chapter{
		"https://wol.jw.org/es/wol/b/r4/lp-s/nwtsty/44/16": {44, 16},
		"https://wol.jw.org/es/wol/b/r4/lp-s/nwt/1/2":      {1, 2},
		jw + "hechos/16/":                    {44, 16},
		jw + "hechos/16":                     {44, 16},
		jw + "hechos/9/#v44009001-v44009009": {44, 9},
		jw + "G%C3%A9nesis/2/":               {1, 2},
		jw + "2-timoteo/1/":                  {55, 1},
		jw + "Filem%C3%B3n/1/":               {57, 1},
		"https://www.jw.org/es/biblioteca/biblia/nwt/libros/rut/1/":            {8, 1},
		"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/libros/Rut/4/": {8, 4},
	} {
		if got, ok := s.Books.chapterOfURL(u); !ok || got != want {
			t.Errorf("chapterOfURL(%q) = %v %v, want %v", u, got, ok, want)
		}
	}
	for _, u := range []string{
		"https://wol.jw.org/es/wol/d/r4/lp-s/1200003406",
		"https://www.jw.org/es/biblioteca/libros/Perspicacia-para-comprender-las-Escrituras/No%C3%A1/",
		"https://www.jw.org/es/biblioteca/biblia/biblia-estudio/apendice-b/ultima-semana-de-jesus-8-a-11-de-nisan/",
		jw + "hechos/",
		jw + "hechos/16/notas/",
		jw + "levitico/1/",
		jw + "G%zz/1/",
		"https://example.org/es/biblioteca/biblia/biblia-estudio/libros/hechos/16/",
	} {
		if c, ok := s.Books.chapterOfURL(u); ok {
			t.Errorf("chapterOfURL(%q) = %v, want no chapter", u, c)
		}
	}
}

// TestChapterURL writes the book the way jw.org does: an accented name as
// it is, hyphenated and escaped; a plain one hyphenated and in lower case.
func TestChapterURL(t *testing.T) {
	cantar := &Book{Num: 22, Slug: "cantar-de-los-cantares", Name: "El Cantar de los Cantares"}
	cronicas := &Book{Num: 13, Slug: "1-cronicas", Name: "1 Crónicas"}
	s := fixture(t)
	for _, c := range []struct {
		b    *Book
		ch   int
		want string
	}{
		{s.Books.Lookup("Génesis"), 2, jw + "G%C3%A9nesis/2/"},
		{s.Books.Lookup("Hechos"), 16, jw + "hechos/16/"},
		{s.Books.Lookup("2 Timoteo"), 1, jw + "2-timoteo/1/"},
		{cantar, 2, jw + "el-cantar-de-los-cantares/2/"},
		{cronicas, 2, jw + "1-Cr%C3%B3nicas/2/"},
	} {
		if got := ChapterURL(c.b, c.ch); got != c.want {
			t.Errorf("ChapterURL(%s, %d) = %q, want %q", c.b.Name, c.ch, got, c.want)
		}
	}
	// Every link the server writes reads back as its own chapter.
	books := append([]*Book{cantar, cronicas}, s.Books.books...)
	table := newBookTable(books)
	for _, b := range books {
		if got, ok := table.chapterOfURL(ChapterURL(b, 1)); !ok || got != (Chapter{b.Num, 1}) {
			t.Errorf("%s: %s reads back as %v %v", b.Name, ChapterURL(b, 1), got, ok)
		}
	}
}

// TestPassageURL follows the site: one verse or a range gets its anchor, a
// list runs from its first to its last verse, a passage that crosses into
// another chapter opens its first chapter to that chapter's last verse, and
// a whole chapter has no anchor.
func TestPassageURL(t *testing.T) {
	s := fixture(t)
	for _, c := range []struct{ ref, want string }{
		{"Hch 16:1", jw + "hechos/16/#v44016001"},
		{"Hch 16:1-5", jw + "hechos/16/#v44016001-v44016005"},
		{"Gé 2:7, 8", jw + "G%C3%A9nesis/2/#v1002007-v1002008"},
		{"Gé 6:1, 2, 4", jw + "G%C3%A9nesis/6/#v1006001-v1006004"},
		{"Hch 16:1, 5", jw + "hechos/16/#v44016001-v44016005"},
		{"Hch 13:5–14:2", jw + "hechos/13/#v44013005-v44013052"},
		{"Hch 13:1–14:28", jw + "hechos/13/"},
		{"Hch 16:1-40", jw + "hechos/16/"},
		{"Hch 16", jw + "hechos/16/"},
		{"Gé 5-7", jw + "G%C3%A9nesis/5/"},
		{"Flm 10", jw + "Filem%C3%B3n/1/#v57001010"},
		{"Flm 4-7", jw + "Filem%C3%B3n/1/#v57001004-v57001007"},
		{"Flm", jw + "Filem%C3%B3n/1/"},
		{"Rut", jw + "rut/1/"},
	} {
		rs, err := s.Books.ParseRefs(c.ref)
		if err != nil {
			t.Fatalf("%s: %v", c.ref, err)
		}
		if got := PassageURL(rs); got != c.want {
			t.Errorf("PassageURL(%s) = %q, want %q", c.ref, got, c.want)
		}
	}
	if got := PassageURL(nil); got != "" {
		t.Errorf("PassageURL(nil) = %q", got)
	}
}

// TestSourceChaptersBothShapes loads the fixture with wol.jw.org chapter
// links and its copy with jw.org links: both give the same chapter index.
func TestSourceChaptersBothShapes(t *testing.T) {
	old := fixture(t)
	f, err := Decode(readFixture(t, "jworg.json"))
	if err != nil {
		t.Fatal(err)
	}
	next := Build(f)
	if len(old.SourceChapter) != 10 {
		t.Errorf("old shape: %d chapter sources, want 10", len(old.SourceChapter))
	}
	if !reflect.DeepEqual(next.SourceChapter, old.SourceChapter) {
		t.Errorf("jw.org shape: chapter sources %v, want %v", next.SourceChapter, old.SourceChapter)
	}
	if !reflect.DeepEqual(next.CitedBy, old.CitedBy) {
		t.Errorf("jw.org shape: cited_by index differs from the old shape")
	}
	if len(next.CitedBy[Chapter{44, 16}]) == 0 {
		t.Error("jw.org shape: nothing cites Hechos 16")
	}
	if w := next.Warnings(); len(w) != 0 {
		t.Errorf("jw.org shape warns: %v", w)
	}
}

// TestEmptyChapterIndexWarns: sources whose links the server cannot read as
// chapters must not leave the chapter index empty without a word.
func TestEmptyChapterIndexWarns(t *testing.T) {
	f, err := Decode(readFixture(t, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range f.Sources {
		if src != nil {
			src.URL = "https://example.org/biblia/" + src.Title
		}
	}
	s := Build(f)
	if len(s.SourceChapter) != 0 {
		t.Fatalf("chapter sources %v, want none", s.SourceChapter)
	}
	w := s.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], "chapter") {
		t.Errorf("warnings %q, want one about the chapter index", w)
	}
	f.Sources = nil
	if w := Build(f).Warnings(); len(w) != 0 {
		t.Errorf("no sources at all warns: %v", w)
	}
}

// TestCommaItemsKeepTheirGaps: a comma list names those verses only, so a
// verse between two named ones is not covered.
func TestCommaItemsKeepTheirGaps(t *testing.T) {
	s := fixture(t)
	rs, err := s.Books.ParseRefs("Hch 16:1, 5")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("want 2 ranges, got %+v", rs)
	}
	mid, _ := s.Books.ParseOne("Hch 16:3")
	for _, r := range rs {
		if r.Overlaps(mid) {
			t.Fatalf("%s covers Hch 16:3", r.Text())
		}
	}
	five, _ := s.Books.ParseOne("Hch 16:5")
	if !rs[1].Overlaps(five) {
		t.Fatal("Hch 16:5 lost")
	}
	// A verse range followed by another keeps the gap between them.
	rs, err = s.Books.ParseRefs("Hch 16:1-20, 30-40")
	if err != nil || len(rs) != 2 || rs[0].V2 != 20 || rs[1].V1 != 30 {
		t.Fatalf("got %+v %v", rs, err)
	}
}

// TestEveryBookFormParses sends every written form of every book, alone and
// with a chapter.
func TestEveryBookFormParses(t *testing.T) {
	s := withLongBook(t)
	for _, b := range s.Books.books {
		forms := append([]string{b.Name, b.Abbr, b.Slug}, b.Forms...)
		forms = append(forms, b.Spoken...)
		for _, f := range forms {
			for _, in := range []string{f, f + " 1"} {
				r, err := s.Books.ParseOne(in)
				if err != nil || r.Book != b {
					t.Errorf("%q: %v", in, err)
				}
			}
		}
	}
}

func TestReversedRangeHasNoChapters(t *testing.T) {
	s := fixture(t)
	rs, err := s.Books.ParseRefs("Hch 16-3")
	if err != nil {
		t.Fatal(err)
	}
	if got := rs[0].Chapters(); got != nil {
		t.Fatalf("reversed range gave chapters %v", got)
	}
	if rs[0].Validate() == nil {
		t.Fatal("reversed range validated")
	}
}
