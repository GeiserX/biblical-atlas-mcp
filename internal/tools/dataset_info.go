package tools

import (
	"time"

	"github.com/geiserx/biblical-atlas-mcp/internal/atlas"
	"github.com/geiserx/biblical-atlas-mcp/version"
)

func datasetInfoTool() Tool {
	return Tool{
		Def: newTool("dataset_info", "Atlas dataset information",
			`What the loaded atlas contains and how fresh it is: build date, record counts, which Bible books the atlas has read verse by verse and which it has not, where the data came from, and filters: the values the kind and role parameters of list_records, list_events and places_near accept. Call it when a user asks how complete or current the atlas is, before treating a missing result as "the Bible does not say", or to pick a valid filter value.`,
			outputSchema(object(schema{
				"format":     strS,
				"generated":  typed("string", "Build date of the data file."),
				"source":     typed("string", "Where the data came from: the URL without query or credentials, or the local file's name."),
				"loaded_at":  strS,
				"checked_at": strS,
				"stale":      boolS,
				"counts":     anyObject,
				"filters": object(schema{
					"place_kinds":  typed("array", "Values of kind for list_records type place and for places_near."),
					"offices":      typed("array", "Values of kind for list_records type person."),
					"period_kinds": typed("array", "Values of kind for list_records type period."),
					"event_kinds":  typed("array", "Values of kind for list_events."),
					"event_roles":  typed("array", "Values of role for list_events."),
				}),
				"books_fully_read":   arrayOf(strS),
				"books_not_yet_read": arrayOf(strS),
				"server_version":     strS,
			}, "format", "stale", "counts", "filters", "server_version")),
		),
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
		keep("filters", newObj().keep("place_kinds", nonNil(s.PlaceKinds)).keep("offices", nonNil(s.OfficeKinds)).
			keep("period_kinds", nonNil(s.PeriodKinds)).keep("event_kinds", nonNil(s.EventKinds)).keep("event_roles", nonNil(s.RoleKinds))).
		set("books_fully_read", read).set("books_not_yet_read", unread).
		keep("site", SiteBase).keep("server_version", version.Version)
	return o, nil
}

// nonNil turns a nil list into an empty one, so it encodes as [].
func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
