-- Step 11: campaigns (spec §2.4). Days are calendar days "2006-01-02" in
-- settings.timezone; the deadline is inclusive. ended_on is NULL while the
-- campaign is active, and at most one is.
CREATE TABLE campaigns (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL CHECK (length(trim(name)) > 0),
    target_count INTEGER NOT NULL CHECK (target_count BETWEEN 1 AND 10000),
    started_on   TEXT NOT NULL,
    deadline     TEXT NOT NULL CHECK (deadline > started_on),
    ended_on     TEXT
);
-- Every active row indexes the same value.
CREATE UNIQUE INDEX campaigns_one_active ON campaigns((ended_on IS NULL)) WHERE ended_on IS NULL;
