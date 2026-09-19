-- Step 27: any number of campaigns can be active at once (spec §2.4), so a
-- closed review keeps the required-hours inputs of each one (spec §2.8).
DROP INDEX campaigns_one_active;

CREATE TEMP TABLE old_needs AS
    SELECT week_of, campaign_id, books_left, avg_pages, pages_per_hour, weeks_left, weekly_hours
    FROM reviews WHERE campaign_id IS NOT NULL;

CREATE TABLE reviews_new (
    week_of   TEXT PRIMARY KEY,
    closed_at TEXT NOT NULL
);
INSERT INTO reviews_new SELECT week_of, closed_at FROM reviews;
DROP TABLE reviews;
ALTER TABLE reviews_new RENAME TO reviews;

-- One row per campaign active and not over when the review closed.
CREATE TABLE review_campaigns (
    week_of        TEXT NOT NULL REFERENCES reviews(week_of) ON DELETE CASCADE,
    campaign_id    TEXT NOT NULL REFERENCES campaigns(id),
    books_left     INTEGER NOT NULL,
    avg_pages      REAL NOT NULL,
    pages_per_hour REAL NOT NULL,
    weeks_left     REAL NOT NULL,
    weekly_hours   REAL NOT NULL,
    PRIMARY KEY (week_of, campaign_id)
);
INSERT INTO review_campaigns SELECT * FROM old_needs;
DROP TABLE old_needs;
