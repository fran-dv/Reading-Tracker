# Datastar 1.0 documentation (vendored)

Local snapshot of the official Datastar docs, taken so frontend work follows
current 1.0 syntax instead of recalled pre-1.0 syntax (`data-store`, `data-model`
are dead; see `RELEASE-NOTES.md`, v1.0.0-RC.6, for the breaking changes).

| What                    | Version  | Date       | Source                                                        |
| ----------------------- | -------- | ---------- | ------------------------------------------------------------- |
| Datastar (browser lib)  | v1.0.3   | 2026-08-27 | https://github.com/starfederation/datastar/releases           |
| Go SDK `datastar-go`    | v1.2.2   | 2026-06-02 | https://github.com/starfederation/datastar-go (tag v1.2.2)    |
| Website docs            | live     | 2026-09-14 | https://data-star.dev                                         |

Fetched 2026-09-14. Each markdown file starts with a `Source:` comment giving
the exact URL it was converted from. Pages were converted from the live site
HTML; code blocks are verbatim, prose is verbatim, only layout chrome (nav,
copy buttons, live demo widgets) was dropped.

## Layout

- `guide/` — the five guide pages, in reading order:
  `getting_started`, `reactive_signals`, `backend_requests`,
  `datastar_expressions`, `the_tao_of_datastar`.
- `reference/` — `attributes`, `actions`, `sse_events`, `security`, `sdks`,
  `rocket`.
- `how_tos/` — the six official how-tos (polling, redirect from backend,
  keep SSE open, keydown to specific keys, load more, keep code DRY).
- `examples/` — all non-Rocket example pages from data-star.dev/examples.
- `sdk-go/` — the Go SDK: its README, the SDK architecture decision record
  (`SDK-ADR.md`, the wire-protocol spec every SDK implements), and the full
  source of `datastar-go` v1.2.2 under `source/`. Go files are renamed
  `*.go.txt` so `go test ./...` never compiles them.
- `RELEASE-NOTES.md` — GitHub release notes v1.0.0-RC.3 through v1.0.3.

## Things to know before writing frontend code

- **Script tag** (version-locked, from `guide/getting_started.md`):
  `<script type="module" src="https://cdn.jsdelivr.net/gh/starfederation/datastar@v1.0.3/bundles/datastar.js"></script>`
  The docs recommend self-hosting the file. This project embeds it via
  `embed.FS`; the bundle is 33 KB.
- **Attribute key delimiter is `:`**, not `-`: `data-on:click`,
  `data-signals:foo`, `data-bind:foo`, `data-class:hidden`. Modifiers use `__`:
  `data-on:input__debounce.300ms`.
- **`data-init`** replaced `data-on-load`.
- **Backend actions** are `@get()`, `@post()`, `@put()`, `@patch()`,
  `@delete()`. Signals are sent as JSON body (non-GET) or `datastar` query
  param (GET). Server reads them with `datastar.ReadSignals(r, &v)`.
- **Two SSE event types only**: `datastar-patch-elements` and
  `datastar-patch-signals` (see `reference/sse_events.md`). Go SDK:
  `sse := datastar.NewSSE(w, r)`, then `sse.PatchElements(html)`,
  `sse.PatchSignals(json)`, `sse.MarshalAndPatchSignals(v)`,
  `sse.ExecuteScript(js)`, `sse.Redirect(url)`.
- **Morphing matches on element `id`.** Put IDs on every top-level element
  you patch.
- **Pro-only features** (do not use, MIT core only): attributes
  `data-animate`, `data-custom-validity`, `data-match-media`, `data-on-raf`,
  `data-on-resize`, `data-persist`, `data-query-string`, `data-replace-url`,
  `data-scroll-into-view`, `data-view-transition`; actions `@clipboard()`,
  `@fit()`, `@intl()`; the bundler. They are marked `[Pro]` in the reference
  pages. Rocket (`reference/rocket.md`) is a separate JS component API; the
  spec does not call for it.
- **CSP mode** (opt-in, v1.0.3) lets Datastar run without `unsafe-eval`. See
  `reference/security.md`.

## Refreshing

The site has no llms.txt and the docs source is not in a public repo. To
refresh, re-fetch the pages listed above from data-star.dev and re-run the
conversion (strip `<article>` chrome, keep `<pre>` line text, decode
Cloudflare email obfuscation which otherwise mangles `datastar@v1.0.3`).
