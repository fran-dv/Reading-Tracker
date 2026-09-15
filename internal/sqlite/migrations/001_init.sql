-- Step 1 schema: shelves, items, tags, rank slots, sessions, settings.
-- Timestamps are TEXT in fixed UTC form "2006-01-02T15:04:05.000Z".
-- Optional strings are stored as '' ; optional numbers and timestamps as NULL.

CREATE TABLE shelves (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE COLLATE NOCASE CHECK (length(trim(name)) > 0),
    sort_order INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE items (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL CHECK (length(trim(title)) > 0),
    url              TEXT NOT NULL DEFAULT '',
    author           TEXT NOT NULL DEFAULT '',
    format           TEXT NOT NULL CHECK (format IN ('book', 'video', 'article', 'paper', 'course')),
    shelf_id         TEXT NOT NULL REFERENCES shelves(id),
    why              TEXT NOT NULL CHECK (length(trim(why)) > 0),
    verdict          TEXT NOT NULL DEFAULT '',
    abandoned_reason TEXT NOT NULL DEFAULT '',
    focus_demand     TEXT NOT NULL CHECK (focus_demand IN ('light', 'medium', 'deep')),
    size_value       INTEGER,
    size_unit        TEXT NOT NULL CHECK (size_unit IN ('pages', 'minutes', 'words')),
    word_count       INTEGER,
    needs_desk       INTEGER NOT NULL DEFAULT 0,
    state            TEXT NOT NULL CHECK (state IN ('pool', 'in_progress', 'finished', 'reference', 'abandoned')),
    on_shortlist     INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    started_at       TEXT,
    finished_at      TEXT
);
CREATE INDEX items_shelf ON items(shelf_id);
CREATE INDEX items_state ON items(state);

CREATE TABLE item_tags (
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    tag     TEXT NOT NULL COLLATE NOCASE CHECK (length(trim(tag)) > 0),
    PRIMARY KEY (item_id, tag)
);
CREATE INDEX item_tags_tag ON item_tags(tag);

-- One row per (shelf, slot). A borrowed item may hold a slot on each shelf
-- that shows it; the same item never holds two slots on one shelf.
CREATE TABLE shelf_ranks (
    shelf_id TEXT NOT NULL REFERENCES shelves(id) ON DELETE CASCADE,
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    slot     INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 3),
    PRIMARY KEY (shelf_id, slot),
    UNIQUE (shelf_id, item_id)
);

CREATE TABLE sessions (
    id                    TEXT PRIMARY KEY,
    item_id               TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
    started_at            TEXT NOT NULL,
    ended_at              TEXT,
    position_start        INTEGER,
    position_end          INTEGER,
    note                  TEXT NOT NULL DEFAULT '',
    entered_retroactively INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX sessions_item ON sessions(item_id, started_at);
-- At most one running session: every running row indexes the same value.
CREATE UNIQUE INDEX sessions_one_running ON sessions((ended_at IS NULL)) WHERE ended_at IS NULL;

CREATE TABLE settings (
    id                      INTEGER PRIMARY KEY CHECK (id = 1),
    timezone                TEXT NOT NULL,
    wip_cap                 INTEGER NOT NULL,
    stall_days              INTEGER NOT NULL,
    review_weekday          INTEGER NOT NULL,
    bucket_quick_max_min    INTEGER NOT NULL,
    bucket_hour_min_min     INTEGER NOT NULL,
    bucket_hour_max_min     INTEGER NOT NULL,
    bucket_long_min_min     INTEGER NOT NULL,
    pace_window_days        INTEGER NOT NULL,
    projection_window_weeks INTEGER NOT NULL,
    seed_pace_light         INTEGER NOT NULL,
    seed_pace_medium        INTEGER NOT NULL,
    seed_pace_deep          INTEGER NOT NULL,
    seed_pace_wpm           INTEGER NOT NULL,
    fallback_book_pages     INTEGER NOT NULL
);
INSERT INTO settings VALUES (1, 'Local', 5, 14, 0, 25, 45, 75, 90, 90, 4, 40, 30, 15, 230, 300);
