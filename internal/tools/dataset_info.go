package tools

import (
	"time"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/geiserx/biblical-atlas-mcp/version"
)

func datasetInfoTool() Tool {
	return Tool{
		Def: newTool("dataset_info", "Atlas dataset information",
			`What the loaded atlas contains and how fresh it is: build date, record counts, which Bible books the atlas has read verse by verse and which it has not, and where the data came from. Call it when a user asks how complete or current the atlas is, or before treating a missing result as "the Bible does not say".`),
		Handle: datasetInfo,
	}
}

func datasetInfo(c *Call) (*obj, error) {
	s := c.Snap
	f := s.File
	st := atlas.Status{Source: s.Source, LoadedAt: s.LoadedAt}
	if c.Store != nil {
		st = c.Store.Status()
	}
	var read, unread []string
	for _, b := range f.Books {
		if s.FullyRead[b.Slug] {
			read = append(read, b.Slug)
		} else {
			unread = append(unread, b.Slug)
		}
	}
	counts := newObj().keep("people", len(f.People)).keep("places", len(f.Places)).keep("events", len(f.Events)).
		keep("periods", len(f.Periods)).keep("letters", len(f.Letters)).keep("journeys", len(f.Journeys)).
		keep("finds", len(f.Finds)).keep("tours", len(f.Tours)).keep("books", len(f.Books)).
		keep("months", len(f.Months)).keep("calendar_topics", len(f.Topics)).keep("sources", len(f.Sources))
	o := newObj().keep("format", s.Format).set("generated", s.Generated).set("source", st.Source)
	if !st.LoadedAt.IsZero() {
		o.keep("loaded_at", st.LoadedAt.UTC().Format(time.RFC3339))
	}
	if !st.CheckedAt.IsZero() {
		o.keep("checked_at", st.CheckedAt.UTC().Format(time.RFC3339))
	}
	o.keep("stale", st.Stale).set("refresh_error", st.RefreshError).keep("counts", counts).
		keep("chapter_sources", len(s.SourceChapter)).set("warnings", s.Warnings()).
		set("books_fully_read", read).set("books_not_yet_read", unread).
		keep("site", SiteBase).keep("server_version", version.Version)
	return o, nil
}
