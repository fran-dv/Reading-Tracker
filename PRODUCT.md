# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

One person: the owner, who also builds it. They read a great deal across books, articles, papers, videos and courses. They want to hold a reading discipline for a year, with a first goal of 100 books.

They use it in two different situations, both from day one:

- **On the phone.** Short windows, in bed or away from the desk, often next to a physical book being read. The job is to pick something fitting the moment, or to log time just read on paper.
- **At the laptop.** Longer focused sessions, capture of links found while working, and the weekly review ritual.

Every earlier tool they tried (Notion, Excel, reading trackers) failed the same way: a well-organised list of 200 items still produced paralysis.

## Product Purpose

Reading Tracker decides what to read next and holds the reading discipline over a year. Success means opening the app, seeing a handful of candidates, picking one and starting to read within seconds. Over the year it means the discipline held, the goals moved, the metrics stayed honest, and the process stayed enjoyable.

The owner's goals, in their words from the spec: consolidate a reading discipline, get insightful metrics on reading habits, reach reading goals (100 books in a year first), manage time well, read a great deal, and enjoy the process. Enjoyment is a requirement, not a nice-to-have.

## Positioning

Three mechanisms a library manager or generic tracker does not have:

1. **It hides things.** Most of the library is invisible most of the time. Home shows where the discipline stands, what is in progress and a weekly shortlist, nothing more. Full access exists behind the weekly review and explicit navigation.
2. **The right item depends on the moment.** Items carry a shape (format, size, focus demand, desk need). A one-tap moment filter (time available, "I'm fried") narrows candidates to what fits right now. The device is never inferred: the phone that logs is often not where the book is read.
3. **It never lets the user be quietly fooled.** Any number that can drift flatteringly is shown with what would explain the drift. Speed is never shown without its material mix, a speed index compares each kind of material only with itself, and a composition report exposes thin-book substitution.

## Operating Context

- Single user, no accounts. The Go binary runs locally with SQLite on the same machine; the phone reaches it over the network. Moving to a server later means moving the binary.
- Physical books are read away from any screen, so retroactive session entry is a primary path with the same prominence as the timer.
- Capture is the most frequent action: paste a URL, search Open Library by title, or type it in. Every item is filed on a shelf immediately with a one-line "why". There is no inbox.
- Weekly review on a configured weekday: reread the whys, prune, adjust each shelf's top three, set the 5–7 item shortlist, check goal status, read the composition report.
- Discipline machinery is punitive by the owner's choice: uncapped debt against committed hours, an hours ramp that holds while anything is owed, a WIP cap, stall flags. A speed ramp raises a reading-speed target from a measured baseline; debt never holds it. Debt and both ramps are replayed from the sessions, so reading logged late corrects the past.
- The plan is where commitments are set and explained: active days, a fixed or rising daily target, the speed ramp, and in plain words how each is counted. Times are typed the way they are said (1h30, 1:30, 90).
- The finished archive is the one surface that shows what was gained. Each item's opening "why" sits beside its closing verdict.

## Capabilities and Constraints

- Screens (spec §6): Home, shelf view, capture, session timer with retroactive entry, plan, weekly review, stats, finished archive. They are built in the spec's §12 order; Home, shelves, capture, session, plan (with the campaign) and the weekly review exist, and the finished archive (step 13) is next.
- Stack is fixed: Go, SQLite (pure Go), Datastar 1.0 over server-sent HTML fragments, html/template, everything embedded in one binary. No JSON API, no client framework, no browser storage.
- PWA-ready: manifest and a service worker caching the app shell only. Android share-target capture is v1.1.
- All assets are self-hosted, including fonts. No CDN, so the cached shell works offline.
- Out of scope for v1: prerequisite links, multi-user, auth, native apps, streaks, badges, gamification, social features, highlights, and any browse-everything view on Home.
- Terminology from the spec: pool, in progress, finished, reference, abandoned; shelf, borrowed, rank slot, shortlist; why, verdict; debt (shown as "owed"), committed vs required hours, hours ramp, speed ramp, baseline, speed index, campaign; provisional estimate; stalled.
- Open decision: the product name. The owner prefers "Reading Tracker" over the working name "Reading Queue" that the page title and binary (`readingqueue`) still use. Renaming the code is not decided.

## Brand Commitments

- **Name:** Reading Tracker (see open decision above).
- **Voice:** English, terse, calm. No exclamation marks, no motivational or cheerful copy, no scolding. Labels are nouns, buttons are verbs. Debt is a plain number, not a message.
- **Dark only.** The owner chose a single dark interface with no light variant.
- **Must feel:** warm, usable, pleasant, sober, and not bloated.
- **Must not feel:** gamified or motivational; a generic business dashboard (KPI tiles, rings, card grids); generic template UI. Where the discipline is measured it reads as a ledger: ruled sections, bars and tables with every figure written out, and one large figure for what is left tonight.

## Evidence on Hand

- No existing library, import file or real content. The app starts empty.
- No logo, icon, illustration or photography assets.
- Covers come from Open Library, article og:image, or YouTube thumbnails when an item has them.
- Design work may use sample items, labelled synthetic. It must not invent the owner's library or reading history.

## Product Principles

1. **Hide by default.** Show a handful of candidates, never the collection, on any surface seen daily.
2. **Fit the moment.** Filter by the shape of now; de-emphasise what doesn't fit rather than removing it.
3. **Honest numbers.** Every figure that can flatter appears with its explanation; provisional estimates say so.
4. **Friction only where it prunes.** Capture and logging stay fast; the mandatory "why" is the one deliberate cost.
5. **Reward is load-bearing.** The discipline is strict by choice, so what was gained must be as visible as what is owed.
