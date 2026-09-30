# Usage

The server has nine tools. All of them are read only and answer from one snapshot of the atlas.

## Conventions

- **Language.** Tool names, parameter names and output keys are English. Values are the atlas's own Spanish text, never translated or rewritten.
- **Types and ids.** Every record has a `type` and an `id`. Ids repeat across types (a person and a place can both be `adan`), so every tool takes and returns both. Find ids with `search`.

  | `type` | What it is | Card link on the site |
  | --- | --- | --- |
  | `person` | a person | yes |
  | `place` | a place | yes |
  | `event` | an event | yes |
  | `period` | a reign, empire, era, high priesthood or governorship | yes |
  | `letter` | a letter of the Christian Greek Scriptures | yes |
  | `journey` | a journey with its stops | yes |
  | `find` | an archaeological find | yes |
  | `tour` | a guided tour of the atlas | yes |
  | `book` | a Bible book; the id is its slug (`2-reyes`) | yes |
  | `month` | a Hebrew month | no |
  | `calendar_topic` | a topic of the Hebrew calendar | no |

- **Years.** Every year in and out is a signed historical year: `-607` is 607 a.e.c. (BCE), `33` is 33 e.c. (CE). There is no year 0. A date covers every year from `from` to `to`; a missing `from` is open to the past and a missing `to` is open to the future.
- **Dates.** A `date` object holds `text` (the atlas's own rendering), `from`, `to`, `approx`, `kind`, `precision`, and `month`, `day`, `season`, `note` when the atlas has them. `kind: "narrativa"` means the date comes only from the order of the story.
- **Links.** Every record type with a card (see the table) comes back with `url`, its card on <https://biblical-atlas.geiser.cloud/>. Every record comes back with `sources` and their links: most on wol.jw.org, and the coordinate sources of places on openbible.info. The atlas links to chapters, never to single verses.
- **Sources in lists.** A list item carries its first source and `sources_total`; `get_record` returns them all, 50 at a time.
- **Paging.** List tools take `limit` (1 to 50, default 20) and `offset`, and answer with `total`, `offset`, `limit`, `next_offset` (absent on the last page), `results` and `dataset` (the build date of the data that answered).
- **Errors.** Nothing found is a normal answer with `total: 0`. A bad parameter is a tool error that names the parameter, the value and what is accepted. An unknown id suggests close ids.

## Tools

| Tool | Parameters | Answers |
| --- | --- | --- |
| `search` | `query` (required, 1 to 100 characters), `types`, `limit` (1 to 50, default 10), `offset` | Records whose name matches, best first, each with `score` and `matched` (`name` or `summary`). A shared person name adds `same_name_count`. |
| `get_record` | `type`, `id` (both required; id case is ignored), `relations_offset` (person only), `sources_offset` | One record in full. Its `sources` come 50 at a time, with `sources_total` and `next_sources_offset` when there are more. See below. |
| `list_records` | `type` (any type but `event`), `kind`, `limit`, `offset` | The small collections in full. `kind` is required for `person` (an office such as `high_priest`) and `place` (a place kind such as `rio`), optional for `period` (`rey`, `potencia`, `era`...). An unknown `kind` lists the values present. |
| `list_events` | `from_year`, `to_year`, `person`, `place`, `role`, `kind`, `book`, `anchored_only`, `limit`, `offset` | Events in date order. Filters combine; at least one of year, person, place, kind or book is required. `book` matches the events of the book's own story, marked `in_story: true`, and also the events that only cite a passage of it; with no year, the story comes first in story order. |
| `people_in_year` | `year` (required, -4100 to 500), `limit`, `offset` | People with the `basis` of the claim and its `evidence`, plus `periods`: every period covering the year. The bases, strongest first: `birth_and_death` (dated birth and death span the year), `dated` (the atlas's date for the person covers it; that date is the known activity, which may be a whole life, a reign or a ministry), `office` and `event`. Only `birth_and_death` bounds the life. |
| `lookup_passage` | `reference` (required), `limit`, `offset` | Events whose passages overlap the reference, their `people` and `places`, `cited_by` (other records that cite the chapter), the `passage` with its wol.jw.org `read_url`, and `fully_read` with a `notice` when the atlas has not read the book verse by verse yet. |
| `find_connection` | `from`, `to` (person ids, required), `max_steps` (1 to 12, default 8) | `connected`, the shortest chain as `steps` (each with the atlas's Spanish `phrase`, `word` and `word_of`, and the Bible `reference`), and the events both take part in. |
| `places_near` | `place`, or `lat` and `lon`; `radius_km` (default 50, at most 2000), `kind`, `limit`, `offset` | Places with a known point, nearest first, with `distance_km`, plus `without_coordinates`: how many atlas places have no point. |
| `dataset_info` | none | Format, build date, source, freshness (`loaded_at`, `checked_at`, `stale`, `refresh_error`), record counts, and the books read and not yet read. |

### `get_record` by type

Every record returns `type`, `id`, `name`, `url`, `summary`, `status`, `checked_on`, `reason`, `links`, `sources`, `not_claimed`, `note`, `alternatives` and `history` when it has them. Each type then adds its own fields:

| Type | Adds |
| --- | --- |
| `person` | `other_names`, `date`, `disambiguation`, `not_to_confuse_with`, `family` (`parents`, `children`, `spouses`, `siblings`, built from relations in both directions), `relations` (both directions, 30 per answer, each with `other`, `phrase`, `type`, `word`, `word_of`, `date`, `reference`, `status`, its first source and `sources_total`; `relations_total` and `next_relations_offset` when more follow, read with `relations_offset`), `offices`, the first 20 `events` with `events_total`, `letters` and `journeys` with the person's part in each |
| `place` | `other_names`, `kind`, `lat`, `lon`, `precision`, `coord_source`, `coord_note`, `candidates` (proposed sites when the site is unknown), `people` born, living and dying there, the first 20 `events` (`main: true` when the place is the main one), `letters`, `journey_stops`, `finds` and `seat_of` |
| `event` | `date`, `places` (the first is where the main action happens), `people`, `present`, `roles`, `passages` with chapter links, `kind`, `narrative_order` and the `periods` that list it |
| `period` | `kind`, `date`, `ruler`, `office`, `places` (the first is the seat), `events` and `attested_from` |
| `letter` | `book`, `writer`, `reference`, `written_in`, `date`, `recipients`, `people`, `carriers`, `origin_context` and `destination_context` |
| `journey` | `traveller`, `reference`, `date`, `companions` and every stop with its place and coordinates |
| `find` | `found_at`, `relates_to`, `object_date` and `identification` |
| `tour` | its stops, each with `about`, `year`, `text`, `passages`, `question` and `not_known` |
| `book` | `number`, `abbreviation`, `chapters`, `verses_per_chapter`, `writer`, `written_in`, `written`, `covers`, `omitted_verses`, `coverage`, `fully_read`, `letter` and `read_url` |
| `month` | `order`, `other_names`, `names_by_era`, `equivalent`, `festivals`, `weather` and `field` |
| `calendar_topic` | `text` |

## Bible references

`lookup_passage` and the `book` filter of `list_events` read Spanish book names in any written form the atlas knows: the name, its abbreviation, its slug or another form, with or without accents and dots. `Hch`, `Hechos`, `HCH.` and `hechos` all name Acts; `2 Reyes`, `2Re` and `2-reyes` all name 2 Kings.

| Reference | Means |
| --- | --- |
| `Rut` | the whole book |
| `Hch 16` | chapter 16 |
| `Gé 5-7` | chapters 5 to 7 |
| `Hch 16:1` | one verse |
| `Gé 2:7, 8` | verses 7 to 8 (items that touch join) |
| `Hch 13:1–14:28` | across chapters |
| `Flm 5` | verse 5 of a one-chapter book |

A tool call takes one passage with no gaps. A reference with `;`, or a comma list with a gap such as `Hch 16:1-3, 6`, is an error that lists the passages; ask for each one in its own call. `Jn 3, 16` is an error too, since it could mean chapter 3, verse 16 or chapters 3 to 16: write `Jn 3:16` or `Jn 3-16`. Event passages in the data keep their gaps the same way, so `Gé 6:1, 2, 4` never matches verse 3.

## Relation words

A relation is stored once, on one of its two people, and its `word` (`mother`, `father`, `master`...) names the role of the person it points at. Seen from the other side, the same `word` names the person looking. `word_of` says which one it is: in `get_record`, `other` or `self`; in a `find_connection` step, `to` or `from`. `phrase` is already worded for the card it appears on.

## An example

`find_connection` with `from: "loida"` and `to: "pablo"` answers, trimmed:

```json
{
  "connected": true,
  "steps": [
    {"from": {"type": "person", "id": "loida"}, "to": {"type": "person", "id": "timoteo"}, "type": "kin", "word": "grandmother", "word_of": "from", "reference": "2Ti 1:5"},
    {"from": {"type": "person", "id": "timoteo"}, "to": {"type": "person", "id": "pablo"}, "type": "accompanies"}
  ],
  "shared_events_total": 0
}
```

`word_of: "from"` says Loida is the grandmother. Each step also carries the atlas's Spanish `phrase`, the `status` and the `sources` with their links.
