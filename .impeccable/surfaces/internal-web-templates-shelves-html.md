---
version: 1
slug: "internal-web-templates-shelves-html"
primary_target: "internal/web/templates/shelves.html"
related_targets: ["internal/web/templates/shelf.html","internal/web/static/app.css"]
---

# The shelves (the index, and one shelf)

Mode: Operate. Two surfaces that share the entry component: `/shelves`, the index, and `/shelves/{id}`, one shelf. The index is a door; the shelf is where the next thing is chosen.

- **Audience and job:** the single owner. On the index: open the right shelf, or put the shelves in the order they think in, rename one, delete an empty one. On a shelf: read the whys, see what is ranked, rank or start something, edit an entry.
- **Content ranges:** 1–30 shelves, names to ~30 characters; a shelf holds 0–40 visible items (pool and in progress only), titles 2–120 characters, whys 20–140, covers present for most books and absent for papers and hand-typed items.
- **States that matter:** no shelves at all; a shelf with nothing on it; three slots taken; an empty slot; a borrowed item; an item being read, with its position and its stall; a row open as a form; a rename open, with its duplicate and blank errors; the delete dialog on an empty shelf.
- **Anti-goals:** counts or contents on the index (spec §0), a per-shelf colour (bookcloth names formats only), cards or cover grids, hover-only controls, a browse-everything surface.

## Direction contract

THESIS: A page of the commonplace book. The index is the list of headings in the owner's own order, numbered down the margin rule in pencil; a shelf is entries filed under one heading, each with its why in the margin and its marks on the rule.

OWN-WORLD: The shell's inks, unchanged: verdigris for actions, focus, rank slots and where the reading is; pencil for labels, places and metadata; ochre for flags without blame; bookcloth only to name a format. Structure is the single margin rule and hairlines, never boxes.

STORY: The owner opens Shelves, reads seven names numbered down the rule, and taps one. On the shelf, the three slots lead, the pool follows past a fading hairline, and the item being read carries its position and stall on the rule. Moving a shelf answers in words where it landed, and the arrow that moved it keeps the focus.

FORM: Motion is the shell's 180ms ink-in and nothing else; the moved row inks in, nothing travels. Every control is always visible and at least 2.75rem, because the page is used one-handed at night.

## Decisions on record

- The index carries the order as a pencil figure on the margin rule (the unclaimed-slot figure, not the verdigris of a rank): a place is where a thing sits, not a claim about it.
- The index says nothing about a shelf's contents, not even whether something on it is being read: Home already answers that, and §0 holds.
- A shelf's rows carry the reading position and the stall point, as Home and the review do; the stall point steps below the track so it never hides the inked length.
- A blank cover plate is mixed strongly enough in its bookcloth to hold 3:1 against the page, so a book, a paper and a course are told apart at a glance.
- Rows carry their shelf's id so a redraw moves the row instead of rewriting it in place, and the focus travels with the shelf.
