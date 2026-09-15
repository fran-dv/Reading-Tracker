---
name: Reading Tracker
description: A reader's commonplace book, written in white ink on warm black stock.
colors:
  stock: "oklch(17% 0.008 70)"
  stock-lift: "oklch(21.5% 0.009 70)"
  rule: "oklch(38% 0.012 75)"
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
    fontSize: "1.0625rem"
    fontWeight: 500
    letterSpacing: "0.06em"
    fontFeature: "c2sc, smcp"
  mark:
    fontFamily: "Alegreya Sans, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 400
    letterSpacing: "0.05em"
    fontFeature: "c2sc, smcp"
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
  sm: "2px"
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
  margin: "11rem"
  gutter: "1.75rem"
  measure: "36rem"
  target: "2.75rem"
components:
  button-primary:
    backgroundColor: "{colors.verdigris}"
    textColor: "{colors.on-verdigris}"
    typography: "{typography.title}"
    rounded: "{rounded.sm}"
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
    rounded: "{rounded.sm}"
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
    rounded: "{rounded.sm}"
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
  entry-slot:
    backgroundColor: "{colors.stock}"
    textColor: "{colors.verdigris}"
    typography: "{typography.small}"
  entry-why:
    textColor: "{colors.ink-soft}"
    typography: "{typography.hand}"
  result-hover:
    backgroundColor: "{colors.stock-lift}"
    rounded: "{rounded.sm}"
    padding: "0.5rem"
  cover:
    backgroundColor: "{colors.stock-lift}"
    width: "2.25rem"
---

# Design System: Reading Tracker

## Overview

**Creative North Star: "The Commonplace Book"**

The app is the reader's commonplace book: a dark-paged notebook written in white ink. An item is an entry filed under a heading, and its why lives in a margin column beside it. Every screen is a page of that book. A narrow margin column on the left holds labels and the owner's marginalia, a single hairline margin rule runs the height of the page, and the entry lines sit to its right at a reading measure. Nothing is boxed; structure comes from rules, alignment and a small set of inks, each with one job.

The density is calm and use-focused. The owner files something in seconds on a phone at night or works through a weekly review at a laptop, so the page stays sparse, quiet and legible, with generous touch targets and no decoration that does not carry meaning. Warmth comes from the stock and the humanist Alegreya family, not from colour or ornament. The interface is dark only.

The system refuses the reading-tracker default of cover grids, cards, progress bars and capture modals, and the generic dashboard of KPI tiles, rings and big-number cards. Numbers are plain figures in a measured column; debt is a number in rubric, not a message.

**Key Characteristics:**
- Warm black stock, white ink, graphite pencil for anything secondary or unconfirmed.
- One margin rule per page, carrying rank slots, reading position and stall points.
- A margin column (11rem) beside entry lines (36rem); the owner's whys sit in the margin in italic.
- Fields written on a rule, choices picked by underlining a word, statuses as small-caps words.
- Restrained inks with fixed roles: verdigris acts, rubric owes, ochre flags, pencil qualifies.
- Flat, unboxed, 2px corners at most; motion is a 180ms ink-in for state only.

## Colors

A near-monochrome page of warm neutrals with three sparing inks, each bound to one meaning.

### Primary
- **Verdigris** (`verdigris`): the ink of action and position. Primary buttons, focus outlines, caret and accent colour, the checked state of checks and choices, the current running-head link, link hover underlines, rank slot numbers and the inked length of the reading-position track. Bright and deep variants (`verdigris-bright`, `verdigris-deep`) are hover and press states of the primary button only. `on-verdigris` is the dark ink written on a verdigris ground. `verdigris-wash` is the text-selection ground.

### Secondary
- **Rubric** (`rubric`): the ink of what is owed, wrong or irreversible. Debt figures in the status strip, owed marks, field error rules and error messages, lookup failures, and the destructive button's text and outline.

### Tertiary
- **Ochre** (`ochre`): flags without blame. The stall point on the margin rule, the stalled mark, and marks for being over a soft limit.

### Neutral
- **Warm Black Stock** (`stock`): the page. Also the knockout behind rank slots and stall points so they sit on the margin rule cleanly.
- **Lifted Stock** (`stock-lift`): the one tonal step up. Hover wash on search results, pressed secondary button, select option lists, empty cover placeholders.
- **White Ink** (`ink`): titles, entered text, settled figures, anything confirmed.
- **Soft Ink** (`ink-soft`): the owner's secondary words (whys in the margin), the running-head name, check labels, quiet buttons, field hover rule.
- **Graphite Pencil** (`pencil`): labels, margin notes, metadata, hints, placeholders, estimates, unselected choices, disabled fields, and lookup-filled values not yet confirmed.
- **Rule** (`rule`): field underlines, the secondary button outline, link underlines at rest, the unread length of the reading-position track.
- **Faint Rule** (`rule-faint`): the margin rule, the running-head hairline, textarea ruling, cover outlines.

### Named Rules

**The One Job Rule.** Each ink has one meaning. Verdigris is only actions, focus, current selection, rank slots and reading position. Rubric is only what is owed, wrong or irreversible: debt, errors and destructive verbs. Ochre is only flags without blame. Pencil is labels, metadata, estimates and unconfirmed values. If a colour would be used for anything else, use white ink or pencil instead.

**The Pencil Until Confirmed Rule.** Anything the app found, guessed or estimated is written in pencil. A lookup-filled field stays pencil until the owner touches it; on focus it inks to white.

**The Plain Number Rule.** Status figures are plain tabular numbers in white ink, never tiles or rings. Debt is the same figure in rubric, with no message attached.

## Typography

**Interface Font:** Alegreya Sans 400 and 500 (with ui-sans-serif, system-ui, sans-serif)
**Hand Font:** Alegreya Italic 400 (with Georgia, serif)

Both are self-hosted WOFF2 subsets under the OFL, with small caps, lining and tabular figures, and kerning retained in the subset.

**Character:** A humanist sans for everything the app writes, paired with its sibling serif italic for everything the owner writes, so the page reads as a printed ledger with handwritten marginalia.

### Hierarchy
- **Heading** (500, 1.625rem, 1.2, -0.005em; 1.3125rem on narrow screens): shelf and page headings. Balanced wrapping.
- **Title, large** (500, 1.3125rem): the first line of an entry page, the title or link field.
- **Title** (500, 1.0625rem, 1.3): entry titles in lists and results; also button text at line-height 1.
- **Body** (400, 1.0625rem, 1.5, lining figures): running text and field values. Entry lines hold a 36rem measure.
- **Small** (400, 0.9375rem, 1.35): margin notes, entry metadata, hints, errors, result metadata.
- **Label** (500, 1.0625rem, all small caps, 0.06em): form labels in the margin column, running-head nav (0.06em) and name (0.08em).
- **Mark** (400, 1.0625rem, all small caps, 0.05em): status words such as format, focus, desk, stalled, review due. Never wraps.
- **Figure** (tabular and lining numerals): any number that may be compared: times, pages, counts, debt.
- **Hand** (Alegreya Italic 400, 1.0625rem in the margin, 1.125rem in fields): the owner's own words only: why, verdict, notes.

### Named Rules

**The Owner's Hand Rule.** Alegreya Italic is used only for words the owner wrote: why, verdict, notes. App-written text (labels, hints, placeholders, confirmations, errors, estimates) is never set in the hand, even inside a hand field.

**The Small Caps Label Rule.** Labels are nouns in all small caps, in pencil, sitting in the margin beside their line. They name a row; they never sit above a heading as a kicker or eyebrow.

## Layout

The page is a single centred column, max width margin + gutter + gutter + measure plus page padding (about 52.5rem). Inside it a two-column grid repeats on every row: the margin column (11rem, right-aligned) and the entry line (flexible, reading measure 36rem), separated by twice the gutter (1.75rem each). The margin rule is drawn once per page at margin + gutter, a 1px faint vertical line running the full page height.

The running head shares the page width: app name at left in small caps, nav at right, a faint hairline beneath. The page has 2rem top and 4.5rem bottom padding, and 1rem side padding.

Vertical rhythm uses an 8-step scale from 0.25rem to 4.5rem. Consecutive form rows sit 1.5rem apart; a new group of rows opens with 3rem. List entries have 0.75rem block padding and 0.75rem between them. Inline clusters (actions, metadata, marks, status strips) wrap with 0.75rem to 1.5rem gaps.

Every interactive element has a minimum 2.75rem target, including choices, checks and nav links.

**Narrow screens (below 48rem):** the margin folds onto the page. The margin rule moves to the left page edge, rows and entries become one column indented by a 1rem gutter, labels and notes sit above their line left-aligned, the why drops beneath its entry body, and the heading steps down to 1.3125rem. Rank slots, progress and stall marks stay on the (now left) margin rule.

### Named Rules

**The One Margin Rule Rule.** A page has exactly one vertical rule, and everything that marks position (rank slot, reading position, stall) sits on it. Do not add second rules, side borders or column dividers.

## Elevation & Depth

The system is flat. There is no elevation, no drop shadow, no blur and no layering of surfaces. Depth is replaced by ink value and rules: the page is one stock, a single lifted stock tone marks hover and pressed states, and hairline rules separate what boxes would separate elsewhere.

The CSS uses `box-shadow` in two non-elevation ways only: a 1px under-rule that thickens a focused or invalid field's underline to 2px, and a 3px stock-coloured ring that knocks the margin rule out around a stall point. Gradients appear only as functional drawing: the textarea's repeating ruled lines and the two-tone reading-position track.

### Named Rules

**The Flat Page Rule.** No cards, no drop shadows, no decorative gradients. If something needs separating, rule it; if something needs emphasis, ink it.

## Shapes

Forms are lines, not boxes. Fields are a bottom rule only with square ends. Buttons, checks, focus outlines and the result hover wash have a barely softened 2px corner. The only round forms are small points: the ochre stall point and the hollow "due" ring, both 50%. Covers are 2:3 portrait thumbnails with a faint inset outline and no radius. Selection is an underline under a word (2px verdigris), not a filled pill.

## Components

### Buttons
Few, plain and verb-labelled; one primary per page.
- **Shape:** 2px corners, 2.75rem tall, 1.5rem side padding, medium weight at line-height 1.
- **Primary:** verdigris ground with dark on-verdigris text. The single committing action of a page (for example File it).
- **Hover / Active:** primary brightens on hover, deepens on press. Transitions are 180ms on background, border and colour. Focus is the global 2px verdigris outline at 3px offset.
- **Secondary:** transparent with a rule-coloured 1px outline; the outline lightens to soft ink on hover and the ground lifts on press.
- **Quiet:** a text button in soft ink with a rule-coloured underline that turns verdigris on hover; 0.25rem side padding. For alternative paths (Paste the text instead).
- **Destructive:** rubric text with a 45% rubric outline; hover adds a 12% rubric wash and a full rubric outline. For irreversible verbs such as Abandon.
- **Disabled:** 45% opacity, not-allowed cursor.
- **Busy:** a 1px pencil stroke draws left to right under the label on a 1.2s loop; with reduced motion it is a static line at 60% opacity. The same stroke marks any inline pending text (Looking up the link) in pencil.

### Choices
Pick a word by underlining it.
- **Style:** a wrapping row of lowercase words in pencil, 1.5rem apart, each a full 2.75rem target with the radio input stretched invisibly over it.
- **State:** hover inks to soft ink; checked inks to white with a 2px verdigris underline; focus draws the verdigris outline around the word; disabled drops to rule colour.

### Checks
- **Style:** a 1.0625rem square with a 1px soft-ink border and 2px corners, label in soft ink.
- **State:** checked fills verdigris and scales in a dark tick over 180ms; disabled uses a rule border and pencil label.

### Marks and the status strip
Small-caps words instead of badges or chips.
- **Marks:** pencil by default, ochre for flags without blame (stalled, over a soft limit), rubric for owed. No background, no border.
- **Status strip:** a wrapping pencil line of plain phrases with white medium tabular figures (Today 1:45 of 2:00); the owed figure is rubric. A due item is a mark led by a small hollow ring.
- **Provisional estimate:** pencil text with a dotted 1px underline, kept on one unbreakable line (spec §7.3), with its reason available on hover.

### Inputs / Fields
Written on the rule.
- **Style:** transparent, no box, a 1px rule-coloured bottom border, square ends, 0.5rem block padding, full width, 2.75rem minimum height. Placeholders are pencil.
- **Hover:** the rule lightens to soft ink.
- **Focus:** the rule turns verdigris and thickens to 2px; no outline box.
- **Error:** the rule turns rubric and thickens to 2px; a small rubric message sits 0.25rem beneath and collapses when empty. Hints use the same slot in pencil.
- **Unconfirmed (lookup-filled):** value in pencil until focused.
- **Disabled:** pencil value on a dotted rule.
- **Variants:** large title field (1.3125rem medium); hand field for the why (Alegreya Italic 1.125rem); short numeric field (7rem, tabular figures); select with a small pencil chevron and lifted-stock option list.
- **Pasted text:** a textarea drawn as a ruled notebook page, faint lines every 1.75rem that scroll with the text, six lines tall, vertically resizable, no bottom rule and no focus under-rule.

### Navigation
- **Running head:** app name at left in soft ink, medium small caps at 0.08em, not underlined. Nav links at right in pencil small caps at 0.06em, 1.5rem apart, 2.75rem targets. Hover inks to white; the current page is white with a verdigris underline. A faint hairline closes the head.
- **Deferred:** the nav appears only once more than one route exists, and links only routes that exist.

### Entries (signature component)
Items as rows of a commonplace book.
- **Structure:** the same margin and line grid as form rows. The title (medium white) and a pencil metadata line of author, marks and figures sit on the entry line; the why sits in the margin, right-aligned, in the owner's italic hand in soft ink.
- **Rank slot:** the slot number in medium verdigris tabular figures, centred on the margin rule with a stock knockout.
- **Reading position:** a 3px wide, 2.5rem tall track laid on the margin rule; the read fraction is inked verdigris from the top, the rest in rule colour. Every track is the same length so entries compare at a glance.
- **Stall:** an 0.5rem ochre point on the margin rule with a 3px stock knockout ring.
- **Doesn't fit the moment:** title, why and slot drop to pencil; the entry stays on the page.

### Search results
- **Style:** a button row of a 2.25rem 2:3 cover beside a title and a small pencil metadata line, bleeding 0.5rem beyond the line so the hover wash aligns text with the fields above.
- **State:** hover lifts the ground to lifted stock with 2px corners. A missing cover is an empty lifted-stock placeholder of the same size.

### Filed confirmation
A plain status line and its margin label ink in over 180ms, fading from pencil to their final colour with no movement.

## Do's and Don'ts

### Do:
- **Do** put every row on the margin grid: label or note in the 11rem margin, content on the entry line.
- **Do** mark rank, reading position and stall on the single margin rule, never elsewhere.
- **Do** write fields on a 1px rule and signal focus by turning the rule verdigris at 2px.
- **Do** keep verdigris for actions, focus, current selection, rank slots and reading position only.
- **Do** keep rubric for what is owed or wrong: debt, field errors and failures, and destructive verbs.
- **Do** use ochre for flags without blame: stalled, over a soft limit.
- **Do** write labels, metadata, estimates and lookup-filled values in pencil.
- **Do** set the owner's why, verdict and notes in Alegreya Italic, and nothing else.
- **Do** show provisional estimates as one unbreakable pencil unit with a dotted underline.
- **Do** use tabular figures for any number that may be compared, and show status numbers plain.
- **Do** keep every target at least 2.75rem and fold the margin onto the page below 48rem.
- **Do** limit motion to 180ms state changes on opacity and colour (and the pending stroke), and honour reduced motion.
- **Do** write terse English copy: nouns for labels, verbs for buttons, no exclamation marks.

### Don't:
- **Don't** use cards, boxed panels, drop shadows or decorative gradients; the textarea ruling and the reading-position track are the only gradients.
- **Don't** add kickers or eyebrow labels above headings or heading rows.
- **Don't** use chips, pills or filled badges; use small-caps marks.
- **Don't** set app-written text in the italic hand.
- **Don't** add a light theme or a second vertical rule.
- **Don't** show debt as a message, a ring, a tile or a celebration; it is a rubric number.
- **Don't** move elements more than 4px in any transition.
- **Don't** use corners larger than 2px except for the small round points.
