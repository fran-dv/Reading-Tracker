# Reading Queue — Build Specification

A single-user application for deciding what to read next, and for holding a reading discipline over a year.

**Version 3 — implementation-ready.** All open decisions resolved. Where a value is called _configurable_, it lives in the settings table (§2.7) with the default stated.

---

## 0. Read this first

This app is **not** a library manager. Every tool the user has tried (Notion, Excel, reading trackers) failed the same way: they present the whole collection, and a well-organised list of 200 items still produces paralysis.

The core job is to **hide things**. Most of the library is invisible most of the time. The user opens the app, sees a handful of candidates, picks one, and starts reading within seconds.

Do not add a "browse everything" view to Home. Do not add an "all items" default listing. Do not surface full-library counts where they'll be seen daily. Full-library access exists, behind the weekly review and explicit navigation.

Second principle: **the right item depends on the moment.** Twenty minutes on a phone in bed and three focused hours at a desk are different questions. Items carry a _shape_ (format, size, focus demand); the app filters candidates against the shape of the moment.

Third principle, the one most likely to be eroded by well-meaning changes: **the app never lets the user be quietly fooled.** Any number that can drift in a flattering direction is displayed alongside the thing that would explain the drift. §9 is not decoration.

### What the user is trying to achieve

1. Consolidate and hold a reading discipline
2. Get insightful metrics on reading habits
3. Progress toward and achieve reading goals — first: **100 books in one year**
4. Maximise effective time management
5. Read a great deal
6. **Enjoy the process**

Item 6 is a requirement. The discipline machinery in §8 is punitive by design and the user chose it. §6.6 is the counterweight and must not be cut.

### Testing expectations

All domain logic in §7–§9 (pace, debt, ramp, campaign projection, shortlist eligibility, stall detection, shelf-with-borrowed-items) must have unit tests in Go that pass without a browser or a running server. `go test ./...` must be green before any screen is considered done.

---

## 1. Stack

| Layer         | Choice                                                                |
| ------------- | --------------------------------------------------------------------- |
| Backend       | Go 1.24+                                                              |
| Storage       | SQLite via `modernc.org/sqlite` (pure Go, no cgo)                     |
| Frontend      | Datastar 1.0, official Go SDK `github.com/starfederation/datastar-go` |
| Transport     | Server-rendered HTML fragments over SSE                               |
| Deployment    | Single static binary, run locally                                     |
| Book metadata | Open Library API (no key required)                                    |

### Non-negotiable architecture constraints

1. **Client/server split from day one.** The browser talks to a local HTTP API. Going cross-device later means moving the binary, not rewriting.
2. **All data in a SQLite file, server-side.** Never `localStorage`, never `IndexedDB`.
3. **Responsive and PWA-ready.** Manifest and a service worker that caches the app shell only. No offline data sync. Android share-target capture is v1.1; keep the groundwork.
4. **Clean HTTP boundary.** The Datastar layer must be replaceable without touching domain logic. Domain logic lives in Go packages with no HTTP or template imports.
5. Embed templates and static assets via `embed.FS`.
6. **Active reading sessions live server-side** (§7.4). A closed tab must not lose a running timer.
7. **Backup and export are required** (§10) and are built early (§12).

### Datastar warning

Datastar's attribute syntax **changed at 1.0**. Pre-1.0 material uses `data-store` and `data-model`; these are dead. Use `data-signals`, `data-bind`, and the current 1.0 set. **Fetch and consult current Datastar 1.0 documentation before writing any frontend code.** Do not rely on recalled syntax.

MIT core only. No Pro features.

---

## 2. Data model

All timestamps stored UTC. All day and week boundaries computed in `settings.timezone`.

### 2.1 Item

| Field                                                   | Type       | Notes                                                                      |
| ------------------------------------------------------- | ---------- | -------------------------------------------------------------------------- |
| `id`                                                    | uuid       |                                                                            |
| `title`                                                 | string     | required                                                                   |
| `url`                                                   | string?    |                                                                            |
| `author`                                                | string?    |                                                                            |
| `format`                                                | enum       | `book`, `video`, `article`, `paper`, `course`                              |
| `shelf_id`                                              | fk         | **required**, exactly one                                                  |
| `why`                                                   | string     | **required**, one line, captured at add time                               |
| `verdict`                                               | string?    | optional one line, prompted on `finished` or `reference`                   |
| `abandoned_reason`                                      | string?    | **required** when state becomes `abandoned`                                |
| `focus_demand`                                          | enum       | `light`, `medium`, `deep`                                                  |
| `size_value`                                            | int?       | in `size_unit`                                                             |
| `size_unit`                                             | enum       | `pages`, `minutes`, `words`                                                |
| `word_count`                                            | int?       | auto-fetched for URLs; manual paste fallback; **never** for physical books |
| `needs_desk`                                            | bool       | unreadable on a phone                                                      |
| `state`                                                 | enum       | `pool`, `in_progress`, `finished`, `reference`, `abandoned`                |
| `rank_slot`                                             | int?       | 1–3 if a shelf leader; null otherwise                                      |
| `on_shortlist`                                          | bool       | orthogonal to state                                                        |
| `created_at`, `updated_at`, `started_at`, `finished_at` | timestamps |                                                                            |

**Tags** are a separate `item_tags(item_id, tag)` table.

**State transitions:** `pool → in_progress → {finished | reference | abandoned}`. Terminal states do not transition further. There is no `shortlist` state; shortlist membership is the `on_shortlist` flag and can be true in `pool` or `in_progress`.

**Rank slots and state:** only `pool` items hold rank slots. Moving an item to `in_progress` clears its slot and shifts the shelf's remaining slots up (§5.1).

**`reference`** is a terminal outcome for material consulted rather than completed — CHIPs, RFCs, specs, papers returned to repeatedly. Counts as a completion for hours and stats, **never toward the book campaign.**

**Counting toward the campaign** is computed, not stored: `format = 'book' AND state = 'finished'`.

**Deletion:** an item may be hard-deleted only if it has zero sessions. Any item with session history must be abandoned instead. History is never destroyed.

The three one-line fields — `why`, `verdict`, `abandoned_reason` — are a deliberate set: an opening reason and a closing one. They are the pruning mechanism (§6.3) and the satisfaction surface (§6.6).

### 2.2 Shelf

`id`, `name` (unique), `sort_order`, `created_at`. **Flat, no nesting.**

### 2.3 Session

| Field                            | Type       | Notes                                              |
| -------------------------------- | ---------- | -------------------------------------------------- |
| `id`, `item_id`                  |            |                                                    |
| `started_at`, `ended_at`         | timestamps | `ended_at` null while running                      |
| `position_start`, `position_end` | int?       | in the item's `size_unit`; both null if unrecorded |
| `note`                           | string?    | never blocks saving                                |
| `entered_retroactively`          | bool       |                                                    |

`duration_minutes` and `progress_delta` are derived, not stored.

**At most one session may be running** (`ended_at IS NULL`) at any time. Starting a new one while another runs prompts to stop the first.

**Rule:** a session with time but no positions counts fully toward hours and debt, and is excluded from pace calculations.

### 2.4 Campaign

| Field          | Notes                                 |
| -------------- | ------------------------------------- |
| `id`, `name`   | e.g. "100 books by 22"                |
| `target_count` | e.g. 100                              |
| `deadline`     | date                                  |
| `started_at`   | date                                  |
| `active`       | bool; **at most one active campaign** |

**Open target.** Any book counts; the set is not fixed. Chosen deliberately over a named set. Consequence: the app cannot prevent substitution of thin books for thick ones, so it instruments (§9.2).

When the deadline passes the campaign shows its final count and stops projecting. It can be archived and a new one started.

### 2.5 Schedule and debt

| Field                     | Notes                                            |
| ------------------------- | ------------------------------------------------ |
| `active_days`             | set of weekdays, e.g. `{Mon,Tue,Wed,Thu,Fri}`    |
| `committed_hours_per_day` | current effective daily target (§8.1)            |
| `override_active`         | bool — user has manually set the committed value |
| `debt_minutes`            | integer ≥ 0                                      |

### 2.6 Ramp

| Field                   | Notes                             |
| ----------------------- | --------------------------------- |
| `active`                | bool; **at most one active ramp** |
| `start_hours_per_day`   | e.g. 1.0                          |
| `increment_hours`       | e.g. 0.5                          |
| `ceiling_hours_per_day` | e.g. 4.0                          |
| `current_hours_per_day` |                                   |
| `started_at`            |                                   |

Ramps target daily hours only in v1. Speed ramps are deferred (§11).

### 2.7 Settings

Single-row table. Defaults:

| Key                                           | Default                      |
| --------------------------------------------- | ---------------------------- |
| `timezone`                                    | system local                 |
| `wip_cap`                                     | 5                            |
| `stall_days`                                  | 14                           |
| `review_weekday`                              | Sunday                       |
| `bucket_quick_max_min`                        | 25                           |
| `bucket_hour_min_min` / `bucket_hour_max_min` | 45 / 75                      |
| `bucket_long_min_min`                         | 90                           |
| `pace_window_days`                            | 90                           |
| `projection_window_weeks`                     | 4                            |
| `seed_pace_pages_per_hour`                    | light 40, medium 30, deep 15 |
| `seed_pace_wpm`                               | 230                          |
| `fallback_book_pages`                         | 300                          |

---

## 3. Grouping model

Every item has **exactly one home shelf** and **any number of tags**.

> A shelf view shows its own items **plus every item from other shelves carrying a tag equal to this shelf's name**, visually marked _borrowed_.

Worked example: a statistics textbook lives on **Statistics** and carries tag `IQ`. The **IQ** shelf shows it, marked borrowed. No duplication, nothing homeless.

Borrowed items may hold rank slots on the borrowing shelf. An item can therefore be slot 2 on Statistics and slot 1 on IQ; that is correct behaviour.

Rejected — do not "improve" back into these: pure tags (nothing has a home), strict folders (breaks the case above), nested groups (forces a false hierarchy).

---

## 4. Capture

The highest-frequency action. It must stay fast.

**Input methods (v1):**

1. **Paste a URL.** Fetch title, author, and where possible length. For web articles: extract text, store `word_count`, set `size_unit = words`. For YouTube: title and channel via oEmbed; duration is best-effort and falls back to manual.
2. **Search Open Library by title.** Populate title, author, page count.
3. **Manual entry.**

**Rules:**

- Items are **filed immediately**. There is no inbox.
- The **shelf field arrives pre-filled**: last-used shelf. Accepting is one tap.
- The **`why` field is mandatory** and blocks saving. One line. Deliberate friction.
- `focus_demand`, `size_*`, and `needs_desk` pre-fill from format (video → light, no desk; book → medium, no desk; paper → deep, desk), correctable in one tap.
- A manual **paste-text field** exists for paywalled articles; word count is computed from it.

---

## 5. Ranking and the shortlist

### 5.1 Per-shelf ranking

Three ordered slots per shelf. Slot 1 is "next up." Everything else is an unordered pool. Reordering uses up/down buttons, not drag-and-drop. Never build total ordering.

When a slotted item leaves the pool (starts, is abandoned, or is deleted), lower slots shift up and slot 3 becomes empty. The app **prompts** to fill it from the pool; it does not auto-select.

### 5.2 The weekly shortlist

- **5–7 items**, set by the user in the weekly review. Soft limit: the UI warns beyond 7 but permits it.
- Sourcing is **loose**: any `pool` or `in_progress` item is eligible.
- The review UI defaults to showing shelf leaders. Reaching into a pool is a deliberate extra action.
- **Carry-over:** the review opens with last week's unfinished shortlist pre-selected. The user confirms or edits.
- Short items compete on equal footing with books and occupy WIP slots like anything else. Deliberate; §9.2 is the counterweight.

---

## 6. Screens

### 6.1 Home

In order, top to bottom:

1. **Status strip** (slim, one line): today's committed hours vs. hours logged so far, and `debt_minutes` if greater than zero. Debt is shown as a plain number in a distinct colour. No copy, no exclamation marks. Nothing about the campaign here.
2. **In progress** — every `in_progress` item, with resume position, last-touched date, and estimated time remaining. Stalled items flagged. The moment filter does not hide these; items that don't fit the current moment are visually de-emphasised, not removed.
3. **Picks** — `on_shortlist` items in `pool`, filtered by the moment.

**Action affordances** present on Home: capture, start session, retroactive session entry, the moment filter, and a single small, neutral indicator when the weekly review is overdue. No other informational content.

**Moment filter:**

- **Time available** — one tap: _quick_ / _an hour_ / _long_. Filters picks by estimated time remaining (§7.2) against the bucket bounds in settings. _Long_ hides nothing.
- **Device** — not inferred. The device that logs is not the device that reads (a physical book at a desk, logged from a phone). `needs_desk` is shown as a mark on every entry and never filters.
- **"I'm fried"** — optional toggle, hides `focus_demand = deep`. Never ask the user to self-rate energy on a scale.

The filter resets on each page load.

### 6.2 Shelf view

Three slots on top, pool below, borrowed items distinguished. Each item shows its `why`.

### 6.3 Weekly review

Configurable weekday. The pruning ritual. Steps, in order:

1. **Reread the whys** of all shelf leaders, in a batch.
2. **Prune** — abandon or delete (§2.1 deletion rule).
3. **Adjust** each shelf's top three.
4. **Set the shortlist** with carry-over (§5.2).
5. **Goal status** — campaign projection, debt, ramp state, and any change to the derived target since last week with its cause (§8.1).
6. **Composition report** (§9.2).

If the review is skipped, the shortlist persists and Home shows the overdue indicator. Nothing else changes.

### 6.4 Session view

Start/stop timer. On stop: one number for end position, optional note. Both dismissible.

**Retroactive entry** is on the same screen with equal prominence: item, start time, duration or end time, optional positions, optional note.

### 6.5 Stats

Hours per day and week vs. committed target; debt over time; completions over time; pace per item, per band, and global, each obeying §9.1; campaign projection. Charts rendered as SVG from Go.

### 6.6 The finished archive

**Exists to satisfy goal #6. Must not be cut.**

Every `finished` and `reference` item, newest first, showing `why` and `verdict` side by side. Visual and pleasant. Shows accumulation — a count and a sense of the shelf being built.

The rest of this app is debt counters, frozen ramps, hard caps, and stall flags. Four punishment mechanisms and no reward surface is a design that gets abandoned in month five. This is the only screen showing what the user has _gained_ rather than what they _owe_. Load-bearing.

---

## 7. Measurement model

### 7.1 Recorded

Two things. Everything else is derived.

1. **Session minutes** from `started_at`/`ended_at`.
2. **Positions** in the item's native unit, when the user enters them.

**Minutes is the fundamental unit.** Every user-facing quantity resolves to it.

### 7.2 Derived quantities

**Item pace** = Σ progress_delta ÷ Σ hours, over that item's sessions that have positions.

**Band pace** = median item pace across items sharing `(format, focus_demand)`, over sessions within `pace_window_days`.

**Global pace** = same, across all items; displayed only with §9.1.

**Estimated time remaining** for an item, three-tier:

1. Item has ≥ 1 session with positions → `(size_value − last_position) ÷ item_pace`.
2. Otherwise, band has data → `(size_value − last_position) ÷ band_pace`.
3. Otherwise → seed pace from settings, and the estimate is **labelled provisional** in every UI that shows it.

Video: remaining = `size_value − last_position`, no pace involved.

**Stalled** = `in_progress` and no session in the last `stall_days`.

### 7.3 Cold start

The first weeks have no pace data. Use seeds and **label estimates provisional**. Do not display confident wrong numbers; a user who learns the estimates lie in week two will ignore them in month six.

### 7.4 Sessions and physical books

**Retroactive entry is a primary path, not a fallback.** The user reads physical books away from the laptop. The timer will frequently not be running. Retroactive entry must be as fast as the timer, reachable from the same places, and never presented as exceptional.

A dataset with silent gaps is worse than no dataset, because debt and projection depend on completeness.

### 7.5 WIP limit

Hard cap on `in_progress` count = `settings.wip_cap`. At the cap, starting something new forces an explicit choice: finish or abandon something first. Never allow silent exceeding.

> _Note to the user, not a build instruction: a cap only does work if it sits below your natural drift. Consider starting at 3._

---

## 8. Goals, debt, and the ramp

### 8.1 Required vs. committed

There are two weekly numbers. They are different things and the UI must never conflate them.

**Required** — what the campaign needs:

```
avg_pages   = mean size_value of pool+shortlist books with size_unit=pages
              (fallback: mean of finished books; fallback: settings.fallback_book_pages)
pace        = band pace for (book, medium), falling through band → seed
hours_left  = (target_count − books_finished) × avg_pages ÷ pace
required_weekly_hours = hours_left ÷ weeks_until_deadline
```

**Committed** — what the user is actually held to this week:

- If a ramp is active: `ramp.current_hours_per_day × |active_days|`.
- Else if `override_active`: `committed_hours_per_day × |active_days|`.
- Else: `required_weekly_hours`, distributed over `active_days`.

**Debt accrues against committed, never against required.** Week one of a ramp does not start the user 13 hours in debt.

The gap between committed and required is shown as a **projection consequence**, in the same breath as any edit: _"Committed 7h/week. Campaign requires 20h. At this rate: 46 of 100."_ Never silently.

`required_weekly_hours` recomputes as pace data arrives. When it changes by more than 10% week over week, the weekly review states the change and its cause (pace changed / average book size changed / weeks remaining changed).

### 8.2 Debt

Computed daily, at the day boundary in `settings.timezone`, for each day in `active_days`:

```
shortfall = committed_hours_per_day × 60 − minutes_logged_that_day
if shortfall > 0:  debt_minutes += shortfall
if shortfall < 0:  debt_minutes = max(0, debt_minutes + shortfall)
```

Non-active days accrue nothing; minutes logged on them still pay debt down.

- Debt is **uncapped** and **carries forward indefinitely**.
- Debt has a **floor of zero. Surplus never banks.** A good week can only get the user back to even, never ahead.
- Debt **cannot be cancelled or forgiven** by any UI action. The only way out is reading.

### 8.3 The pain mechanism

**While `debt_minutes > 0`, the ramp does not advance.** The user's own progress is the collateral.

This is the only punishment mechanism. Do not add streaks, shaming copy, or notifications that scold.

### 8.4 The ramp

A **ramp block** is temporary. Fields in §2.6.

- On activation, `current_hours_per_day = start_hours_per_day`.
- **Advance rule:** at each week boundary, if `debt_minutes == 0`, `current += increment`, capped at `ceiling`. Otherwise hold.
- On reaching `ceiling` the block **ends**: `active = false`, and `committed_hours_per_day` is set to the ceiling value with `override_active = true`, so the user stays at the achieved level until they change it.
- Only one ramp may be active. A new block may be started at any time from a slump.

### 8.5 Rolling projection

Because the campaign is open-target, feasibility is projected continuously:

```
recent_weekly_hours = mean over last projection_window_weeks
projected_finish    = books_finished
                    + (weeks_until_deadline × recent_weekly_hours × pace ÷ avg_pages)
```

Shown on the weekly review and stats. Not on Home.

---

## 9. Honesty instrumentation

Two mechanisms, present because the user chose metrics that can drift flatteringly. **Not optional. Do not simplify away.**

### 9.1 Speed is never shown naked

Wherever a pace number is displayed, the **material mix** behind it is displayed alongside: for the window in question, the share of hours by `(format, focus_demand)`. Global pace rises when lighter material is chosen; the user must never read the number without its ingredients. Applies to stats, the review, and anywhere else pace appears.

### 9.2 Composition report

In the weekly review, covering the last four weeks:

- Completed items by format and by size bucket — e.g. _"11 short items, 0 books."_ Short items win shortlist competition by design; hours alone will not reveal it.
- **Mean page count of books finished, trended** across the campaign. Under an open target the cheapest route to 100 is thinner books. If this slides from 320 to 180, the user sees it happening rather than discovering it at the deadline.

The report blocks nothing.

---

## 10. Backup and export

- **Automatic backups:** copy the SQLite file daily to a backup directory; keep the last 14 daily and last 8 weekly.
- **Manual export** to JSON: items, tags, shelves, sessions, campaign, schedule, ramp, settings. Complete enough to reconstruct the library elsewhere.
- **Import** from that JSON into an empty database.

---

## 11. Out of scope for v1

- **Speed ramps** — v1.1. Speed _stats_ are in v1. Reason: the user has no baseline yet; ramp numbers set blind are meaningless.
- **Prerequisite links** between items — deferred.
- Staleness prompts on `reference` — declined.
- Android share-target capture — v1.1; keep PWA groundwork.
- Native mobile app; multi-user; auth; Datastar Pro.
- Any full-library browse surface on Home.
- Streaks, badges, gamification, social features.
- Highlights or notes beyond the per-session note.

---

## 12. Build order

1. Domain packages: items, shelves, tags, sessions. Schema and migrations. Unit tests for shelf-with-borrowed, rank-slot shifting, WIP cap, deletion rule.
2. HTTP layer with a clean boundary. Verify with curl.
3. **Backup and export.** Trivial, and unacceptable to omit.
4. Datastar setup. Confirm an SSE round-trip on 1.0 syntax before any real screen.
5. Capture: manual → Open Library → URL fetch.
6. Shelf view.
7. Session timer **and retroactive entry together** — equal priority.
8. Measurement: pace, time-remaining, stall. Unit-tested.
9. Home: status strip, in-progress, picks, moment filter.
10. Schedule, debt, ramp. Unit-tested against fixed clocks; the date arithmetic is where bugs hide.
11. Campaign, required-vs-committed, projection. Unit-tested.
12. Weekly review including composition report.
13. Finished archive.
14. Stats.
