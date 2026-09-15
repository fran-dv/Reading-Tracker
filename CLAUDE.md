# Reading Queue — agent instructions

Read `SPEC.md` in full before doing anything. It is the source of truth. Where this file and the spec disagree, the spec wins.

## Non-negotiables

- **Build in the order given in SPEC.md §12.** One step per session. Do not start a step until the previous one is committed with passing tests.
- **Do not add features not in the spec.** No browse-all views, no streaks, no dashboards, no "helpful" extras. §0 and §11 explain why. If something seems missing, ask; do not invent.
- **Domain logic lives in Go packages with no HTTP, template, or Datastar imports.** It must be testable with `go test` alone.
- **`go test ./...` must be green before a step is declared done.** For debt, ramp, and campaign logic, tests inject a fixed clock and cover week boundaries and the configured timezone.
- **All timestamps stored UTC.** Day/week boundaries computed in `settings.timezone`.
- **Never use browser storage.** All state is server-side SQLite.

## Style

- Go 1.24+. Standard library where it suffices.
- SQLite via `modernc.org/sqlite`. No cgo.
- Sober, functional UI. Beautiful but calm and use-focused.
- Prefer boring code. The owner is learning Go by reading this codebase; clarity over cleverness, always.
- Simplicity in every layer. A reliable, clean, well-designed system where every line earns its place. Before adding a type, method, or abstraction, ask whether removing it would hurt. Follow best practices and idiomatic use.

## When unsure

Stop and ask. The owner wants to be consulted on every design decision. A wrong guess costs more than a question.
