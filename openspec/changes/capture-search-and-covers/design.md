# Design: Capture search that finds the book, and covers the owner controls

Change: `capture-search-and-covers` · Phase: design · Date: 2026-09-28 · Revised 2026-09-28 (owner decisions applied; export and import redesigned to stream)
Inputs: `proposal.md` (with Owner answers 2026-09-28), `explore.md`, docs/SPEC.md, DESIGN.md, the code as of `c50af91`.
Engram mirror: `sdd/capture-search-and-covers/design`

## Technical Approach

Four SPEC §12 steps, each deployable on its own, each keeping `go test ./...` green:

- **29 Search.** `internal/metadata` runs Open Library (`q=` relevance) and Google Books (keyless) concurrently under one 4s deadline and merges them in a pure function. A tiny new `internal/isbn` package normalizes and checks ISBNs for both `metadata` and `library`. Items gain `publisher` and `isbn`. The results list becomes a WAI-ARIA combobox driven by a small script in the same style as `picker.js`. Stale responses are prevented by Datastar's own request cancellation (verified in the embedded bundle), with both search fields sharing one URL.
- **30 Cover control.** One new item column, `cover_choice` (`found | picked | uploaded | removed`), carries both the lock and the origin. A chosen cover is held bytes that are never refetched; the found link (`cover_url`) keeps following lookups, so the lock needs no extra enforcement. Every image, whatever its origin, goes through one pure pipeline in `internal/covers` (decode, EXIF orientation, bounded resize, JPEG). Uploads and picks are staged in an in-memory draft store and applied when the form is filed or saved, so capture (where the item does not exist yet) and shelf edit share one path. Covers join the JSON export, which is written and read as a stream (a read snapshot on export, token streaming on import), so memory stays flat.
- **31 Edition picker.** `metadata.Editions` gathers cover-bearing candidates from Open Library editions of the work and Google Books volumes, reusing the step 29 merge. Picking one is a draft like an upload, plus pencilled ISBN, publisher and pages.
- **32 Generated cover and book object.** Drawn inline in the `plate` template: one SVG ornament sprite coloured through CSS properties, HTML text in a self-hosted Alegreya SC face, detail tiers by container queries. Go decides only the cloth index. The book object is a CSS class on focal book plates: a static tilt and page-edge shadows.

Specs: `openspec/changes/capture-search-and-covers/specs/` did not exist when this design was written (spec runs in parallel). Capability names follow the proposal: `book-search`, `cover-management`, `generated-cover`, `book-object-plates`. Where this design refines a proposal detail, the decision says so, and the refinement is listed under Open Questions for the owner.

## Architecture Decisions

### ADR-1: Search stays in `internal/metadata`; merge is a pure function there

**Choice**: `metadata.Client.SearchBooks(ctx, query) Search` and `LookupISBN(ctx, isbn) Search` run both sources concurrently (`sync.WaitGroup.Go`, Go 1.25) under `context.WithTimeout(ctx, 4*time.Second)`. Each source is a private method (`openLibrary`, `googleBooks`) returning `([]Book, total int, error)`. A pure `merge(ol, gb []Book, olTotal, limit int) (books []Book, more int)` de-duplicates and ranks. `Search.Unanswered` names each source that failed or timed out. No error is returned: a search that nobody answered is a `Search` with both names in `Unanswered`, which is exactly what the page must say.
**Alternatives considered**: a new `internal/books` package (rejected: `metadata` already is "lookups for capture, knowing nothing about the library"; a new package would not remove a single import); a source interface with two implementations (rejected: two sources, fixed; an interface adds a type that only tests would use, and the existing `rewrite` transport already lets tests serve both hosts from one `httptest` mux); `errgroup` (rejected: not a dependency today, and it cancels siblings on the first error, which is the opposite of graceful degradation).
**Rationale**: smallest change to an existing, tested boundary. The merge carries the risk, so it is the pure, table-tested part.

Merge rules (pure, table-tested):
1. Open Library results keep their relevance order and lead.
2. A Google result is a duplicate when it shares any ISBN-13 with an Open Library result, or, when either side has no ISBN, when the folded main title (text before the first `:`; lower-cased; letters and digits only) and folded first-author surname match.
3. A duplicate is dropped, but fills fields the Open Library result lacks (cover, pages, publisher, ISBN).
4. Google-only results follow in Google's order. The list is cut to `limit` (8).
5. `more = olTotal + googleOnlyFetched - len(shown)`, floored at 0. Open Library's `numFound` is exact; Google's `totalItems` is an unreliable estimate and is ignored beyond what was fetched, so the count is honest as "at least".

Fetch sizes: Open Library `limit=20`, Google `maxResults=10`. Open Library fields: `key,title,author_name,first_publish_year,number_of_pages_median,cover_i,isbn,publisher,editions,editions.key,editions.title,editions.publisher,editions.isbn,editions.publish_date`.

**Publisher, ISBN and year on an Open Library work result: best edition, pencilled** (owner decision, 2026-09-28, superseding this ADR's original "exactly one publisher" plan; see the Open Questions finding it replaces). A live check against `search.json` found that the `editions.*` sub-fields above return, per work, one best-matching edition in `editions.docs[0]` — the one the query itself matched by relevance and language, not an arbitrary one (`"dune herbert"` → Berkley 1978, ISBN 9780425038918; `"the name of the rose"` → the Italian work, but its English Minerva 1992 edition). The work-level `publisher` and `isbn` arrays list every edition's values (Dune alone carries 92 publishers), so "the work has exactly one publisher" would almost never fire. Trusting the best edition instead:
- `Publisher` is the best edition's first publisher; empty when the work has no edition on offer.
- `ISBN` (the one that fills the form) is the best edition's ISBN, normalized to ISBN-13 via `isbn.Normalize`, preferring a source already 13 digits; empty when none validates. `ISBNs` (every ISBN known, work and edition, for de-duplication) is never shown.
- `Year` is a 4-digit year extracted from the best edition's `publish_date`, falling back to the work's own `first_publish_year` when the edition has none.
- Title stays the work's own title: an edition's title may be a translation.
- In the item form, a picked result's publisher and ISBN land pencil (unconfirmed), like any other guessed value, since they name the best-matching edition rather than what the owner searched for; title, author and size land confirmed, as today.

### ADR-2: A tiny `internal/isbn` package

**Choice**: `isbn.Normalize(s string) (string, bool)` drops spaces and hyphens, accepts ISBN-10 (including a final `X`) or ISBN-13, verifies the checksum, and returns the ISBN-13 form. It is used by `metadata` (to normalize source ISBNs for de-duplication and the ISBN lookup) and by `library` (to validate and store `items.isbn`).
**Alternatives considered**: put it in `library` (rejected: `metadata` must not know the library); put it in `metadata` (rejected: `library` would then import a package that imports `net/http`, against CLAUDE.md's rule for domain packages); checksum in JavaScript for the "live" error (rejected: two sources of truth; see ADR-5).
**Rationale**: one pure function needed by two packages that must not depend on each other. About 40 lines plus a table test.

### ADR-3: Stale-response guard (P5) is Datastar's request cancellation plus one shared URL

**Choice**: both the title field and the ISBN field call the same URL, `@get('/books', {payload: {...}})`: the title field sends `{title: $title}`, the ISBN field sends `{isbn: $isbn}`. The handler searches by ISBN when `isbn` is present, else by words. Datastar 1.0.3 aborts the previous in-flight request with the same method and URL before sending a new one, so a response for an older query cannot arrive after a newer request started, whichever field sent it. The handler passes `r.Context()` to the lookups (an aborted request cancels the outbound calls) and returns without writing when `r.Context().Err() != nil`. The patched list carries `data-query` (the query it answers); while the field no longer matches it, the list is drawn in pencil (P4) and never blanked.
**Alternatives considered**: a sequence-number signal echoed by the server (rejected: Datastar applies element patches unconditionally, so a late stale patch would still replace newer content; it guards display, not content); separate URLs per field (rejected: cancellation is keyed by URL, so a slow title search could overwrite an ISBN result); a custom request header to mark the mode (rejected: `payload` already says it in the data).
**Rationale**: uses what the framework already does by default (`requestCancellation: "auto"`), adds no state. See "Datastar 1.0 capabilities" for what was verified.

### ADR-4: Keyboard results (P1) are a small `combobox.js`, in `picker.js`'s style

**Choice**: `static/combobox.js` (about 50 lines, ES module, no browser storage): the title input is `role=combobox` with `aria-controls`, `aria-expanded` and `aria-activedescendant`; results are `li[role=option]` in a `ul[role=listbox]`. Arrow keys move an `.is-active` class, Enter clicks the active option, and the option's own `data-on:click` does the filling, exactly as the picker's contract works. Escape and focus-out stay with the existing Datastar handlers. The last option is "None of these" (P7). Results change from `<button>` rows to options, because interactive children inside a listbox break the pattern.
**Alternatives considered**: pure Datastar attributes with an `_active` index signal (rejected: expressions over 200 characters on three elements, unreadable for the owner learning from the code); extending `picker.js` (rejected: its markup contract is button plus list; merging two contracts makes both harder to read).
**Rationale**: the same pattern the codebase already uses for a drawn list, applied to the combobox shape.

### ADR-5: The ISBN field validates on the server, triggered by digit count

**Choice**: the ISBN field (books only, `inputmode="numeric"`) sends `@get('/books', {payload: {isbn: $isbn}})` with no debounce as soon as it holds 10 or 13 digits (counted in the expression: `el.value.replace(/[^0-9Xx]/g, '').length`). The server normalizes with `isbn.Normalize`; an invalid checksum answers `errors.isbn` with the plain line (P9); a valid one runs `LookupISBN` (Open Library `search.json?isbn=` and Google `q=isbn:`). An ISBN with no match keeps the ISBN and moves focus to Title (P7).
**Alternatives considered**: a JavaScript checksum (rejected: duplicate rules; `POST /words` set the precedent of keeping one source of truth on the server).
**Rationale**: the round trip is local and only fires on a complete-length entry, so it is as "live" as a checksum can usefully be.

### ADR-6: Barcode scan (P10) is `scan.js`, feature-detected and hidden by default

**Choice**: `static/scan.js` (about 70 lines) removes `hidden` from the Scan button only when `window.isSecureContext`, `"BarcodeDetector" in window` and `navigator.mediaDevices` exist. It opens a `<dialog>` with the camera `<video>`, detects `ean_13` about five times a second, and on a 978/979 code writes the ISBN input and dispatches `input` (Datastar's `data-bind` and the lookup trigger take it from there). Closing the dialog stops every media track. Per the owner's answer, it stays invisible over plain HTTP.
**Alternatives considered**: a JavaScript barcode library (rejected: a large vendored dependency for a feature the platform offers); server-side decoding of a photo (rejected: complexity out of proportion).
**Rationale**: the platform feature, feature-detected, with no broken button anywhere.

### ADR-7: Cover state is one `cover_choice` column on the item

**Choice**: `items.cover_choice TEXT NOT NULL DEFAULT 'found' CHECK (cover_choice IN ('found','picked','uploaded','removed'))`, Go type `library.CoverChoice`.

| Choice | What is drawn | Where the bytes come from | Lookups |
|---|---|---|---|
| `found` (default) | the cover at `cover_url`, if any | fetched lazily from `cover_url` (as today) | may change `cover_url`; the plate follows |
| `picked` | the held image | fetched once when picked; `covers.source_url` keeps the edition link | never replace it |
| `uploaded` | the held image | the owner's upload; exists only here | never replace it |
| `removed` | nothing (step 32: the generated cover) | none; the row is deleted | never bring one back |

`cover_url` keeps meaning "the cover the lookups found", whatever the choice. That is why the lock needs no enforcement code: a lookup still updates `cover_url` in pencil, the plate simply does not follow it while the choice is not `found`, and _Use the found cover_ returns to the latest found link. `Item.ShowsCover()` (found with a link, or picked or uploaded) is the one rule the `plate` template and the cover route read. `UpdateItem` never changes `cover_choice`, the way it never changes the shortlist flag; only `Service.SetCover` does.
**Alternatives considered**: the proposal's `items.cover_locked` bool plus an `origin` column on `covers` (rejected: a bool cannot tell "uploaded" from "removed" without reading the covers table, and the plate is drawn for every list row from the item alone; two places would also have to agree, locked if and only if origin is not fetched); reusing `cover_url` with a sentinel for uploads (rejected: a hidden meaning in a URL field).
**Rationale**: one column encodes the lock, the origin and the removed state, with no invariant between tables. The export carries it with the item. This refines the proposal's wording; the owner confirmed it (Owner decisions, 1).

### ADR-8: Chosen covers live in the library transaction; fetched covers stay a cache

**Choice**: `library.Service.SetCover(ctx, id, choice, img *CoverImage) (*Item, error)` sets `cover_choice` and writes or deletes the `covers` row in one `Tx`, through new `Repo` methods (`PutCoverImage`, `DeleteCoverImage`; export and import add `EachCoverImage` and `InsertCoverImage`, ADR-11). Rules: `picked` and `uploaded` require bytes (`picked` also a source link); `found` and `removed` delete the row, so a discarded upload is not kept; every call bumps `updated_at`. The lazy cache (`covers.Cache` with `covers.Store`) keeps its own write path for fetched bytes, and `sqlite.PutCover` becomes guarded: it only writes while the item's choice is `found` (`... ON CONFLICT DO UPDATE ... WHERE (SELECT cover_choice FROM items WHERE id = excluded.item_id) = 'found'`, and the insert is skipped likewise). That closes the one real race: a lazy fetch for the found link landing after the owner uploaded.
**Alternatives considered**: everything through `covers.Store` outside the library (rejected: the owner's choice and upload are library state now; they must be exported and imported atomically with items); moving the fetch cache into `library` (rejected: fetching is I/O and a cache, not a rule).
**Rationale**: owner data goes through the domain transaction; someone else's bytes stay a cache, and the SQL guard makes the cache unable to overwrite owner data.

`covers.Cache.Cover(ctx, itemID, url string, locked bool)`: when `locked`, it returns what is held and never fetches; otherwise it behaves as today, plus normalization on write (ADR-9). The cover route answers 404 early when `!item.ShowsCover()`.

**Cache busting.** The route sends `Cache-Control: immutable` for a year, which was safe while bytes changed only with the link. Uploads, picks and reverts break that, so the plate URL becomes `/items/{id}/cover?v={{.UpdatedAt.UnixMilli}}`. `SetCover` and `UpdateItem` both bump `updated_at`. A 404 is not cached, so a failed fetch still retries on the next draw.

### ADR-9: One image pipeline, `covers.Normalize`, for every origin

**Choice**: `covers.Normalize(data []byte) ([]byte, error)` in `internal/covers/image.go`, with `internal/covers/exif.go` for the orientation tag. Pure, no HTTP.
1. `image.DecodeConfig` with the stdlib JPEG, PNG and GIF decoders plus `golang.org/x/image/webp`. An unknown format is `ErrUnreadable` (HEIC, AVIF, SVG and anything else).
2. Refuse more than 40 megapixels or more than 12,000 px on a side before decoding (`ErrTooLarge`): decompression bombs never reach `image.Decode`.
3. Skip path: a JPEG that already fits in 600 px, has no APP1 Exif segment, and is at most 256 KiB is returned unchanged. That makes the pipeline idempotent: re-normalizing never re-encodes, so no generational loss on filing, import or the startup pass.
4. Decode; for JPEG, read EXIF orientation (IFD0 tag `0x0112`, both byte orders, bounded reads, any parse error means orientation 1) and apply the eight transforms.
5. Scale so the long side is at most 600 px with `golang.org/x/image/draw.CatmullRom`; never upscale.
6. Composite transparency over the page stock colour (`#120f0c`, the manifest's theme colour), because JPEG has no alpha.
7. Encode JPEG at quality 82. Re-encoding also drops EXIF, including GPS positions from phone photos.

Every write path in the running server uses it: lazy fetch (`Cache`), upload, pick, and the startup pass. Import does not decode images; it checks that each cover is a JPEG within the pipeline's bounds (ADR-11). The output is always `image/jpeg`. `metadata.Image` drops `image/avif` from its accepted types (Go cannot decode it without cgo), so an AVIF `og:image` is remembered as a failed fetch and the plate stays drawn.
**Alternatives considered**: keep original bytes when decoding fails (rejected: two kinds of held cover, and unbounded export size); `nfnt/resize` or similar third-party libraries (rejected: unmaintained; `x/image` is maintained by the Go team and pure Go); 800 px (rejected: 600 px is the proposal figure and covers a 9rem plate at 3× within about 8%).
**Rationale**: one bounded, tested pipeline; each held cover is about 30 to 80 KB.

**Startup pass.** `covers.Cache.NormalizeHeld(ctx) (int, error)` runs in `cmd/readingqueue` before the server listens. It lists rows that are worth a look (`length(bytes) > 0 AND (media_type <> 'image/jpeg' OR length(bytes) > 262144)`), normalizes each, and writes it back through the guarded `PutCover`. A cover that fails to normalize is logged and left as it is. It is idempotent and SQL-filtered, so after the first run it touches nothing and needs no marker table. It runs synchronously so it cannot race a lazy fetch.

### ADR-10: Cover changes in the form are staged as drafts and applied on File it or Save

**Choice**: `covers.Drafts`, an in-memory map from a random token to `{Image{Bytes, SourceURL}, at}` behind a mutex, with a one-hour TTL swept on each `Put` and an injectable clock. The web handler owns one instance, created in `web.New`.
- Upload (step 30): `<input type="file" name="cover" accept="image/*" form="cover-upload">` inside the item form, tied to a separate `<form id="cover-upload" enctype="multipart/form-data" hidden>` placed outside it. Its change handler shows a local preview and posts `@post('/covers/upload', {contentType: 'form', selector: '#cover-upload'})`. The server wraps the body in `http.MaxBytesReader` (16 MiB), reads the single part through `r.MultipartReader()` with a 15 MB limit (no temporary files), normalizes, and stores a draft. It answers with signals `coverDraft` (token), `coverChoice: 'uploaded'`, `_coverLocal: ''` and a cleared `errors.cover`. Refusals answer 200 with the plain line in `errors.cover` (the existing in-band convention).
- Pick (step 31): `@post('/covers/pick', {payload: {url: ...}})`. The server accepts only cover hosts it produced (`covers.openlibrary.org`, `books.google.com`, `books.googleusercontent.com`), fetches through `metadata.Image`, normalizes, and stores a draft with the source link.
- `GET /covers/drafts/{token}` serves a draft's bytes (`Cache-Control: no-store`) for the plate preview.
- Filing and saving: `postItem` and `postEntry` resolve `coverDraft` **before** creating or updating the item. A missing draft (expired, or the server restarted) answers the form error "The uploaded cover was lost. Upload it again." and files nothing. Then `CreateItem` or `UpdateItem` runs, then `SetCover` when the form's choice differs from the item's or a draft is present. Drafts are read, not taken, so a submit refused for a missing why can be resubmitted.
- Revert actions set signals only: _Use the found cover_ sets `coverChoice='found'`, _Remove the cover_ sets `coverChoice='removed'`. P13 is kept inside the form: removing an uploaded cover, or choosing _Use the found cover_ over one (owner decision 3), opens the confirmation dialog, while removing a found or picked one acts at once, with a quiet _Undo_ that restores the previous `coverChoice` and `coverDraft` from an underscore signal. Nothing is lost until File it or Save, and the form's Cancel discards every staged change.
**Alternatives considered**: `data-bind` on the file input, which Datastar 1.0.3 turns into a base64 signal `[{name, contents, mime}]` (rejected: a 15 MB photo becomes a 20 MB signal that rides every request without a filter, including the shelf form's Cancel and "Look it up again"); base64 normalized bytes in a signal (rejected for the same reason at about 80 KB); immediate writes for filed items (rejected: capture has no item yet, so two code paths would be needed); a `cover_drafts` table (rejected: drafts are seconds-to-minutes ephemera; losing them on restart is honest and handled).
**Rationale**: one path for capture and shelf edit; signals stay small; server state is one map with a TTL.

### ADR-11: Export and import stream; covers inline; version pins the format

Owner decision (2026-09-28): the export streams from the start. Memory stays flat on both sides.

**Document shape (unchanged in spirit).** One JSON object, keys in today's order: `version` (always first), `exported_at`, `settings`, `shelves`, `items`, `ranks`, `sessions`, `active_days`, `commitments`, `speed_ramps`, `campaigns`, `campaign_items`, `reviews`, `moments_seen`, then the new `covers`. The order is dependency order (shelves before items, items before ranks and covers, campaigns before campaign items). Each cover is `{item_id, source_url, media_type, fetched_at, bytes}`, where `bytes` is standard base64. Only rows with bytes are written; remembered failures are not. `library.Export` stays as the declared type of the document (with `Covers []CoverImage` added), but only tests materialize it. No production path builds it.

**Writer: `Service.ExportTo(ctx, w io.Writer) error`** (`internal/library/export.go`).
- It runs in a read snapshot (below), so the download never holds a write lock.
- A small unexported `docWriter` does hand-written object and array framing over a `bufio.Writer`. It has `field(name, v)`, which writes `,"name":` and one `json.Encoder.Encode(v)`, and `array(name, func(put func(v any)) error)`, which writes the `[`, the commas and the `]`. It keeps the first error it meets, so call sites stay flat. It is about 50 lines and keeps today's two-space indentation.
- Small sections (settings, shelves, ranks, sessions, plan rows, campaigns, reviews, moments) are read with the existing `List…` methods and encoded element by element.
- Items are listed once (item rows are small), and each is encoded with its tags as soon as `ListTags` returns, so no `[]ExportItem` is ever built.
- Covers stream through a cursor: `Repo.EachCoverImage(fn func(CoverImage) error) error` scans one row at a time, and `fn` only encodes. A rule documented on the method forbids `fn` from calling the repo, because the rows are still open. Memory is bounded by one cover: normalized to at most 256 KiB, and at most about 340 KiB as base64 inside `Encode`.
- Base64 comes from `encoding/json`'s `[]byte` encoding per element. A hand-written `base64.NewEncoder` field was considered and rejected: it saves one bounded 340 KiB buffer per cover at the cost of hand-writing the cover object's JSON.
- `getExport` sets the headers and calls `ExportTo(r.Context(), w)`. A client that disconnects fails the next write, which ends the snapshot.

**Read snapshot.** `sqlite.Open` sets `_txlock=immediate` (`internal/sqlite/sqlite.go`), so every `Store.Tx` takes the write lock. Streaming inside it would block every write, including the running timer, for the length of the download. So:
- `library.Store` gains `Snapshot(ctx, fn func(Repo) error) error`: a read-only view in which writers are not blocked.
- `sqlite.Store` implements it with a second `*sql.DB` handle on the same file, opened without `_txlock=immediate` (deferred transactions, same pragmas). With WAL, the snapshot is fixed at the first read and writers carry on.
- The snapshot always rolls back. Only `ExportTo` uses it. It is about 15 lines.

**Reader: `Service.ImportFrom(ctx, r io.Reader) error`** (new `internal/library/import.go`). It replaces `Import(*Export)`.
- A `json.Decoder` over a `bufio.Reader` expects `{`. The first key must be `version`, or the file is refused ("the export's version must come first"); every export this app has ever written puts it first. The version is checked (1 to `ExportVersion`) before any transaction opens.
- Then, inside one `Store.Tx`, it checks the library is empty and walks the remaining keys in file order:
  - `items` and `covers` are streamed: expect `[`, then while `dec.More()` decode one element, apply version defaults (items below version 10 get `found`), and insert it. For an item that is `InsertItem` plus `ReplaceTags`; for a cover it is the new `Repo.InsertCoverImage`, after a bounds check in the library itself (`media_type` must be `image/jpeg` and bytes non-empty and at most 256 KiB, the pipeline's own bound; anything else refuses the import). The check keeps `library` from importing `covers`, and every export this app writes passes it. Then expect `]`.
  - Every other section is decoded whole (they are small) and inserted as today. Ranks are still grouped by shelf before `ReplaceRanks`. Campaigns below version 8 still get the `count` kind.
  - `settings` is decoded and validated when met (version 1 gets the default words per page) and written last, as today. A file without settings is refused.
  - Unknown keys are skipped with `dec.Skip`-style decoding into `json.RawMessage`. They are small by construction, and forward compatibility within one version is not a goal.
- Transaction semantics: all of it is one immediate transaction. Any decode error, truncation (a missing closing `]` or `}`), foreign-key failure (a section out of dependency order) or validation error rolls everything back, and the database stays empty. Import runs only from the `-import` flag, before the server starts, so holding the write lock is harmless. Memory is one item or one cover at a time, plus the small sections.
- `cmd/readingqueue/main.go` passes the opened file to `ImportFrom` instead of decoding it itself.

**Versions.** Step 29 bumps `ExportVersion` to 9 (`publisher`, `isbn`). Step 30b bumps it to 10, when `Item` gains `cover_choice` and so the document changes. Sub-PR 30e adds `covers` under the same version 10, before step 30 is deployed. A version 10 file without `covers` imports with no covers. Versions 1 to 9 import through the same streaming reader with their defaults.

**Rollback tolerance.** `Import` refuses any version above its own, and a reverted binary refuses a version 10 file with "unsupported export version" rather than silently dropping covers or choices. A reverted binary that predates streaming decodes the whole file into memory, which still works; it just uses more memory. The daily SQLite backup (VACUUM INTO) is unaffected.

**Truncated downloads.** Headers are sent before the body, so a failure mid-stream cannot change the 200 status. The server logs it, and the file lacks its closing `}`, which the importer refuses. The closing brace is the completeness marker; no extra field is added.

**Alternatives considered**: in memory (the earlier recommendation; overruled by the owner); encoding the whole library part and splicing in a covers array (rejected: string surgery on JSON, and a second read outside the snapshot); covers in a separate file (rejected: SPEC §10 wants one document); streaming inside `Store.Tx` (rejected: immediate transactions would block every write during the download); `base64.NewEncoder` for the bytes field (rejected above).
**Rationale**: flat memory on both sides, atomic import, a consistent export that never blocks writes. Only two element types are ever streamed; the rest stays the plain `List…`/`Insert…` code the owner already reads.

### ADR-12: Generated cover drawn inline; ornament is one SVG sprite; Go decides only the cloth

**Choice**: inside the `plate` block, a book for which `ShowsCover` is false draws:
```
<span class="entry-plate gen-cover gc-{{coverCloth .Title}}" aria-hidden="true">
  <svg class="gc-frame"><use href="#gc-frame-full"/></svg>   (plus a simple frame for small tiers)
  <span class="gc-author">…</span> <span class="gc-title">…</span> <span class="gc-publisher">…</span>
</span>
```
- **Ornament**: symbols in a sprite inlined once in `layout.html` (the `nav-marks` precedent): `#gc-frame-full` (double gilt rule with stepped corners, four corner rosettes, eight palmettes, after the reference image) and `#gc-frame-simple` (double rule only), in a 200×300 viewBox, filled and stroked with `var(--gc-gilt)`. The simple frame uses `vector-effect: non-scaling-stroke`, so its rules stay one pixel at small sizes.
- **Cloth**: eight muted cloth grounds as CSS classes `.gc-0` to `.gc-7` setting `--gc-cloth`, with one gilt ink `--gc-gilt: oklch(80% 0.07 88)`. Proposed grounds (lightness 30 to 38%, chroma 0.012 to 0.06, far below the inks' lightness and chroma): claret h 15 (the reference's own colour), navy h 262, slate h 235, forest h 145, moss h 120, plum h 330, aubergine h 300, charcoal h 70 at chroma 0.012. They stay out of verdigris's hue band (h 170 ±20). Claret sits near rubric's hue but at less than half its lightness and chroma, and gilt near ochre's hue but paler and at two thirds of its chroma; both are cover pigments, not inks (see the DESIGN exemptions).
- **Cloth index**: `coverCloth(title) = fnv1a32(asciiFold(trim(title))) % 8`, where `asciiFold` lower-cases A to Z only. The same six-line function runs in JavaScript (`window.coverCloth` in `static/covers.js`) for the capture preview (P16), so the preview and the filed book always wear the same cloth. Folding only ASCII keeps Go and JavaScript identical (full Unicode lower-casing differs between them, for example Greek final sigma).
- **Text**: Alegreya SC 400, one self-hosted WOFF2 subset (Latin, Latin-1, Latin Extended-A), which gives true small caps for the title like the reference, and caps for author and publisher. Fallback `Georgia, serif` for scripts outside the subset. Sizes in container-query units (`container-type: inline-size` on the plate), `text-wrap: balance`, and a four-line clamp with ellipsis on the title. The cover shows the main title (text before the first `: `); the full title is always set beside or above the plate.
- **Detail tiers** by `@container` inline size: below 2.75rem (2.25rem thumbnails): cloth plus the simple frame, no text. From 2.75rem to 5.5rem (3rem list plates, the 4.75rem margin plate): simple frame plus the title as texture. From 5.5rem (6rem book page, 9rem plates): the full ornament, author, title, and the publisher only when present.
- **Accessibility (P11)**: every current plate site sets its title next to the plate and marks the plate `aria-hidden`, so the generated cover stays hidden from assistive technology; its text is real HTML, never an image of text.
**Alternatives considered**: server-rendered SVG cached in `covers` (rejected: stored derived data that must be invalidated on every title edit and exported; text fitting in Go would need font metrics); CSS-only ornament (rejected: rosettes and palmettes are unconvincing as gradients); a continuous hue from a hash (rejected: untestable contrast; a fixed palette is checked once); cloth from the item id (rejected: the capture preview has no id, so the cloth would change on filing, which fools the owner).
**Rationale**: computed on read like debt, nothing stored or exported, and the only Go is a hash. The cloth follows the title, so editing a title can change a cover's cloth; the owner confirmed it (Owner decisions, 4).

### ADR-13: Book object is a CSS class on focal book plates only

**Choice**: `.plate-object`, added in templates only on 2:3 book plates at focal sites: the book page head plate and the capture plate, at 6rem and 9rem only (never on 3rem or 2.25rem plates, never on the 4.75rem margin plate, never on wide covers). CSS: `transform: perspective(40rem) rotateY(-4deg)` with `transform-origin: left center` (the spine side stays put); the fore-edge page block drawn as stacked right-offset box shadows in a paper tone ahead of the existing Float shadow (`2px 0 0 -1px`, `4px 0 0 -2px`, `6px 0 0 -3px`, then `var(--shadow-float)`); a faint spine shade as an inset left gradient. It is static: no transition and no hover. `prefers-reduced-motion` needs no rule because nothing moves.
**Alternatives considered**: pseudo-elements for the page edges (rejected: the plate needs `overflow: hidden` to crop covers, which would clip them; box shadows draw outside regardless); a new shadow token (rejected: the page edges are drawing, not elevation; elevation stays the Float shadow, so the Shadow Vocabulary gains no entry).
**Rationale**: pure CSS, opt-in, one named exception to the Flat Page Rule.

### ADR-14: Edition candidates come from both sources through the same merge

**Choice**: `metadata.Client.Editions(ctx, workKey, title, author string) Search`. It resolves the work key when the form has none (a filed item never stored it) with an Open Library `q=` search for title and author (`limit=1`); then, concurrently under the same 4s deadline, it reads `GET /works/{key}/editions.json?limit=50` and Google `q=intitle:"…"+inauthor:"…"`. It keeps candidates with a cover, merges them with the step 29 function (ISBN de-duplication), and caps them at 12. `Book` gains `Language` (a display name from a small map of MARC and ISO 639-1 codes; unknown codes are omitted). The form carries the work key as a signal `workKey`, set when a search result is picked; it is not stored on the item. The route is `GET /covers/editions` with the payload `{workKey, title, author}`.
**Alternatives considered**: storing the Open Library work key on the item (rejected: a column for one lookup that a search reproduces); Open Library only (rejected: the owner asked for both sources).
**Rationale**: reuses the search machinery end to end; a pick is just a draft (ADR-10).

## Data Flow

**Search (step 29)**

    title input ──debounce 400ms──┐
    ISBN input ──10/13 digits─────┴─→ @get('/books', {payload})   (Datastar aborts the previous /books)
                                          │
                         web.getBooks ── isbn.Normalize (ISBN mode) ── errors.isbn on bad checksum
                                          │
                     metadata.SearchBooks / LookupISBN (ctx + 4s deadline)
                          ├── openLibrary (q=, limit 20) ──┐
                          └── googleBooks (maxResults 10) ─┴→ merge() → Search{Books, More, Unanswered}
                                          │
                         ctx cancelled? → return, write nothing
                                          │
                  patch #search-results (data-query) + signals {_showResults, _searchNote, errors}

**Upload and file (step 30)**

    file chooser / drop / paste ─→ input[type=file form=cover-upload] ─change→ $_coverLocal = objectURL (preview)
                                          │
                     @post('/covers/upload', {contentType:'form'}) multipart
                                          │
               MaxBytesReader → covers.Normalize → Drafts.Put → signals {coverDraft, coverChoice:'uploaded'}
                                          │
    File it / Save ─→ postItem/postEntry: Drafts.Get(coverDraft) ─missing→ errors.cover, nothing filed
                                          │
                     CreateItem/UpdateItem ──→ Service.SetCover(id, choice, img)   (one Tx: item + covers row)

**Drawing a cover**

    plate template ── item.ShowsCover? ──no──→ book: generated cover (step 32); other formats: banded cloth
                          │yes
               <img src="/items/{id}/cover?v=updatedAt">
                          │
    getCover ── Cache.Cover(id, cover_url, locked = choice != found)
                   ├─ locked: held bytes only, never fetch
                   └─ found: held if same link, else metadata.Image → Normalize → guarded PutCover

**Export and import (streamed)**

    GET /export ─→ Service.ExportTo(ctx, w)  inside Store.Snapshot (deferred read tx on the second handle; writers not blocked)
                     docWriter: {"version":10, "exported_at", "settings", "shelves",
                                 "items": [ one item + tags at a time ],
                                 … small sections …,
                                 "covers": [ EachCoverImage cursor → one cover (base64) at a time ] }
                     bufio → http.ResponseWriter (chunked)

    -import file ─→ Service.ImportFrom(ctx, r): json.Decoder tokens
                     "version" first → range check (before any Tx)
                     Store.Tx (immediate, empty DB): sections in file order;
                       items/covers streamed element by element (covers bounds-checked: JPEG, ≤ 256 KiB);
                       settings written last; any error or truncation → rollback, DB stays empty

## File Changes

| File | Action | Step | Description |
|------|--------|------|-------------|
| `docs/SPEC.md` | Modify | 29–32 | §1, §2.1, §4, §10, §12 per the proposal, one `docs(spec):` commit before each step's code |
| `DESIGN.md` | Modify | 29–32 | Search results, fields, cover row, generated cover, exemptions, Flat Page exception; `docs(design):` commits |
| `internal/isbn/isbn.go`, `isbn_test.go` | Create | 29 | `Normalize`: ISBN-10/13, checksum, ISBN-13 out |
| `internal/metadata/metadata.go` | Modify | 29 | `Book` fields (`Key`, `Publisher`, `ISBN`, `ISBNs`, later `Language`); `Search`; `SearchBooks` concurrent with deadline; `openLibrary` with `q=`; `LookupISBN` |
| `internal/metadata/googlebooks.go` | Create | 29 | Keyless volumes search and ISBN query; `http://` thumbnails rewritten to `https://` |
| `internal/metadata/merge.go`, `merge_test.go` | Create | 29 | Pure de-duplication, ranking and `more` count |
| `internal/metadata/testdata/*.json` | Create/Modify | 29, 31 | Real-shaped Open Library search, Google volumes, ISBN and editions fixtures |
| `internal/metadata/metadata_test.go` | Modify | 29 | Both sources via the `rewrite` transport; one failing source; deadline; both down |
| `internal/library/items.go` | Modify | 29, 30 | `Publisher`, `ISBN` (books only, normalized); `CoverChoice`, `ShowsCover`; `UpdateItem` never changes the choice |
| `internal/library/covers.go` | Create | 30 | `CoverImage`, `SetCover` rules |
| `internal/library/library.go` | Modify | 30 | `Repo`: `PutCoverImage`, `DeleteCoverImage`, `InsertCoverImage`, `EachCoverImage`; `Store`: `Snapshot` |
| `internal/library/export.go` | Modify | 29, 30 | Version 9 (29a), version 10 (30b); 30d replaces `Export(ctx)` with `ExportTo(ctx, w)` and `docWriter` framing; 30e adds the streamed `covers` section; `Export` stays as the document's declared type |
| `internal/library/import.go` | Create | 30 | `ImportFrom(ctx, r)`: token-streamed reader, `version` first, one transaction, streamed items and covers, cover bounds check, version defaults |
| `internal/library/export_test.go` and the tests that call `svc.Export` (`schedule_test.go`, `review_test.go`, `speed_test.go`, `internal/web/capture_test.go`, `web_test.go`) | Modify | 30 | Shared helper `exportOf(t, svc) *Export` (`ExportTo` into a buffer, then decode), replacing direct `Export(ctx)` calls |
| `internal/sqlite/sqlite.go` | Modify | 30 | Second read handle (deferred transactions, same pragmas) and `Snapshot` |
| `internal/sqlite/migrations/012_publisher_isbn.sql` | Create | 29 | `items.publisher`, `items.isbn` (`TEXT NOT NULL DEFAULT ''`) |
| `internal/sqlite/migrations/013_cover_choice.sql` | Create | 30 | `items.cover_choice` with CHECK, default `found` |
| `internal/sqlite/items.go` | Modify | 29, 30 | New columns in `itemCols`, scan, insert, update (update leaves `cover_choice` to `SetCover`) |
| `internal/sqlite/covers.go` | Modify | 30 | Guarded `PutCover`; repo methods for cover images (put, delete, insert, a streaming `EachCoverImage` cursor); rows to normalize |
| `internal/covers/image.go`, `exif.go`, `drafts.go` (+ tests, `testdata/tiny.webp`) | Create | 30 | Normalize pipeline, orientation reader, draft store |
| `internal/covers/covers.go` | Modify | 30 | `locked` parameter; normalize on fetch; `NormalizeHeld` |
| `internal/metadata/image.go` | Modify | 30 | Drop `image/avif` |
| `internal/metadata/editions.go`, `editions_test.go` | Create | 31 | `Editions`: work key resolution, Open Library editions, Google volumes, language names |
| `cmd/readingqueue/main.go` | Modify | 30 | Run `NormalizeHeld` before listening; `-import` passes the file to `ImportFrom` |
| `go.mod`, `go.sum` | Modify | 30 | `golang.org/x/image` |
| `internal/web/search.go` | Create | 29 | `getBooks` (moved from `capture.go`), matched-word segments (P6), note text (P2, P8) |
| `internal/web/capture.go` | Modify | 29, 30 | `metadataClient` interface; filing applies the cover draft |
| `internal/web/itemform.go` | Modify | 29–31 | Signals `publisher`, `isbn`, `workKey`, `coverChoice`, `coverDraft`, `coverSource`; pencil and error slots |
| `internal/web/shelves.go` | Modify | 30 | `postEntry` applies the cover draft |
| `internal/web/cover.go` | Modify | 30, 31 | `getCover` via `ShowsCover`; upload, draft, pick and editions handlers |
| `internal/web/gencover.go`, `gencover_test.go` | Create | 32 | `coverCloth`, `coverTitle` template functions |
| `internal/web/web.go` | Modify | 29–32 | Routes, `Drafts` on the handler, template functions; `getExport` streams through `ExportTo` (30d) |
| `internal/web/templates/item-form.html` | Modify | 29–31 | Combobox results, "None of these", ISBN and Publisher rows, Cover row, candidates row |
| `internal/web/templates/capture.html` | Modify | 29, 30, 32 | Payload searches, sources note, upload form, plate preview, generated preview |
| `internal/web/templates/shelf.html` | Modify | 29, 30 | "Look it up again" with explicit filters; upload form outside the entry form |
| `internal/web/templates/layout.html` | Modify | 29, 30, 32 | Scripts; `plate` with `ShowsCover`, `?v=`, generated cover; ornament sprite |
| `internal/web/templates/item.html` | Modify | 32 | `plate-object` on the book page plate |
| `internal/web/static/combobox.js` | Create | 29 | Keyboard results (P1) |
| `internal/web/static/scan.js` | Create | 29 | Barcode scan (P10) |
| `internal/web/static/covers.js` | Create | 30, 32 | Drop and paste into the file input (P12); `coverCloth` (P16) |
| `internal/web/static/fonts/AlegreyaSC-Regular.woff2`, `README.txt` | Create/Modify | 32 | Cover face subset (OFL) |
| `internal/web/static/app.css` | Modify | 29–32 | Results states, fields, cover row, candidates row, generated cover, tiers, book object |

## Interfaces / Contracts

```go
// internal/isbn
// Normalize returns the ISBN-13 form of s, which may be an ISBN-10 or -13
// written with spaces or hyphens. ok is false when the digits do not make one.
func Normalize(s string) (isbn13 string, ok bool)

// internal/metadata
type Book struct {
	Key       string   // Open Library work key ("/works/OL123W"); "" for a Google-only result
	Title     string
	Author    string
	Publisher string   // "" unless the result names exactly one
	ISBN      string   // this edition's ISBN-13, only when the result is one edition
	ISBNs     []string // every ISBN-13 known for the result; used to de-duplicate
	Language  string   // editions only; "" when unknown
	CoverURL  string
	ThumbURL  string
	Year      int
	Pages     int
}

// Search is what a lookup across both sources found.
type Search struct {
	Books      []Book
	More       int      // matches found but not in Books
	Unanswered []string // sources that failed or ran out of time: "Open Library", "Google Books"
}

func (c *Client) SearchBooks(ctx context.Context, query string) Search
func (c *Client) LookupISBN(ctx context.Context, isbn13 string) Search
func (c *Client) Editions(ctx context.Context, workKey, title, author string) Search // step 31
func merge(ol, gb []Book, olTotal, limit int) (books []Book, more int)             // pure

// internal/library
type CoverChoice string

const (
	CoverFound    CoverChoice = "found"    // follows cover_url, which lookups keep up to date
	CoverPicked   CoverChoice = "picked"   // an edition's cover, held; lookups never replace it
	CoverUploaded CoverChoice = "uploaded" // the owner's image, held only here
	CoverRemoved  CoverChoice = "removed"  // nothing drawn; lookups never bring one back
)

// Item gains: Publisher string `json:"publisher"`; ISBN string `json:"isbn"`;
// CoverChoice CoverChoice `json:"cover_choice"`.
func (it Item) ShowsCover() bool

// CoverImage is a held cover: what SetCover stores and what the export carries.
type CoverImage struct {
	ItemID    string    `json:"item_id"`
	SourceURL string    `json:"source_url"` // the edition link for a pick; "" for an upload
	MediaType string    `json:"media_type"`
	FetchedAt time.Time `json:"fetched_at"`
	Bytes     []byte    `json:"bytes"` // base64 in JSON
}

func (s *Service) SetCover(ctx context.Context, id string, choice CoverChoice, img *CoverImage) (*Item, error)

// Repo gains:
//   PutCoverImage(*CoverImage) error        // SetCover
//   DeleteCoverImage(itemID string) error   // SetCover
//   InsertCoverImage(*CoverImage) error     // import
//   // EachCoverImage calls fn for every held cover with bytes, one row at a
//   // time, oldest item first. fn must not call the repo: the rows are open.
//   EachCoverImage(fn func(CoverImage) error) error

// Store gains:
//   // Snapshot runs fn over a read-only view of the library. Writers are not
//   // blocked while it runs; it always rolls back.
//   Snapshot(ctx context.Context, fn func(Repo) error) error

// Streaming export and import replace Export(ctx) (*Export, error) and Import(ctx, *Export) error.
// Export stays as the declared type of the document (tests decode into it).
func (s *Service) ExportTo(ctx context.Context, w io.Writer) error
func (s *Service) ImportFrom(ctx context.Context, r io.Reader) error

// internal/covers
var ErrUnreadable = errors.New("covers: not an image this can read")
var ErrTooLarge = errors.New("covers: image too large")

func Normalize(data []byte) ([]byte, error) // always image/jpeg out
func (c *Cache) Cover(ctx context.Context, itemID, url string, locked bool) (*Cover, error)
func (c *Cache) NormalizeHeld(ctx context.Context) (int, error)

// Store gains: CoversToNormalize(ctx context.Context, overBytes int) ([]string, error)
// PutCover becomes guarded: it writes only while the item's cover_choice is 'found'.

type Image struct{ Bytes []byte; SourceURL string }
type Drafts struct{ /* mutex, map[token]draft, now func() time.Time */ }

func NewDrafts() *Drafts
func (d *Drafts) Put(img Image) string          // random token; sweeps entries older than an hour
func (d *Drafts) Get(token string) (Image, bool)
```

**HTTP routes (new)**

| Route | Step | In | Out (SSE unless noted) |
|---|---|---|---|
| `GET /books` (changed) | 29 | payload `{title}` or `{isbn}` | `#search-results` patch; signals `_showResults`, `_searchNote`, `errors.title` / `errors.isbn` |
| `POST /covers/upload` | 30 | multipart part `cover` (≤ 15 MB) | signals `coverDraft`, `coverChoice`, `_coverLocal`, `errors.cover` |
| `GET /covers/drafts/{token}` | 30 | – | image bytes, `no-store`; 404 when expired |
| `GET /covers/editions` | 31 | payload `{workKey, title, author}` | `#cover-candidates` patch; signals `workKey`, `_editionsNote` |
| `POST /covers/pick` | 31 | payload `{url}` (allow-listed hosts) | signals `coverDraft`, `coverChoice:'picked'`, `coverSource`, `errors.cover` |

**Datastar 1.0 capabilities** (read from the embedded `static/datastar.js`, header `Datastar v1.0.3`; no web tools were available in this phase)

| Capability | Verified in the bundle | Used as |
|---|---|---|
| `requestCancellation: "auto"` default; aborts the previous controller keyed by HTTP method, then URL string | Yes | Stale-response guard (ADR-3) |
| `payload` option replaces the filtered signals (GET: `datastar` query parameter) | Yes | Per-field search payloads; small, explicit requests |
| `contentType: 'form'` with `selector`; `FormData(form)`; multipart when the form has `enctype="multipart/form-data"`; `checkValidity()` is run first | Yes | Upload (ADR-10) |
| `data-bind` on `input[type=file]` produces `[{name, contents(base64), mime}]` | Yes | Deliberately not used (ADR-10) |
| Default filter excludes signals matching `/(^\|\.)_/` even when only `include` is given | Yes | Underscore signals stay in the browser |
| Keyboard listbox or combobox helpers | None in core | `combobox.js` (ADR-4) |

Gap: SPEC §1 requires consulting the current Datastar 1.0 documentation before writing frontend code. The apply phase must confirm these behaviours against the docs for the pinned client (v1.0.3) and SDK (`datastar-go` v1.2.2) before step 29's frontend commit.

**Embedded scripts** (each small, an ES module, no browser storage, loaded by `layout.html` next to `picker.js`): `combobox.js` (P1), `scan.js` (P10), `covers.js` (P12 drop and paste, P16 `coverCloth`). Everything else stays in Datastar attributes.

## Testing Strategy

The apply config has `tdd: false`; `go test ./...`, `go vet ./...` and `gofmt -l .` must be clean at the end of every sub-PR.

| Layer | What to test | Approach |
|---|---|---|
| Unit (pure) | `isbn.Normalize`: valid 10 and 13, `X` check digit, hyphens and spaces, bad checksum, wrong length, 979 prefix with no ISBN-10 form | Table test |
| Unit (pure) | `merge`: ISBN duplicate, title and author duplicate, subtitle variant, missing ISBN on either side, field filling, Open Library order leads, cut to limit, `more` arithmetic, empty sides | Table test with real-shaped `Book` values |
| Unit (pure) | Matched-word segments (P6), search note text for each `Unanswered` combination and `More` (P2, P8) | Table tests in `internal/web` |
| Unit (pure) | `covers.Normalize`: JPEG, PNG with alpha, GIF, WebP fixture; the eight orientations (test helper builds a JPEG and splices an APP1 segment, both byte orders); malformed EXIF means upright; decompression bomb (hand-built PNG IHDR claiming 100k×100k is refused before decode); HEIC or unknown bytes are `ErrUnreadable`; skip path returns identical bytes; output long side ≤ 600 and no upscaling | Table tests, in-memory images |
| Unit (pure) | `Drafts`: put and get, unknown token, expiry at the TTL with an injected clock, sweep on put | Fixed clock |
| Unit (pure) | `coverCloth` fixed vectors (the same vectors are written in the `covers.js` comment), range 0–7, ASCII-only folding; `coverTitle` main-title split | Table test |
| Integration (HTTP fakes) | `SearchBooks` and `LookupISBN`: both sources answer; Google 429, 403 or 500; Google slower than the deadline (handler sleeps past a shortened test deadline); both down; `http` thumbnails rewritten; query parameters asserted | `httptest` mux behind the existing `rewrite` transport |
| Integration (HTTP fakes) | `Editions`: with a work key, without one (resolution search), no covers, de-duplication across sources | Same |
| Integration (real SQLite) | Migrations 012 and 013 on a database holding rows (defaults `''` and `found`); guarded `PutCover` refuses for a locked item; `SetCover` rules and `updated_at`; `UpdateItem` keeps `cover_choice`; `ShowsCover` per choice; ISBN refused on a video | Existing real-database test helpers; `library.WithClock` for timestamps |
| Integration (real SQLite) | Export and import round trip with `publisher`, `isbn`, each `cover_choice`, and covers of each origin (bytes identical); version 8 and 9 files import with `found`; a version above current is refused | Extend `export_test.go` |
| Integration (real SQLite), streaming | **Shape**: `ExportTo` output decodes into `Export` with `DisallowUnknownFields`, and `version` is the first key. **Old files**: fixtures of versions 1, 8 and 9 produced by the old struct encoder import through `ImportFrom`. **Order and refusal**: a file whose first key is not `version` is refused; a version above current is refused with the database untouched. **Rollback**: a file truncated inside `covers`, a cover with a non-JPEG media type or over 256 KiB, and a section out of dependency order each fail and leave the database empty (one transaction). **Unknown keys**: skipped. **Snapshot**: `ExportTo` into an `io.Pipe` that is not read (the snapshot stays open) while another goroutine runs `CreateItem`; the write succeeds well within `busy_timeout`. **Failing writer**: a writer that errors after N bytes returns the error without panicking and ends the snapshot. **Many covers**: 200 covers round-trip identically; every cover is written through `EachCoverImage` (no `[]CoverImage` exists on the export path, checked in review) | `t.TempDir()` databases; `io.Pipe`; a failing-writer helper |
| Integration (real SQLite) | `NormalizeHeld`: an oversized PNG row is rewritten as JPEG; a normalized row is untouched; an unreadable row is left as it was | covers and sqlite together |
| Handler (`internal/web`) | `/books`: cancelled context writes nothing; ISBN mode with a bad checksum gives `errors.isbn`; one-source note; both-failed line; results markup carries `data-query` and `role=option` | `httptest` with `fakeMeta` (extended) and a cancelled request context |
| Handler | `/covers/upload`: multipart happy path returns a draft token; over 15 MB, non-image, and HEIC give the plain lines; filing with an expired draft refuses and files nothing; filing with a draft stores the cover and choice | `mime/multipart` writer; real SQLite |
| Handler | `getCover`: removed gives 404 without a fetch; locked never calls the fetcher; the `?v=` URL changes after `SetCover`; `/covers/pick` refuses a host outside the list | `fakeMeta` counts `Image` calls |
| Manual, one bounded pass per step | Keyboard-only capture and screen-reader announcements (29); phone upload by chooser, drop and paste, EXIF-rotated photo (30); edition row at phone width (31); generated cover at 2.25, 3, 4.75, 6 and 9rem, long titles, non-Latin title, reduced motion, list plates stay flat (32) | Headless Chromium over CDP (project memory) at desktop and phone widths, then stop |

There is no day or week arithmetic in this change. The clocks that matter are the draft TTL, the cache's 24h retry, and `updated_at` bumps, each injected.

## Threat Matrix

N/A: no shell, subprocess, VCS or PR automation, executable-file classification, or process-integration boundary. The rows of `references/threat-matrix.md` (documentation paths, git selection, commit, push, PR) do not apply.

Input safety is still a design requirement, covered by the tests above:
- **Untrusted images**: `DecodeConfig` limits (40 MP, 12,000 px) before any decode; 16 MiB `MaxBytesReader` and a 15 MB part limit; SVG is never accepted; every output is re-encoded JPEG (no active content, no EXIF).
- **Server-side fetch of client-supplied links**: `/covers/pick` fetches only from the cover hosts the server itself produced. Found links keep today's trust level: they come from lookups or from links the owner pasted.
- **No browser storage**: drafts are server memory; local previews are object URLs in page memory (SPEC §1.2).

## Migration / Rollout

- **Delivery** (owner answer 1): each §12 step is its own session. Steps 29 and 30 are chained sub-PRs of about 400 authored lines; steps 31 and 32 are one PR each (a small `size:exception` if slightly over). The chain strategy is chosen at the tasks phase. Each step opens with its `docs(spec):` and `docs(design):` commits.
  - **29a** `isbn` package; `publisher` and `isbn` on items (migration 012, sqlite, library validation, export version 9); Open Library `q=` with `Search`, `More`, `Key`, `Publisher`, `ISBNs`; the form's Publisher and ISBN rows as plain fields.
  - **29b** Google Books client, `merge`, concurrent deadline, `Unanswered`; the sources note (P2) and failure line with Try again (P8).
  - **29c** Results UX: `combobox.js` (P1), pending in pencil (P4), payload and cancellation guard (P5), matched words (P6), "None of these" (P7), live status (P11).
  - **29d** ISBN lookup (P9) and `scan.js` (P10).
  - **30a** `x/image`; `covers.Normalize` with EXIF; normalize on fetch; drop AVIF; `NormalizeHeld` at startup.
  - **30b** Migration 013; `CoverChoice`, `ShowsCover`, `SetCover`; guarded `PutCover`; locked cache; `?v=` plate URLs; export version 10 (items now carry `cover_choice`; older files import as `found`).
  - **30c** `Drafts`; `/covers/upload` and `/covers/drafts/{token}`; Cover row in the form with upload, drop and paste, preview, revert, remove confirmation and Undo (P12, P13, P15); filing and saving apply the cover.
  - **30d** Streaming export and import with no format change: `Store.Snapshot` and the second read handle, `ExportTo` with `docWriter`, `ImportFrom` with token streaming, `getExport` and `-import` switched over, tests moved to `exportOf`, and old-version fixtures.
  - **30e** Covers in the stream: `EachCoverImage` and `InsertCoverImage`, the `covers` section, the import bounds check, round trip with each origin. Version 10 was already set in 30b by `cover_choice`.
  - **31** `Editions`, `/covers/editions`, `/covers/pick`, candidates row (P14, P15, P11).
  - **32** Generated cover, sprite, font, cloth, tiers, capture preview (P16, P11); then the book object.
- **Migrations** 012 and 013 are additive with defaults: existing items get `''` for publisher and ISBN and `found` for the choice, so behaviour is unchanged until the owner acts. The daily backup precedes each deploy (`deploy/deploy.sh`).
- **One-time data rewrite**: only `NormalizeHeld` (step 30a), which shrinks held covers. The originals remain in the daily SQLite backups.
- **Rollback**: `git revert` per PR. A reverted binary ignores the new columns (it names its columns explicitly) and refuses newer export versions (ADR-11). Reverting 30b after owners' choices exist leaves held uploads unused but intact. Google Books can be disabled at its single call site.

## Size Forecast

Authored changed lines (code, tests, templates, CSS, scripts; docs commits and binary assets excluded). The streaming export adds about 250 to 300 lines to step 30 compared with the proposal's forecast.

| Sub-PR | Contents | Forecast |
|---|---|---|
| 29a | isbn, publisher and isbn items, migration 012, export version 9, Open Library `q=` | ~350–420 |
| 29b | Google Books, merge, deadline, source notes | ~350–420 |
| 29c | Combobox and results UX (P1, P4–P7, P11) | ~300–380 |
| 29d | ISBN lookup and scan (P9, P10) | ~200–260 |
| 30a | Image pipeline, EXIF, normalize on fetch, startup pass | ~380–450 |
| 30b | `cover_choice`, `SetCover`, guarded cache, `?v=`, export version 10 | ~300–380 |
| 30c | Drafts, upload route, Cover row UI (P12, P13, P15), filing applies the cover | ~380–450 |
| 30d | Streaming export and import, snapshot, test migration | ~380–460 |
| 30e | Covers in the stream | ~180–240 |
| 31 | Edition picker | ~420–580 (a small `size:exception` if over) |
| 32 | Generated cover and book object | ~450–630 (a small `size:exception` if over) |
| **Total** | | **~3,700–4,700** |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| The export stream fails mid-download: the file is saved with a 200 status and no closing brace | Low | Logged on the server; the importer refuses a truncated file and rolls back; the daily SQLite backup is independent |
| The second read handle behaves differently from the write handle (pragmas, WAL) | Low | Same DSN pragmas except the lock mode; the `io.Pipe` snapshot test proves writers proceed |
| A slow export client holds a WAL snapshot open, so the WAL grows until it ends | Low | Single user and local; the WAL checkpoints after the snapshot closes |
| Hand-written framing drifts from the declared `Export` type | Med | The shape test decodes with `DisallowUnknownFields` and checks key order; the framing uses one `Encode` per value, never hand-written values |
| A hand-edited or foreign export with `version` not first, or sections out of order, is refused | Low | Every file this app ever wrote satisfies both; the refusal message says why |
| Step 30 grows past its forecast; 30d touches several existing test files | Med | Its own sub-PR with no format change, so review is mechanical |
| Google Books keyless quota or format surprises | Med | ADR-1 degradation; record fixtures at apply |
| Cloth hash written in Go and JavaScript drifts | Low | Shared test vectors; one manual check |
| Generated cover flashes before a lazily fetched cover loads | Low | Local server; the cover is cached immutably after the first load |

## Owner decisions (2026-09-28)

| # | Decision | Outcome |
|---|---|---|
| 1 | One `cover_choice` column (`found`, `picked`, `uploaded`, `removed`) instead of `cover_locked` plus an origin on covers (ADR-7) | Confirmed |
| 2 | Cover changes are a draft applied on File it or Save, in capture and shelf edit alike; Cancel discards them (ADR-10) | Confirmed |
| 3 | _Use the found cover_ on an uploaded cover asks first, as removing does (P13) | Confirmed |
| 4 | Generated cover cloth follows the title, not the item id (ADR-12) | Confirmed |
| 5 | Generated cover shows the main title only, in Alegreya SC (ADR-12) | Confirmed |
| 6 | Export built in memory | **Overridden**: the export and import stream from the start (ADR-11 redesigned) |
| 7 | AVIF covers are no longer kept (ADR-9) | Confirmed |

## Open Questions

No owner decisions remain open.

Technical gaps to verify during apply (no web tools were available in this phase):

- [ ] Datastar 1.0 docs for `payload`, `contentType: 'form'` with `selector`, and `requestCancellation` (read from the bundle, not yet from the docs; SPEC §1).
- [ ] Google Books keyless: the `fields=` partial-response syntax, the thumbnail `zoom` parameter for a larger cover, the "image not available" placeholder image, and 429 behaviour. Record real fixtures.
- [x] Open Library: whether `search.json` offers per-result best-edition fields (`editions`, `editions.publisher`, `editions.isbn`) — confirmed 2026-09-28: it does, one best-matching edition per work in `editions.docs[0]`. P3 uses it for a real publisher on work results (ADR-1, "Publisher, ISBN and year on an Open Library work result"). Still open: the field names in `/works/{key}/editions.json` (`publishers`, `publish_date`, `number_of_pages`, `covers`, `isbn_13`, `isbn_10`, `languages`), needed by PR 31's edition picker, not this finding's endpoint.
- [ ] BarcodeDetector support in the owner's phone browser (Chrome on Android has it; Safari on iOS did not at the time of training).
- [ ] That Alegreya SC's WOFF2 subset (OFL) carries the glyphs the reference style needs, and the final size of the subset.
