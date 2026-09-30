package tools

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
)

// Year limits for tool input. The atlas runs from 4026 a.e.c. to the first
// centuries e.c.; anything outside this range is a typo.
const (
	MinYear = -4100
	MaxYear = 500
)

func fmtNum(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return strconv.Quote(x)
	}
	b, err := marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// wholeNumber reads an optional whole-number argument in [lo, hi]. JSON
// numbers arrive as floats, so NaN, infinities and fractions are refused
// here instead of being truncated.
func wholeNumber(args map[string]any, key string, lo, hi, def int) (int, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return def, false, nil
	}
	bad := fmt.Errorf("%s must be a whole number from %d to %d, got %s", key, lo, hi, fmtNum(raw))
	v, isNum := raw.(float64)
	if !isNum {
		return 0, true, bad
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v < float64(lo) || v > float64(hi) {
		return 0, true, bad
	}
	return int(v), true, nil
}

// year reads an optional signed historical year.
func year(args map[string]any, key string) (int, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	bad := fmt.Errorf("%s must be a whole number from %d to %d and not 0 (years are signed: -607 is 607 a.e.c.), got %s", key, MinYear, MaxYear, fmtNum(raw))
	v, isNum := raw.(float64)
	if !isNum || math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v < MinYear || v > MaxYear || v == 0 {
		return 0, true, bad
	}
	return int(v), true, nil
}

// number reads an optional finite number in [lo, hi].
func number(args map[string]any, key string, lo, hi float64, loOpen bool) (float64, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return 0, false, nil
	}
	rule := fmt.Sprintf("from %s to %s", fmtNum(lo), fmtNum(hi))
	if loOpen {
		rule = fmt.Sprintf("above %s and at most %s", fmtNum(lo), fmtNum(hi))
	}
	bad := fmt.Errorf("%s must be a number %s, got %s", key, rule, fmtNum(raw))
	v, isNum := raw.(float64)
	if !isNum || math.IsNaN(v) || math.IsInf(v, 0) || v < lo || v > hi || (loOpen && v == lo) {
		return 0, true, bad
	}
	return v, true, nil
}

// str reads an optional string, trimmed, of at most maxLen characters.
func str(args map[string]any, key string, maxLen int) (string, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return "", false, nil
	}
	v, isStr := raw.(string)
	if !isStr {
		return "", true, fmt.Errorf("%s must be a string, got %s", key, fmtNum(raw))
	}
	v = strings.TrimSpace(v)
	if n := len([]rune(v)); n > maxLen {
		return "", true, fmt.Errorf("%s must be at most %d characters, got %d", key, maxLen, n)
	}
	return v, v != "", nil
}

// requiredStr reads a string that must be present and not blank.
func requiredStr(args map[string]any, key string, maxLen int) (string, error) {
	v, ok, err := str(args, key, maxLen)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%s is required and must not be empty", key)
	}
	return v, nil
}

// boolean reads an optional boolean.
func boolean(args map[string]any, key string, def bool) (bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return def, nil
	}
	v, isBool := raw.(bool)
	if !isBool {
		return false, fmt.Errorf("%s must be true or false, got %s", key, fmtNum(raw))
	}
	return v, nil
}

// paging reads limit and offset.
func paging(args map[string]any, defLimit, maxLimit int) (limit, offset int, err error) {
	if limit, _, err = wholeNumber(args, "limit", 1, maxLimit, defLimit); err != nil {
		return
	}
	offset, _, err = wholeNumber(args, "offset", 0, 1_000_000, 0)
	return
}

// recordType reads a type argument.
func recordType(args map[string]any, key string, allowed []atlas.Type) (atlas.Type, error) {
	v, err := requiredStr(args, key, 40)
	if err != nil {
		return "", err
	}
	for _, t := range allowed {
		if string(t) == v {
			return t, nil
		}
	}
	return "", fmt.Errorf("%s must be one of %s, got %q", key, joinTypes(allowed), v)
}

func joinTypes(ts []atlas.Type) string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = string(t)
	}
	return strings.Join(s, ", ")
}

func typeNames(ts []atlas.Type) []string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = string(t)
	}
	return s
}

// oneOf checks a value against the values the loaded file contains.
func oneOf(key, v string, allowed []string) error {
	for _, a := range allowed {
		if a == v {
			return nil
		}
	}
	return fmt.Errorf("%s %q is not in the atlas; values present: %s", key, v, strings.Join(allowed, ", "))
}

// recordID reads an id argument and checks that the record exists.
func recordID(s *atlas.Snapshot, args map[string]any, key string, t atlas.Type, required bool) (string, bool, error) {
	v, ok, err := str(args, key, 200)
	if err != nil {
		return "", false, err
	}
	if !ok {
		if required {
			return "", false, fmt.Errorf("%s is required: give a %s id from search", key, t)
		}
		return "", false, nil
	}
	id := strings.ToLower(v)
	if !s.Has(t, id) {
		return "", true, unknownID(s, t, v)
	}
	return id, true, nil
}

func unknownID(s *atlas.Snapshot, t atlas.Type, id string) error {
	sugg := s.Suggest(t, id)
	if len(sugg) == 0 {
		return fmt.Errorf("no %s with id %q. Find ids with search", t, id)
	}
	parts := make([]string, len(sugg))
	for i, r := range sugg {
		parts[i] = fmt.Sprintf("%s (%s)", r.ID, r.Name)
	}
	return fmt.Errorf("no %s with id %q. Did you mean: %s?", t, id, strings.Join(parts, ", "))
}
