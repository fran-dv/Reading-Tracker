# Exploration: capture search improvements, cover upload/placeholder, 3D book presentation

Change: `capture-search-and-covers` · Phase: explore · Date: 2026-09-27
Engram mirror: `sdd/capture-search-and-covers/explore`

## Current State

### Search (Open Library only)

- `internal/metadata/metadata.go:127-158` `SearchBooks` calls `GET https://openlibrary.org/search.json?title=<query>&limit=5&fields=title,author_name,first_publish_year,number_of_pages_median,cover_i`. Single field (`title=`), single source, hard cap `maxBooks=5` (metadata.go:27), no ISBN/author params, no language filter.
- Trigger: `internal/web/templates/capture.html:37` — `data-on:input__debounce.400ms`, fires `/books` only when `$title.trim().length >= 3` and the string is not a pasted URL. `internal/web/capture.go:239` (`getBooks`) re-checks `len([]rune(query)) < 3` server-side.
- Results UI: `internal/web/templates/item-form.html:145-160` (`search-results` block) — 2.25rem thumbnail, title, author/year/pages line; a pick fills title/author/size/cover and jumps focus to Why. No keyboard nav beyond native tab order, no ISBN/edition picker; empty state is a plain hint ("No matches on Open Library. Keep typing, or fill in the details yourself.").
- Verified against the live API (2026-09-27): `search.json?title=the algorithm design manual` returns only `numFound: 2`. `title=` is a narrow, tokenized match against the canonical title field only, not the broader relevance search openlibrary.org's own search box runs. Open Library docs (openlibrary.org/dev/docs/api/search) describe `q=` as "the solr query… default is to sort by relevance," and dedicated `author=` / `lang=` params exist; `title=` matching semantics are undocumented.

### Root cause of the reported misses

1. **"Not among suggestions though it exists on openlibrary.org"** — `title=` searches only the title field; the site's search box runs a broader multi-field `q=`-style query (title, subtitle, alternate titles, series). A book found there by subtitle/alt-title/series is invisible to our query.
2. **"Only appears after typing more" / "depends on phrasing"** — (a) `limit=5` with relevance sorting truncates anything ranked 6th+ at a short query; narrowing lifts it into the top 5. (b) Solr tokenization is order/stem-sensitive, so phrasing changes token overlap and score.

Also: no ISBN search, no author-assisted query, no second source, and the UI does not distinguish "still typing" from "Open Library genuinely has nothing more."

### Covers

- `internal/covers/covers.go` — `Cache.Cover` fetches only from `item.CoverURL` (Open Library `cover_i` → `covers.openlibrary.org/b/id/{id}-L.jpg`, YouTube oEmbed thumbnail, or article `og:image` via `internal/metadata/html.go`), stores bytes in SQLite (`internal/sqlite/migrations/009_covers.sql`: `covers(item_id, source_url, media_type, bytes, fetched_at)`), served via `GET /items/{id}/cover` (`internal/web/cover.go`). A failed/absent fetch is remembered for 24h (`covers.RetryAfter`) and yields 404.
- Items without a `CoverURL` never reach the cache (`covers.go:63` returns `nil, nil` when `url == ""`).
- Blank cover today: `internal/web/templates/layout.html:36-44` (`plate` block) draws `<span class="entry-plate cloth-{{.Format}}">` with no `<img>`. CSS (`internal/web/static/app.css:884-921`) paints a flat `color-mix(cloth 55%, stock-lift)` fill with two faint spine-band lines — no text, no ornament, same colour for every book of that format.
- `metadata.Image` (`internal/metadata/image.go`) accepts `image/jpeg|png|webp|avif|gif`, caps at 2 MiB, no resizing.
- No upload endpoint, no replace/edition picker UI. Shelf's "Look it up again" (`internal/web/templates/shelf.html:118`) only re-runs the lookup.
- `library.Item` (`internal/library/items.go:51-73`) has no `Publisher` field.
- `go.mod` has no image-processing dependency. Stdlib handles JPEG/PNG/GIF; WEBP decode needs `golang.org/x/image/webp`, resizing needs `golang.org/x/image/draw` (both pure Go, no cgo).

### Presentation

- Plates render as flat 2:3 (16:9 for video/article) boxes with a faint 1px outline and 4–8px radius (`.entry-plate`, `.cover`, `.capture-cover`; DESIGN.md "Cover plate"). The only lift: the book page plate gets `box-shadow: var(--shadow-float)` at ≥76rem (`app.css:1198-1199`). List plates (shelf, home, review, archive, search results) are flat.
- DESIGN.md's "Flat Page Rule": no cards, no drop shadows except the shared float shadow; brand anti-goal "generic template UI." Any 3D treatment must be an explicit, scoped exception.

## Affected Areas

- `internal/metadata/metadata.go:127-191` — `SearchBooks`, `Book`, `getJSON`/`get`.
- `internal/web/capture.go:230-262` (`getBooks`), `templates/capture.html:37`, `templates/item-form.html:145-160`.
- `internal/covers/covers.go`, `internal/sqlite/covers.go`, `internal/sqlite/migrations/` — origin discriminator (fetched / uploaded / generated) and an override so refetch never clobbers an uploaded cover.
- `internal/web/cover.go`, `internal/web/web.go:74` — upload route, possibly generated placeholder route.
- `internal/metadata/image.go` — decode/validate/resize.
- `internal/library/items.go` — optional `Publisher`.
- `templates/layout.html:36-44` `plate` block and `static/app.css:884-932,1113-1260,1532-1580` — every plate site.
- `templates/item-form.html` — cover replace UI (shared by capture and shelf edit).
- `docs/SPEC.md` §4, possibly §2.1; `DESIGN.md` "Cover plate".

## Approaches

### Search

| | Approach | Pros | Cons | Effort |
|---|---|---|---|---|
| A | Query fix: `q=` instead of/alongside `title=`; raise `limit` or "show more" | Cheap, no dependency, fixes both reported misses | Still one source | Low |
| B | Google Books as second merged source (`intitle=`/`inauthor=`/`isbn=`, developers.google.com/books/docs/v1/using) | Real redundancy, ISBN + covers, better recall for newer/non-English | Second dependency; merge/dedup/rank policy (ISBN, else normalized title+author); docs ambiguous on keyless use, no published anonymous quota — a free API key removes that | Medium |
| C | ISBN field / auto-detect | Precise when ISBN at hand | Only helps then | Low-Medium |

**Recommendation:** A first, then B (redundancy was requested), C as a fast-follow.

### Covers

| | Approach | Pros | Cons | Effort |
|---|---|---|---|---|
| D | Upload into existing `covers` table, validate + resize (`x/image/draw`), `source`/manual column | Direct fix, reuses storage/serving | Migration; override design | Medium |
| E | Edition picker among multiple `cover_i` | Reuses results UI | Editions API behaviour unverified | Medium |
| F | Generated placeholder, server-rendered SVG: deterministic palette from item id within the muted oklch family, double gilt border + corner motifs, small-caps author, title in self-hosted Alegreya, cached through the existing cover store (`source=generated`), detail scaled by size | SVG precedent (`internal/web/charts.go`), no dependency, one serving path | Ornament and long-title fitting is fiddly craft | Medium-High |
| G | Generated placeholder, CSS-only | Cheaper | Ornament unconvincing at 3rem | Low-Medium |

**Recommendation:** D + F core; E fast-follow.

### Presentation

| | Approach | Pros | Cons | Effort |
|---|---|---|---|---|
| H | Scoped "book object": keep 2:3 box, add page-edge hint + 2–4° perspective tilt + soft shadow on single-focal-object plates only (book page, capture's found cover, generated placeholder); lists stay flat; reduced-motion shows static state | Light, crafted, narrow named exception to Flat Page Rule | Needs DESIGN.md amendment | Low-Medium |
| I | Full pseudo-3D everywhere (hover flip, spine width by pages) | Striking | Conflicts with Flat Page Rule and anti-goals; motion; perf on lists | High — not recommended |

**Recommendation:** H.

## Recommendation

A as an immediate low-risk win; core change D + F + B; presentation H. E and C as fast-follows. Defer the decisions that expand scope (new external dependency, new `Publisher` field) to owner sign-off.

## Candidate improvements tied to SPEC §0

- ISBN capture — good use of time: fewer failed searches.
- Intentional generated covers on the finished archive (§6.6) and book page — enjoyment without gamification.
- "N more results not shown" when `numFound > limit` — honesty in the spirit of §9.
- Not recommended: multi-cover gallery, crop/rotate editor, cover-style theme picker.

## Spec/design sections to change

- `docs/SPEC.md` §4 (Capture): search sources/ISBN, cover upload/replace, generated placeholder rule.
- `docs/SPEC.md` §2.1 (Item): only if `Publisher` is added.
- `DESIGN.md`: "Generated cover" component (palette, ornament, text fit); "Cover plate" upload/replace affordance; scoped exception to the Flat Page Rule.

## Open Product Questions (owner decides)

1. Add Google Books as a second source (keyless or free API key), or stay Open-Library-only with the query/limit fix?
2. ISBN: separate field, or auto-detect inside the existing smart field?
3. Should an uploaded cover be locked against "Look it up again," with an explicit "revert to found cover"?
4. Publisher line on generated covers: add a `Publisher` field (Open Library can supply it; manual items would lack it), or omit the line?
5. Generated ornamental cover for books only, or for other formats too?
6. 3D scope: single-focal-object plates only, or every list plate too?
7. Edition/cover picker now, or later?

## Ready for Proposal

Yes, once questions 1–2 and 4–6 are answered.

## Owner Decisions (2026-09-27)

1. Sources: add **Google Books, keyless**, merged with Open Library (plus the `q=` / more-results fix).
2. ISBN: **separate ISBN field** at capture.
3. Manual covers: **locked** against "Look it up again", with explicit revert actions (use found cover / remove cover).
4. Publisher: **add an optional `Publisher` field** to items (SPEC §2.1), filled from lookups when available, editable; generated cover shows the line only when present.
5. Generated ornamental cover: **books only**; other formats keep today's cloth plate.
6. 3D treatment: **focal plates only** (book page, capture's picked cover, large plates); dense lists stay flat.
7. Edition/cover picker: **in this change** — candidates from Open Library editions and Google Books, alongside upload.
