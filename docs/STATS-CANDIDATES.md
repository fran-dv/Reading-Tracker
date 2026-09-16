# Stats candidates

Analyses beyond SPEC.md §6.5 that might earn a place on the stats screen.
Parked until step 14, when real sessions exist to judge them against.
Nothing here is a build instruction. Each entry must survive two tests
before it is specced: it answers a question the user actually has, and the
data in the file is enough to answer it honestly (§7.3: no confident wrong
numbers).

Every one of these is a pivot, a quantile, or a bootstrap over the two
recorded quantities (§7.1). None needs anything beyond Go and the export.
Exploration happens in a notebook over `GET /export`; only what proves
useful gets specced and ported.

| Candidate | Question it answers | Method | Minimum data |
| --- | --- | --- | --- |
| Weekday × hour heatmap | When do I actually read, versus when I think I do? | Sum of session minutes by weekday and start hour, in `settings.timezone`. Sessions spanning hours split proportionally. | ~8 weeks of sessions |
| Fatigue curve | Does my pace decay after 45 minutes? What session length is optimal? | Pace per session bucketed by session length (15/30/45/60/90+ min), per band. Median per bucket, count shown. | ~100 sessions with positions in one band |
| Pace drift within an item | Do I slump in the middle of a book? Does a slump predict abandonment? | Per item, pace in first / middle / last third of `size_value`. Compare finished vs abandoned. | ~20 finished + ~10 abandoned books with positions |
| Finish date with uncertainty | Not "when" but "how sure": a range instead of one date. | Bootstrap weekly hours over `projection_window_weeks`; report P10/P50/P90 finish date per item and for the campaign. Honesty upgrade to §8.5. | ~12 weeks of hours |
| Stall → abandon survival | After how many silent days is an item effectively dead? Calibrates `stall_days` from my own behaviour. | For every item, longest silence before its next session or its end state. Fraction that resumed, by silence length. | ~30 closed items |
| Debt recovery time | How long do I take to get back to zero? Are slumps getting longer? | Length of each debt > 0 episode, trended across the campaign. | 3+ debt episodes |
| Consistency | Is reading spread out or bursty? Bursty habits are the ones that break. | Gini or coefficient of variation of daily minutes over the last 4 weeks, on active days only. | ~4 weeks |

## Rejected on sight

- Anything "predictive" beyond quantiles of the user's own history. n is a
  few hundred self-reported sessions a year; a model on that is noise with
  decimals.
- Cross-user benchmarks. Single user, by design.
- Reading-speed leaderboards, streaks, or scores. §11.
