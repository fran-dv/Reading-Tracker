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

### Changing this spec

This spec is the owner's current best thinking, not a fixed contract. It is expected to change as the app gets used. Its prohibitions exist to stop unrequested scope creep, not to overrule the owner.

- **Owner proposals are spec proposals.** When the owner suggests something the spec doesn't say, or says differently, "the spec doesn't say that" is not an answer. Judge the proposal on its merits: does it serve the goals above — clarity, usefulness, insight, enjoyment — better than what is written?
- **Give an honest opinion, then defer.** If a proposal cuts against a principle in this section, §9, or a reason given in §11, name the specific principle and the risk, once and plainly. Then the owner decides.
- **Agents are expected to propose.** Treat the spec as incomplete, not finished. Whenever a part of it looks insufficient, wrong, or improvable — a small tweak or a large rework — raise it unprompted, with the reasoning. The north star for every proposal is the goals above: help the user build and hold a reading discipline, reach their reading goals and set new ones, get real insight into their habits, use their time well, and enjoy it. Do not implement a proposal until the owner accepts it.
- **Accepted changes land in the spec.** Edit the relevant section in place, in its own `docs(spec):` commit, before or alongside the code. The spec stays the single source of truth; git history is the amendment log.

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

**Editing** an item changes its description and shape, never its shortlist flag (only shortlisting does). Once an item has sessions, a format whose `size_unit` differs is refused: its positions are measured in the old unit.

**Closing an item** stops a timer running on it: finishing or keeping for reference at the position reached (§6.1), abandoning with time only.

**Deletion:** an item may be hard-deleted only if it has zero sessions. Any item with session history must be abandoned instead. An item's history is never destroyed with it; single sessions can be corrected (§2.3).

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
| `edited_at`                      | timestamp? | set when the session is corrected; shown as _edited_ |

`duration_minutes` and `progress_delta` are derived, not stored.

**At most one session may be running** (`ended_at IS NULL`) at any time. Starting a new one while another runs prompts to stop the first.

**Rule:** a session with time but no positions counts fully toward hours and debt, and is excluded from pace calculations.

**Positions chain.** Only the position reached is typed. A session's `position_start` is the furthest position any earlier session of the item reached (0 before the first), recomputed for the whole item whenever a session is added, so a session logged late starts where reading stood at its own time and the ones after it follow. A position below the start is rereading: the session counts for hours and debt, adds no progress, and is left out of pace. The item's position is the furthest reached.

**Refused, with a plain reason:**

- a session that overlaps another (reading time is one at a time, across items);
- a session longer than 16 hours (the reason says so and asks for the real times);
- a position past the item's size;
- a session that ends in the future, or on an item not in progress.

Stopping the timer may set an earlier stop time, for a timer left running.

**Corrections.** A closed session can be edited (start, length, position reached, note) under the same rules, and is then marked edited. A session can be deleted, and a running one discarded. Debt, ramps and pace simply replay from what is left. Right after logging, the status line offers **Undo**, which deletes that session without asking.

### 2.4 Campaign

| Field          | Notes                                                                 |
| -------------- | --------------------------------------------------------------------- |
| `id`, `name`   | e.g. "100 books by 22"; optional, generated from target and deadline  |
| `target_count` | e.g. 100                                                              |
| `deadline`     | date, inclusive; after the start and after the day it is created      |
| `started_on`   | date; the day it is created or earlier                                |
| `ended_on`     | date; null while active. **At most one active campaign**              |

Dates are calendar days in `settings.timezone`.

**Open target.** Any book counts; the set is not fixed. Chosen deliberately over a named set. Consequence: the app cannot prevent substitution of thin books for thick ones, so it instruments (§9.2).

**Counting:** a book counts when `format = 'book'`, `state = 'finished'`, and `finished_at` falls on a day from `started_on` through `deadline`.

Only the name can be edited. To change the target, deadline or start, end the campaign and start a new one, so a moved goalpost leaves a trace.

A campaign can be ended at any time. When the deadline passes it shows its final count and stops projecting until it is ended. Ended campaigns are kept; a new one can then be started.

### 2.5 Schedule and debt

Only the user's **decisions** are stored. Debt and each ramp's current value are **derived by replay** (§8.2) from the decisions and the sessions, on every read. Nothing is cached, and no background job runs at the day boundary. A retroactive session entered late therefore corrects past debt and past ramp checks, which is what §7.4 asks of a dataset.

Decisions are dated by **calendar day in `settings.timezone`** and take effect **that same day**. Saving again on the same day replaces that day's decision. Nothing can be dated into the past, so no edit can rewrite a day already lived.

**`active_days`** — history of `(effective_on, days)`, where `days` is a non-empty set of weekdays, e.g. `{Mon,Tue,Wed,Thu,Fri}`. The latest row on or before a date governs it.

**`commitments`** — history of `(effective_on, kind, …)`. The latest row on or before a date governs it:

| Kind       | Fields                                                     | Daily target                                    |
| ---------- | ---------------------------------------------------------- | ----------------------------------------------- |
| `fixed`    | `minutes_per_day`                                          | `minutes_per_day`                               |
| `ramp`     | `start_minutes`, `increment_minutes`, `ceiling_minutes`    | the ramp's current value (§8.4)                 |

Latest decision wins: saving fixed minutes ends a running ramp, and starting a ramp replaces fixed minutes or an older ramp. Days before the first decision have no target and accrue no debt.

There is no commitment that follows the campaign on its own. Required hours (§8.1) move with pace and book sizes, and past values cannot be replayed, so a following target would rewrite days already lived. Instead the plan offers _Match the campaign_ (§6.7), which fills a fixed target from today's required hours; the user saves it as a decision.

All durations are whole minutes.

### 2.6 Speed ramps

| Field                | Notes                                                        |
| -------------------- | ------------------------------------------------------------ |
| `started_on`         | calendar day in `settings.timezone`                          |
| `increment_percent`  | e.g. 5                                                       |
| `ceiling_percent`    | e.g. 130; above 100                                          |
| `stopped_on`         | day the user stopped it; null while it runs                  |

At most one speed ramp runs. Starting a new one ends the old one. The target always starts at 100%. Baselines and the current target are derived by replay (§8.6), like debt.

### 2.7 Settings

Single-row table, edited on the Settings screen (linked from the plan's heading): time zone and the day weeks start on, then each whole-number setting with its unit and what it does, refused in its field with the reason; and _Download everything_ (§10). A time zone still stored as `Local` is offered by its name, so days stay the same wherever the app runs. `bucket_quick_max_min` stays below `bucket_hour_max_min`. Defaults:

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
| `words_per_page`                              | 300 (display conversion only) |

### 2.8 Weekly reviews

One row per week the review was closed (§6.3). Weeks start at 00:00 on `settings.review_weekday` (§8.4).

| Field                | Notes                                                                        |
| -------------------- | ---------------------------------------------------------------------------- |
| `week_of`            | calendar day the week starts on, in `settings.timezone`; unique              |
| `closed_at`          | timestamp; closing again in the same week replaces the row                   |
| `campaign_id`        | the active campaign when closed; null without one                            |
| `books_left`, `avg_pages`, `pages_per_hour`, `weeks_left`, `weekly_hours` | the required-hours inputs at close (§8.1); null without a campaign |

Required hours cannot be replayed (past book sizes and states are not kept), so the review keeps what it showed. Nothing else about a review is stored; the shortlist and pruning write to items as they happen.

### 2.9 Moments seen

`moments_seen(key, seen_at)`: the achievements (§6.10) whose moment on Home was closed. The achievements themselves are derived by replay, never stored; this only remembers what was already acknowledged. Exported with the rest.

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

- **5–7 items**, set by the user in the weekly review. Soft limit: the UI says so calmly below 5 and beyond 7, and permits both. Each tick writes at once.
- Sourcing is **loose**: any `pool` or `in_progress` item is eligible.
- The review UI defaults to showing shelf leaders. Reaching into a pool is a deliberate extra action.
- **Carry-over:** the review opens with last week's unfinished shortlist pre-selected. The user confirms or edits.
- Short items compete on equal footing with books and occupy WIP slots like anything else. Deliberate; §9.2 is the counterweight.

---

## 6. Screens

### 6.1 Home

In order, top to bottom:

1. **The board** — where the discipline stands, as ruled sections titled in the margin, with no explanations on Home:
   - **Hours:** the time left to read tonight as the one large figure (the rest of today's target plus what is owed), or _Done for today_ / _Rest day_; a bar for today (read against the target, what is owed as a hatched rubric zone after it) and one for the week (read against what is due so far, out of the week's total); the week as a strip of seven days; the hours ramp's next rise and last check. A week ahead of what is due that still owes time from a short day says so in rubric, and the ramp line says that time owed holds the next rise unless it is read by then. Without a plan it shows only the minutes read today and a link to the plan. Debt is always a figure in a distinct colour, and it pays down live as today's reading passes the target; a shortfall is added only at midnight. No sentences that scold, no exclamation marks. Nothing about the campaign here.
   - **Speed:** reading speed for the last closed week, labelled with its dates, with a pages/h ↔ words/min toggle and a bar of the material mix (§9.1); with a speed ramp, the index this week so far as a bar with baseline and target marks, and a short ledger of this week and the last check.
2. **In progress** — every `in_progress` item, with resume position, last-touched date, and estimated time remaining. Stalled items flagged. Each can be finished or abandoned (with its required reason) in place, so the WIP cap (§7.5) never waits for the review. Finishing offers the last stretch of reading (how long, and the position reached, filled with the item's size) so it is logged in the same step; a running timer on the item stops there. An item whose position has reached its size is marked _at the end_. The moment filter does not hide these; items that don't fit the current moment are visually de-emphasised, not removed.
3. **Picks** — `on_shortlist` items in `pool`, filtered by the moment.

**Action affordances** present on Home: capture, start session, retroactive session entry, the moment filter, and a single small, neutral indicator when the weekly review is overdue: from the day after `review_weekday` until a review is closed in the current week, and before the first review ever, once the library holds anything. No informational content beyond the board.

**Moment filter:**

- **Time available** — one tap: _quick_ / _an hour_ / _long_. Filters picks by estimated time remaining (§7.2) against the bucket bounds in settings. _Long_ hides nothing. Books and courses are read in stretches, so any time at hand suits them and they are never filtered: short windows are not steered away from the books a campaign counts.
- **Device** — not inferred. The device that logs is not the device that reads (a physical book at a desk, logged from a phone). `needs_desk` is shown as a mark on every entry and never filters.
- **"I'm fried"** — optional toggle, hides `focus_demand = deep`. Never ask the user to self-rate energy on a scale.

The filter resets on each page load.

### 6.2 Shelf view

Three slots on top, pool below, borrowed items distinguished. Each item shows its `why`.

The list of shelves (still no counts, §0) is where shelves are ordered with up/down buttons, renamed, and deleted. Renaming a shelf renames every tag equal to its old name, so borrowed items and their slots stay. Only a shelf with no items in any state can be deleted, through the confirmation dialog; a shelf holding history is emptied by moving its items first.

### 6.3 Weekly review

Configurable weekday, reachable any day from the navigation. The pruning ritual. One page, as ruled sections in order:

1. **Whys** — reread, in a batch and without controls: every `in_progress` item first (stalled flagged, last read), then each shelf's leaders by slot.
2. **Prune and rank** — the same groups, now with controls: abandon (required reason, inline), delete (only with zero sessions, §2.1, through the confirmation dialog), and each shelf's top three adjusted with up/down, unrank, and filling an empty slot from that shelf's pool, which is shown only on request. The review does not start items.
3. **Shortlist** (§5.2) — in-progress items, shelf leaders, and anything already on the shortlist; the rest of the pools on request. The current shortlist is last week's carry-over, pre-selected.
4. **Reached** — every achievement (§6.10) since the last review closed (or over the last week before any), newest first, big ones in verdigris; then last week item by item: time, sessions, and progress with the position it got to.
5. **Goal status** — the campaign's count and projection with needed against committed, what is owed, the closed week's ledger, and each ramp's last check; then required hours against the last closed review of the same campaign (§8.1).
6. **Composition report** (§9.2).

_Close the review_ records the week (§2.8) with today's required-hours inputs. If the review is skipped, the shortlist persists and Home shows the overdue indicator. Nothing else changes.

### 6.4 Session view

Start/stop timer. On stop: one number for end position, optional note. Both dismissible. _Stopped earlier?_ reveals a stop time for a timer left running. A running timer can be discarded. While a timer runs, every other screen shows it under the running head — the item, a ticking clock and _Stop_, which leads here — so a timer is never left running unseen.

**Today** closes the screen: every session of the day, newest first, with its times, length, item, positions and note, and _edited_ where corrected. Each can be edited in place or deleted (a dialog asks, as for anything that can't be undone).

**Retroactive entry** is on the same screen with equal prominence: item, start time, duration or end time, optional positions, optional note.

Everywhere in the app, time is typed the way it is said (1h30, 1:30, 1.5h, or 90 for minutes), read back live, and stored in minutes. Positions in things measured in minutes (videos, courses) accept the time the player shows (1:12:30).

### 6.5 Stats

Hours per day and week vs. committed target; debt over time; completions over time; pace per item, per band, and global, each obeying §9.1; campaign projection. Charts rendered as SVG from Go.

As built, linked from History and the Record: hours each week (the last 16) and each day (the last 28) as columns against a target mark, short closed days in rubric; what was owed at each day's close (the last 12 weeks); items completed each month (the last 12) stacked as books, other items and reference; pace over `pace_window_days` — everything read, in pages/h or words/min, with the mix of its hours, then by kind of material (the median of its items' paces) and item by item (faint under an hour); and the active campaign's books counted over time against an even pace to its target. Every mark carries a tooltip, every chart a legend or a title, and the column charts their figures as a table.

### 6.6 The finished archive

**Exists to satisfy goal #6. Must not be cut.**

Every `finished` and `reference` item, newest first, showing `why` and `verdict` side by side. Visual and pleasant. Shows accumulation — a count and a sense of the shelf being built.

As built: _Finished_ in the navigation. The count of books finished as the large figure, with other items, reference, pages and hours beside it; a drawn shelf of spines, oldest first, each as thick as its item is long, in its format's cloth (reference items outlined), each opening its book page; then month by month, each item with its why in the margin beside its verdict, its size, when it finished and the time it took.

The rest of this app is debt counters, frozen ramps, hard caps, and stall flags. Four punishment mechanisms and no reward surface is a design that gets abandoned in month five. This is the only screen showing what the user has _gained_ rather than what they _owe_. Load-bearing.

### 6.7 Plan

Where the schedule is decided and explained, as four ruled sections:

- **Campaign:** the count of books so far as the large figure, the deadline and weeks left; a bar of books finished against the target with a mark where the projection (§8.5) lands; a bar of recent book hours a week against what the campaign needs; a short ledger of what required hours are built from (books left, average pages, book pace with its focus mix, hours left) and required against committed per week. After the deadline, the final count. The form to start one, the name to rename, and ending as a quiet action that asks in a confirmation dialog, saying what it ends, with the safe choice focused. Closes with a plain account of how the campaign is counted.
- **This week:** the same today and week bars and day strip as Home, then the week as a ledger (target, read and what is owed after each closed day), the hours ramp's next rise, and a plain account of how hours and debt are counted.
- **Daily target:** active days, _same every day_ or _rising each week_, and the times, typed the way they are said (1h30, 1:30, 1.5h, 90) with a live readback. An _If you save_ summary, rendered by the server as the form is typed, says what saving does, including that today counts and closes at midnight. When the form matches the target in effect it says so, and that saving changes nothing. A save that lowers today's target asks for confirmation with a short, calm line explaining what changes. Never shaming. With an active campaign the summary also states the gap (§8.1), and _Match the campaign_ fills _same every day_ with the required book hours, divided by the recent share of reading on books (all of it without one, said so), spread over the active days, rounded up to the minute; it is not offered with no active campaign, after the deadline, or when that exceeds a day.
- **Speed:** last week's speed and mix; the speed ramp's target, its progress to the ceiling, this week's index so far, a ledger of every check (index, what it needed, hours measured, result) and last week's index worked through by material, so every figure traces to its sessions. Stopping is a destructive action that says what it ends. Without a running ramp, the baselines a new one would use and the form to start it. Closes with a plain account of how speed and the index are measured.

Explanations on this screen are at reading size and sit with the section they explain.

### 6.8 History

Every session, a week at a time, drawn as a calendar. In the navigation after Session.

- **The week:** its dates with the weeks before and after (never before the first week with a session or a plan, never after this one), the time read against the week's planned target, and a bar of what the time went to, item by item.
- **The calendar:** hours down the margin, a column per day headed by its date and what it read against its target (a short closed day in rubric). Each session is a block at the time it started, as tall as it ran, in its format's cloth, with its title and length. The grid spans at least 08:00–22:00 and widens to fit.
- **The chosen day** (today by default; a column head or a block chooses another): its sessions newest first, each correctable as on Session (§2.3).
- **The week went to:** each item's time, share, sessions and progress.

Days and weeks are addresses (`/history?day=…`). The day names on Home's week strip link to their day here.

### 6.9 The book page

Every item has one page (`/items/{id}`); its title opens it wherever it is listed. Reached by explicit navigation only, so §0 holds.

- **What it is:** title, state (in the pool, reading, finished or abandoned with the date), the why in the margin, author, format, size, shelf, tags, and its verdict or abandon reason.
- **Progress:** the furthest position against its size, the time left and what that rests on (its own pace, similar items, or provisional), started and last read. While reading, **when it finishes**: the day the time left runs out at the time a day it got over the last 14 days; with no reading in that fortnight, it says there is no date.
- **Reading:** total time over how many days and sessions, and its own pace with the time it rests on.
- **Sessions:** every one, newest first, correctable in place (§2.3).

### 6.10 Achievements

Reaching something the user set out to reach is acknowledged plainly and warmly, once, with the numbers that make it real (§11). Everything is derived by replay from items, sessions, campaigns, commitments and speed ramps, so a correction to the data corrects the achievements too.

| Achievement              | When                                                                        | Tier  |
| ------------------------ | --------------------------------------------------------------------------- | ----- |
| Book finished            | a book is finished (with its count toward the active campaign)             | small |
| Campaign halfway         | the ⌈target ÷ 2⌉-th counted book, for a target of 4 or more                 | big   |
| Campaign met             | the target-th counted book, before or on the deadline                      | big   |
| Hours ramp step          | a week-boundary check raises the daily target                              | small |
| Hours ramp at its top    | the daily target reaches the ramp's ceiling                                 | big   |
| Speed ramp step          | a check raises the speed target                                            | small |
| Speed ramp at its top    | the speed target reaches its ceiling                                        | big   |

**Big:** a moment above Home's board: a short headline, the dated fact, the few figures behind it (for a campaign: pages, hours and weeks it took, first and last book; at halfway, books ahead of or behind an even pace), and _Close_. It stays until closed (§2.9), one at a time, newest first, for 30 days after the day it happened.

**Small:** a warm line where it happens. Finishing a book on Home says the title, its count toward the campaign, and pages and time it took, with the verdict; the board's hours ramp line says what it rose to and when.

A campaign's end is always stated as it is: met, with the date and how early; or short, with the count. Achievements are shown only for real outcomes: nothing is awarded for logging, opening the app, or streaks.


### 6.11 Record

The long view of what the reading has reached, kept for good (_Record_ in the navigation, and where a moment's _See the record_ leads).

- **All told:** books finished as the large figure; other items, pages, hours of reading and on how many days, since when.
- **Campaigns:** every one, newest first, with its dates, a bar of books against its target, and its result as it went: met (the date and how early, and the count in all when it went past), ended with its count, the deadline passed with its count, or under way; and the day it was halfway.
- **Daily target** and **Speed:** every ramp from its start value to its ceiling, with a bar of how far it got and its result: reached its top (after how long), replaced or stopped (at what value), or rising (where it stands); and how many weeks it held on the way.
- **Year by year:** books, other items, pages, hours and goals reached per calendar year.
- **Bests,** as plain facts: the longest book finished, the most reading in a day and in a week, and the best closed-week speed index above the baseline. Nothing shaped like a run of days (§11).

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

The first weeks have no pace data. Use seeds and **label estimates provisional**. An estimate from an item's own pace that rests on under an hour of its reading is labelled _rough_. Do not display confident wrong numbers; a user who learns the estimates lie in week two will ignore them in month six.

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
avg_pages   = mean size_value of pool and in_progress books with size_unit=pages
              (fallback: mean of finished books; fallback: settings.fallback_book_pages)
book_pace   = Σ progress_delta ÷ Σ hours over sessions with positions on books in pages,
              every focus demand, started within pace_window_days
              (fallback, below 120 minutes of such sessions: the seeds for the focus of the
              waiting books, weighted by their pages — total pages ÷ Σ pages ÷ seed —
              or the medium seed with none; labelled provisional)
hours_left  = (target_count − books_finished) × avg_pages ÷ book_pace
weeks_until_deadline  = time from now to the end of the deadline day ÷ 7 days
required_weekly_hours = hours_left ÷ weeks_until_deadline
```

Required hours are **book hours**: time spent on videos or articles does not produce books. Book pace is shown with the focus mix of its hours (§9.1).

**Committed** — what the user is actually held to: the daily target of the governing commitment (§2.5) on each day in `active_days`, zero on other days. The week's committed total is the sum over its days. Where committed is compared with required it means a typical week: today's daily value × the number of active days (a ramp at its current value).

**Debt accrues against committed, never against required.** Week one of a ramp does not start the user 13 hours in debt.

The gap between committed and required is shown as a **projection consequence**, in the same breath as any edit: _"Committed 7 h a week; lately 60% of your hours went to books. At that: 46 of 100. The campaign needs 20 h of books a week."_ The projection reads the typed target as the plan it would start (§8.5, _If the plan holds_) at the recent book share (the share of hours on books over the weeks §8.5 uses); with no such weeks, it says it assumes all of it goes to books. Never silently.

`required_weekly_hours` recomputes as pace data arrives. The weekly review compares it with the inputs stored by the last closed review of the same campaign (§2.8), naming that review's date, as a ledger of books left, average book, book pace, weeks left and needed each week, then against now. When it changes by more than 10%, one line names the cause: the input with the largest share of the change, and a second when its share is at least half as large. Because required = books_left × avg_pages ÷ book_pace ÷ weeks_left, the log of the ratio splits exactly into one term per input, so the shares are exact. The four causes: books finished, average book size, pace, weeks remaining.

### 8.2 Debt

Derived by replay, from the first decision's day through yesterday, one calendar day at a time in `settings.timezone`:

```
target = committed minutes for that day (0 outside active_days)
logged = minutes of sessions clipped to that day
debt   = max(0, debt + target − logged)
```

Non-active days accrue nothing; minutes logged on them still pay debt down. When a day is also a week boundary, the day closes first and the ramp checks run after.

- Debt is **uncapped** and **carries forward indefinitely**.
- Debt has a **floor of zero. Surplus never banks.** A good week can only get the user back to even, never ahead.
- Debt **cannot be cancelled or forgiven** by any UI action. No stored number exists to edit. The only way out is reading.

### 8.3 The pain mechanism

**While debt is greater than zero, the hours ramp does not advance.** The user's own progress is the collateral. The speed ramp is not held by debt.

This is the only punishment mechanism. Do not add streaks, shaming copy, or notifications that scold.

### 8.4 The hours ramp

**Weeks** start at 00:00 on `settings.review_weekday`. Every ramp check happens there, so the review opens on a fresh result. A skipped review changes nothing.

- The ramp's value starts at `start_minutes` on its first day. Days before the first boundary count toward debt at once.
- **Advance rule:** at each week boundary, if debt is zero **and** the daily target has held at its current value for at least seven days, the value rises by `increment_minutes`, capped at `ceiling_minutes`. Otherwise it holds.
- "Held for seven days" follows the value, not the decision. Editing a running ramp saves a new ramp starting at the current value, so raising the ceiling mid-ramp loses no week.
- On reaching the ceiling the ramp **ends** and the commitment behaves as `fixed` at the ceiling until the user decides otherwise.
- A new ramp may be started at any time from a slump.

### 8.5 Rolling projection

Because the campaign is open-target, feasibility is projected continuously:

```
recent_weekly_book_hours = mean book hours over the last projection_window_weeks closed weeks
projected_finish         = books_finished
                         + (weeks_until_deadline × recent_weekly_book_hours × book_pace ÷ avg_pages)
```

Weeks start on `settings.review_weekday` (§8.4). Weeks before the first session ever logged are not counted as zero. With no closed week yet there is no projection, and the screen says so. Books are rounded down.

Shown on the plan, the weekly review and stats. Not on Home.

**If the plan holds.** Recent weeks lag a rising plan, so beside them the plan projects the daily target in effect read in full on every active day, a ramp rising at every check to its ceiling, from today to the deadline, at the recent book share (or all of it as books, said so) and today's book pace and average book: _If your plan holds, N_. With it: **the hours a book can take** under the plan (its book hours ÷ books left) against what the books waiting need at the book pace, and **the plan so far**: the books the targets lived since the campaign began come to at today's share, pace and size, beside the count finished. The daily target form's _If you save_ projects the typed target the same way, a ramp rising from the first check a full week after today. The hours ramp states the day it reaches its top if it rises every week.

### 8.6 Speed and the speed ramp

**Band speed** for a period = Σ progress_delta ÷ Σ hours over that band's finished sessions with positions whose `started_at` falls in the period. Bands are `(format, focus_demand, size_unit)`. Bands in `minutes` have no speed.

**Reading speed** for a period = the same over every band with a speed, with words converted to pages by `settings.words_per_page`. Shown in pages/h or words/min; the unit is a view toggle that is never stored.

A week needs **at least 120 minutes of positioned sessions** in bands with a speed before any speed is shown for it. A thinner week says so plainly and shows no number.

**Speed index** compares each item only with itself, so neither the mix of material nor a book printed larger can raise it. For a closed week, its **step** is the time-weighted mean, over the items read both in that week and in the period before it, of `item speed this week ÷ item speed then`; an item first read in a week counts from the week after. The period before is the last closed week with at least 120 minutes of positioned reading since the ramp began, or, for the first week, the `pace_window_days` before the ramp. The **index** multiplies the steps of the weeks with enough evidence since the ramp began: 1.0 is where it started. A session whose speed is over three times, or under a third of, its item's own pace over all its positioned sessions (for an item with at least three) is left out as a typo.

A ramp can start once any band has a measured speed over the `pace_window_days` before today.

**Advance rule:** the target starts at 100%. At each week boundary, if the closed week has the minimum evidence, its index is at least the target, and the target has held for at least seven days, the target rises by `increment_percent`, capped at `ceiling_percent`. Otherwise it holds. Reaching the ceiling ends the ramp as _reached_. The user may stop it at any time.

---

## 9. Honesty instrumentation

Two mechanisms, present because the user chose metrics that can drift flatteringly. **Not optional. Do not simplify away.**

### 9.1 Speed is never shown naked

Wherever a pace or speed number is displayed, the **material mix** behind it is displayed alongside: for the window in question, the share of hours by `(format, focus_demand)`. Global pace rises when lighter material is chosen; the user must never read the number without its ingredients. Applies to Home, the plan, stats, the review, and anywhere else pace appears.

The speed index (§8.6) exists for the same reason: it compares each kind of material only with itself, so choosing lighter material cannot raise it.

### 9.2 Composition report

In the weekly review, covering the last `projection_window_weeks` closed weeks (§8.4):

- Completed items (`finished` and `reference`, by `finished_at`) by format and by size bucket — e.g. _"11 short items, 0 books."_ Short items win shortlist competition by design; hours alone will not reveal it. An item's size bucket is the time actually logged on it, over all its sessions, against Home's bounds: _short_ up to `bucket_quick_max_min`, _an hour_ up to `bucket_hour_max_min`, _long_ above; items with no session are _no time logged_. Items abandoned in the window are counted on their own line.
- **Mean page count of books finished, trended** across the campaign: the same window repeated back from this week to the campaign's start (to the first finished book without a campaign), each block with its mean pages and its book count, newest first. Only books sized in pages count toward the mean; the rest are counted as having no page count. Under an open target the cheapest route to 100 is thinner books. If this slides from 320 to 180, the user sees it happening rather than discovering it at the deadline.

The report blocks nothing.

---

## 10. Backup and export

- **Automatic backups:** copy the SQLite file daily to a backup directory; keep the last 14 daily and last 8 weekly. Each copy is written under a temporary name and renamed once whole, so a copy cut short never passes for the day's backup.
- **Manual export** to JSON: items, tags, shelves, sessions (with `edited_at`), moments seen, campaign, active days, commitments, speed ramps, weekly reviews, settings. Complete enough to reconstruct the library elsewhere.
- **Import** from that JSON into an empty database: no shelves, items, sessions, plan decisions, speed ramps, campaigns, reviews or moments.

---

## 11. Out of scope for v1

- **Prerequisite links** between items — deferred.
- Staleness prompts on `reference` — declined.
- Android share-target capture — v1.1; keep PWA groundwork.
- Native mobile app; multi-user; auth; Datastar Pro.
- Any full-library browse surface on Home.
- Streaks, badges, points, scores, gamification, social features.

**Achievements are not gamification.** The app acknowledges outcomes the user set out to reach, never activity for its own sake. Big goals (a campaign met, its halfway mark, a ramp reaching its ceiling) get one warm, calm moment on Home until it is dismissed; small ones (a book finished, a ramp rising a step) get a warm line where they happen. Every one is kept on the record, and so are the goals missed. No exclamation marks, no confetti, nothing awarded for logging.
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
10. Schedule, debt, hours ramp, speed and speed ramp, the plan screen and Home's metrics section. Unit-tested against fixed clocks; the date arithmetic is where bugs hide.
11. Campaign, required-vs-committed, projection. Unit-tested.
12. Weekly review including composition report; abandon on Home; shelf ordering, renaming and deletion.

Steps 13 onward come from the audit of the first twelve: bugs in session data, gaps in daily use, goal numbers that mislead a rising plan, and goals reached without a trace. Each step's detail lands in the sections above when that step is agreed.

13. Session integrity: a late-logged session starts where reading stood at its own time; overlapping, implausible and backwards sessions are refused; editing an item keeps its shortlist flag and cannot change the unit its sessions are measured in; finishing offers the last stretch; the provisional book pace follows the focus of the books waiting.
14. Editing and deleting sessions, marked as edited; undo after logging.
15. The running timer shown on every screen.
16. History: a calendar of sessions by day and week with totals against the target, filters, and editing. Prototyped in throwaway directions first; the owner chooses.
17. The book page: its sessions, pace, progress and estimated finish.
18. Finished archive.
19. Achievements: derived by replay, acknowledged in two tiers (§11).
20. Record: every goal met or missed, year by year, all-time totals, personal bests.
21. Weekly review: what was reached since the last review, and each book's progress in the week.
22. Plan-aware projection, and the hours a book can afford under the plan.
23. A speed index that compares each item only with itself.
24. Board and plan clarity; the moment filter never hides books; _Match the campaign_ honours the book share.
25. Settings screen and data hygiene.
26. Stats.
