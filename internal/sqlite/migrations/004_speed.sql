-- Step 10: speed ramps (spec §2.6, §8.6) and the display conversion between
-- pages and words. Baselines are replayed from sessions, not stored.
CREATE TABLE speed_ramps (
    started_on        TEXT PRIMARY KEY, -- calendar day "2006-01-02"; one ramp per day
    increment_percent INTEGER NOT NULL CHECK (increment_percent >= 1),
    ceiling_percent   INTEGER NOT NULL CHECK (ceiling_percent > 100 AND ceiling_percent <= 1000),
    stopped_on        TEXT
);

ALTER TABLE settings ADD COLUMN words_per_page INTEGER NOT NULL DEFAULT 300;
