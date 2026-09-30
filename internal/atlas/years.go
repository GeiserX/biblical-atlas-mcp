package atlas

// The data file stores astronomical years: 0 is 1 a.e.c. (BCE), -1 is 2 a.e.c.
// Every tool input and output uses the historical year instead, where -607 is
// 607 a.e.c. and there is no year 0. These two functions are the only place
// the server crosses between the two conventions.

// ToHistorical turns an astronomical year into a signed historical year.
func ToHistorical(y int) int {
	if y <= 0 {
		return y - 1
	}
	return y
}

// ToAstronomical turns a signed historical year (never 0) into an astronomical year.
func ToAstronomical(h int) int {
	if h < 0 {
		return h + 1
	}
	return h
}

// Covers reports whether the date overlaps the astronomical interval [a, b].
// A missing "desde" is open to the past and a missing "hasta" open to the
// future. A date with neither bound covers nothing.
func (d *Date) Covers(a, b int) bool {
	if d == nil || (d.From == nil && d.To == nil) {
		return false
	}
	if d.From != nil && *d.From > b {
		return false
	}
	if d.To != nil && *d.To < a {
		return false
	}
	return true
}

// Start returns the earliest astronomical year of the date, falling back to
// its end when it is open to the past. ok is false for a date with no bound.
func (d *Date) Start() (int, bool) {
	switch {
	case d == nil:
		return 0, false
	case d.From != nil:
		return *d.From, true
	case d.To != nil:
		return *d.To, true
	}
	return 0, false
}

// End returns the latest astronomical year of the date, falling back to its
// start when it is open to the future. ok is false for a date with no bound.
func (d *Date) End() (int, bool) {
	switch {
	case d == nil:
		return 0, false
	case d.To != nil:
		return *d.To, true
	case d.From != nil:
		return *d.From, true
	}
	return 0, false
}
