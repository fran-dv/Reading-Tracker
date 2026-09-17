-- Step 19: achievements are replayed, never stored (spec §6.10). This only
-- remembers which moments on Home were closed, by the achievement's key.
CREATE TABLE moments_seen (
    key     TEXT PRIMARY KEY,
    seen_at TEXT NOT NULL
);
