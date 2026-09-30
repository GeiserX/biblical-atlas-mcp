package atlas

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixture(t *testing.T) *Snapshot {
	t.Helper()
	f, err := Decode(readFixture(t, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	return Build(f)
}

func intp(n int) *int { return &n }
