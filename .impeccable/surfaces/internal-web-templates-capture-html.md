---
version: 1
slug: "internal-web-templates-capture-html"
primary_target: "internal/web/templates/capture.html"
related_targets: ["internal/web/templates/layout.html","internal/web/static/app.css"]
---

# Capture (and the app shell it establishes)

Mode: Operate. First screen of the app; its tokens, shell and primitives become the world every later screen inherits. A temporary, uncommitted `/kit` page exercises the primitives while this surface is built.

- **Audience and job:** the single owner, on a phone at night or at a laptop, filing something to read in seconds. Paste a URL, search Open Library, or type it in; pick a shelf; write one line of why; file it.
- **Content ranges:** titles 2–120 characters, authors often absent, whys 20–140 characters, 1–30 shelves with names up to ~30 characters, tags 0–5. Covers present for most books, absent for manual items.
- **States that matter:** empty library (no shelves: new shelf first), looking up, lookup failed, search results / no matches, field errors, filed confirmation, pasted text open.
- **Anti-goals:** gamified or motivational tone, dashboard tiles, modal-first flows, bloat, a light theme.

## Direction contract

THESIS: The app is the reader's commonplace book, a dark-paged notebook written in white ink. An item is an entry filed under a heading, and its why lives in a margin column beside it. Refused default: the reading-tracker cover grid with cards, progress bars and a capture modal.

OWN-WORLD: Black stock (warm near-black) with white ink for text and graphite pencil for anything provisional, secondary or estimated. Verdigris ink marks actions, focus and the current selection. Rubric red is reserved for what is owed or wrong (debt, field errors). Alegreya Sans is the interface hand; Alegreya Italic is used only for the owner's own words (why, verdict, notes). Structure is ruled: hairline rules instead of boxes, a single margin rule dividing margin notes from entry lines, running heads in small caps, tabular figures in one measured column. Inputs are written on a rule, not drawn as boxes. No cards, no shadows, no gradients.

STORY: The owner opens capture and sees a fresh entry page. They write or paste on the first line, and the page fills what it can find, in pencil until confirmed. They choose the heading, write the why in the margin, and file it. The entry settles onto the page as filed, and a new blank line waits.

FIRST VIEWPORT: The shell's running head holds the app name at left in small caps and the nav at right, separated from the page by a hairline rule. Beneath it sits one entry page, max ~46rem, with a margin column (~11rem on desktop) left of a single vertical margin rule. Each form line is a row: its label in small caps in the margin, its field written on the rule in the body. The first line is the large title/URL field; lookup progress and results appear as pencil notes beside it. Then the heading (shelf) and format; then the why, written in Alegreya Italic. The shape row (focus, size, desk) is compact. "File it" is the one verdigris button at the foot of the entry lines. In list rows (shelf, home, archive) the why moves into the margin beside its title. On phones the margin collapses: labels sit above their line, notes indent beneath it, and "File it" stays reachable.

FORM: Commonplace book with marginalia, position 6 of 7 on the grounded list; seed key c00fd8e2. Signature interaction: filing, where the entry's pencil fields ink in and the confirmation settles as a new filed line in the margin while the page resets. Motion grammar: 180 ms ink-in (opacity and colour only), no movement beyond 4 px, reduced-motion respected. Raised lines: entries edited where they are read; one measured column for figures; the moment filter gathers the page in place; every flattering number tied to its explanation by a margin note; one margin rule carries alignment, reading position and stall marks.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Adaptations and deferrals

- Page measure: the built page is ~50rem (margin 11rem, rule at 12.75rem, entry lines 36rem) rather than ~46rem, so entry lines keep a 65–75ch reading measure beside the margin.
- Running-head nav ships with the capture route: one link per existing route, Capture first.
- Pending strokes animate; still captures cannot show them.
- Row order: format sits with title and author, before the shelf, because a lookup sets the format; the three rows that a lookup fills stay together.
- Lookup results render as an ink list beneath the title line (DESIGN.md "Search results"), not as pencil notes in the margin: five candidates with covers need the entry column's width, and each is a tappable choice rather than a note.
- "File it stays reachable" on phones is met by Enter-to-submit from any single-line field, plus the button at the foot of the form.
- The filed confirmation sits directly above the title line, where focus returns after filing, so it inks in inside the viewport on both devices.

## Owner polish round (2026-09-15)

After using the built screen, the owner compared variants on a temporary page and chose these. They supersede the contract where the two disagree.
- Labels, marks and the running head are plain sentence case with no small caps or tracking. The app name is verdigris.
- Softened shape: 4/8/10px radii, and pick-a-word choices as soft pills instead of underlines.
- Bookcloth: each format has one muted colour, on its choice pill and as a dot beside its mark.
- The shelf picker is a drawn listbox (`.picker`) instead of the browser select.


## Owner polish round two (2026-09-15)

Chosen from a second variants page. These supersede the earlier cover placement.
- Faded dividers: the running-head rule fades at both ends, and the margin rule fades in beside the heading and out down the page.
- On wide screens (≥76rem) the capture page gains a plate column. The found cover is a 9rem plate top-aligned with the title field, one gutter from the entry lines, and the whole composition centres.
- On phones the plate sits centred under the title, in space reserved only once a cover exists.
- Picked books keep Open Library's large cover; search results show the medium thumbnail.
