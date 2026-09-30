package atlas

import (
	"errors"
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

func TestChapterOfURL(t *testing.T) {
	for u, want := range map[string]Chapter{
		"https://wol.jw.org/es/wol/b/r4/lp-s/nwtsty/44/16": {44, 16},
		"https://wol.jw.org/es/wol/b/r4/lp-s/nwt/1/2":      {1, 2},
	} {
		if got, ok := chapterOfURL(u); !ok || got != want {
			t.Errorf("chapterOfURL(%q) = %v %v", u, got, ok)
		}
	}
	if _, ok := chapterOfURL("https://wol.jw.org/es/wol/d/r4/lp-s/1200003406"); ok {
		t.Error("a publication URL parsed as a chapter")
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
