---
name: Reading Tracker
description: A reader's commonplace book, written in white ink on warm black stock.
colors:
  stock: "oklch(17% 0.008 70)"
  stock-lift: "oklch(21.5% 0.009 70)"
  stock-raised: "oklch(27% 0.01 70)"
  rule: "oklch(50% 0.012 75)"
  rule-faint: "oklch(28% 0.01 72)"
  ink: "oklch(93% 0.016 85)"
  ink-soft: "oklch(82% 0.014 82)"
  pencil: "oklch(68% 0.012 78)"
  verdigris: "oklch(77% 0.08 170)"
  verdigris-bright: "oklch(83% 0.08 170)"
  verdigris-deep: "oklch(71% 0.08 170)"
  verdigris-wash: "oklch(31% 0.04 170)"
  on-verdigris: "oklch(20% 0.03 170)"
  rubric: "oklch(70% 0.15 33)"
  ochre: "oklch(81% 0.11 80)"
  cloth-book: "oklch(74% 0.07 262)"
  cloth-video: "oklch(74% 0.08 318)"
  cloth-article: "oklch(76% 0.08 132)"
  cloth-paper: "oklch(76% 0.07 12)"
  cloth-course: "oklch(76% 0.07 222)"
  float-shadow: "oklch(0% 0 0 / 0.7)"
typography:
  heading:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.625rem"
    fontWeight: 500
    lineHeight: 1.2
    letterSpacing: "-0.005em"
  heading-narrow:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.3125rem"
    fontWeight: 500
    lineHeight: 1.2
    letterSpacing: "-0.005em"
  title-lg:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.3125rem"
    fontWeight: 500
    lineHeight: 1.5
  title:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 500
    lineHeight: 1.3
  body:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.5
    fontFeature: "lnum"
  small:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
    lineHeight: 1.35
  label:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 500
    lineHeight: 1.35
  mark:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.9375rem"
    fontWeight: 400
  figure:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontFeature: "tnum, lnum"
  hand:
    fontFamily: "Alegreya, Georgia, serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    lineHeight: 1.35
  hand-field:
    fontFamily: "Alegreya, Georgia, serif"
    fontSize: "1.125rem"
    fontWeight: 400
rounded:
  none: "0"
  track: "2px"
  sm: "4px"
  md: "8px"
  lg: "10px"
  pill: "999px"
  point: "50%"
spacing:
  "1": "0.25rem"
  "2": "0.5rem"
  "3": "0.75rem"
  "4": "1rem"
  "5": "1.5rem"
  "6": "2rem"
  "7": "3rem"
  "8": "4.5rem"
  margin: "9.5rem"
  gutter: "1.75rem"
  measure: "50rem"
  prose: "36rem"
  target: "2.75rem"
  plate: "9rem"
components:
  button-primary:
    backgroundColor: "{colors.verdigris}"
    textColor: "{colors.on-verdigris}"
    typography: "{typography.title}"
    rounded: "{rounded.lg}"
    padding: "0 1.5rem"
    height: "2.75rem"
  button-primary-hover:
    backgroundColor: "{colors.verdigris-bright}"
  button-primary-active:
    backgroundColor: "{colors.verdigris-deep}"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.title}"
    rounded: "{rounded.lg}"
    padding: "0 1.5rem"
    height: "2.75rem"
  button-secondary-active:
    backgroundColor: "{colors.stock-lift}"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.ink-soft}"
    typography: "{typography.title}"
    padding: "0 0.25rem"
    height: "2.75rem"
  button-destructive:
    backgroundColor: "transparent"
    textColor: "{colors.rubric}"
    typography: "{typography.title}"
    rounded: "{rounded.lg}"
    padding: "0 1.5rem"
    height: "2.75rem"
  field:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0.5rem 0"
    height: "2.75rem"
  field-pencil:
    textColor: "{colors.pencil}"
  field-lg:
    typography: "{typography.title-lg}"
  field-hand:
    typography: "{typography.hand-field}"
  field-hand-placeholder:
    textColor: "{colors.pencil}"
    typography: "{typography.body}"
  entry-slot:
    backgroundColor: "{colors.stock}"
    textColor: "{colors.verdigris}"
    typography: "{typography.small}"
  entry-why:
    textColor: "{colors.ink-soft}"
    typography: "{typography.hand}"
  choice:
    backgroundColor: "transparent"
    textColor: "{colors.pencil}"
    typography: "{typography.body}"
    rounded: "{rounded.pill}"
    padding: "0.3rem 0.75rem"
  choice-hover:
    backgroundColor: "{colors.stock-lift}"
    textColor: "{colors.ink-soft}"
  choice-checked:
    backgroundColor: "{colors.verdigris-wash}"
    textColor: "{colors.ink}"
  choice-disabled:
    backgroundColor: "transparent"
    textColor: "{colors.rule}"
  check:
    rounded: "{rounded.sm}"
    size: "1.0625rem"
  picker-button:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.none}"
    padding: "0.5rem 0"
    height: "2.75rem"
  picker-list:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.md}"
    padding: "0.25rem"
  picker-option:
    textColor: "{colors.ink-soft}"
    typography: "{typography.body}"
    rounded: "{rounded.sm}"
    padding: "0 0.75rem"
    height: "2.5rem"
  picker-option-selected:
    textColor: "{colors.ink}"
  picker-new:
    textColor: "{colors.verdigris}"
  result-hover:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.md}"
    padding: "0.5rem"
  cover:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.sm}"
    width: "2.25rem"
  cover-preview:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.sm}"
    width: "4.75rem"
  cover-preview-wide:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.sm}"
    width: "5rem"
  cover-plate:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.md}"
    width: "9rem"
  cover-plate-wide-narrow:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.md}"
    width: "min(100%, 20rem)"
---

# Design System: Reading Tracker

## Overview

**Creative North Star: "The Commonplace Book"**

The app is the reader's commonplace book: a dark-paged notebook written in white ink. An item is an entry filed under a heading, and its why lives in a margin column beside it. Every screen is a page of that book. A narrow margin column on the left holds labels and the owner's marginalia, a single hairline margin rule runs the height of the page, and the entry lines sit to its right at a reading measure. Nothing is boxed; structure comes from rules, alignment and a small set of inks, each with one job.

The density is calm and use-focused. The owner files something in seconds on a phone at night or works through a weekly review at a laptop, so the page stays sparse, quiet and legible, with generous touch targets and no decoration that does not carry meaning. Warmth comes from the stock and the humanist Alegreya family, not from colour or ornament. The interface is dark only.

The system refuses the reading-tracker default of cover grids, cards and capture modals, and the generic dashboard of KPI tiles and rings. Where the discipline is measured (Home's board, the plan) it becomes a ledger: ruled sections titled in the margin, figures in aligned columns, thin bars that show what was read against what is due, and one large figure for what is left tonight. Debt stays a rubric number, never a message.

**Key Characteristics:**
- Warm black stock, white ink, graphite pencil for anything secondary or unconfirmed.
- One margin rule per page, fading at its ends, carrying rank slots, reading position and stall points.
- A margin column (9.5rem) beside an entry line (50rem) wide enough for ledgers and bars; running text keeps a 36rem reading measure; the owner's whys sit in the margin in italic.
- Fields written on a rule, choices picked as a soft pill around a word, labels and statuses as plain sentence-case pencil words.
- Restrained inks with fixed roles: verdigris acts, rubric owes, ochre flags, pencil qualifies, bookcloth names the format.
- Flat and unboxed, soft corners on what you press; open lists and cover plates are the only raised things; motion is a 180ms ink-in for state, and bars fill once in 600ms when a board draws.

## Colors

A near-monochrome page of warm neutrals with three sparing inks, each bound to one meaning.

### Primary
- **Verdigris** (`verdigris`): the ink of action and position. Primary buttons, focus outlines, caret and accent colour, the checked state of checks, the running-head name (it links home), the current running-head link, link hover underlines, the picker's selected check and its New shelf… option, rank slot numbers, the reading mark on an item in progress, and the inked length of the reading-position track. Bright and deep variants (`verdigris-bright`, `verdigris-deep`) are hover and press states of the primary button only. `on-verdigris` is the dark ink written on a verdigris ground. `verdigris-wash` is the text-selection ground and the checked pill of a choice.

### Secondary
- **Rubric** (`rubric`): the ink of what is owed, wrong or irreversible. Owed figures on the board, the hatched owed zone of the today bar, short days in the day strip, owed marks, field error rules and error messages, lookup failures, and the destructive button's text and outline.

### Tertiary
- **Ochre** (`ochre`): flags without blame. The stall point on the margin rule, the stalled mark, and marks for being over a soft limit.

### Bookcloth
Five muted cloths, one per format, all at about the same lightness and chroma so none outranks another.
- **Book Cloth** (`cloth-book`), **Video Cloth** (`cloth-video`), **Article Cloth** (`cloth-article`), **Paper Cloth** (`cloth-paper`), **Course Cloth** (`cloth-course`): each names its format and nothing else. A format carries its cloth as a local colour: on a format choice the checked pill is a 24% cloth mix with the word at 92% lightness in a cloth tint, and hover tints the word in cloth at 86% lightness; wherever an item shows its format, a 0.5rem cloth dot sits before the format word. Without a cloth, the dot falls back to pencil.

### Neutral
- **Warm Black Stock** (`stock`): the page. Also the knockout behind rank slots and stall points so they sit on the margin rule cleanly.
- **Raised Stock** (`stock-raised`): the step above lifted stock, only for hover and the keyboard-active row inside a lifted surface such as the picker list.
- **Lifted Stock** (`stock-lift`): the one tonal step up. Hover wash on search results and choices, pressed secondary button, the picker list ground, empty cover placeholders.
- **White Ink** (`ink`): titles, entered text, settled figures, anything confirmed.
- **Soft Ink** (`ink-soft`): the owner's secondary words (whys in the margin), check labels, quiet buttons, picker options at rest, field hover rule.
- **Graphite Pencil** (`pencil`): labels, margin notes, metadata, hints, placeholders, estimates, unselected choices, disabled fields, the picker chevron, and lookup-filled values not yet confirmed.
- **Rule** (`rule`): the only edge an empty field has, so it holds about 3:1 against the stock. The margin rule and the running-head rule (both faded at their ends, so they need the brighter rule to still read), field underlines, the secondary button outline, link and quiet-button underlines at rest, disabled choices and check borders, the scrollbar thumb, the unread length of the reading-position track.
- **Faint Rule** (`rule-faint`): textarea ruling, cover outlines, the picker list border and its separator.
- **Float Shadow** (`float-shadow`): the 70% black of the float shadow under open lists and cover plates, and nothing else.

### Named Rules

**The One Job Rule.** Each ink has one meaning. Verdigris is only actions, focus, current selection, rank slots and where the reading is. Rubric is only what is owed, wrong or irreversible: debt, errors and destructive verbs. Ochre is only flags without blame. Pencil is labels, metadata, estimates and unconfirmed values. Cloth is only the format: a format's cloth names that format and is never used for a status, shelf, action or emphasis, and on a format choice it stands in for verdigris as the checked colour. If a colour would be used for anything else, use white ink or pencil instead.

**The Pencil Until Confirmed Rule.** Anything the app found, guessed or estimated is written in pencil. A lookup-filled field stays pencil until the owner touches it: on focus it inks to white, and typing in it or picking a search result confirms it for good.

**The Plain Number Rule.** Status figures are plain tabular numbers in white ink, never tiles or rings. Debt is the same figure in rubric, with no message attached.

## Typography

**Interface Font:** Alegreya Sans 400 and 500 (with ui-sans-serif, system-ui, sans-serif)
**Hand Font:** Alegreya Italic 400 (with Georgia, serif)

Both are self-hosted WOFF2 subsets under the OFL, with lining and tabular figures and kerning retained in the subset.

**Character:** A humanist sans for everything the app writes, paired with its sibling serif italic for everything the owner writes, so the page reads as a printed ledger with handwritten marginalia.

### Hierarchy
- **Heading** (500, 1.625rem, 1.2, -0.005em; 1.3125rem on narrow screens): shelf and page headings. Balanced wrapping.
- **Title, large** (500, 1.3125rem): the first line of an entry page, the title or link field.
- **Title** (500, 1.0625rem, 1.3): entry titles in lists and results; also button text at line-height 1.
- **Body** (400, 1.0625rem, 1.5, lining figures): running text and field values. Entry lines hold a 36rem measure.
- **Small** (400, 0.9375rem, 1.35): margin notes, entry metadata, hints, errors, result metadata.
- **Label** (500, 0.9375rem, 1.35, sentence case, no letter spacing): form labels and the filed label in the margin column.
- **Mark** (400, 0.9375rem, sentence case, no letter spacing): status words such as format, focus, desk, stalled, review due. Never wraps.
- **Figure** (tabular and lining numerals): any number that may be compared: times, pages, counts, debt.
- **Hand** (Alegreya Italic 400, 1.0625rem in the margin, 1.125rem in fields): the owner's own words only: why, verdict, notes.

### Named Rules

**The Owner's Hand Rule.** Alegreya Italic is used only for words the owner wrote: why, verdict, notes. App-written text (labels, hints, placeholders, confirmations, errors, estimates) is never set in the hand, even inside a hand field.

**The Plain Label Rule.** Labels are plain sentence-case words in pencil (Title or link, Shelf, Filed): no small caps, no all-caps, no letter spacing. They sit in the margin beside their line, or just above it on narrow screens. They name a row; they never sit above a heading as a kicker or eyebrow.

## Layout

The page is a single centred column, max width margin + gutter + gutter + measure plus page padding (about 64rem). Inside it a two-column grid repeats on every row: the margin column (9.5rem, right-aligned) and the entry line (flexible, up to 50rem), separated by twice the gutter (1.75rem each). The wide line keeps the margin rule well left of centre and gives ledgers and bars room; running text (notes, help, confirmations, summaries) caps itself at the 36rem reading measure. The margin rule is drawn once per page at margin + gutter, a 1px vertical line in rule colour that fades: transparent at the top of the page, full from 12rem (beside the heading) to 55% of the page height, transparent again at the bottom. Anything that marks position on it (rank slots, reading position, stall point) carries its own ink, so it stays visible where the rule fades.

The running head shares the page width: app name at left in medium verdigris, nav at right, and a 1px rule beneath that fades at both ends (transparent to full rule colour at 25%, full to 75%, transparent at the right). The page has 2rem top and 4.5rem bottom padding, and 1rem side padding.

Vertical rhythm uses an 8-step scale from 0.25rem to 4.5rem. Consecutive form rows sit 1.5rem apart; a new group of rows opens with 3rem. The one exception is the reserved filed line under a page heading (see Filed confirmation), which stands in for the group gap. List entries have 0.75rem block padding and 0.75rem between them. Inline clusters (actions, metadata, marks, status strips) wrap with 0.75rem to 1.5rem gaps.

Every interactive element has a minimum 2.75rem target, including choices, checks and nav links.

**Plate column (76rem and wider):** a page may opt in to a plate column at its right (capture does). The running head and the page both widen by one gutter plus the plate width (9rem), and the page reserves that width as right padding. The whole composition (margin column, entry lines, plate column) centres, and the head rule spans it. The column is reserved whether or not a cover is showing, so nothing moves when one appears. See Cover plate.

**Narrow screens (below 48rem):** the margin folds onto the page. The margin rule moves to the left page edge, rows and entries become one column indented by a 1rem gutter, labels and notes sit above their line left-aligned, the why drops beneath its entry body, and the heading steps down to 1.3125rem. Rank slots, progress and stall marks stay on the (now left) margin rule. A found cover plate sits centred under the title (see Cover plate).

### Named Rules

**The One Margin Rule Rule.** A page has exactly one vertical rule, and everything that marks position (rank slot, reading position, stall) sits on it in its own ink. Do not add second rules, side borders or column dividers.

## Elevation & Depth

The system is flat. The page is one stock, a single lifted stock tone marks hover and pressed states, and hairline rules separate what boxes would separate elsewhere. There is no blur and no layering of surfaces, with two exceptions that share one shadow: an open list (the picker list), which floats over the page while it is open, and a cover plate, the found cover lifted off the page like a picture tipped in.

The CSS uses `box-shadow` in two non-elevation ways: a 1px under-rule that thickens a focused or invalid field's underline to 2px, and a 3px stock-coloured ring that knocks the margin rule out around a stall point. The float shadow under open lists and cover plates is the one elevation shadow. Gradients appear only as functional drawing: the textarea's repeating ruled lines, the two-tone reading-position track, and the fades at the ends of the running-head rule and the margin rule.

### Shadow Vocabulary
- **Field under-rule** (`box-shadow: 0 1px 0 var(--verdigris)`, rubric when invalid): thickens a field's rule to 2px. Not elevation.
- **Stall knockout** (`box-shadow: 0 0 0 3px var(--stock)`): clears the margin rule around the ochre stall point. Not elevation.
- **Float** (`--shadow-float`, `box-shadow: 0 16px 40px -12px oklch(0% 0 0 / 0.7)`): open lists and cover plates only. The small margin cover between 48rem and 76rem takes no shadow.

### Named Rules

**The Flat Page Rule.** No cards, no drop shadows, no decorative gradients. If something needs separating, rule it; if something needs emphasis, ink it. The only exceptions are open lists and cover plates, both lifted by the one float shadow; nothing else is ever raised. Boards separate with ruled sections, not boxes: a hairline over each block's body and its title in the margin. The lifted-stock ground appears only as a bar track, the "If you save" summary, and today's row in a ledger (a verdigris wash).

## Shapes

Forms are lines, not boxes. Fields, including the picker button, are a bottom rule only with square ends. What you press is softened on a small scale: checks and cover thumbnails take a small corner (4px), search results, the picker list, cover plates and focus rings a gentle one (8px, `--radius`), buttons a slightly rounder one (10px); a picker option nests inside its list at 4px. Pick-a-word choices are fully rounded pills (999px). The reading-position track has 2px corners. The only circles are small points: the ochre stall point, the hollow "due" ring and the format dot, all 50%. Covers are 2:3 portrait images with a faint inset outline; wide covers from videos and articles are 16:9. A cover plate is 9rem wide with 8px corners.

## Components

### Buttons
Few, plain and verb-labelled; one primary per page.
- **Shape:** 10px corners, 2.75rem tall, 1.5rem side padding, medium weight at line-height 1.
- **Primary:** verdigris ground with dark on-verdigris text. The single committing action of a page (for example File it).
- **Hover / Active:** primary brightens on hover, deepens on press. Transitions are 180ms on background, border and colour. Focus is the global 2px verdigris outline at 3px offset with 8px corners.
- **Secondary:** transparent with a rule-coloured 1px outline; the outline lightens to soft ink on hover and the ground lifts on press.
- **Quiet:** a text button in soft ink with a rule-coloured underline; hover inks the text white and turns the underline verdigris; 0.25rem side padding. For alternative paths (Paste the text instead), including as the summary of a disclosure, which stays white while open.
- **Destructive:** rubric text with a 45% rubric outline; hover adds a 12% rubric wash and a full rubric outline. For irreversible verbs such as Abandon.
- **Disabled:** 45% opacity, not-allowed cursor.
- **Busy:** a 1px pencil stroke draws left to right under the label on a 1.2s loop; with reduced motion it is a static line at 60% opacity. The same stroke marks any inline pending text (Looking it up) in pencil.

### Choices
Pick a word, and it sits in a soft pill.
- **Style:** a wrapping row of lowercase words in pencil, each a full 2.75rem target with the radio input stretched invisibly over it. The word carries a pill (999px, 0.3rem by 0.75rem padding); pills sit 0.5rem apart. The row hangs 0.75rem to the left so the words, not the pills, align with the labels and fields above.
- **State:** at rest, pencil with no ground. Hover: soft ink on lifted stock. Checked: white ink on the verdigris wash. Focus: a 2px verdigris outline around the pill at 2px offset. Disabled: rule-coloured word with no wash. Colour and ground transition over 180ms.
- **Format choice:** each format word wears its bookcloth instead of verdigris. Checked is a 24% cloth mix with the word in a 92%-lightness cloth tint; hover tints the word in cloth over the lifted-stock wash.

### Checks
- **Style:** a 1.0625rem square with a 1px soft-ink border and 4px corners, label in soft ink.
- **State:** checked fills verdigris and scales in a dark tick over 180ms; disabled uses a rule border and pencil label.

### Marks
Plain words instead of badges or chips.
- **Marks:** sentence-case words at 0.9375rem, pencil by default, ochre for flags without blame (stalled, over a soft limit), rubric for owed. No background, no border.
- **Format mark:** the format word led by a 0.5rem dot of its bookcloth, 0.35rem before it.
- **Due:** a due item is a mark led by a small hollow ring.
- **Provisional estimate:** pencil text with a dotted 1px underline, kept on one unbreakable line (spec §7.3), with its reason available on hover.

### Inputs / Fields
Written on the rule.
- **Style:** transparent, no box, a 1px rule-coloured bottom border, square ends, 0.5rem block padding, full width, 2.75rem minimum height. Placeholders are pencil.
- **Hover:** the rule lightens to soft ink.
- **Focus:** the rule turns verdigris and thickens to 2px; no outline box.
- **Error (in band):** the rule turns rubric and thickens to 2px; a small rubric message sits 0.25rem beneath and collapses when empty. A rejected submit moves focus to the first invalid field, and typing in it clears its error. Lookup failures use the same slot on the field they concern. Hints use the same slot in pencil.
- **Unconfirmed (lookup-filled):** value in pencil; it inks to white on focus and loses the pencil state on the first input.
- **Disabled:** pencil value on a dotted rule.
- **Variants:** large title field (1.3125rem medium); hand field for the why (Alegreya Italic 1.125rem, its placeholder upright in the interface face at body size, per the Owner's Hand Rule); short numeric field (7rem, tabular figures).
- **Pasted text:** a textarea drawn as a ruled notebook page, faint lines every 1.75rem that scroll with the text, six lines tall, vertically resizable, no bottom rule and no focus under-rule.
- **State attributes:** invalid and busy states are always written as explicit `"true"` or `"false"` strings, never left absent or implied.
- **Copy:** one plain sentence that says what to do next, no blame, no exclamation: Give it a title. Write one line on why before filing it. Pick a shelf, or add the new one first. Use a whole number. Name the shelf. A shelf with that name already exists. That link doesn't look right. Couldn't read this page. Fill in the title by hand. Open Library didn't answer. Type the details in.

### Navigation
- **Running head:** app name at left in medium verdigris, sentence case, not underlined; it links home. Nav links at right in plain pencil at body size, 1.5rem apart, 2.75rem targets. Hover inks to white; the current page is white with a verdigris underline. A rule that fades at both ends closes the head; on a page with a plate column it spans the whole composition.
- **Links:** one link per route that exists, in the order screens arrive; Capture is the first. Never a link to a screen not built yet.
- **Shelves index:** shelf names in their own order, as entry titles, one per line, each linking to its shelf. No counts and no contents: it is a door, not a summary of the library (spec §0).

### Entries (signature component)
Items as rows of a commonplace book.
- **Structure:** the same margin and line grid as form rows. The title (medium white) and a pencil metadata line of author, marks and figures sit on the entry line; the why sits in the margin, right-aligned, in the owner's italic hand in soft ink.
- **Rank slot:** the slot number in medium verdigris tabular figures, centred on the margin rule with a stock knockout.
- **Reading position:** a 3px wide, 2.5rem tall track with 2px corners laid on the margin rule; the read fraction is inked verdigris from the top, the rest in rule colour. Every track is the same length so entries compare at a glance.
- **Stall:** an 0.5rem ochre point on the margin rule with a 3px stock knockout ring.
- **Doesn't fit the moment:** title, why and slot drop to pencil; the entry stays on the page.
- **Borrowed:** an item shown on a shelf because a tag matches that shelf's name carries a pencil mark naming its home — from Statistics — in the metadata line beside author, format and size. No badge, no border, no indent: the grid is never broken for a status.
- **Already being read:** a pencil-sized verdigris mark reading *reading* at the end of the metadata line. Verdigris marks where the reading is, so this belongs to it rather than to ochre or cloth.
- **Empty rank slot:** the slot number in pencil (not verdigris: the position is unclaimed) on the margin rule, with the word Empty in pencil on the entry line. An empty slot is drawn, never omitted — a slot that is not there prompts nothing.
- **Actions:** a quiet cluster on its own line under the entry, on the entry line. A shelf leader offers ↑ and ↓ (plain glyphs, no underline, verdigris on hover, disabled rather than hidden at the ends so the row never reflows) and Unrank; a pool item offers Rank when a slot is free, and nothing when all three are taken. Every entry offers Edit. Controls are always visible — hover-reveal is no control at all on a phone.
- **Divider:** between the three slots and the pool, a hairline on the entry line only, fading out to the right. The one horizontal rule any list gets.
- **Opened for editing:** the entry gives way to the item form in its own place, with room above and below so it reads as the one thing being worked on. While a row is open, no other row shows its controls, so a stray click cannot discard what is being typed.
- **Narrow screens:** title and metadata, then the why beneath them, then the actions. The why always stays next to what it explains.

### Board (Home and the plan)
Where the discipline is measured: hours and speed, as ruled sections.
- **Block:** a section with its title (medium white, body size) and a pencil aside in the margin column, right-aligned; its body on the entry line under a 1px faint hairline, children 1.5rem apart. Blocks sit 2rem apart. On narrow screens the title and aside share a line above the hairline.
- **Tonight's figure:** the one large number, 2.75rem medium white tabular ("1 h 35 min"), followed by "to go tonight" in soft ink at 1.3125rem and one soft-ink sentence saying what reading it achieves. When nothing is left it reads "Done for today" or "Rest day".
- **Meter:** a line of words with white figures right-aligned ("0 min of 1 h 30 min"), a bar, and a small pencil line under it ("1 h 30 min to the target", owed in rubric).
- **Bar:** a 0.625rem rounded track in lifted stock with a faint inset edge; the read part fills in verdigris from the left; a 2px white tick marks what is due; what is owed is a hatched rubric zone after the target that reading fills; a pencil tick marks a baseline. A legend names read, due and owed. Bars fill once over 600ms when drawn; reduced motion shows them filled.
- **Mix bar:** one track split into rounded parts, each in its format's bookcloth and sized by share of hours, with a legend of dots, words and percentages. It always sits under a speed (§9.1).
- **Day strip:** seven columns, one per day of the week in week order: day name, a 0.75rem-wide rounded column filled to the share of the target read (verdigris, rubric when a closed day fell short), the time read and the target. Rest days are a dashed outline; today is outlined in verdigris.
- **Ledger:** a full-width table, pencil column heads over a rule, a faint hairline under every row, figures right-aligned in tabular numerals, today's or this week's row on a 40% verdigris wash in medium white, results that rose in verdigris, quiet rows in pencil. Wide ledgers scroll inside their own wrapper; the page never scrolls sideways.
- **Notes:** "How it works" prose on the plan only, at body size in soft ink, 1.6 line height, capped at 36rem, under a faint hairline with a medium white subheading. Home carries no explanations.
- **Time field:** a field (12rem max) that reads time the way it is said ("1h30", "1:30", "1.5h", "90"), with a pencil readback beneath ("= 1 h 30 min") so a typo cannot slip through; positions in videos read player time ("1:12:30").
- **Summary:** "If you save: …" in soft ink on lifted stock, 8px corners, capped at 36rem, redrawn from the server as the form is typed.

### Picker
Our own list for choosing one of many, in place of the native select.
- **Button:** a field on a rule (square ends, 2.75rem, bottom rule, verdigris under-rule on focus, rubric when invalid) showing the current value in white ink, with a 0.75rem pencil chevron at the right that turns 180° over 180ms while the list is open.
- **List:** opens 0.375rem under the button at its full width: lifted stock, a 1px faint-rule border, 8px corners, 0.25rem inner padding, the float shadow, at most 22rem tall and scrolling beyond that.
- **Options:** rows 2.75rem tall (the touch target) with 0.75rem side padding and 4px corners, soft ink at rest. Hover and the keyboard-active row ink to white on raised stock (`stock-raised`). The selected row is white with a 0.75rem verdigris check at its right.
- **Separator and add:** a faint hairline sets the add option apart; the add option (New shelf…) is written in verdigris.
- **Behaviour:** click toggles the list; a click outside or focus leaving the picker closes it. Arrow keys open it on the current value and move the active row; Enter or Space picks; Escape closes. The button carries aria-haspopup="listbox", aria-expanded and aria-activedescendant; options carry aria-selected.

### Search results
Candidates as an ink list directly beneath the field that searched, on the entry line, 0.75rem below it.
- **Style:** each is a button row of a 2.25rem 2:3 cover (Open Library's medium thumbnail) beside a white medium title with a small pencil metadata line under it (author, year, pages in tabular figures), bleeding 0.5rem beyond the line so the hover wash aligns text with the fields above. Covers have 4px corners. A missing cover is an empty lifted-stock placeholder of the same size.
- **State:** hover lifts the ground to lifted stock with 8px corners. Picking a result fills its fields as confirmed (white, not pencil), keeps Open Library's large cover for the cover plate, and moves focus to the why.
- **Empty:** a single pencil hint in the list's place: No matches on Open Library. Keep typing, or fill in the details yourself.
- **Dismissal:** the list closes on Escape, when focus moves to anything outside the searching field and the list, and on submit.

### Form reveals
Parts of a form appear in place only when they apply; nothing opens over the page.
- **Pending lookup:** a pencil hint with the pending stroke (Looking it up) in the searching field's hint slot.
- **Cover plate:** a found cover shown beside the lines it describes, like a picture tipped into a notebook page. Appearing never moves the focused title field.
  - **76rem and wider:** in the page's plate column, 1.75rem (one gutter) from the entry lines, top-aligned with the title field (0.6rem down the title row), 9rem wide for tall and wide covers alike, with 8px corners and the float shadow. The column is reserved before a cover exists.
  - **48rem to 76rem:** tipped into the free left part of the margin, beside the title and author lines: books 4.75rem wide at 2:3, videos and articles 5rem wide at 16:9, 4px corners, no shadow. It floats, so it never pushes the form.
  - **Below 48rem:** once there is a cover, the title row reserves bottom padding of the plate height plus 2rem, and the plate sits centred on the entry line at the bottom of that space: 9rem wide, or for wide covers the line width up to 20rem at 16:9, with 8px corners and the float shadow. Only rows below the title move; the title field keeps no right padding.
- **New shelf:** the shelf picker's last option, New shelf…, reveals an inline name field and a secondary Add shelf button on one line, 0.75rem beneath the picker, and moves focus to the field; Enter in the field adds too, and the picker is redrawn with the new shelf selected.
- **Pasted text:** for articles only, a quiet Paste the text instead summary 0.75rem beneath the size line opens the ruled textarea 0.75rem below it.

### Filed confirmation
A plain status line under the page heading: Filed as a label in the margin, and "<title>, on <shelf>." on the entry line.
- **Reserved line:** the line keeps one body line of height while empty, 0.5rem under the heading and 0.75rem above the first row, so filing never moves the form and the heading-to-first-field distance stays close to a 3rem group gap.
- **Ink-in:** on filing, label and line ink in over 180ms, from transparent pencil to their final colours, with no movement. The form resets and focus returns to the first field.

## Do's and Don'ts

### Do:
- **Do** put every row on the margin grid: label or note in the 11rem margin, content on the entry line.
- **Do** mark rank, reading position and stall on the single margin rule, never elsewhere.
- **Do** write fields on a 1px rule and signal focus by turning the rule verdigris at 2px.
- **Do** keep verdigris for actions, focus, current selection, rank slots and where the reading is only.
- **Do** give each format its own bookcloth and use it only to name that format: the format choice and the format dot.
- **Do** keep rubric for what is owed or wrong: debt, field errors and failures, and destructive verbs.
- **Do** use ochre for flags without blame: stalled, over a soft limit.
- **Do** write labels, metadata, estimates and lookup-filled values in pencil.
- **Do** set the owner's why, verdict and notes in Alegreya Italic, and nothing else.
- **Do** show provisional estimates as one unbreakable pencil unit with a dotted underline.
- **Do** use tabular figures for any number that may be compared, and show status numbers plain.
- **Do** keep every target at least 2.75rem and fold the margin onto the page below 48rem.
- **Do** write labels and marks as plain sentence-case words, with no small caps or letter spacing.
- **Do** choose one of many with the drawn picker, not a native select.
- **Do** limit motion to 180ms state changes on opacity and colour (and the pending stroke, the picker chevron's turn, and a bar filling once in 600ms), and honour reduced motion.
- **Do** measure the discipline as a board: ruled blocks titled in the margin, one large figure for tonight, bars and ledgers with every figure also written out.
- **Do** reserve the space of a confirmation line or a cover plate so an action never moves the field in use.
- **Do** write terse English copy: nouns for labels, verbs for buttons, no exclamation marks.

### Don't:
- **Don't** use cards, boxed panels, drop shadows or decorative gradients; the only gradients are the textarea ruling, the reading-position track, the hatched owed zone of a bar and the end fades of the running-head rule and the margin rule, and open lists and cover plates are the only surfaces with a shadow.
- **Don't** add kickers or eyebrow labels above headings or heading rows.
- **Don't** use chips or filled badges for status; use plain pencil marks. Pills belong only to pick-a-word choices.
- **Don't** set app-written text in the italic hand.
- **Don't** add a light theme or a second vertical rule.
- **Don't** show debt as a message, a ring, a tile or a celebration; it is a rubric number.
- **Don't** move elements more than 4px in any transition.
- **Don't** use radii outside the scale: 2px track, 4px checks and cover thumbnails, 8px results, lists, cover plates and focus rings, 10px buttons, full pills for choices, 50% for points; fields stay square-ended.
