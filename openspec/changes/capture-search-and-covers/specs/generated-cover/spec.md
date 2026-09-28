# Generated Cover Specification

## Purpose

A book with no cover today draws the same flat, anonymous banded-cloth swatch as every other coverless book of its format. This capability replaces that placeholder, for books only, with a deterministic ornamental hardcover drawn from the item's own title, author, and (optionally) publisher — so a coverless book still looks like a book, and building the shelf stays enjoyable (SPEC §0 goal 6, §6.6). It covers palette derivation, ornament and text rendering, detail scaling by plate size, the books-only rule, and the live capture preview (SPEC §12 step 32; capture UX items P11, P16).

The palette derivation MUST be a pure function of the book's title text, testable with `go test` alone with no HTTP or template imports (CLAUDE.md). Rendering (ornament, text layout, fitting) is HTML/CSS/SVG and is specified behaviorally below.

## Requirements

### Requirement: Books-Only Generated Cover Rule

The generated ornamental cover MUST be drawn only for items whose `format` is `book` and which show no real cover — that is, its cover choice is `removed`, or its cover choice is `found` with nothing currently found (see the `cover-management` capability). Every other format without a real cover MUST continue to draw today's flat banded-cloth plate, unchanged by this capability, regardless of its cover choice.

#### Scenario: Coverless book draws the generated cover

- GIVEN a book-format item that shows no real cover (cover choice `removed`, or `found` with nothing found)
- WHEN its plate renders anywhere in the app
- THEN the generated ornamental cover is drawn

#### Scenario: An explicitly removed book cover draws the generated cover

- GIVEN a book-format item whose cover choice is `removed`
- WHEN its plate renders
- THEN the generated ornamental cover is drawn, not a blank plate

#### Scenario: Coverless video keeps the banded cloth plate

- GIVEN a video-format item that shows no real cover
- WHEN its plate renders
- THEN the existing flat banded-cloth plate is drawn, not the generated cover

#### Scenario: Book with a real cover never shows the generated one

- GIVEN a book-format item whose cover choice is `found` with a cover currently found, `picked`, or `uploaded`
- WHEN its plate renders
- THEN the real cover is shown, and the generated cover is never drawn for that item

### Requirement: Deterministic Palette Derivation From the Title

The system MUST derive a book's generated-cover palette deterministically from its (normalized) title — trimmed of surrounding whitespace and case/character-folded — using a pure function with no randomness and no external state, drawn from a small fixed set of muted cloth hues paired with a gilt ink. The same normalized title text MUST always produce the same palette across renders, restarts, and environments. The palette MUST NOT depend on the item's id or any other identity the item only gains once filed, because the capture-time preview (see "Generated Cover Preview at Capture") must derive the same palette from the same typed title before the item exists at all.

#### Scenario: Same title, same palette every time

- GIVEN a book-format item with a fixed title and no cover
- WHEN its generated cover is rendered on two separate occasions (including after an app restart)
- THEN the derived palette is identical both times

#### Scenario: Capture preview matches the filed book

- GIVEN the owner types a title while capturing a new book with no cover chosen
- WHEN the live preview derives a palette from that typed title, and the item is then filed with the same title unchanged
- THEN the filed book's generated cover shows the identical palette the preview showed, because neither depends on the item's id

#### Scenario: Editing the title may change the palette

- GIVEN a filed, coverless book with a derived palette
- WHEN the owner edits its title to different words
- THEN the generated cover's palette may change to the one deterministically derived from the new title — this is expected behavior, not a defect, since the palette is a pure function of the title text

#### Scenario: Different titles, potentially different palettes

- GIVEN two distinct book-format items with different titles and no cover
- WHEN their generated covers are rendered
- THEN each palette is derived independently from its own normalized title, and the function contains no randomness that would make either non-reproducible

#### Scenario: Palette drawn from the fixed muted set

- GIVEN any title
- WHEN a palette is derived
- THEN the resulting cloth hue and gilt ink both belong to the small, fixed, tested set of muted cloth hues and gilt ink, never an arbitrary generated colour

### Requirement: Ornament Frame Rendering

The generated cover MUST draw a cloth ground in the derived per-book colour with a gilt double border and corner rosettes and palmettes, as one embedded SVG ornament coloured through CSS custom properties (not baked-in colour values), so the same SVG asset serves every palette.

#### Scenario: Ornament reflects the derived palette

- GIVEN a book with a derived palette
- WHEN its generated cover renders
- THEN the gilt double border, corner rosettes, and palmettes render in that palette's colours via CSS custom properties, not a palette-specific SVG file

#### Scenario: One shared SVG asset

- GIVEN two books with different derived palettes
- WHEN both generated covers render
- THEN both reference the same embedded ornament SVG, differing only in the CSS custom properties applied

### Requirement: Main Title Only, Set in Alegreya SC

The generated cover MUST show only the book's main title — the text before the first colon in its full title, or the whole title when it has none — rather than any subtitle that follows a colon. The main title, the author, and the publisher line (when present) MUST all be rendered in the self-hosted Alegreya SC small-caps face, as real HTML text (not rasterized into the SVG ornament), so the title reads in true small caps rather than a simulated capitalization. Wherever a coverless book's plate is shown, the item's complete title (including any subtitle) MUST remain available in the entry's own title text beside or above the plate, so the full title is never lost — only the cover art itself is abbreviated.

#### Scenario: Title with a subtitle

- GIVEN a book titled "Example Title: A Longer Subtitle"
- WHEN its generated cover renders
- THEN only "Example Title" appears on the cover art, set in Alegreya SC

#### Scenario: Title without a colon

- GIVEN a book titled with no colon anywhere in it
- WHEN its generated cover renders
- THEN the whole title appears on the cover art

#### Scenario: Full title still shown next to the plate

- GIVEN a coverless book with a subtitle
- WHEN its entry renders anywhere in the app
- THEN the entry's own title text (beside or above the plate) shows the complete title including the subtitle, even though the cover art itself shows only the main title

### Requirement: Text Layout and Fitting

The generated cover's author, main title, and publisher line (when present) MUST be sized with container query units and balanced wrapping, so long text fits without server-side font-metrics computation, and without overflowing or being clipped at any supported plate size.

#### Scenario: Short title and author fit cleanly

- GIVEN a book with a short main title and author name
- WHEN the generated cover renders at a typical plate size
- THEN both are fully legible within the plate's bounds

#### Scenario: Long title wraps and fits without overflow

- GIVEN a book with an unusually long main title
- WHEN the generated cover renders
- THEN the title wraps and its type size adapts via container query units so it fits within the plate without overflowing or being clipped, with no server-computed font metrics involved

### Requirement: Publisher Line Rule

The generated cover MUST show the publisher at the foot of the cover, below the main title, only when the item's `publisher` field is non-empty AND the cover is rendered at a detail tier that shows text beyond the title (see "Detail Tiers by Plate Size"). When `publisher` is empty, no publisher line — and no placeholder in its place — MUST appear at any tier.

#### Scenario: Publisher present at a detail tier that shows it

- GIVEN a book-format item with a non-empty `publisher`
- WHEN its generated cover renders at a plate size whose detail tier includes the publisher line
- THEN the publisher appears as a line at the foot of the cover

#### Scenario: Publisher absent

- GIVEN a book-format item with an empty `publisher`
- WHEN its generated cover renders at any plate size
- THEN no publisher line, and no empty placeholder for one, appears at the foot of the cover

### Requirement: Detail Tiers by Plate Size

The generated cover MUST scale its ornamental and textual detail across three tiers as the plate grows, so it stays recognisable as a book at the smallest dense-list sizes and shows full detail at the largest, single-focal sizes:
1. **Smallest (dense-choice thumbnail size, e.g. 2.25rem):** the cloth ground and a simple border only — no author, title, or publisher text.
2. **Middle (list and margin-plate sizes, e.g. 3rem and 4.75rem):** the simple border plus the main title as texture — no author or publisher text yet.
3. **Largest (focal plate sizes, e.g. the book page plate and the 9rem capture plate):** the full ornament (double gilt border, corner rosettes and palmettes), the author, the main title, and the publisher line when present.

#### Scenario: Smallest tier shows no text

- GIVEN a coverless book rendered at the smallest, dense-choice thumbnail size
- WHEN the generated cover renders
- THEN it shows only the cloth ground and a simple border, with no author, title, or publisher text, yet remains identifiable as a generated book cover rather than a blank swatch

#### Scenario: Middle tier shows the title only

- GIVEN a coverless book rendered at a list or margin-plate size
- WHEN the generated cover renders
- THEN it shows the simple border and the main title, but no author and no publisher line, even when `publisher` is set

#### Scenario: Largest tier shows full detail

- GIVEN a coverless book rendered at a focal plate size (the book page plate or the 9rem capture plate)
- WHEN the generated cover renders
- THEN the full ornament (double border, corner rosettes and palmettes), author, main title, and publisher line (if present) all render

#### Scenario: Title legible at the largest tier, recognisable at the smallest

- GIVEN a coverless book with typical title and author lengths
- WHEN rendered at the largest tier
- THEN title and author are legible
- WHEN the same book renders at the smallest tier
- THEN it is still recognisable as a generated book cover, even though no text is shown at that size

### Requirement: Accessible Name for the Generated Cover (P11)

The generated cover MUST be exposed to assistive technology as one accessible element named by the book's complete title (including any subtitle, unlike the abbreviated main-title-only text drawn on the cover art) and author, with its decorative ornament hidden from the accessibility tree.

#### Scenario: Screen reader announces the full title and author

- GIVEN a coverless book with a subtitle
- WHEN a screen reader encounters its generated cover
- THEN it is announced by the book's complete title (including the subtitle) and author, not by a generic "image", the ornament's decorative markup, or the cover art's abbreviated main-title-only text

#### Scenario: Ornament hidden from assistive technology

- GIVEN a rendered generated cover
- WHEN inspected for accessibility
- THEN the ornamental SVG elements carry `aria-hidden` (or equivalent) so they are not read as separate content

### Requirement: Generated Cover Preview at Capture (P16)

While capturing a new item with format set to `book` and no cover yet chosen, capture's plate MUST draw the generated cover live, derived from the currently typed title, author, and publisher, updating as those fields change, so the owner sees the book being filed.

#### Scenario: Live preview while typing

- GIVEN a new item being captured with format `book` and no cover chosen
- WHEN the owner types or edits the title, author, or publisher
- THEN the plate's generated cover preview updates to reflect the current values

#### Scenario: Preview replaced once a real cover is chosen

- GIVEN the live generated-cover preview is showing during capture
- WHEN the owner picks, uploads, or is fetched a real cover
- THEN the plate switches to showing that real cover instead of the generated preview

#### Scenario: Preview absent for non-book formats

- GIVEN a new item being captured with a format other than `book`
- WHEN the owner types title/author/publisher
- THEN no generated-cover preview is shown; the existing flat plate behavior applies

### Requirement: Contrast-Safe Palette Set

Every palette in the fixed, tested set MUST maintain adequate contrast between the gilt ink and its paired cloth ground, verified by a bounded visual check, and MUST NOT read as any of the app's semantic status inks (verdigris for actions/current-selection, rubric for owed/errors, ochre for flags) per DESIGN's One Job Rule — the generated cover's hues are decorative, not status inks, and are exempt from that rule's status meaning but must still avoid visually reading as one of those inks.

#### Scenario: Every palette entry passes a contrast check

- GIVEN the fixed set of muted cloth hues paired with the gilt ink
- WHEN each pairing is checked for contrast
- THEN gilt text and ornament lines remain legible against their cloth ground

#### Scenario: No palette reads as a status ink

- GIVEN the fixed palette set
- WHEN compared against the app's verdigris, rubric, and ochre status inks
- THEN none of the generated-cover hues is close enough to be mistaken for a status meaning
