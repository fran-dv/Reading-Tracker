-- Step 14: sessions can be corrected (spec §2.3). edited_at is when a
-- closed session was last edited, NULL for one never touched.
ALTER TABLE sessions ADD COLUMN edited_at TEXT;
