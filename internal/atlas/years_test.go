package atlas

import "testing"

func TestYearConversion(t *testing.T) {
	cases := []struct{ astro, hist int }{
		{-606, -607}, {0, -1}, {-1, -2}, {1, 1}, {33, 33}, {-4025, -4026},
	}
	for _, c := range cases {
		if got := ToHistorical(c.astro); got != c.hist {
			t.Errorf("ToHistorical(%d) = %d, want %d", c.astro, got, c.hist)
		}
		if got := ToAstronomical(c.hist); got != c.astro {
			t.Errorf("ToAstronomical(%d) = %d, want %d", c.hist, got, c.astro)
		}
	}
}

func TestCovers(t *testing.T) {
	closed := &Date{From: intp(10), To: intp(20)}
	openPast := &Date{To: intp(20)}
	openFuture := &Date{From: intp(10)}
	cases := []struct {
		name string
		d    *Date
		a, b int
		want bool
	}{
		{"inside", closed, 15, 15, true},
		{"left edge", closed, 10, 10, true},
		{"right edge", closed, 20, 20, true},
		{"before", closed, 9, 9, false},
		{"after", closed, 21, 21, false},
		{"range overlapping start", closed, 0, 10, true},
		{"range around", closed, 0, 100, true},
		{"open past far back", openPast, -4000, -4000, true},
		{"open past after", openPast, 21, 30, false},
		{"open future far ahead", openFuture, 400, 400, true},
		{"open future before", openFuture, 0, 9, false},
		{"no bounds", &Date{}, 0, 0, false},
		{"nil", nil, 0, 0, false},
	}
	for _, c := range cases {
		if got := c.d.Covers(c.a, c.b); got != c.want {
			t.Errorf("%s: Covers(%d, %d) = %v, want %v", c.name, c.a, c.b, got, c.want)
		}
	}
}

func TestStartEnd(t *testing.T) {
	if s, ok := (&Date{To: intp(5)}).Start(); !ok || s != 5 {
		t.Errorf("Start of open past = %d %v", s, ok)
	}
	if e, ok := (&Date{From: intp(5)}).End(); !ok || e != 5 {
		t.Errorf("End of open future = %d %v", e, ok)
	}
	var d *Date
	if _, ok := d.Start(); ok {
		t.Error("nil date has a start")
	}
	if _, ok := d.End(); ok {
		t.Error("nil date has an end")
	}
}
