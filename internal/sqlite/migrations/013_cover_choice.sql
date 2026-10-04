-- One cover choice per item: the single source of truth for both what
-- cover is drawn and whether it is locked against automatic replacement
-- (cover-management: One Cover Choice Per Item). Existing items default to
-- `found`, the unlocked state that keeps following fresh lookups as today.
ALTER TABLE items ADD COLUMN cover_choice TEXT NOT NULL DEFAULT 'found'
    CHECK (cover_choice IN ('found', 'picked', 'uploaded', 'removed'));
