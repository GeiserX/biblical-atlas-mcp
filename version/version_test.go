package version

import "testing"

func TestString(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = origVersion, origCommit, origDate })

	tests := []struct {
		name, version, commit, date, want string
	}{
		{"default dev values", "dev", "none", "unknown", "dev (none) unknown"},
		{"release values", "0.1.0", "abc1234", "2026-09-30", "0.1.0 (abc1234) 2026-09-30"},
		{"empty values", "", "", "", " () "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			Version, Commit, Date = tc.version, tc.commit, tc.date
			if got := String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
