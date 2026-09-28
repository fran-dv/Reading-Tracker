# Book Object Plates Specification

## Purpose

Cover plates today are flat 2:3 boxes, consistent with DESIGN.md's Flat Page Rule. This capability adds one narrow, named exception: a subtle 3D "book object" treatment (a page-edge hint, a static perspective tilt, and an adjusted shadow) on exactly two focal, single-cover plate sites — the book page's plate and capture's cover plate — only at their 6rem and 9rem sizes, while every dense list plate and every wide (16:9) cover stays exactly as flat as before. It defines which plates receive the treatment, which do not, and the reduced-motion behavior (SPEC §12 step 32; capture UX item P11 for naming/labels carried over unchanged, the book object itself is CSS-only).

This is a presentation-only capability: it changes no data model and has no domain logic requiring `go test` coverage; its requirements are verified visually and via CSS/markup inspection.

## Requirements

### Requirement: Focal Plate Book-Object Treatment

The book-object treatment (a thin fore-edge page hint, a static 2–4° `perspective`/`rotateY` tilt, and a shadow adjusted from the shared Float shadow) MUST apply, via one opt-in CSS class, only to the book page's own plate and to capture's cover plate, and only when either renders at the 6rem or 9rem plate size. It MUST NOT apply automatically to any other plate, to either of those two plates at any other size, or to a wide (16:9, video/article) cover regardless of size — a plate receives the treatment only by explicitly carrying the opt-in class at one of its two qualifying sizes.

#### Scenario: Book page plate gets the treatment

- GIVEN the book page for a book-format item with a real or generated cover
- WHEN its plate renders at 6rem or 9rem
- THEN it carries the book-object class and shows the page-edge hint, static tilt, and adjusted shadow

#### Scenario: Capture's cover plate gets the treatment at 6rem or 9rem

- GIVEN capture is open with a found, uploaded, or generated cover showing
- WHEN capture's cover plate renders at 6rem or 9rem
- THEN it carries the book-object class

#### Scenario: A wide cover never gets the treatment

- GIVEN a video or article item whose plate is a wide (16:9) cover
- WHEN it renders at any size, including 6rem or 9rem
- THEN it never carries the book-object class, because the treatment is reserved for 2:3 book covers only

#### Scenario: A plate not opted in never receives the treatment automatically

- GIVEN a newly added 2:3 book cover plate elsewhere in the app that has not been given the opt-in class
- WHEN it renders
- THEN it shows no page-edge hint, tilt, or adjusted shadow — the treatment is never inferred from size or content alone

### Requirement: List Plates Stay Flat

Dense list plates MUST NOT receive the book-object class and MUST remain visually flat exactly as before this change: the 3rem entries-list plate, the 2.25rem dense-choice thumbnail (search results, the item picker, the shortlist), and the 4.75rem margin cover plate.

#### Scenario: 3rem entries-list plate stays flat

- GIVEN an entry row on a shelf, Home, or another list view
- WHEN its 3rem cover plate renders
- THEN it shows no page-edge hint, tilt, or adjusted shadow

#### Scenario: 2.25rem search-result thumbnail stays flat

- GIVEN a book search result or item-picker row
- WHEN its 2.25rem cover thumbnail renders
- THEN it shows no book-object treatment

#### Scenario: 4.75rem margin cover plate stays flat

- GIVEN capture viewed at the width where the margin cover plate renders at 4.75rem, with a cover showing
- WHEN that plate renders
- THEN it shows no book-object treatment, consistent with DESIGN's existing rule that this size takes no shadow at all

### Requirement: Static Tilt, No Hover Motion

The book-object tilt MUST be static: applied once at render and never animated on hover, focus, or any other interaction. No flip, spin, or hover-triggered transform MUST be added to any plate by this capability.

#### Scenario: Hovering a focal plate changes nothing about its tilt

- GIVEN a focal plate with the book-object treatment
- WHEN the pointer hovers over it
- THEN the tilt, shadow, and page-edge hint remain exactly as they render at rest — no additional transform or animation triggers

#### Scenario: No flip or spin exists anywhere

- GIVEN any book-object plate
- WHEN inspected for interactive motion
- THEN no hover-flip, spin, or other animated transform is present in its styling

### Requirement: Reduced Motion Shows the Same Static Object

Because the tilt is already static, a user with `prefers-reduced-motion` enabled MUST see the identical book-object presentation (same tilt, same shadow, same page-edge hint) as a user without that preference — reduced motion changes nothing about this treatment, since there is no motion to reduce.

#### Scenario: Reduced motion preference set

- GIVEN a browser with `prefers-reduced-motion: reduce` set
- WHEN a focal book-object plate renders
- THEN it shows the same static tilt, shadow, and page-edge hint as it would without that preference

### Requirement: Scoped Opt-In Class, Never on Lists

The book-object CSS class MUST be applied only at the specific template sites named in "Focal Plate Book-Object Treatment," and the templates for list-style plate sites MUST NOT include that class under any state or configuration.

#### Scenario: List template never emits the class

- GIVEN the shelf, Home, review, archive, and search-result list templates
- WHEN their cover plates render in any state
- THEN none of them ever emit the book-object opt-in class

#### Scenario: Focal templates consistently emit the class

- GIVEN the book page and capture's cover-plate templates rendered at 6rem or 9rem
- WHEN they render with a cover (real or generated) showing
- THEN they consistently emit the book-object opt-in class
