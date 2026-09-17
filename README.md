# Reading Tracker

> ⚠️ This app was built entirely with AI for personal use. Use at your own risk.

A small, single-user web app for deciding what to read next and for keeping a reading discipline over a year.

It runs as one Go binary with a SQLite file next to it. You open it in a browser, on the same machine or on your phone.

> **Status:** in development, used day to day. Home, capture, shelves, sessions, the plan and the campaign work. The weekly review, the finished archive and stats come next (see [Build status](#build-status)).

---

## Contents

- [What it is for](#what-it-is-for)
- [How it works](#how-it-works)
- [Screens](#screens)
- [Running it](#running-it)
- [Running it as a service](#running-it-as-a-service)
- [Your data](#your-data)
- [Development](#development)
- [Build status](#build-status)

---

## What it is for

Most reading tools show the whole library. A tidy list of 200 items still makes it hard to pick one. This app takes the opposite approach:

- **It hides things.** Home shows where you stand, what you are reading, and a short weekly shortlist. The rest of the library stays out of the way until you go looking for it.
- **It fits the moment.** Twenty minutes in bed and three hours at a desk call for different things. Each item has a shape (format, length, how much focus it needs), and Home can filter by the moment you have.
- **It keeps the numbers honest.** Any figure that could drift in a flattering direction is shown next to whatever explains the drift. Reading speed, for example, always appears with the mix of material behind it.

The full design lives in [`docs/SPEC.md`](docs/SPEC.md).

---

## How it works

### Items and shelves

An **item** is anything you mean to read or watch: a `book`, `article`, `paper`, `video` or `course`. Each one has:

| Field          | Meaning                                                            |
| -------------- | ------------------------------------------------------------------ |
| **Shelf**      | Its one home. Required.                                            |
| **Why**        | One line on why you want to read it. Required when you add it.     |
| **Tags**       | Any number, e.g. `Algorithms`, `C++`.                              |
| **Focus**      | `light`, `medium` or `deep`.                                       |
| **Size**       | In `pages`, `words` or `minutes`.                                  |
| **Needs desk** | Can't be read on a phone. Shown as a mark, never used as a filter. |

A shelf shows its own items **plus any item from another shelf that has a tag matching the shelf's name**. Those appear marked as _borrowed_.

> A statistics textbook lives on **Statistics** and has the tag `IQ`. It also shows up on the **IQ** shelf, marked borrowed. Nothing is duplicated.

Each shelf has **three ranked slots** (slot 1 is "next up"). Everything else on the shelf is the unordered **pool**.

### States

```
pool ──▶ in progress ──┬──▶ finished
  │                    ├──▶ reference
  └────────────────────┴──▶ abandoned
```

- **Finished** asks for an optional one-line _verdict_, which later sits beside the _why_.
- **Reference** is for material you consult rather than finish: specs, RFCs, papers you return to. It counts for hours, never toward a book goal.
- **Abandoned** requires a reason. An item with no sessions can be deleted instead; one with sessions never can.

Items only move forward. There is no way back from _in progress_ to the pool.

The number of items in progress is capped (5 by default). At the cap, you must finish or abandon something before starting another.

### The shortlist and the moment filter

The **shortlist** is the 5–7 items you want in front of you this week. Mark them from a shelf. Home shows shortlisted items from the pool as _picks_.

On Home, the **moment filter** narrows the picks:

| Filter      | Effect                             |
| ----------- | ---------------------------------- |
| _Quick_     | Items with 25 minutes or less left |
| _An hour_   | Items with 75 minutes or less left |
| _Long_      | Hides nothing                      |
| _I'm fried_ | Hides items that need deep focus   |

The filter resets each time the page loads.

### Sessions

A **session** is a stretch of reading with a start, an end and, optionally, where you started and stopped. There are two ways to record one, and both are first-class:

- **Timer.** Start it, read, stop it, and type the page you reached. The timer lives on the server, so closing the tab doesn't lose it.
- **Earlier.** Enter a session after the fact: item, start time, duration, optional positions and a note. This is the normal path for paper books read away from the screen.

Durations are typed the way you say them and read back as you type:

```
1h30    1:30    1.5h    90        → 90 minutes
```

For videos and courses, positions accept what the player shows, e.g. `1:12:30`.

### Estimates

From sessions that have positions, the app learns your **pace** per item and per kind of material. It uses that to estimate how long each item has left.

- If the item has no sessions of its own, the estimate comes from similar material.
- If there is no data at all, it falls back to a default pace and is marked **provisional**.
- An item in progress with no session in 14 days is marked **stalled**.

### The plan

The plan screen is where you set your commitments and see how they are counted.

**Daily target.** Choose your active days and a number of minutes, either the same every day or **rising each week** (the hours ramp).

**Debt.** Each day, whatever you read short of the target is added to what you owe. Reading beyond the target pays that down, but surplus is never banked:

```
Target 60 min, Monday to Friday

Mon  read 40   owed 20
Tue  read 90   owed  0     (20 + 60 − 90 → 0; the extra 10 is not kept)
Wed  read  0   owed 60
Sat  read 30   owed 30     (rest day: no target, but reading still pays down)
```

Debt can't be edited or forgiven. It is recalculated from your sessions every time, so a session entered late corrects past days.

**Hours ramp.** For example: start at 30 minutes and add 10 each week, up to 90. It rises at the start of each week only if you owe nothing and the target has held for seven days. While you owe anything, it holds.

**Speed and the speed ramp.** Reading speed is measured per week, in pages/h or words/min, and always shown with the mix of material it came from. The **speed index** compares each kind of material only with its own baseline, so choosing lighter reading can't inflate it. A speed ramp raises the index target week by week (e.g. +5%, up to 130%).

**Campaign.** A goal such as _100 books by 17 Sep 2027_. A book counts when it is finished between the campaign's start and its deadline. The plan shows:

- books so far and where the current pace would land;
- the hours of **book reading** a week the goal needs;
- how that compares with what you have committed to.

Only the name can be edited. Changing the target or deadline means ending the campaign and starting a new one.

---

## Screens

| Screen      | What you do there                                                                                                                  |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| **Home**    | See today's hours and what you owe, this week, reading speed, items in progress, and picks for the moment. Start and finish items. |
| **Session** | Run the timer, or enter a session that already happened.                                                                           |
| **Capture** | Add an item: paste a URL, search Open Library by title, or type it in.                                                             |
| **Shelves** | Browse a shelf, edit items, set the three ranked slots, mark the shortlist.                                                        |
| **Plan**    | Set active days, the daily target, the speed ramp and the campaign, with the reasoning behind each number.                         |

The app installs as a PWA from the browser menu. Only the app shell is cached; your data always comes from the server.

---

## Running it

**Requirements:** Go 1.25 or newer. No C compiler is needed (SQLite is pure Go).

From the repository root:

```sh
go run ./cmd/readingqueue
```

Open <http://127.0.0.1:8080>.

The binary is still called `readingqueue`, the project's working name.

### Options

| Flag       | Default                                       | Meaning                                              |
| ---------- | --------------------------------------------- | ---------------------------------------------------- |
| `-addr`    | `127.0.0.1:8080`                              | Address to listen on                                 |
| `-db`      | `~/.local/share/readingqueue/readingqueue.db` | SQLite database file; created if missing             |
| `-import`  |                                               | Load a JSON export into an empty database, then exit |
| `-version` |                                               | Print the version and exit                           |

The database schema is created and upgraded automatically on start.

### Reaching it from another device

By default the app only listens on the local machine. **It has no login**, so do not expose it to the internet. To use it from your phone, put both devices on a private network (for example [Tailscale](https://tailscale.com)) and listen on that interface:

```sh
readingqueue -addr 100.x.y.z:8080
```

---

## Running it as a service

For daily use, the app runs in the background as a **systemd user service**.

systemd is the Linux service manager: it starts programs, restarts them when they crash and collects their logs. A _user_ service runs as you, without root, and starts when you log in. That suits a personal app whose data lives in your home directory.

The recipe is [`deploy/readingqueue.service`](deploy/readingqueue.service):

```ini
[Service]
ExecStart=%h/go/bin/readingqueue -addr 127.0.0.1:8080   # what to run (%h = your home)
Restart=on-failure                                       # start again after a crash

[Install]
WantedBy=default.target                                  # start at login once enabled
```

It assumes `go install` puts binaries in `~/go/bin`, which is Go's default.

### Install or update

Both are the same command:

```sh
./deploy/deploy.sh
```

The script:

1. refuses to run if there are uncommitted changes, so what runs is always a commit;
2. runs `go test ./...` and stops if anything fails;
3. builds and installs the binary with `go install`;
4. installs the unit file and stops the running service;
5. copies the database to `backups/pre-deploy-<commit>.db`;
6. starts the service and enables it at login.

Then reload the page.

### Managing the service

```sh
systemctl --user status readingqueue     # is it running, and since when
systemctl --user restart readingqueue    # restart it
systemctl --user stop readingqueue       # stop it until the next login
systemctl --user disable readingqueue    # stop starting it at login
journalctl --user -u readingqueue -f     # follow its logs
```

### Rolling back

Schema upgrades only go forward, so rolling back means restoring the database copy taken just before the bad update, then running the older commit:

```sh
data=~/.local/share/readingqueue

systemctl --user stop readingqueue
cp "$data/backups/pre-deploy-<commit>.db" "$data/readingqueue.db"
rm -f "$data/readingqueue.db-wal" "$data/readingqueue.db-shm"

git checkout <commit>
./deploy/deploy.sh
```

Use the copy made just before the bad update, and the commit you were running before it. When the problem is fixed, `git checkout main` and deploy again.

---

## Your data

Everything lives in one directory:

```
~/.local/share/readingqueue/
├── readingqueue.db                     the database
└── backups/
    ├── readingqueue-2026-09-17.db      automatic daily backups
    └── pre-deploy-53a0311.db           copies taken by deploy.sh
```

- **Automatic backups.** Once a day the app writes a consistent copy of the database. It keeps the last 14 daily and the last 8 weekly copies. Pre-deploy copies are never removed automatically.
- **Export.** <http://127.0.0.1:8080/export> downloads everything as JSON: shelves, items, tags, sessions, plan, campaigns and settings.

  ```sh
  curl -o reading-export.json http://127.0.0.1:8080/export
  ```

- **Import.** Loads an export into an empty database. Stop the service first:

  ```sh
  systemctl --user stop readingqueue
  readingqueue -db ~/.local/share/readingqueue/readingqueue.db -import reading-export.json
  systemctl --user start readingqueue
  ```

Backups sit on the same disk as the database. To survive losing the machine, also copy the `backups/` directory, or a regular export, somewhere else.

All timestamps are stored in UTC. Days and weeks follow the timezone in settings (by default the machine's).

---

## Development

Keep development away from your real data. The everyday commands are in the `Makefile`, and all of them use a separate port and the ignored `.dev/` directory:

```sh
make dev      # http://127.0.0.1:8081, on .dev/readingqueue.db
make lan      # the same, reachable from the phone on this network
make test     # go test ./...
make check    # formatting, vet and tests: run before committing
make deploy   # deploy/deploy.sh
```

`make` on its own lists them. Templates, styles and fonts are embedded in the binary, so `make dev` has to be restarted to see a change to any of them; only the database is live.

A few rules keep the running app safe:

- Deploy only committed work with passing tests (the script enforces both).
- Never edit a migration that has already been committed. Schema changes go in a new file in `internal/sqlite/migrations/`.

### Layout

```
cmd/readingqueue/     entry point: flags, server, shutdown
internal/library/     domain logic: items, shelves, sessions, pace, debt, ramps, campaign
internal/sqlite/      storage and migrations
internal/web/         HTTP handlers, templates and static assets (Datastar over SSE)
internal/metadata/    URL and Open Library lookups
internal/backup/      daily backups and retention
deploy/               systemd unit and deploy script
docs/SPEC.md          the specification
```

The domain logic has no HTTP or template imports and is tested with `go test` alone. Date logic is tested against a fixed clock.

### Stack

Go standard library, SQLite via [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite), and [Datastar](https://data-star.dev) with its Go SDK. Pages are rendered on the server and updated with HTML fragments over server-sent events. Templates, styles, fonts and scripts are embedded in the binary. Nothing is stored in the browser.

---

## Build status

Built in the order set in [`docs/SPEC.md` §12](docs/SPEC.md#12-build-order):

|     | Step                                                             |
| --- | ---------------------------------------------------------------- |
| ✓   | Domain, storage and HTTP layer                                   |
| ✓   | Backup and export                                                |
| ✓   | Capture, shelves, sessions                                       |
| ✓   | Pace, estimates, stall detection                                 |
| ✓   | Home                                                             |
| ✓   | Plan: schedule, debt, hours and speed ramps                      |
| ✓   | Campaign and projection                                          |
| ·   | Weekly review, abandoning and deleting items, composition report |
| ·   | Finished archive                                                 |
| ·   | Stats                                                            |
