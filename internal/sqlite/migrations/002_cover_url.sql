-- Cover image for an item, from Open Library, og:image or a video thumbnail.
-- Not in SPEC §2.1; agreed as a deviation in step 5. '' when absent.
ALTER TABLE items ADD COLUMN cover_url TEXT NOT NULL DEFAULT '';
