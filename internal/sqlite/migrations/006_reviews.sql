-- Step 12: closed weekly reviews (spec §2.8). week_of is the calendar day
-- "2006-01-02" the week starts on in settings.timezone; closing again in the
-- same week replaces the row. The other columns are the active campaign's
-- required-hours inputs as the review showed them, all NULL without one.
CREATE TABLE reviews (
    week_of        TEXT PRIMARY KEY,
    closed_at      TEXT NOT NULL,
    campaign_id    TEXT REFERENCES campaigns(id),
    books_left     INTEGER,
    avg_pages      REAL,
    pages_per_hour REAL,
    weeks_left     REAL,
    weekly_hours   REAL,
    CHECK (
        (campaign_id IS NULL AND books_left IS NULL AND avg_pages IS NULL
            AND pages_per_hour IS NULL AND weeks_left IS NULL AND weekly_hours IS NULL)
        OR (campaign_id IS NOT NULL AND books_left IS NOT NULL AND avg_pages IS NOT NULL
            AND pages_per_hour IS NOT NULL AND weeks_left IS NOT NULL AND weekly_hours IS NOT NULL)
    )
);
