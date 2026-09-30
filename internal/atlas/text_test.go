package atlas

import "testing"

func TestNorm(t *testing.T) {
	cases := map[string]string{
		"Jerusalén":       "jerusalen",
		"JERUSALEN":       "jerusalen",
		"  Año   Nuevo  ": "ano nuevo",
		"Éx":              "ex",
		"Abí-Ézer":        "abi-ezer",
		"Ñandú\tüber":     "nandu uber",
	}
	for in, want := range cases {
		if got := Norm(in); got != want {
			t.Errorf("Norm(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScoreTiers(t *testing.T) {
	cases := []struct {
		q, text string
		want    int
	}{
		{"jerusalen", "jerusalen", ScoreExact},
		{"jeru", "jerusalen", ScorePrefix},
		{"tadeo", "santiago padre de tadeo", ScoreWord},
		{"ezer", "abi-ezer", ScoreWord},
		{"usal", "jerusalen", ScoreInside},
		{"us", "jerusalen", 0},
		{"roma", "jerusalen", 0},
	}
	for _, c := range cases {
		if got := ScoreText(c.q, c.text); got != c.want {
			t.Errorf("ScoreText(%q, %q) = %d, want %d", c.q, c.text, got, c.want)
		}
	}
}

func TestSearch(t *testing.T) {
	s := fixture(t)
	hits := s.Search("JERUSALÉN", nil)
	if len(hits) == 0 || hits[0].Rec.ID != "jerusalen" || hits[0].Score != ScoreExact {
		t.Fatalf("first hit = %+v", hits)
	}

	// Ids collide across types: both adan records come back, person first.
	hits = s.Search("adan", nil)
	if len(hits) < 2 || hits[0].Rec.Type != TypePerson || hits[1].Rec.Type != TypePlace || hits[0].Rec.ID != "adan" || hits[1].Rec.ID != "adan" {
		t.Fatalf("adan hits = %v", describe(hits))
	}
	// Type filter.
	hits = s.Search("adan", map[Type]bool{TypePlace: true})
	for _, h := range hits {
		if h.Rec.Type != TypePlace {
			t.Fatalf("type filter leaked %s", h.Rec.Type)
		}
	}
	// Homonyms: two people named Santiago, ordered by id.
	hits = s.Search("santiago", map[Type]bool{TypePerson: true})
	if len(hits) != 2 || hits[0].Rec.ID != "santiago-hijo-de-alfeo" || hits[1].Rec.ID != "santiago-padre-de-tadeo" {
		t.Fatalf("homonyms = %v", describe(hits))
	}
	if s.SameName["santiago"] != 2 {
		t.Fatalf("same name count = %d", s.SameName["santiago"])
	}
	// Summary tier: every query word starts a summary word.
	hits = s.Search("apostol viajo", nil)
	if len(hits) != 1 || hits[0].Rec.ID != "pablo" || hits[0].Matched != "summary" || hits[0].Score != ScoreSummary {
		t.Fatalf("summary hits = %v", describe(hits))
	}
	// Short queries never use the summary tier.
	if hits := s.Search("zz", nil); len(hits) != 0 {
		t.Fatalf("zz hits = %v", describe(hits))
	}
	// Books by abbreviation and months by another name.
	if hits := s.Search("hch", nil); len(hits) == 0 || hits[0].Rec.Type != TypeBook {
		t.Fatalf("hch hits = %v", describe(hits))
	}
	if hits := s.Search("abib", nil); len(hits) == 0 || hits[0].Rec.ID != "nisan" {
		t.Fatalf("abib hits = %v", describe(hits))
	}
	// Stable order.
	a, b := describe(s.Search("a", nil)), describe(s.Search("a", nil))
	if len(a) != len(b) {
		t.Fatal("unstable")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("unstable order")
		}
	}
}

func describe(hits []Hit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, string(h.Rec.Type)+":"+h.Rec.ID)
	}
	return out
}

func TestSuggest(t *testing.T) {
	s := fixture(t)
	sugg := s.Suggest(TypePerson, "pablo-apostol")
	if len(sugg) == 0 || sugg[0].ID != "pablo" {
		t.Fatalf("suggestions = %v", sugg)
	}
	sugg = s.Suggest(TypePerson, "timotteo")
	if len(sugg) == 0 || sugg[0].ID != "timoteo" {
		t.Fatalf("suggestions = %v", sugg)
	}
	if got := s.Suggest(TypePerson, "zzzzzzzzzz"); len(got) != 0 {
		t.Fatalf("suggestions for nonsense = %v", got)
	}
}

func TestLevenshtein(t *testing.T) {
	if d := levenshtein("kitten", "sitting"); d != 3 {
		t.Fatalf("got %d", d)
	}
	if d := levenshtein("", "abc"); d != 3 {
		t.Fatalf("got %d", d)
	}
}
