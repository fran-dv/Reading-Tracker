-- Step 10: the decisions debt and the hours ramp are replayed from (spec §2.5).
-- effective_on is a calendar day "2006-01-02" in settings.timezone; one
-- decision per day, a later save on the same day replaces it.

-- days is a weekday set: bit 0 Sunday … bit 6 Saturday.
CREATE TABLE active_days (
    effective_on TEXT PRIMARY KEY,
    days         INTEGER NOT NULL CHECK (days BETWEEN 1 AND 127)
);

-- Columns that do not belong to the kind hold 0.
CREATE TABLE commitments (
    effective_on      TEXT PRIMARY KEY,
    kind              TEXT NOT NULL CHECK (kind IN ('fixed', 'ramp')),
    minutes_per_day   INTEGER NOT NULL DEFAULT 0,
    start_minutes     INTEGER NOT NULL DEFAULT 0,
    increment_minutes INTEGER NOT NULL DEFAULT 0,
    ceiling_minutes   INTEGER NOT NULL DEFAULT 0,
    CHECK (
        (kind = 'fixed' AND minutes_per_day BETWEEN 1 AND 1440)
        OR (kind = 'ramp' AND start_minutes >= 1 AND increment_minutes >= 1
            AND ceiling_minutes > start_minutes AND ceiling_minutes <= 1440)
    )
);
