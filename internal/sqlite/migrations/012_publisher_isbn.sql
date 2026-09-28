-- Publisher and ISBN, filled from a search or ISBN lookup and editable
-- afterward (spec §2.1, step 29). ISBN is stored in normalized ISBN-13 form
-- and is only ever set for books.
ALTER TABLE items ADD COLUMN publisher TEXT NOT NULL DEFAULT '';
ALTER TABLE items ADD COLUMN isbn TEXT NOT NULL DEFAULT '';
