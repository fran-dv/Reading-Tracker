# Proposal: Capture search that finds the book, and covers the owner controls

Change: `capture-search-and-covers` · Phase: propose · Revised: 2026-09-28 (owner answers applied)
Inputs: `explore.md` (incl. Owner Decisions 2026-09-27, settled), owner answers to the proposal's open questions (2026-09-28), docs/SPEC.md, DESIGN.md, PRODUCT.md.
Engram mirror: `sdd/capture-search-and-covers/proposal`

## Intent

Capture is the highest-frequency action (SPEC §4) and it currently fails at its first job for books:

1. **Search misses books that exist.** `metadata.SearchBooks` queries Open Library with `title=` (title field only), `limit=5`, one source. Books found on openlibrary.org by subtitle, alternate title or series never appear; others appear only after the query is narrowed into the top five. The UI cannot tell "keep typing" from "there is nothing more", which quietly misleads (SPEC §0, third principle).
2. **The owner cannot fix a wrong or missing cover.** Covers come only from the lookup URL. There is no upload, no choice of edition, and "Look it up again" can only re-run the same lookup.
3. **A book without a cover is a flat colour swatch.** Every coverless book draws the same banded `cloth-book` plate: no title, no author, nothing that makes the shelf the owner is building feel like books (goal 6, "Enjoy the process"; §6.6 is load-bearing).

Why now: steps 1–28 are built and the app is in real daily use; capture friction and anonymous plates are felt every day. Success: the book the owner is holding is found in one search (by words or ISBN), its cover is the one the owner wants and stays that way, a book with no cover still looks like a book, and the capture flow meets the quality bar of current industry reading apps without adding anything outside capture, search and covers.

## Owner decisions applied

Settled 2026-09-27 (explore.md): Google Books keyless merged with Open Library; separate ISBN field; manual covers locked with explicit revert; optional `publisher`; generated cover for books only; book object on focal plates only; edition picker in this change.

Settled 2026-09-28 (answers to this proposal):
1. **ISBN is stored** on the item: optional, books only, editable.
2. **Every stored cover goes into the JSON export** (fetched, picked and uploaded), and import restores them.
3. **P1–P3 accepted**, and the owner asked for whatever else industry-level capture UX needs. P4–P16 below are accepted in principle by that request; each is listed separately so the owner can strike any.
4. **Delivery: four SPEC §12 steps, each its own session and PR**: 29 Search, 30 Cover control, 31 Edition picker, 32 Generated cover and book object.

## Scope

### In Scope

**Step 29: Search**
- Open Library query moves from `title=` to relevance `q=`, with more results fetched and an honest line when more exist than are shown ("12 more not shown. Add a word to narrow it.").
- Google Books (keyless) as a second source, queried alongside Open Library, merged and de-duplicated (ISBN first, else normalized title + first author). Any failure, timeout or quota refusal of Google Books degrades to Open Library alone, and the page says so (P2).
- A separate ISBN field at capture (books), which looks the edition up directly in both sources.
- New optional item fields (SPEC §2.1): `publisher` and `isbn` (books only). Both are filled from lookups when present, editable in the item form, and exported and imported (§10).
- P1–P11 where they belong to search (see UX items).

**Step 30: Cover control**
- Cover upload in the item form (capture and shelf edit): JPEG, PNG, GIF, WebP; decoded, validated, EXIF-oriented, resized and re-encoded server-side; stored in the existing `covers` table.
- Every cover is normalized on write (fetched ones too) to one bounded size and format, and existing held covers are normalized once, so the export stays small.
- Cover origin (`fetched` / `picked` / `uploaded`) and a lock on the item. A cover chosen by the owner is locked: "Look it up again" never replaces it. Two explicit revert actions: _Use the found cover_ and _Remove the cover_.
- Every held cover is written to the JSON export and restored by import.
- P11–P13, P15 where they belong to upload and revert.

**Step 31: Edition picker**
- Edition/cover picker in the item form: candidates from Open Library editions of the picked work and from Google Books volumes of the same title/author, beside the upload.
- A picked cover is locked like an uploaded one.
- P11, P14, P15 where they belong to the picker.

**Step 32: Generated cover and book object**
- Generated ornamental hardcover for **books without a cover only**: cloth ground in a per-book colour, gilt double border, corner rosettes and palmettes, author in small caps at the top, title in a large serif, publisher at the foot only when present (after the reference image). Detail scales down with plate size. Other formats keep today's banded cloth plate.
- "Book object" treatment on **focal 2:3 book plates only** (book page plate, capture's cover plate at 9rem, other 9rem plates): a page-edge hint on the fore-edge, a slight static perspective tilt, a soft shadow. Dense lists (3rem and 2.25rem plates, the 4.75rem margin plate) stay flat. No hover motion; reduced motion shows the same static object.
- P11, P16 where they belong to the generated cover.

**Documentation**
- SPEC.md and DESIGN.md amendments listed below, each in its own `docs(spec):` / `docs(design):` commit before or alongside the code of its step (SPEC §0).

### Out of Scope
- Cropping, rotating or editing an uploaded image beyond automatic orientation and resize.
- A cover gallery, multiple covers per item, or a cover-style/theme picker.
- Generated covers for video, article, paper or course.
- 3D treatment on list plates, the finished shelf spines, or any hover flip/animation.
- A Google Books API key or any setting for one (keyless was decided; a key can be a later change if quota bites).
- Search sources beyond Open Library and Google Books; language filters; auto-detecting ISBNs in the title field; a "Show more" button (the honest count and narrowing were chosen).
- Changing URL capture (articles, YouTube) beyond sharing the upload, lock and export path for their covers.
- Skeleton screens, search history, recent searches, suggestions from the owner's own library, or any browse-all surface or new navigation entry (§0, §11).

## Capabilities

> Contract with sdd-spec. `openspec/specs/` holds no specs yet, so every capability is new.

### New Capabilities
- `book-search`: capture's book lookup: Open Library `q=` relevance search, Google Books merge and de-duplication, graceful single-source degradation, "more not shown" honesty, which sources answered, the ISBN field (validation, lookup, optional scan), `publisher` and `isbn` filling, result list behaviour (keyboard, highlighting, pending, cancellation, failure and "none of these" paths).
- `cover-management`: how an item's cover is obtained and kept: fetched from a lookup, uploaded, or picked from edition candidates; origin and lock; "Look it up again" respecting the lock; revert to the found cover or remove; image validation, orientation, normalization; the edition picker; export and import of every held cover.
- `generated-cover`: the ornamental placeholder drawn for a book with no cover: deterministic per-book palette, ornament, text layout and fitting, publisher line rule, detail tiers by plate size, accessible name, live preview at capture.
- `book-object-plates`: which plates are drawn as a 3D book object and which stay flat, and the reduced-motion rule.

### Modified Capabilities
- None (no capability specs exist in `openspec/specs/`; the SPEC.md and DESIGN.md sections affected are listed under Documentation changes).

## Approach

### Search (step 29)
- `internal/metadata`: `SearchBooks` switches to `search.json?q=…&limit=N&fields=key,title,author_name,first_publish_year,number_of_pages_median,cover_i,isbn,publisher` and returns results plus the total found. New Google Books query against `https://www.googleapis.com/books/v1/volumes?q=…&maxResults=N` (and `q=isbn:…` for the ISBN field). Both run concurrently under one short deadline (about 4s) so capture never waits on a slow source. The merge (dedup key: any shared ISBN, else normalized title + first author) is a pure, table-tested function. Open Library order leads; Google-only results follow. The result records which sources answered.
- ISBNs are normalized to ISBN-13 (ISBN-10 converted, hyphens and spaces dropped) and checksum-validated in a pure function; that is the stored form.
- `publisher` and `isbn` land in `library.Item`; a migration adds `items.publisher TEXT` and `items.isbn TEXT`; export and import carry them. `isbn` is refused on non-book formats.
- Result thumbnails keep loading from their source while choosing, as today (nothing is filed yet); Google's `http://` thumbnail links are rewritten to `https://`. Once filed, a cover is served from this machine (DESIGN Do's).

### Cover control (step 30)
- `covers` rows gain an origin. The lock lives on the item (`cover_locked`), so a removed cover (locked, empty) stays removed after "Look it up again". `Cache.Cover` never refetches over a locked cover.
- Upload: multipart POST from the item form (Datastar 1.0 file handling must be checked against current docs before writing it, per SPEC §1). Decode with stdlib `image/jpeg|png|gif` plus `golang.org/x/image/webp`; read the JPEG EXIF orientation tag with a small pure-Go reader and rotate; resize with `golang.org/x/image/draw` to a fixed maximum (about 600px on the long side); store as JPEG. `golang.org/x/image` is pure Go and maintained by the Go team; it is the smallest honest way to decode WebP and resize.
- **Normalize on write for every origin**, so the export and the page never carry multi-megabyte images. Existing held covers (fetched before this change, up to 2 MiB each) are normalized by one idempotent pass at startup.
- **Export and import (SPEC §10):** each held cover is exported as `{item_id, origin, source_url, media_type, fetched_at, bytes (base64)}`; remembered failures (no bytes) are not exported. The export is encoded as a stream rather than built in memory. Import restores every cover as held, with its origin, and the items' lock flags, so an imported library draws its covers without refetching.
- Image processing, orientation, normalization and origin/lock rules live in domain packages (`internal/covers`, `internal/metadata`) with no HTTP or template imports.

### Edition picker (step 31)
- From the picked work's key, `GET /works/{key}/editions.json` (Open Library) plus Google Books volumes of the same title/author, reduced to candidates with a cover and de-duplicated with the step 29 merge. Choosing one sets the cover, locks it, and offers that edition's ISBN, publisher and pages (P14).

### Generated cover (step 32)
- Recommended rendering: **drawn inline, never stored.** The cover is derived from the item (id, title, author, publisher), so, like debt, it is computed on read and needs no export. The ornament frame is one embedded SVG coloured through CSS custom properties. Title, author and publisher are HTML text in self-hosted faces, sized with container query units and balanced wrapping, so long titles fit without Go-side font metrics. Go decides only the palette (deterministic from the item id, drawn from a small set of muted cloth hues with a gilt ink) and the detail tier by plate size.
- Serif face: the reference uses a classical serif. The candidate is Alegreya roman and small caps (the family is already self-hosted for the hand), which needs an upright subset added. The design phase confirms.

### Book object (step 32)
- CSS only, scoped by a class the focal plate sites opt into: a thin fore-edge page hint, a 2–4° static `perspective`/`rotateY` tilt, and the float shadow adjusted for the object. Lists never receive the class.

## Capture UX items

Each item is tied to a SPEC §0 goal and placed in a step. P1–P3 were accepted explicitly. P4–P16 are accepted in principle by the owner's request for industry-level quality; the owner may strike any. All stay inside capture, search and covers; none adds a view, a count of the library, or anything shaped like gamification (§0, §11). Industry baseline was judged against current readers' capture flows (search-as-you-type with keyboard selection, ISBN and barcode lookup, edition choice, cover upload) and WAI-ARIA combobox practice.

| # | Item | §0 goal | Step |
|---|------|---------|------|
| P1 | **Keyboard through results.** Arrow keys move through results and Enter picks, reusing the picker's keyboard pattern; results are a listbox tied to the field (combobox, `aria-activedescendant`). | 4 (time: capture stays fast and mouse-free) | 29 |
| P2 | **Say which sources answered.** When one source does not answer, one pencil line says so ("Only Open Library answered."); the empty state names both sources. | §0 never quietly fooled | 29 |
| P3 | **Publisher and year on each result.** Tells editions apart before picking. | 4 | 29 |
| P4 | **Pending without flicker.** While a refined query runs, the previous results stay visible in pencil and the field's hint slot shows the pending stroke ("Looking it up"); the list never blanks and reflows between keystrokes. The system's existing pending idiom, not skeleton rows. | 4 | 29 |
| P5 | **Superseded requests are cancelled.** A new keystroke cancels the in-flight search (Datastar request cancellation, verified against current docs); the server drops cancelled work; a response that does not match the field's current query is never drawn. | §0 never fooled (no results for a query no longer typed); 4 | 29 |
| P6 | **Matched words marked in result titles.** With relevance search a result may match on a subtitle or series; the words that matched are set apart within the title by weight or ink per DESIGN's ink rules (no new colour), so why a result is there is visible at a glance. | 4 | 29 |
| P7 | **"None of these" path.** A last row in the results closes the list, keeps the typed words as a confirmed title, sets the format to book and moves focus to Author; an ISBN with no match keeps the ISBN and moves focus to Title. Manual entry never starts from a blank form. | 4 | 29 |
| P8 | **Plain failure states with a retry.** Each source's failure is stated per P2; when neither answers, one rubric line ("Neither Open Library nor Google Books answered. Type the details in.") and a quiet _Try again_. Filing is never blocked by a lookup. | §0 honesty; 4 | 29 |
| P9 | **An ISBN field that forgives typing.** Accepts ISBN-10 or 13 with hyphens or spaces, numeric keyboard on phones (`inputmode`), live checksum with a plain error ("Those digits don't make an ISBN. Check them against the book."), and looks up at once when complete and valid, without the debounce. | 4 | 29 |
| P10 | **Scan the barcode on a phone.** A _Scan_ action beside the ISBN field reads the back-cover barcode with the camera, only where the browser offers barcode detection in a secure context; hidden otherwise, never a broken button. The owner reads physical books, so this is the fastest capture there is. See Open question 2. | 4, 5 (read a great deal: less friction to file the next book) | 29 |
| P11 | **Screen reader and focus.** A polite live region announces the result count, what is not shown and which sources answered; informative covers carry `alt` ("Cover of <title>"); a generated cover is named by its title and author with its ornament hidden; focus lands predictably after pick, upload, revert and edition choice. Each step owns its own surface. | 4 (usable at speed by keyboard and assistive tech) | 29–32 |
| P12 | **Upload the way people do now.** _Upload a cover_ opens the file chooser with `accept="image/*"` (a phone offers camera or gallery); drag-and-drop onto the plate on desktop; pasting an image while the form has focus. An immediate local preview (an object URL in memory, not browser storage) shows in the plate with the busy stroke, replaced by the processed image. Refusals are plain lines ("That file isn't an image this can read. Use JPEG, PNG or WebP.", "That image is over 15 MB."). | 4, 6 | 30 |
| P13 | **Removing an uploaded cover asks first.** An uploaded image cannot be fetched again, so _Remove the cover_ on it opens the confirmation dialog ("Remove the cover of Dune?", "The uploaded image is deleted."). Removing a fetched or picked cover acts at once and offers _Undo_ in the status line. | §0 nothing lost quietly | 30 |
| P14 | **Informed edition choice.** Each candidate is labelled with publisher, year, language and pages; the current cover is marked selected; choosing one also fills that edition's ISBN, publisher and pages in pencil until confirmed (Pencil Until Confirmed Rule). The edition sets the page count, and page counts drive pace, time left and each campaign's average book (§7, §8.1). | 2 (accurate metrics), 3 (campaign sizing) | 31 |
| P15 | **Phone layout for the new controls.** At phone width, edition candidates are a horizontally scrolling row with snap points and 2.75rem targets, and the upload and revert actions sit full-width under the plate; nothing scrolls the page sideways. | 4 (capture happens on a phone at night) | 30–31 |
| P16 | **Generated cover previewed while capturing.** With the format set to book and no cover chosen, capture's plate draws the generated cover live from the typed title, author and publisher, so the owner sees the book being filed. | 6 (enjoy the process) | 32 |

## Affected Areas

| Area | Impact | Step | Description |
|------|--------|------|-------------|
| `internal/metadata/metadata.go` | Modified | 29 | `SearchBooks` to `q=`, larger limit, total found, ISBN and publisher fields |
| `internal/metadata/googlebooks.go` | New | 29 | Keyless Google Books search and ISBN lookup |
| `internal/metadata/merge.go`, `isbn.go` | New | 29 | Pure merge/dedup; ISBN normalize and checksum |
| `internal/metadata/editions.go` | New | 31 | Edition candidates from both sources |
| `internal/metadata/image.go` (or `internal/covers`) | Modified | 30 | Decode, orient, validate, normalize; WebP via `x/image` |
| `internal/covers/covers.go` | Modified | 30 | Origin and lock; no refetch over a locked cover; revert; normalize pass |
| `internal/library/items.go`, export/import | Modified | 29, 30 | `Publisher`, `ISBN`, cover lock; covers in the export and import |
| `internal/sqlite/migrations/012_*.sql`, `013_*.sql` | New | 29, 30 | `items.publisher`, `items.isbn`; cover origin and `items.cover_locked` |
| `internal/sqlite/covers.go`, items repo | Modified | 29, 30 | Read/write the new columns |
| `internal/web/capture.go`, `cover.go`, `web.go` | Modified | 29–31 | ISBN query, cancellation, upload, choose-cover and revert routes |
| `internal/web/templates/capture.html`, `item-form.html`, `layout.html` (`plate`), `shelf.html`, book page template | Modified | 29–32 | ISBN field, results, upload/picker/revert UI, generated plate, book-object class |
| `internal/web/static/app.css`, a small script for P1/P10/P12 if Datastar attributes do not suffice, ornament SVG, font subset | Modified/New | 29–32 | Styles and assets |
| `go.mod` | Modified | 30 | Adds `golang.org/x/image` |
| `docs/SPEC.md`, `DESIGN.md` | Modified | 29–32 | See below (separate docs commits) |

## Documentation changes required

**docs/SPEC.md**
- §1 Stack: "Book metadata" row adds Google Books (keyless, optional; the app works with Open Library alone).
- §2.1 Item: add `publisher` (string?, from lookups, editable), `isbn` (string?, books only, ISBN-13, from lookups or typed, editable) and `cover_locked` (bool: the owner chose or removed the cover; lookups never replace it).
- §4 Capture: input method 2 rewritten (search both sources by words, or by ISBN in its own field or by scanning; honest count of what is not shown; says which sources answered; "none of these" leads to manual entry); new cover rules (upload, choose from editions, lock against lookup, revert actions; generated cover for books without one).
- §10 Export: add `publisher`, `isbn`, `cover_locked`, and **covers**: every held cover image, base64, already normalized, with its origin and source link; remembered failures are not exported. Import restores them.
- §12: new steps 29 Search, 30 Cover control, 31 Edition picker, 32 Generated cover and book object, each landing its own detail in the sections above when agreed.

**DESIGN.md**
- Search results: sources line, "N more not shown", matched words, pending state, failure lines and retry, the "none of these" row, keyboard and listbox behaviour, publisher and year in the metadata line, new empty copy.
- Inputs / Fields: the ISBN field (numeric, checksum error copy) and the _Scan_ action.
- Cover plate / Form reveals: upload, drag-and-drop and paste, local preview, choose-a-cover row (including its phone layout), revert actions, the removal dialog copy.
- New component "Generated cover": palette family, gilt ink, ornament, text layout, publisher rule, detail tiers by size, capture preview.
- Named exceptions: the generated cover is a cover image, not interface, so it is exempt from the Plain Label Rule (small caps), the Marks Stay in the Head Rule (drawn ornament) and the One Job Rule (its hues are not inks). Its palette stays muted and must not read as rubric, verdigris or ochre.
- Elevation & Depth / Flat Page Rule: the book object as the one scoped exception on focal book plates; Shadow Vocabulary gains its shadow if it differs from Float.
- Entries → Cover plate and Bookcloth: coverless books draw the generated cover; other formats keep the banded cloth plate. Copy list: "Open Library didn't answer." broadened to both sources.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Google Books keyless use is rate-limited per IP with no published anonymous quota; 429s or 403s during capture | Med | Concurrent call under a short deadline; any failure yields Open Library results only, stated (P2, P8). A key can come later as its own change. |
| Merge produces duplicates or hides the right edition (missing ISBNs, subtitle variants) | Med | Pure, table-tested merge with real-shaped fixtures; Open Library order leads; normalized title + first author fallback; the edition picker (step 31) corrects a wrong edition. |
| Export size grows with every cover | Med | Normalize every cover to about 600px JPEG (roughly 30–80 KB), so 500 covers come to roughly 20–50 MB of JSON after base64's one-third overhead. The export is stream-encoded, not held in memory. Existing un-normalized covers (up to 2 MiB) are normalized once at startup. |
| Import of a large export | Low | Import already reads one file into an empty database; covers are inserted in the same transaction as their items. Round-trip tested with covers of each origin. |
| Phone photos: EXIF rotation, large files, HEIC | High for rotation | Orientation read and applied; 15 MB request cap before resize; HEIC refused with a plain line. |
| Decompression bombs / malicious images | Low | `image.DecodeConfig` dimension check before full decode; request body cap. |
| Barcode scan unavailable (browser without barcode detection, or the app reached over plain HTTP from the phone) | Med | Feature-detected; the action is simply absent. See Open question 2. |
| Datastar 1.0 support for file upload, request cancellation and keyboard lists may need a small script | Med | Verify against current docs first (SPEC §1); any script stays small, embedded, and uses no browser storage. |
| Migrations on the real database | Low | Additive columns with defaults; existing covers become `fetched`, unlocked; the daily backup precedes deploy. |
| Generated cover craft: long titles, tiny plates, non-Latin titles, contrast of gilt on some hues | Med | Detail tiers by size (3rem draws border and title only), container-unit text fitting with clamping, a fixed tested palette with contrast checks; one bounded visual verification pass. |
| Book object erodes the Flat Page Rule over time | Low | Explicit named exception in DESIGN.md, opt-in class, never on lists. |
| Every step exceeds the 400-line review budget | High | See forecast and Open question 1. |

## Rollback Plan

- Each step is its own PR and reverts cleanly with `git revert`.
- Migrations are additive; a reverted binary ignores the new columns. No data is rewritten except the one-time cover normalization, which only shrinks held images. The daily SQLite backup precedes each deploy, so the original bytes are recoverable from it.
- A reverted exporter simply omits covers; an export that holds covers still imports into a reverted binary only if import ignores unknown keys. The design phase confirms import tolerates the `covers` key, or pins the export version.
- Google Books can be disabled by removing one call site; Open Library alone is the pre-change behaviour with a better query.
- Generated covers and the book object are template and CSS only; reverting restores the banded cloth plate.
- Uploaded covers stay in the `covers` table after a revert (harmless bytes); a forward fix restores their use.
- Deploy with `deploy/deploy.sh` only after a step passes `go test ./...`.

## Dependencies

- `golang.org/x/image` (pure Go, no cgo) for WebP decode and resizing.
- Google Books API v1 (`googleapis.com/books/v1/volumes`), keyless.
- Open Library search and `/works/{key}/editions.json` endpoints.
- Current Datastar 1.0 documentation for file upload, request cancellation and keyboard lists before writing that frontend (SPEC §1).
- The browser's barcode detection in a secure context, for P10 only.
- An upright Alegreya (or chosen serif) WOFF2 subset for the generated cover, self-hosted under the OFL.

## Success Criteria

- [ ] "The Algorithm Design Manual" and a book known only by its subtitle on openlibrary.org both appear in the first results after typing their words once.
- [ ] A valid ISBN, typed with or without hyphens, or scanned where available, returns that edition (title, author, pages, publisher, ISBN, cover) from whichever source has it; an invalid one is refused with the plain line.
- [ ] With Google Books unreachable (test double), capture still returns Open Library results and says only Open Library answered; with both down, the retry line appears and filing still works.
- [ ] When more results exist than are shown, the list says how many are not shown.
- [ ] Fast typing never draws results for an older query.
- [ ] Results can be chosen by keyboard alone, and a screen reader hears the result count and source status.
- [ ] "None of these" leaves a confirmed title and focus in Author.
- [ ] An uploaded or picked cover survives "Look it up again"; _Use the found cover_ and _Remove the cover_ each do exactly what they say; removing an uploaded cover asks first.
- [ ] A phone photo uploads by chooser, drop or paste, previews at once, appears upright, and is stored at no more than the normalized size.
- [ ] Choosing an edition fills its ISBN, publisher and pages in pencil.
- [ ] Every coverless book draws the generated cover with title and author legible at 9rem and recognisable as a book at 3rem; publisher appears only when present; other formats are unchanged; capture previews it while typing.
- [ ] Focal book plates show the book object; list plates are unchanged; reduced motion shows no animation.
- [ ] `publisher`, `isbn`, `cover_locked` and every held cover round-trip through export and import.
- [ ] `go test ./...`, `go vet ./...` and `gofmt -l .` are clean; merge, ISBN, lock, orientation, normalization and palette derivation are unit-tested without HTTP.
- [ ] SPEC.md and DESIGN.md amended in their own docs commits.

## Size forecast per step

Authored changed lines, including tests, CSS and templates; docs commits excluded; the ornament SVG counted as authored.

| SPEC §12 step | Contents | Forecast | vs 400 budget |
|---|---|---|---|
| **29 Search** | `q=` + total, Google Books + merge + degradation, ISBN field + normalize/checksum, `publisher` + `isbn` + migration + export, P1–P9, P10, P11 (search part) | ~950–1,250 (P10 alone ~100–140) | ~2.5–3× |
| **30 Cover control** | origin/lock migration, upload + orientation + normalize + one-time pass, lock and revert, covers in export/import, `x/image`, P11–P13, P15 (upload part) | ~800–1,050 | ~2–2.5× |
| **31 Edition picker** | editions + Google candidates, choose-a-cover UI, picked lock, P11, P14, P15 (picker part) | ~420–580 | ~1–1.5× |
| **32 Generated cover and book object** | palette, ornament, text fit, detail tiers, book object, P11, P16 | ~450–630 | ~1.1–1.6× |
| **Total** | | **~2,600–3,500** | |

Natural work-unit commit boundaries inside each step, usable either as reviewable commits within one PR or as sub-PRs (Open question 1):
- 29: (a) Open Library `q=` + total + ISBN/publisher fields and migration; (b) Google Books + merge + source status; (c) result-list UX P1, P4–P8, P11; (d) ISBN field P9 and scan P10.
- 30: (a) origin/lock + revert + normalize on write and one-time pass; (b) upload pipeline and UI P12, P13, P15; (c) covers in export and import.
- 31: (a) candidate sources; (b) picker UI P14, P15.
- 32: (a) generated cover and P16; (b) book object.

## Open questions for the owner

1. **Review size per step.** Steps 29 and 30 forecast at about 2–3× the 400-line review budget, steps 31 and 32 slightly over. Keep one PR per step and accept the size, with the work-unit commits above as the review path; or split steps 29 and 30 into sub-PRs along those boundaries (still one §12 step each)?
2. **How does the phone reach the app?** Camera barcode scanning (P10) works only in a secure context: `localhost` or HTTPS. If the phone opens the app over plain HTTP on the local network, P10 would never appear there. Keep P10 (useful now or once served over HTTPS), or strike it from step 29 (saving about 100–140 lines)?

## Owner answers (2026-09-28)

1. PR size: steps 29 and 30 split into ~400-line sub-PRs along the listed commit boundaries; steps 31 and 32 one PR each (small `size:exception` if slightly over). Chain strategy to be chosen at the tasks phase.
2. P10 barcode scan: kept; the Scan action stays hidden until the app is served over HTTPS (or localhost).
3. P4–P9 and P11–P16: all kept.

No open questions remain for the proposal.
