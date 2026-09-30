# biblical-atlas-mcp design

An MCP server for [biblical-atlas](https://github.com/GeiserX/biblical-atlas), the Spanish Bible atlas at <https://biblical-atlas.geiser.cloud/>. The server ships no atlas data. It downloads the atlas's one public data file at run time, keeps it in memory, and answers tool calls from it. Every answer carries the atlas id, a link to the card on the site where the record has one, and the sources with their links, most of them on wol.jw.org.

The code, tool names, descriptions and docs are English. The values the tools return are Spanish, exactly as the atlas has them. The server never translates or rewrites a value.

## 1. Decisions in short

- Go, `github.com/mark3labs/mcp-go` v1.1.1, `go 1.27`. One more direct dependency, `golang.org/x/text`, for Unicode normalisation. No godotenv.
- Nine tools, all read only. No resources and no prompts.
- One data file, held in memory as an immutable snapshot with indexes built once per load. No database.
- Output keys are English and stable. Output values are the atlas's own.
- Years use one convention in every input and every output: a signed historical year, negative for a.e.c., no year zero.
- Transports are stdio and streamable HTTP, chosen with `TRANSPORT`.
- We publish release binaries, an npm wrapper, and a multi-arch image on GHCR and Docker Hub. Image tags are semver only. We publish no `latest` tag.
- GoReleaser builds binaries and the GitHub release only. A separate job builds the images. Steps that need setup the repo does not have yet sit behind repository variables, so the release stays green without them.

## 2. The data file

The default source is `https://biblical-atlas.geiser.cloud/data.json`. It is about 7 MB of JSON, 1.1 MB gzipped, with an ETag and `cache-control: max-age=600`. The top-level keys we read are `formato`, `generado`, `fuentes`, `libros`, `calendario`, `lugares`, `personas`, `viajes`, `cartas`, `eventos`, `periodos`, `hallazgos`, `recorridos` and `cobertura`.

Facts the design depends on:

- `formato` is `biblical-atlas/v0`; older atlas builds write the same format as `biblical-earth/v0`. The atlas adds fields without changing it, so the decoder ignores unknown keys and treats almost every field as optional.
- Ids are unique inside a collection and collide across collections. 24 ids are both a person and a place. Every reference the server takes or returns carries a type.
- A relation is stored once, on one of the two people. The other person's view needs a reverse index. Places store no back-links at all.
- Relations carry both old Spanish fields and newer English ones. The atlas plans to drop the Spanish ones. The decoder reads the English field first and falls back to the Spanish one: `type` then `tipo`, `word` then `relacion`, `status` then `estado`, `checked_on` then `consultado`.
- Dates store astronomical years, where 0 is 1 a.e.c. Each date also has `texto`, the ready Spanish rendering, which holds nuance the numbers lack.
- `stats.json` adds nothing the ETag does not give us. We do not fetch it.

## 3. Conventions shared by every tool

### 3.1 Record types

The tool vocabulary is English. The site's selection prefix is Spanish. The mapping is fixed:

| `type` | Collection | Site prefix | Card link |
| --- | --- | --- | --- |
| `person` | `personas` | `persona` | yes |
| `place` | `lugares` | `lugar` | yes |
| `event` | `eventos` | `evento` | yes |
| `period` | `periodos` | `periodo` | yes |
| `letter` | `cartas` | `carta` | yes |
| `journey` | `viajes` | `viaje` | yes |
| `find` | `hallazgos` | `hallazgo` | yes |
| `tour` | `recorridos` | `recorrido` | yes |
| `book` | `libros`, id is the slug | `libro` | yes |
| `month` | `calendario.meses` | none | no |
| `calendar_topic` | `calendario.explicacion` | none | no |

The card link is `https://biblical-atlas.geiser.cloud/#sel=<prefix>:<id>`, for example `https://biblical-atlas.geiser.cloud/#sel=persona:pablo`. The site reads `sel` from the hash and fills the other view parameters with defaults. A chapter links as `#sel=pasaje:<abbr>-<chapter>`, where `<abbr>` is the normalised book abbreviation, for example `#sel=pasaje:hch-16`. The site base in card links is a constant. It does not follow `BIBLICAL_ATLAS_DATA_URL`.

### 3.2 Years

Inputs and outputs use a signed historical year. `-607` is 607 a.e.c. and `33` is 33 e.c. `0` is invalid. The server converts at the edge: astronomical `y <= 0` becomes `y - 1`, and historical `h < 0` becomes `h + 1`. The model never sees an astronomical year.

A date covers every year from `from` to `to`. A missing `from` means open to the past and a missing `to` means open to the future. A year or range matches a date when the two intervals overlap.

### 3.3 Shared output objects

All results are one compact JSON object in a single text content item. Keys with no value are left out. There are no nulls and no empty arrays, except `results`, which is always present.

`ref` points at a record:

```json
{"type":"person","id":"timoteo","name":"Timoteo","url":"https://biblical-atlas.geiser.cloud/#sel=persona:timoteo"}
```

`date`:

```json
{"text":"c. 607 a.e.c.","from":-607,"to":-607,"approx":true,"kind":"anclada","precision":"año","month":"nisan","day":14,"season":"primavera","note":"..."}
```

`text` is `fecha.texto` verbatim. `kind` is `fecha.tipo` and `precision` is `fecha.precision`, both verbatim. `month`, `day` and `season` come from `fecha.detalle`. `chronology` appears only when it is not `tnm`.

`source`:

```json
{"id":"it-timoteo","title":"...","work":"...","url":"https://wol.jw.org/es/wol/d/r4/lp-s/..."}
```

Sources are resolved from the record's `fuentes`. Two source ids with the same URL collapse into the first. An id missing from `fuentes` is skipped.

`item` is the short form used in lists: `type`, `id`, `name`, `url`, then `date`, `summary`, `disambiguation`, `status`, `sources` and `sources_total` when they apply. `name` is `nombre`, `titulo` or `libro`, whichever the record has. `summary` is `resumen` whole, never cut. `sources` in an item holds 1 entry, the first of the record's `enlaces`, or its first `fuentes` entry when it has no `enlaces`. `sources_total` says how many the full record has. One source keeps a page of 50 items well inside the tool-output limit of common clients; `get_record` returns them all, 50 at a time.

### 3.4 Paging

Every list tool takes `limit` and `offset` and returns:

```json
{"total":127,"offset":0,"limit":20,"next_offset":20,"results":[]}
```

`limit` defaults to 20 and accepts 1 to 50. `offset` defaults to 0. `next_offset` is absent on the last page. Order is always deterministic, so paging is stable while the snapshot stays the same. Every response also carries `dataset`, the `generado` date of the snapshot that answered, so a caller can tell when paging crossed a refresh.

### 3.5 Errors and empty results

- Nothing matched: a normal result with `total: 0` and `results: []`. Never an error.
- Bad input: a tool error (`isError: true`) whose text names the parameter, the value received and what is accepted. Example: `limit must be a whole number from 1 to 50, got 250`. The server never falls back to a default on a bad value.
- Unknown id in `get_record`, or in any `person` or `place` parameter: a tool error with up to 5 suggestions from the search index. Example: `no person with id "pablo-apostol". Did you mean: pablo (Pablo)?`
- Data not available, which can only happen when the first load fails and there is no cached copy: a tool error with the cause and the URL tried. The next call retries.
- Handlers return `mcp.NewToolResultError(msg), nil`. They never return a Go error.

Whole-number parameters arrive as JSON floats. One helper, `wholeNumber(args, key, min, max)`, rejects NaN, infinities, fractions and values outside the range.

### 3.6 Annotations and server instructions

Every tool gets `ReadOnlyHint: true`, `DestructiveHint: false`, `IdempotentHint: true`, `OpenWorldHint: false` and a title. One helper sets all four.

The server passes this text through `server.WithInstructions`:

```text
This server answers from biblical-atlas, a Spanish Bible atlas (https://biblical-atlas.geiser.cloud/).
Names, summaries and phrases come back in Spanish as the atlas has them. Quote them or translate them yourself; say so when you translate.
Records have a type and an id. Ids repeat across types, so always pass both. Find ids with search.
Years are signed historical years: -607 is 607 a.e.c. (BCE), 33 is 33 e.c. (CE). There is no year 0.
A date with kind "narrativa" is placed only by the order of the story, not by a dated source. Say so when you report it.
status "pendiente" means the atlas has not yet confirmed the claim against its source.
Every result includes sources with links. Cite them. The atlas links to Bible chapters, never to single verses.
A place without lat/lon has an unknown site; its candidates list the proposed sites.
```

## 4. Tools

### 4.1 `search`

Description: `Find people, places, events, periods, letters, journeys, archaeological finds, tours, Bible books and Hebrew months in the atlas by name. Use this first to get the type and id that every other tool needs. Matching ignores accents and case. Names are Spanish ("Pablo", "Jerusalén", "Diluvio"). Homonyms come back as separate results, each with a disambiguation line.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `query` | string | required | 1 to 100 characters after trimming |
| `types` | array of string | all types | Values from the `type` column in 3.1 |
| `limit` | integer | 10 | 1 to 50 |
| `offset` | integer | 0 | 0 or more |

Text searched per type:

- `person`: `nombre` and `nombres[].nombre`
- `place`: `nombre`, `nombres[].nombre` and `candidatos[].nombre`
- `event`: `titulo` and `buscar`
- `period`: `nombre` and `buscar`
- `letter`: `libro`, plus the book's `abr` and `formas`
- `journey`, `find`: `nombre`
- `tour`: `titulo`
- `book`: `nombre`, `abr` and `formas`
- `month`: `nombre`, `otros_nombres` and `nombres[].nombre`

Scoring takes the best score over a record's texts, with query and text both normalised as in 5.5: 100 for an exact match, 80 for a prefix, 60 when any word of the text starts with the query, 30 for a substring when the query has 3 or more characters. One further tier scores 20 when every word of the query, 4 or more characters in total, appears in the record's `resumen`. Order is score descending, then the type order of 3.1, then name, then id.

Output: the paging object. Each result is an `item` plus `score` and `matched`, which is `name` or `summary`. A person whose name is shared also gets `same_name_count`.

### 4.2 `get_record`

Description: `Get one atlas record in full: a person with family, relations, offices and events; a place with coordinates, candidate sites and what happened there; an event with its date, people, places and Bible passages; or a period, letter, journey with its stops, find, tour, Bible book, Hebrew month or calendar topic. Needs the type and the id from search. Returns the record's sources with their links, 50 at a time; a person's relations carry their first source and a count, and come 30 at a time.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `type` | string | required | One of the types in 3.1 |
| `id` | string | required | Exact id. Case is ignored. |
| `relations_offset` | integer | 0 | Person only; any other type rejects it. Relations to skip. |
| `sources_offset` | integer | 0 | The record's sources to skip. |

Every record returns `type`, `id`, `name`, `url`, `summary`, `status`, `checked_on`, `reason` (`razon`), `links` (`enlaces` as `{title,url,kind}`), `sources` (50 per answer from `sources_offset`, with `sources_total` when the answer does not hold them all and `next_sources_offset` when more follow; the most cited places and people have well over 100, most of them Bible chapters), `not_claimed` (`no_afirmamos`), `note`, `alternatives` (each a `date` plus `note` and `sources`) and `history` (each `{date,change,source}`). Then, by type:

**person.** `other_names` (`nombres` as `{name,note}`), `date`, `disambiguation`, `not_to_confuse_with` (refs), `family`, `relations`, `offices`, `events`, `events_total`, `letters`, `journeys`.

- `family` is `{parents, children, spouses, siblings}`, each a list of refs. A relation stored on this person puts its target in the group named by `family`. A relation stored on someone else and pointing here puts that person in the group named by `inverse_family`.
- `relations` lists the relations in both directions, 30 per answer from `relations_offset`, with `relations_total` when the answer does not hold them all and `next_relations_offset` when more follow. Each entry has `other` (a ref to a person or a place), `phrase`, `type`, `word`, `word_of`, `date`, `reference`, `inferred` (`deducido`), `status`, `certainty`, and `sources` with its first source only plus `sources_total` when it has more. `phrase` is the atlas's Spanish wording for this card, `verb` when the relation is stored here and `inverse_verb` when it is stored on the other person. `word` is the atlas's role word for the person the stored relation points at (`persona`), so read alone it can name either side: `word_of` is `other` when the relation is stored on this person and `self` when it is stored on the other one. A relation stored on Obed pointing at Rut with `word: mother` shows on Rut's card as `other: obed, word: mother, word_of: self`. Order is by type in this fixed sequence: `kin`, `same_as`, `succeeds`, `disciple_of`, `accompanies`, `tie`, `appears_to`, `born_in`, `lived_in`, `died_in`, then any unknown type, and inside a type by the other record's name.
- `offices` entries: `office`, `label` (`es`), `place` (ref), `date`, `inferred`, `status`, `sources`.
- `events` holds the first 20 events that list the person, in file order, as items with the person's `role` when `roles` has one. `list_events` with `person` pages the rest.
- `letters` are refs to letters the person wrote, received, carried or is named in, each with `as`: `writer`, `recipient`, `carrier` or `named`. `journeys` are refs with `as`: `traveller` or `companion`.

**place.** `other_names` (with `from` and `to` years when dated), `kind` (`tipo`), `lat`, `lon`, `precision`, `coord_source` (`{id,url}`), `coord_note`, `candidates`, `people`, `events`, `events_total`, `letters`, `journey_stops`, `finds`, `seat_of`.

- `candidates` entries: `name`, `status` (`estado`), `shape` (`geometria.tipo`), `lat`, `lon`, `radius_km`, `to` (`{lat,lon}`), `reason`, `note`, `sources`.
- `people` is `{born, lived, died}`, refs from the reverse index of `born_in`, `lived_in` and `died_in`, at most 30 per group, with `born_total`, `lived_total` and `died_total`.
- `events` holds the first 20 events whose `lugares` include the place. Each item has `main: true` when the place is first in the list.
- `letters` are refs with `as`: `written_here` or `sent_here`. `journey_stops` entries: `journey` (ref), `order`, `date`, `reference`. `finds` and `seat_of` are refs to finds and periods.

**event.** `date`, `places` (refs, the first is where the main action happens), `people` (refs), `present` (refs), `roles` (each `{person, role, date, place}`), `passages`, `kind` (`type`), `narrative_order` (`{series, order, after}` with `after` a ref), `periods` (refs to periods that list it in `sucesos`). `passages` entries are `{text, url}`, where `text` is the citation verbatim and `url` is the wol.jw.org link to its first chapter.

**period.** `kind` (`tipo`), `date`, `ruler` (ref), `office`, `places` (refs, the first is the seat), `events` (refs from `sucesos`), `attested_from` (`consta_desde`).

**letter.** `book` (ref), `writer` (ref), `reference`, `written_in` (refs, the first is preferred), `date`, `recipients` (`{text, places, people}`), `people`, `carriers`, `origin_context` and `destination_context` (each `{summary, sources}`).

**journey.** `traveller` (ref), `reference`, `date`, `companions` (refs), `stops`. Each stop: `order`, `place` (ref with `lat` and `lon`), `reference`, `date`, `note`, `status`, `sources`. Stops are never cut. The longest journey has fewer than 40.

**find.** `found_at` (ref), `relates_to` (refs parsed from `relaciona`), `object_date`, `identification`.

**tour.** `stops`, each `{about, year, text, passages, question, not_known}`. `about` is a ref parsed from `sel`. A `pasaje:` selection becomes `{type:"passage", id, url}`. `year` is the signed historical year of `floor(t)`. `question` is `{text, options, answer, explanation}`.

**book.** `number`, `abbreviation`, `chapters`, `verses_per_chapter`, `writer`, `written_in` (`lugar`), `written` (date), `covers` (date), `omitted_verses`, `coverage` (`{chapters, chapters_read, verses, verses_read}`, absent when the atlas has not read the book verse by verse), `letter` (ref, when the book is a letter), `url` (the card) and `read_url` (chapter 1 on wol.jw.org).

**month.** `order`, `other_names`, `names_by_era`, `equivalent` (`equivale`), `festivals` (each `{name, from_day, to_day, instituted, sources}`), `weather`, `field`.

**calendar_topic.** `text`.

### 4.3 `list_records`

Description: `List the atlas's small collections in full: periods (reigns, empires, eras, high priests, governors), letters, journeys, archaeological finds, guided tours, Bible books, Hebrew months and calendar topics. Also lists places of one kind (for example every river) and people who held one office (for example every high priest). Use search when you have a name, and list_events for events.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `type` | string | required | Any type in 3.1 except `event` |
| `kind` | string | none | Required for `person` and `place`. Optional for `period`. Rejected for other types. |
| `limit`, `offset` | integer | 20, 0 | As in 3.4 |

`kind` means `offices[].office` for a person, `lugares[].tipo` for a place and `periodos[].tipo` for a period. Accepted values are whatever the loaded file contains. An unknown value is a tool error that lists the values present. Order is file order, except people, which sort by the office's start year and then by name. Results are items. A person item adds the matching `office` entry, with its first source and `sources_total`, a period item adds `kind` and `ruler`, a place item adds `kind`, `lat` and `lon`, a book item adds `number`, `abbreviation`, `chapters` and `fully_read`.

### 4.4 `list_events`

Description: `List events in date order, filtered by any mix of year or year range, person, place, kind of event and Bible book. Answers "what happened in 607 a.e.c.", "what happened at Jerusalén", "what did Pablo take part in" and "which events does the book of Rut tell or cite" (in_story marks the ones its own story tells). Years are signed: -607 is 607 a.e.c., 33 is 33 e.c. At least one filter is required.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `from_year` | integer | none | -4100 to 500, not 0 |
| `to_year` | integer | `from_year` | Same range, not before `from_year` |
| `person` | string | none | Person id |
| `place` | string | none | Place id |
| `role` | string | none | Needs `person`. One of the roles present in the file, today `born`, `died`, `spoke`, `wrote`, `raised`. |
| `kind` | string | none | `eventos[].type`, today `speech`, `death`, `birth`, `writing` |
| `book` | string | none | Book name, abbreviation or slug. Matches the events of the book's story (`orden_relato.serie` is the slug or starts with `<slug>-`) and the events with a passage in the book. |
| `anchored_only` | boolean | false | Drops events whose date kind is `narrativa` |
| `limit`, `offset` | integer | 20, 0 | As in 3.4 |

`to_year` without `from_year` is an error. Filters combine with AND. Order is file order, which is by date, except when `book` is set and no year is given. Then the book's story comes first in `orden_relato.orden`, and the events that only cite the book follow in file order. The wider match is on purpose: the Gospels have no story series, and a strict series match would return nothing for them. Results are items with `date`, `places` (refs), `people_total`, `passages` (the citation strings) and `kind`. With `book`, an item of the book's story adds `in_story: true`, so a caller can tell what the book narrates from what only cites it. With `person`, the item adds `role` and `present`. With `place`, it adds `main`.

The year range runs to 500 because the atlas's periods reach 476 e.c.; a narrower range would leave them unreachable.

### 4.5 `people_in_year`

Description: `Who was alive, ruling or active in a given year, and which reigns, empires and eras covered it. Each person comes with the basis of the claim: "birth_and_death" (dated birth and death events span the year), "dated" (the atlas's date for the person covers the year; that date is the person's known activity, which may be the whole life, a reign or a ministry), "office" (an office held that year) or "event" (an event dated to that year). Only "birth_and_death" bounds the life; the others show the person was alive and active that year. Years are signed: -607 is 607 a.e.c.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `year` | integer | required | -4100 to 500, not 0 |
| `limit`, `offset` | integer | 20, 0 | Page the `results` list only |

Output: the paging object plus `periods`, the items of every period whose date covers the year, never paged. Each result is a person item plus `basis` and `evidence`. A person appears once, with the strongest basis that applies:

1. `birth_and_death`: the person has a `born` role and a `died` role, and the year lies between them. `evidence` is `{born, died}`, two dates.
2. `dated`: the person's `fecha` covers the year. `evidence` is that date. The atlas's `fecha` for a person is the person's known activity, drawn on the site as the activity bar: a whole life for some, a reign for a king, a ministry for a prophet or an apostle. It is not a lifespan, so it ranks below `birth_and_death`.
3. `office`: an entry in `offices` has a date that covers the year. `evidence` is the office entry.
4. `event`: the person is in an event whose date covers the year and whose kind is not `narrativa`. `evidence` is a ref to the first such event.

Order is basis in the order above, then name, then id. An `office` in `evidence` carries its first source and `sources_total`. The tool description says that only `birth_and_death` bounds the life.

### 4.6 `lookup_passage`

Description: `What the atlas has on a Bible passage: the events whose passages overlap it, the people and places in those events, and the letters, journey stops and other records that cite the chapter. Takes a Spanish reference such as "Hch 16:1", "Hechos 16", "2 Reyes 3", "Gé 2:7, 8", "Hch 13:1–14:28" or just a book, "Rut". Returns the link to read the chapter on wol.jw.org.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `reference` | string | required | One reference, 1 to 80 characters, parsed as in 5.6 |
| `limit`, `offset` | integer | 20, 0 | Page the `results` list of events only |

Output: the paging object plus:

- `passage`: `{book (ref), chapter, verse, to_chapter, to_verse, text, url, read_url}`. `text` is the normalised reference in the atlas's own abbreviation. `read_url` is the wol.jw.org chapter link. `url` is the site's `pasaje` card and is absent for a whole book or a span of chapters.
- `fully_read`: whether `cobertura` lists the book as complete. When false the response adds `notice`: `The atlas has not yet read this book verse by verse. Absence of an event here does not mean the passage has none.`
- `results`: event items whose parsed `pasajes` overlap the reference, in file order. An event passage with no verses covers its whole chapter. A reference with no verses covers the whole chapter. A reference with only a book covers every chapter.
- `people` and `places`: refs drawn from the union of `personas` and `lugares` over all matching events, not only the page, at most 30 each, ordered by how many matching events name them (a person named twice in one event counts once), with `people_total` and `places_total`.
- `cited_by`: refs to non-event records tied to any chapter in the reference, at most 30, with `cited_by_total`. A record counts when its `fuentes` include a chapter source for that chapter, or when one of its structured citation fields parses to it. Those fields are a letter's `referencia`, a journey's and a stop's `referencia`, a relation's `reference` and a tour stop's `pasajes`. We do not scan free text such as `razon`.

An unknown book, a chapter beyond `capitulos` or a verse beyond the chapter's last verse is a tool error that says which part failed and, for an unknown book, lists up to 5 close book names. So is a reference with a gap, `Hch 16:1-3, 6`, which names two passages: the error lists them and asks for each one separately. `Jn 3, 16`, a comma between two bare numbers, is an error that says to write `Jn 3:16` for the verse or `Jn 3-16` for the chapters, since the server cannot tell which one was meant.

### 4.7 `find_connection`

Description: `How two people are connected: the shortest chain of family ties, succession, discipleship and companionship between them, one step at a time with the atlas's Spanish phrase and the Bible reference behind each step, plus the events both take part in. Use for "how is Rut related to David" or "did Timoteo know Pedro".`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `from` | string | required | Person id |
| `to` | string | required | Person id, different from `from` |
| `max_steps` | integer | 8 | 1 to 12 |

The graph has one undirected edge per person-to-person relation. Relations to places are left out. The search is breadth first, with neighbours visited in ascending order of relation `key`, so the same snapshot always gives the same path.

Output: `from` and `to` (refs), `connected` (boolean), `steps`, `shared_events` (the first 10 event items that list both people) and `shared_events_total`. Each step has `from`, `to` (refs), `phrase`, `type`, `word`, `word_of`, `date`, `reference`, `inferred`, `status`, `certainty` and `sources` (all of them). `phrase` follows the rule of `get_record`: it is the wording the atlas shows on the `from` person's card next to `to`, `verb` when the relation is stored on `from` and `inverse_verb` when it is stored on `to`. `word_of` says which side `word` describes: `to` when the relation is stored on `from`, `from` when it is stored on `to`. From Rut to Obed the step reads `word: mother, word_of: from`. When no path exists within `max_steps`, `connected` is false, `steps` is absent and `shared_events` still answers. That is a normal result.

### 4.8 `places_near`

Description: `Places in the atlas within a distance of a place or a coordinate, nearest first. Use for "what is near Capernaum" or "which cities lie within 30 km of Jerusalén". Only places with a known point are returned; the count of places without one is reported.`

| Parameter | Type | Default | Rules |
| --- | --- | --- | --- |
| `place` | string | none | Place id. Give this or `lat` and `lon`, not both. |
| `lat`, `lon` | number | none | -90 to 90 and -180 to 180 |
| `radius_km` | number | 50 | Above 0, at most 2000 |
| `kind` | string | none | `lugares[].tipo` filter |
| `limit`, `offset` | integer | 20, 0 | As in 3.4 |

Distance is great-circle on a sphere of radius 6371 km, rounded to 0.1 km. Results are place items with `kind`, `lat`, `lon`, `precision` and `distance_km`. The origin place is left out. The response adds `origin` (`{lat, lon, place}`) and `without_coordinates`, the number of atlas places that have no point and so could not be measured. A `place` with no point is a tool error that names its candidate sites with their coordinates, so the caller can retry with `lat` and `lon`.

### 4.9 `dataset_info`

Description: `What the loaded atlas contains and how fresh it is: build date, record counts, which Bible books the atlas has read verse by verse and which it has not, and where the data came from. Call it when a user asks how complete or current the atlas is, or before treating a missing result as "the Bible does not say".`

No parameters. Output:

```json
{
  "format": "biblical-atlas/v0",
  "generated": "2026-09-30",
  "source": "https://biblical-atlas.geiser.cloud/data.json",
  "loaded_at": "2026-09-30T12:00:00Z",
  "checked_at": "2026-09-30T13:00:00Z",
  "stale": false,
  "refresh_error": "",
  "counts": {"people": 0, "places": 0, "events": 0, "periods": 0, "letters": 0, "journeys": 0, "finds": 0, "tours": 0, "books": 66, "sources": 0},
  "books_fully_read": ["genesis"],
  "books_not_yet_read": ["2-reyes"],
  "site": "https://biblical-atlas.geiser.cloud/",
  "server_version": "0.1.0"
}
```

`stale` is true when the last refresh failed and the server is answering from an older copy. `refresh_error` then holds the cause and is otherwise absent. Counts are computed from the loaded file.

### 4.10 What we left out

- A separate tool per record type. `get_record` with a type covers all eleven with one schema.
- A "who was at place X in year Y" tool. `list_events` with `place` and a year answers it from events, which is the only evidence the file has.
- A sources tool. Every result already resolves its sources.
- Free-text search over `razon`. That field is a citation trail and matching it returns noise.

## 5. Data layer

Package `internal/atlas`. No other package reads the file or the network.

### 5.1 Snapshot

`Snapshot` is immutable after build. It holds the decoded records, the indexes of 5.4, `Format`, `Generated`, `ETag`, `LoadedAt` and the source string. `Store` owns an `atomic.Pointer[Snapshot]` and the refresh state. A tool handler calls `store.Snapshot(ctx)` once and uses that one snapshot for the whole call.

### 5.2 Loading and refresh

Start-up does not block the MCP handshake. `Store.Start` does this in a goroutine:

1. If a disk cache exists for the same source URL, decode it and publish it as the snapshot. Its saved ETag and fetch time come with it. The start-up load ends there, so a call waiting for the first snapshot gets the cached one at once.
2. If there is no snapshot, fetch. If the cached one is older than the TTL, start one background refresh, the same path an old snapshot takes below, so no call waits for the download.

`store.Snapshot(ctx)`:

- With a snapshot younger than the TTL, return it.
- With an older snapshot, return it at once and start one background refresh if none is running.
- With no snapshot, wait for the load in progress, or start one, bounded by the fetch timeout and by `ctx`. On failure return the error. The next call tries again, no sooner than 5 seconds after the last failed attempt.

Fetch:

- `GET` with `User-Agent: biblical-atlas-mcp/<version>` and, when we hold an ETag, `If-None-Match`. We never set `Accept-Encoding`, so Go's transport negotiates gzip and decompresses for us.
- One `http.Client` with the configured timeout covering the whole exchange.
- `304`: keep the snapshot, set `checked_at` to now.
- `200`: read through `io.LimitReader` at 64 MB plus one byte. A body over 64 MB is an error, `data file is larger than 64 MB`. Decode, validate as in 5.3, build indexes, swap the pointer, write the disk cache.
- Any other status, a network error, a decode error, a validation error, or a panic while decoding or indexing (caught in the build step and turned into an error): keep the current snapshot, record `refresh_error`, log one line to stderr, and try again after the TTL has passed once more. We serve the old copy for as long as refreshes fail.

Only one fetch runs at a time. A mutex guards the refresh state and the swap is a single pointer store.

### 5.3 Validation

A file is accepted when all of these hold:

- It is a JSON object.
- `formato` equals `biblical-atlas/v0` or `biblical-earth/v0`, two ids of one format. Any other value fails with `unsupported data format "<value>": this server reads biblical-atlas/v0. Upgrade biblical-atlas-mcp.` A missing `formato` fails the same way with an empty value.
- `personas`, `lugares` and `fuentes` are non-empty objects, and `eventos` and `libros` are non-empty arrays.

The known-format set is one Go slice, so adding `v1` later is a one-line change plus whatever the decoder needs. Records that point at a missing id are kept. The dangling ref is dropped from the output and counted in a load-time log line. A null entry in any array is dropped. A citation in the data that does not parse, names a chapter or verse its book lacks, or ends before it starts, is skipped and counted.

### 5.4 Indexes, built once per load

- Records by id, per type.
- Search entries: `(normalised text, type, id)` for every text in 4.1, plus the normalised `resumen` per record.
- Relations by person, both directions, and by place.
- Person adjacency for `find_connection`.
- Events by person and by place, as ascending positions in the event list.
- Letters, journeys and stops, finds and period seats by place. Letters and journeys by person.
- Passage index: for every event, its parsed passage ranges. For every `(book number, chapter)`, the non-event records that cite it.
- Chapter sources: `(book number, chapter)` parsed from each source URL with `/wol/b/r4/lp-s/nwt(?:sty)?/(\d+)/(\d+)`.
- Book lookup table of 5.6.
- Life evidence per person for `people_in_year`: the `fecha` span, the `born` and `died` dates from roles, and office spans.
- Coverage sets.

Building takes well under a second for the current file. Memory is one decoded copy, two during a swap.

### 5.5 Text normalisation

`norm(s)`: Unicode NFD, drop every rune in U+0300 to U+036F, lowercase, trim, collapse runs of whitespace to one space. This is the site's own rule, so `Jerusalén`, `jerusalen` and `JERUSALEN` match, and `ñ` matches `n`. Words split on spaces and hyphens.

### 5.6 Bible references

The book table maps a key to a book. The key is `norm(s)` with all whitespace and dots removed. Each book contributes every entry of `formas`, its `abr`, its `nombre` and its slug with hyphens removed. So `Hch`, `hechos`, `Hechos` and `HCH.` reach book 44, and `2 Reyes`, `2Re` and `2-reyes` reach book 12. The table comes from the loaded file. The server hard-codes no book name.

One parser serves both the tool input and the citation fields in the data:

1. Split on `;`. A part with no book continues in the book of the part before it.
2. Per part: an optional book, which is an optional leading `1`, `2` or `3`, then words of letters, and an optional dot. The book is the longest run of leading words the table knows, so a name of any length works (`El Cantar de los Cantares` has five words). Then a chapter. Then optionally `:` and a verse, optionally followed by `, n` items and by a `-` or `–` range.
3. A range whose right side has a `:` ends in another chapter. `Hch 13:1–14:28` is chapters 13 to 14. `Gé 5-7` with no verses is chapters 5 to 7.
4. Every comma item is its own range, so a list keeps its gaps: `Gé 6:1, 2, 4` covers verses 1, 2 and 4, never 3, and `Nú 26:1-51, 57-65` leaves out 52 to 56. Items that touch or overlap the one before join it: `Gé 2:7, 8` is one range, verses 7 to 8.
5. A part with a book and no chapter is the whole book. Books with one chapter accept `Flm 5` as verse 5.

The result is a list of ranges `(book, chapter, verse) to (book, chapter, verse)`, with verse 0 standing for "whole chapter" at the start and the chapter's last verse at the end. The tool input must yield exactly one range: a gap is an error that lists the ranges, and a comma between two bare numbers (`Jn 3, 16`) is an error that shows how to write the verse and the chapter span. Data citations that fail to parse are skipped and counted in the load log. The live test asserts that count is 0 for event passages.

Chapter link: `https://wol.jw.org/es/wol/b/r4/lp-s/nwtsty/<libros[].num>/<chapter>`. The atlas links to chapters, so we do too.

### 5.7 Disk cache

Two files in the cache directory: `data.json`, the body as received, and `meta.json`, `{"url","etag","fetched_at"}`. Both are written to a temp file in the same directory and renamed. The cache is used only when `meta.url` equals the configured URL. If the directory cannot be created or written, the server logs one line and runs from memory. A cache file that fails validation is ignored and overwritten by the next good fetch.

### 5.8 Local file

With `BIBLICAL_ATLAS_DATA_FILE` set, the server reads that path and never touches the network or the disk cache. After each TTL it stats the file and reloads when the modification time or size changed. The same validation and the same keep-the-old-copy rule apply.

## 6. Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `BIBLICAL_ATLAS_DATA_URL` | `https://biblical-atlas.geiser.cloud/data.json` | Where to download the data file. Must be `http` or `https`. |
| `BIBLICAL_ATLAS_DATA_FILE` | unset | Path to a local copy. When set, no network is used and the URL is ignored. |
| `BIBLICAL_ATLAS_CACHE_DIR` | `<user cache dir>/biblical-atlas-mcp` | Where the downloaded file is kept between runs. `off` disables the disk cache. |
| `BIBLICAL_ATLAS_REFRESH` | `1h` | How long a copy is served before the server asks again. A Go duration of at least `1m`, or `0` to never refresh. |
| `BIBLICAL_ATLAS_TIMEOUT` | `30s` | Limit for one download. A Go duration from `1s` to `5m`. |
| `TRANSPORT` | unset | `stdio` for stdio. Unset or `http` for streamable HTTP. Any other value is an error. |
| `LISTEN_ADDR` | `127.0.0.1:8080` | HTTP listen address. The container image sets `0.0.0.0:8080`. |
| `MCP_AUTH_TOKEN` | unset | When set, HTTP requests need `Authorization: Bearer <token>`. |

`<user cache dir>` is `os.UserCacheDir()`. The image sets `BIBLICAL_ATLAS_CACHE_DIR=/tmp/biblical-atlas-mcp`.

A value that does not parse stops the server at start with exit code 2 and a message naming the variable. Flags: `--version` or `-v` prints `version.String()`, `--help` or `-h` prints the variable table. Both exit 0 before anything else runs. Any other argument exits 2.

The size cap, the retry pause and the site base for card links are constants.

## 7. Transports

`cmd/server/main.go` is `func main() { os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }`. `run` holds everything, so tests call it directly.

- The server is `server.NewMCPServer("biblical-atlas-mcp", version.Version, server.WithToolCapabilities(false), server.WithRecovery(), server.WithInstructions(tools.Instructions))`.
- `tools.Register(s, store)` loops over one `[]Tool` slice. A test walks the same slice.
- **stdio.** `server.NewStdioServer`, logs to stderr only.
- **HTTP.** `server.NewStreamableHTTPServer` on `/mcp`, stateless, since no tool keeps per-session state, and with the `GET` event stream turned off (`405`), since the server never sends a notification. A request body over 1 MB gets `413` before the MCP handler reads it; a tool call is a few hundred bytes. The `http.Server` sets a 10 second header timeout, a 30 second read timeout, a write timeout of the download timeout plus 30 seconds (a call can wait for the first download), and a 2 minute idle timeout. `GET /healthz` returns 200 and `ok` with no auth and no data access. `signal.NotifyContext` on SIGINT and SIGTERM, then `Shutdown` with a 5 second limit.
- **Auth.** The data is public, so a token is optional on every address. When `MCP_AUTH_TOKEN` is set, `/mcp` sits behind a constant-time bearer check. Put the server behind a reverse proxy if it is exposed beyond one machine.

## 8. Packaging and release

### 8.1 What we publish

| Artifact | Where | Tag or version |
| --- | --- | --- |
| Binaries for linux, darwin and windows on amd64 and arm64, plus `checksums.txt` | GitHub release | `vX.Y.Z` |
| Image, linux/amd64 and linux/arm64 | `ghcr.io/geiserx/biblical-atlas-mcp` | `vX.Y.Z` only |
| Same image | Docker Hub, `<namespace>/biblical-atlas-mcp` | `vX.Y.Z` only |
| npm package `biblical-atlas-mcp` | npmjs.com | `X.Y.Z` |
| Registry entry `io.github.GeiserX/biblical-atlas-mcp` | Official MCP registry | `X.Y.Z` |

`<namespace>` is the repository variable `DOCKERHUB_NAMESPACE`. See open question 1.

### 8.2 GoReleaser

`.goreleaser.yaml` uses `version: 2`. One build, `id: bin`, `main: ./cmd/server`, `binary: biblical-atlas-mcp`, `CGO_ENABLED=0`, the three systems and two architectures, ldflags `-s -w` plus `-X github.com/geiserx/biblical-atlas-mcp/version.Version`, `.Commit` and `.Date`. Archives: `ids: [bin]`, `formats: [tar.gz]`, zip on windows, `name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"`. `checksum.name_template: checksums.txt`. `snapshot.version_template: "{{ .Version }}-snapshot-{{ .ShortCommit }}"`. Changelog groups for `feat` and `fix`.

GoReleaser does not build images. Its image step builds from a reduced file list that silently drops any new package directory, and that failure shows only on a tag. A plain buildx job uses the whole checkout and runs on every pull request too.

### 8.3 Dockerfile

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.27 AS builder
ARG TARGETOS TARGETARCH VERSION=dev COMMIT=none DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags "-s -w -X github.com/geiserx/biblical-atlas-mcp/version.Version=${VERSION} -X github.com/geiserx/biblical-atlas-mcp/version.Commit=${COMMIT} -X github.com/geiserx/biblical-atlas-mcp/version.Date=${DATE}" \
    -o /out/biblical-atlas-mcp ./cmd/server

FROM alpine:3.24
LABEL io.modelcontextprotocol.server.name="io.github.GeiserX/biblical-atlas-mcp"
COPY --from=builder /out/biblical-atlas-mcp /usr/local/bin/biblical-atlas-mcp
USER 65534:65534
EXPOSE 8080
ENV LISTEN_ADDR=0.0.0.0:8080
ENV BIBLICAL_ATLAS_CACHE_DIR=/tmp/biblical-atlas-mcp
ENTRYPOINT ["/usr/local/bin/biblical-atlas-mcp"]
```

The builder stage runs on the build machine's architecture and cross-compiles, so the arm64 image needs no emulation. `alpine:3.24` carries the CA certificates the download needs. A `.dockerignore` keeps `dist/`, `node_modules/` and `.git/` out of the context.

### 8.4 npm wrapper

`package.json`: name `biblical-atlas-mcp`, `mcpName` `io.github.GeiserX/biblical-atlas-mcp`, license `MIT`, `bin: {"biblical-atlas-mcp": "run.js"}` with no `./`, `files: ["run.js", "postinstall.js"]`, `engines.node >= 18`, `repository`, `homepage`, `bugs`, keywords including `mcp-server`. No dependencies.

`postinstall.js` maps platform and architecture to the GoReleaser archive name, downloads it and `checksums.txt` from the GitHub release for the package version over HTTPS only, follows at most 10 redirects resolved with `new URL(location, url)`, times out at 30 seconds, verifies SHA-256, extracts with `execFileSync("tar", [...])` or PowerShell `Expand-Archive` on Windows, sets mode 755 and deletes the archive in a `finally`.

`run.js` spawns the binary with `process.argv.slice(2)`, `stdio: "inherit"` and the environment plus `TRANSPORT=stdio`. It forwards SIGINT, SIGTERM and SIGHUP and exits with the child's code, or 128 plus the signal number.

### 8.5 Registry files

`server.json` uses schema `2025-12-11`, name `io.github.GeiserX/biblical-atlas-mcp`, a description under 100 characters, and one package: npm, `transport: stdio`, with `BIBLICAL_ATLAS_DATA_URL`, `BIBLICAL_ATLAS_DATA_FILE`, `BIBLICAL_ATLAS_CACHE_DIR` and `BIBLICAL_ATLAS_REFRESH` listed as optional and not secret. No OCI package. The image starts in HTTP mode, and an entry that claims stdio for it would be false.

`glama.json` is `{"$schema":"https://glama.ai/mcp/schemas/server.json","maintainers":["GeiserX"]}`.

The version in `package.json` and both versions in `server.json` always match. A test reads the two files and fails when they differ.

### 8.6 Workflows

All jobs run on `ubuntu-latest`, since the repo is public. Every workflow sets top-level `permissions: contents: read` and every checkout sets `persist-credentials: false`. Actions are pinned by major tag.

**ci.yml**, on push to main, pull requests to main and manual dispatch:

- `test`: checkout, `setup-go` with `go-version-file: go.mod`, the gofmt gate `test -z "$(gofmt -l .)"`, `go vet ./...`, `go test -race -coverprofile=coverage.out ./...`, `node --test postinstall.test.js` for the npm wrapper's download checks, then the Codecov upload on pushes with `fail_ci_if_error: false`.
- `build`: `goreleaser-action@v7` with `version: "~> v2"` and `release --snapshot --clean --skip=publish`. Then a script asserts that the six archives `postinstall.js` asks for exist in `dist/` and appear in `checksums.txt`, and `npm pack --dry-run` lists exactly `package.json`, `run.js`, `postinstall.js`, `README.md` and `LICENSE`.
- `image`: buildx build of `linux/amd64,linux/arm64` with `push: false` and the GitHub Actions cache.

**release.yml**, on tags `v*.*.*`:

- `goreleaser`: `contents: write`, checkout with `fetch-depth: 0`, Go from `go.mod`, GoReleaser `~> v2` with `release --clean`.
- `image`: `contents: read`, `packages: write`. Logs in to GHCR with `GITHUB_TOKEN`, builds both platforms with the version build args (`VERSION` is the tag without its `v` and `COMMIT` the 7-character sha, the same strings GoReleaser puts in the binaries), pushes `ghcr.io/geiserx/biblical-atlas-mcp:<tag>`. When `vars.DOCKERHUB_ENABLED == 'true'` it also logs in to Docker Hub and pushes `<namespace>/biblical-atlas-mcp:<tag>`.
- `npm`: `needs: goreleaser`, `if: vars.NPM_PUBLISH_ENABLED == 'true'`, `id-token: write`, Node 24 with `check-latest: true`, `npm version "${GITHUB_REF_NAME#v}" --no-git-tag-version --allow-same-version`, `npm publish`. Trusted publishing, no token.

**dockerhub-description.yml**, on README pushes to main and manual dispatch, with the job under `if: vars.DOCKERHUB_ENABLED == 'true'`. **stale.yml** as the bootstrap template has it.

### 8.7 What needs setup the repo does not have yet

| Step | Needs | Until then |
| --- | --- | --- |
| GitHub release, binaries | Nothing | Works on the first tag |
| GHCR image | Nothing. After the first push, set the package to public once in the package settings. | Works on the first tag |
| Docker Hub image and description | Secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`, variables `DOCKERHUB_NAMESPACE` and `DOCKERHUB_ENABLED=true` | Skipped, release green |
| npm | A trusted publisher saved on npmjs.com for this repo and `release.yml`, with publishing allowed, then variable `NPM_PUBLISH_ENABLED=true`. npm may require the package to exist first. If so, the first version is published by hand once. | Skipped, release green |
| Codecov | Nothing for a public repo | Upload may be rate limited and never fails CI |
| Official MCP registry | `mcp-publisher` run by hand after the npm version exists | Manual, written down in `docs/development.md` |

## 9. Tests

All default tests run offline against one fixture. A single `go test ./...` takes seconds.

### 9.1 Fixture

`internal/atlas/testdata/data.json`, written by hand, under 40 KB. It keeps the real structure and real ids of a handful of records and replaces every long text with one short invented sentence. It contains:

- 7 people. One pair shares a display name. One shares its id with a place. One has `fecha`, one has only born and died roles, one has only an office. A chain of 4 joined by relations stored on alternating sides.
- 6 places: one with a point, one with no point and two candidates, one zone, one whose id collides with a person.
- 9 events: an open-ended date, a `narrativa` range, a range that crosses from a.e.c. to e.c., one with `roles`, one with `presentes`, one with empty `lugares`, passages that cover a verse list, a chapter span and a bookless continuation.
- 2 periods, 1 letter, 1 journey with 3 stops, 1 find, 1 tour with a `pasaje:` stop and a quiz, 2 months, 1 calendar topic.
- 5 books with true `num`, `abr`, `formas`, `capitulos` and `versiculos`, one with a single chapter and one with `omitidos`.
- About 15 sources, including two ids with the same chapter URL and one id that nothing in `fuentes` defines.
- `cobertura` for 2 of the 5 books.
- One unknown top-level key and one unknown field on a person, to prove the decoder ignores them.

Two more small files: `unknown-format.json` with `formato: "biblical-atlas/v9"`, and `english-only.json`, a copy of the fixture's people with the Spanish relation fields removed, to prove the fallback order of section 2 works the other way round.

### 9.2 What is tested

- **Store** against an `httptest` server that serves the fixture with an ETag and honours `If-None-Match`. Cases: first load, 304 keeps the snapshot and moves `checked_at`, a changed ETag swaps it, a 500 keeps the old snapshot and sets `refresh_error`, a truncated body is rejected, a body over the cap is rejected with the cap lowered through an unexported field, the unknown format gives the exact error text, HTML in place of JSON is rejected, the disk cache round trip, a cache saved for another URL is ignored, an unwritable cache directory still serves, local-file mode never opens a connection, which the test proves by pointing the URL at a server that fails the test on any request. Time comes from an injected clock, so no test sleeps.
- **Years and dates**: a table for the two conversions, overlap with open ends, and the rejection of 0.
- **Normalisation and search**: accents, case, `ñ`, each score tier, the type filter, homonyms, paging, stable order.
- **References**: a table of inputs to ranges covering every form in 5.6, plus unknown book, chapter too high and verse too high, a comma list that keeps its gap, a gap and a chapter comma rejected as tool input, a five-word book name, every form of every book alone and with a chapter, and a reversed range that touches no chapter.
- **Malformed data**: a null book and reversed ranges in a journey, a stop and a relation decode and build without a panic and are counted; a panic in the build step keeps the old snapshot.
- **npm wrapper**: `node --test postinstall.test.js` covers a tampered archive, a wrong sum, a missing entry, a `checksums.txt` that answers 404, a redirect to plain HTTP and an HTTPS redirect.
- **Tools**: every tool through `s.HandleMessage` with a `tools/call` JSON-RPC message, so schemas and registration are part of the test. One table has a row per tool with the arguments and the expected JSON. `TestEveryToolHasACase` walks the registered slice and fails for a tool with no row. Each tool also has rows for no match and for each bad input of its table in section 4, asserting `isError` and the message.
- **Annotations**: every registered tool has the four hints of 3.6.
- **Links**: every `url` in every tool output of the fixture starts with the site base or `https://wol.jw.org/`.
- **`run`**: `--version`, `--help`, an unknown argument exits 2, a bad `BIBLICAL_ATLAS_REFRESH` exits 2, HTTP mode serves `/healthz`, the bearer check rejects a missing or wrong token and accepts the right one.
- **Manifests**: `package.json` and `server.json` versions agree, and the archive names in `postinstall.js` match the GoReleaser template.

A check earns trust by failing once. Before the suite counts as done, the builder breaks one line per area, for example the year conversion, the reverse relation index and the ETag header, watches the matching test fail, and restores the line.

### 9.3 Live test

`BIBLICAL_ATLAS_LIVE=1 go test ./internal/... -run TestLive`. It downloads the real file once and asserts that the format is known, that the file loads, that no record points at a missing id, that every event passage parses, that every form of every book parses, that every source URL is `https`, and that each tool returns a non-error result for one id taken from the file itself. `TestLiveAnswerSizes` then calls `get_record` for every record and the list tools across the whole file, with default arguments and with the largest page, and fails when an answer passes 60 KB with default arguments or 70 KB at all: Claude Code refuses a tool answer above 25,000 tokens by default, and this JSON runs near 3 bytes a token. It is skipped without the variable and never runs in CI.

## 10. File tree

```text
.coderabbit.yaml                       Review bot settings for Go: gitleaks, golangci-lint, actionlint, yamllint, markdownlint, hadolint
.dockerignore                          Keeps dist, node_modules and .git out of the image build
.env.example                           Every variable of section 6, commented out, with its default
.github/FUNDING.yml                    Sponsor links, from the bootstrap template
.github/dependabot.yml                 gomod, npm, docker and github-actions, each with a 3 day cooldown and a security group; the package ecosystems ignore semver-major updates, github-actions runs monthly with labels ci and automated
.github/workflows/ci.yml               Test, snapshot build and image build
.github/workflows/release.yml          Binaries, images and npm on a version tag
.github/workflows/dockerhub-description.yml  Copies the README to Docker Hub when enabled
.github/workflows/stale.yml            Closes idle issues, from the bootstrap template
.gitignore                             /dist/, /bin/, /biblical-atlas-mcp, *.out, *.test, .DS_Store, node_modules/, .env
.goreleaser.yaml                       Binaries, archives, checksums and changelog
AGENTS.md                              Guide for coding agents: the data contract, test rules, release coupling
CHANGELOG.md                           Release notes, newest first
CLAUDE.md                              The single line @AGENTS.md
CONTRIBUTING.md                        How to build, test and send a change
Dockerfile                             Two-stage, cross-compiled image
LICENSE                                MIT
README.md                              Banner, pitch, features, quick start, documentation, related projects, license
SECURITY.md                            Private vulnerability reporting, from the bootstrap template
codecov.yml                            Coverage targets, 90% project and patch
glama.json                             Glama listing metadata
go.mod, go.sum                         Module github.com/geiserx/biblical-atlas-mcp
package.json                           npm wrapper manifest
postinstall.js                         Downloads and verifies the release binary
postinstall.test.js                    node --test cases for the checksum and HTTPS checks
run.js                                 Starts the binary in stdio mode and forwards signals
server.json                            Official MCP registry manifest
cmd/server/main.go                     run(): flags, config, store, server, transport
cmd/server/main_test.go                Tests for run()
docs/configuration.md                  Every variable and flag, client config for stdio and HTTP
docs/design/DESIGN.md                  This file
docs/development.md                    Build, test, the live test, release steps, registry publishing
docs/getting-started.md                Install paths and a first call
docs/how-it-works.md                   Download, cache, refresh, what happens offline
docs/usage.md                          Every tool, its parameters and what it returns
docs/images/banner.svg                 Repo banner in the atlas family design
docs/images/social-preview.png         1280x640 social card
internal/atlas/model.go                Structs for the data file, tolerant decoding with field fallbacks
internal/atlas/store.go                Snapshot, Store, fetch, refresh, validation
internal/atlas/cache.go                Disk cache read and write
internal/atlas/index.go                All indexes of section 5.4
internal/atlas/years.go                Year conversion and interval overlap
internal/atlas/text.go                 Normalisation and search scoring
internal/atlas/reference.go            Book table and Bible reference parser
internal/atlas/*_test.go               One test file beside each file above, plus live_test.go
internal/atlas/testdata/data.json      The fixture
internal/atlas/testdata/unknown-format.json   Fixture with a format the server does not know
internal/atlas/testdata/english-only.json     Fixture without the old Spanish relation fields
internal/config/config.go              Reads and validates the environment
internal/config/config_test.go         Defaults and every rejection
internal/tools/register.go             The []Tool slice, Register, Instructions
internal/tools/args.go                 wholeNumber, string, enum and year argument helpers
internal/tools/output.go               ref, date, source, item and paging builders
internal/tools/search.go               search
internal/tools/get_record.go           get_record and the per-type views
internal/tools/list_records.go         list_records
internal/tools/list_events.go          list_events
internal/tools/people_in_year.go       people_in_year
internal/tools/lookup_passage.go       lookup_passage
internal/tools/find_connection.go      find_connection
internal/tools/places_near.go          places_near
internal/tools/dataset_info.go         dataset_info
internal/tools/tools_test.go           The per-tool case table and TestEveryToolHasACase
internal/tools/manifest_test.go        Version and archive-name agreement across manifests
version/version.go                     Version, Commit, Date and String()
version/version_test.go                Table test for String()
```

`docs/usage.md` holds the full tool table. The README names the tools in its Features bullets and keeps no second table.

## 11. Open questions

1. **Docker Hub namespace and the sponsor handles.** The sibling servers publish under one Docker Hub account and ship a `FUNDING.yml` with sponsor handles. Both are account names. Default: they are public publishing identities and we use them. The namespace stays out of tracked files where a repository variable can carry it, and appears in `README.md` and the docs only after the first image exists.
2. **README before the first release.** The bootstrap README check looks up `npx biblical-atlas-mcp` and the image tag on the registries, so it fails until v0.1.0 is out. Default: write the README for the released state, ship CI and license badges only, tag v0.1.0 soon after the first merge, then run the check and add the npm and image badges.
3. **First npm publish.** Trusted publishing may need the package to exist before a publisher can be saved. Default: `NPM_PUBLISH_ENABLED` stays off, the first version is published by hand once, then the publisher is saved and the variable turned on.
4. **Official MCP registry.** Default: published by hand after each npm release, with the steps in `docs/development.md`. We add a release job once the manual run has worked once.
5. **Image in `server.json`.** Default: npm package only, for the reason in 8.5.
6. **`latest` image tag.** Default: none. Docs always show a version tag.
7. **Refresh interval.** Default 1 hour. The site allows 10 minutes. A conditional request costs one round trip with no body, so a shorter default is cheap if fresher data matters.
8. **Id renames.** The data file publishes no forwarding for a renamed id, so a saved id can stop resolving after a refresh. Default: the unknown-id error suggests close matches. A lasting fix is for the atlas to publish its redirect list in the data file.
9. **Resources and prompts.** Default: none. The tools return the same data with sources attached, and the server instructions carry the orientation a prompt would. We add resource templates if a client that cannot call tools needs them.
10. **Banner text.** The Spanish timeline labels and the relief credit stay as in the atlas banner. The tagline default is `THE BIBLE ATLAS, AS TOOLS FOR AI AGENTS`.
