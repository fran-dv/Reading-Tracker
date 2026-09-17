-- Cover images, kept as bytes so one file is still the whole state: a
-- VACUUM INTO backup and GET /export carry the covers with them.
-- A row with an empty media_type records a fetch that failed, so a dead
-- link is not retried on every page draw.
CREATE TABLE covers (
    item_id    TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    source_url TEXT NOT NULL,
    media_type TEXT NOT NULL DEFAULT '',
    bytes      BLOB NOT NULL DEFAULT x'',
    fetched_at TEXT NOT NULL
);
