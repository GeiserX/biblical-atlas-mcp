package atlas

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// LiveURL is the real data file. The live tests run only with
// BIBLICAL_ATLAS_LIVE=1 and never in CI.
const LiveURL = "https://biblical-atlas.geiser.cloud/data.json"

func TestLiveDataFile(t *testing.T) {
	if os.Getenv("BIBLICAL_ATLAS_LIVE") != "1" {
		t.Skip("set BIBLICAL_ATLAS_LIVE=1 to download the real data file")
	}
	store := NewStore(Options{URL: LiveURL, Timeout: 2 * time.Minute, UserAgent: "biblical-atlas-mcp-live-test", Logf: t.Logf})
	snap, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(KnownFormats, snap.Format) {
		t.Fatalf("format %q", snap.Format)
	}
	if snap.Stats.Dangling != 0 {
		t.Errorf("%d references point at missing records", snap.Stats.Dangling)
	}
	if snap.Stats.BadEventPassages != 0 {
		t.Errorf("%d event passages do not parse", snap.Stats.BadEventPassages)
	}
	// Every written form of every book parses, alone and with a chapter.
	for _, b := range snap.File.Books {
		forms := append([]string{b.Name, b.Abbr, b.Slug}, b.Forms...)
		for _, f := range append(forms, b.Spoken...) {
			for _, in := range []string{f, f + " 1"} {
				if r, err := snap.Books.ParseOne(in); err != nil || r.Book != b {
					t.Errorf("book form %q: %v", in, err)
				}
			}
		}
	}
	// Every chapter link the server writes reads back as its own chapter.
	for _, b := range snap.File.Books {
		if c, ok := snap.Books.chapterOfURL(ChapterURL(b, 1)); !ok || c != (Chapter{b.Num, 1}) {
			t.Errorf("%s: %s reads back as %v %v", b.Name, ChapterURL(b, 1), c, ok)
		}
	}
	for _, w := range snap.Warnings() {
		t.Error(w)
	}
	for id, src := range snap.File.Sources {
		if src != nil && !strings.HasPrefix(src.URL, "https://") {
			t.Errorf("source %s has a non-https URL %q", id, src.URL)
		}
	}
	t.Logf("generated %s: %d people, %d places, %d events; %d citations not parsed; %d chapter sources",
		snap.Generated, len(snap.File.People), len(snap.File.Places), len(snap.File.Events), snap.Stats.BadCitations, len(snap.SourceChapter))
}
