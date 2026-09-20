-- Step 28: a campaign is a count of books or a named set of items (spec
-- §2.4). A set has no target_count; its items live in campaign_items, each
-- dated by the day it was added, and a set can only grow.
CREATE TABLE campaigns_new (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL CHECK (length(trim(name)) > 0),
    kind         TEXT NOT NULL CHECK (kind IN ('count', 'set')),
    target_count INTEGER,
    started_on   TEXT NOT NULL,
    deadline     TEXT NOT NULL CHECK (deadline > started_on),
    ended_on     TEXT,
    CHECK ((kind = 'count' AND target_count BETWEEN 1 AND 10000) OR (kind = 'set' AND target_count IS NULL))
);
INSERT INTO campaigns_new SELECT id, name, 'count', target_count, started_on, deadline, ended_on FROM campaigns;

-- review_campaigns references campaigns(id), so it is rebuilt with the
-- parent. A set keeps only the hours it needs; a count keeps its inputs.
CREATE TEMP TABLE old_needs AS SELECT * FROM review_campaigns;
DROP TABLE review_campaigns;
DROP TABLE campaigns;
ALTER TABLE campaigns_new RENAME TO campaigns;

CREATE TABLE review_campaigns (
    week_of        TEXT NOT NULL REFERENCES reviews(week_of) ON DELETE CASCADE,
    campaign_id    TEXT NOT NULL REFERENCES campaigns(id),
    books_left     INTEGER,
    avg_pages      REAL,
    pages_per_hour REAL,
    hours_left     REAL NOT NULL,
    weeks_left     REAL NOT NULL,
    weekly_hours   REAL NOT NULL,
    PRIMARY KEY (week_of, campaign_id),
    CHECK ((books_left IS NOT NULL AND avg_pages IS NOT NULL AND pages_per_hour IS NOT NULL)
        OR (books_left IS NULL AND avg_pages IS NULL AND pages_per_hour IS NULL))
);
INSERT INTO review_campaigns
    SELECT week_of, campaign_id, books_left, avg_pages, pages_per_hour,
           books_left * avg_pages / pages_per_hour, weeks_left, weekly_hours
    FROM old_needs;
DROP TABLE old_needs;

-- An item with history is abandoned rather than deleted, so a set loses a
-- member only when an item with no sessions at all is deleted.
CREATE TABLE campaign_items (
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    item_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    added_on    TEXT NOT NULL,
    PRIMARY KEY (campaign_id, item_id)
);
