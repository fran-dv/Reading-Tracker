# Cover Management Specification

## Purpose

Once a book is found, the owner must be able to fix a wrong or missing cover: upload one, pick an edition, and trust that a chosen cover stays chosen. This capability covers how an item's cover is obtained (fetched from a lookup, uploaded, or picked from an edition candidate), its choice and lock state, staging cover changes in the form, revert and removal, image validation and normalization, the edition/cover picker, and export/import of every held cover (SPEC §12 steps 30 Cover control and 31 Edition picker; capture UX items P11–P15).

Domain rules (choice semantics, image validation, orientation, normalization) MUST be implementable and testable with `go test` alone, in packages with no HTTP or template imports (CLAUDE.md), except where a requirement explicitly concerns the HTTP/UI surface. This spec states behavior; it does not prescribe storage layout, route names, or other implementation mechanism, which the design phase owns.

## Requirements

### Requirement: One Cover Choice Per Item

Each item MUST carry exactly one cover choice, with four possible values: `found` (the default — the item shows whatever cover the most recent lookup found, if any), `picked` (the owner selected an edition's cover), `uploaded` (the owner supplied an image file), or `removed` (the owner explicitly chose to show no real cover). This single choice is the one source of truth for both what is drawn and whether the cover is locked against automatic replacement (see "Automatic Lookups Never Override a Locked Choice"); no separate flag or table is needed to express the lock.

#### Scenario: Default choice is found

- GIVEN a newly filed book with a cover found automatically during capture
- WHEN the item is saved
- THEN its cover choice is `found`

#### Scenario: Uploading sets the choice to uploaded

- GIVEN an item with any cover choice
- WHEN the owner's uploaded image is applied to the item
- THEN the item's cover choice becomes `uploaded`

#### Scenario: Picking an edition sets the choice to picked

- GIVEN an item with any cover choice
- WHEN the owner's chosen edition cover is applied to the item
- THEN the item's cover choice becomes `picked`

#### Scenario: Removing sets the choice to removed

- GIVEN an item with any cover choice
- WHEN the owner's removal is applied to the item
- THEN the item's cover choice becomes `removed` and no real cover is held for it

### Requirement: Automatic Lookups Never Override a Locked Choice

The cover choice is "locked" whenever it is anything other than `found`. A lookup (search, ISBN lookup, or "Look it up again") MAY keep discovering an updated found-cover link for the item, but the item's drawn cover MUST continue to reflect its held or removed state whenever the choice is `picked`, `uploaded`, or `removed`; the system MUST NOT replace a held cover or restore a removed one because a fresh lookup produced a link.

#### Scenario: A fresh lookup does not replace an uploaded cover

- GIVEN an item with choice `uploaded`
- WHEN "Look it up again" finds a new candidate cover from a source
- THEN the uploaded cover keeps being shown; the new candidate is not applied automatically

#### Scenario: A fresh lookup does not restore a removed cover

- GIVEN an item with choice `removed`
- WHEN "Look it up again" finds a candidate cover from a source
- THEN the item still shows no real cover; the removal is not undone by the lookup

#### Scenario: An unlocked item still updates from lookups

- GIVEN an item with choice `found`
- WHEN "Look it up again" finds an updated cover link
- THEN the item's drawn cover may change to reflect that new link, since `found` is the only unlocked choice

### Requirement: Removed Stays Removed Indefinitely

Once an item's cover choice is `removed`, it MUST remain `removed` — and the item MUST keep showing no real cover — through any number of subsequent lookups, edits to other fields, or app restarts, until the owner explicitly changes the choice again (by uploading, picking, or reverting to the found cover).

#### Scenario: Removed cover survives repeated re-lookups

- GIVEN an item with choice `removed`
- WHEN "Look it up again" is run multiple times over multiple days
- THEN the item's choice stays `removed` and it never regains a real cover on its own

### Requirement: Revert to the Found Cover

The item form MUST offer a _Use the found cover_ action whenever the item's choice is not already `found`. Applying it MUST set the item's cover choice back to `found`, so the item shows whatever the most recent lookup found (which may be nothing).

#### Scenario: Reverting a picked cover

- GIVEN an item with choice `picked`
- WHEN the owner applies _Use the found cover_
- THEN the item's choice becomes `found` and it shows the latest found cover, if any

#### Scenario: Reverting when no found cover exists

- GIVEN an item with choice `uploaded` and no cover was ever found automatically for it
- WHEN the owner applies _Use the found cover_
- THEN the item's choice becomes `found` and it shows no real cover, since none was ever found

### Requirement: Remove the Cover

The item form MUST offer a _Remove the cover_ action. Applying it MUST set the item's cover choice to `removed` and discard any held cover bytes, so the item shows no real cover and stays that way (see "Removed Stays Removed Indefinitely").

#### Scenario: Removing a found cover

- GIVEN an item with choice `found` and a cover currently showing
- WHEN the owner applies _Remove the cover_
- THEN the item's choice becomes `removed` and it shows no real cover afterward

### Requirement: Cover Changes Are Staged in the Form Until Filed or Saved

Every cover change the owner makes in the item form — uploading, picking an edition, reverting to the found cover, or removing — MUST be held as a staged, unapplied change within that form session, in both capture (where the item does not yet exist) and shelf edit (where it already does). None of these changes MUST take effect on the item's stored cover choice until the form is submitted with its filing or saving action (_File it_ / _Save_). Selecting _Cancel_ on the form MUST discard every staged cover change, leaving the item's stored cover exactly as it was before the form was opened (or, in capture, leaving no item created).

#### Scenario: Upload takes effect only on File it

- GIVEN the owner is filling out capture and uploads an image
- WHEN the owner has not yet selected _File it_
- THEN no item exists yet with that cover; the upload is held only as a staged change in the open form

#### Scenario: Cancel discards a staged cover change

- GIVEN the owner is editing an item on the shelf and stages a new upload, replacing its previous found cover
- WHEN the owner selects _Cancel_ instead of _Save_
- THEN the item's stored cover choice and cover are unchanged; the staged upload is discarded entirely

#### Scenario: Staged removal takes effect on Save

- GIVEN the owner is editing an item and stages a removal of its current cover
- WHEN the owner selects _Save_
- THEN the item's stored cover choice becomes `removed`

#### Scenario: Multiple staged changes in one session resolve to the last one

- GIVEN the owner uploads an image, then within the same form session reverts to the found cover, then selects _Save_
- WHEN the form is submitted
- THEN only the final staged state (`found`) is applied; the intermediate upload never reaches the item's stored state

### Requirement: Confirmation Before Discarding a Staged or Held Uploaded Cover (P13)

Because an uploaded image cannot be re-obtained from any source, any action that would discard it — _Remove the cover_ or _Use the found cover_ — MUST require confirmation first whenever the cover currently in effect (staged in this form session, or already held on the item) is `uploaded`. The confirmation MUST open the confirmation dialog (DESIGN.md "Confirmation dialog") naming the book by its current title and stating plainly that the uploaded image is discarded. The action MUST only take effect (even as a staged change) if the owner confirms.

#### Scenario: Removing an uploaded cover asks first

- GIVEN the form's current cover state (staged or held) is `uploaded`
- WHEN the owner selects _Remove the cover_
- THEN a confirmation dialog opens asking to remove the cover of the current title and stating the uploaded image is discarded, and the removal is staged only if confirmed

#### Scenario: Reverting an uploaded cover to found also asks first

- GIVEN the form's current cover state (staged or held) is `uploaded`
- WHEN the owner selects _Use the found cover_
- THEN the same confirmation dialog opens before the revert is staged, because it discards the uploaded image just as removing does

#### Scenario: Declining the confirmation makes no change

- GIVEN the confirmation dialog is open for discarding an uploaded cover
- WHEN the owner declines (keeps it)
- THEN neither the removal nor the revert is staged, and the uploaded cover remains in effect

### Requirement: Immediate Staged Change with Undo for Non-Uploaded Covers (P13)

When the form's current cover state (staged or held) is `found` or `picked` (never `uploaded`), _Remove the cover_ and _Use the found cover_ MUST take effect immediately as a staged change within the form, without a confirmation dialog, and MUST offer a quiet _Undo_ in the status line that restores the immediately prior staged cover state.

#### Scenario: Removing a found cover acts at once with Undo available

- GIVEN the form's current cover state is `found`
- WHEN the owner selects _Remove the cover_
- THEN the staged state becomes `removed` immediately, with no confirmation dialog, and a quiet _Undo_ appears

#### Scenario: Undo restores the prior staged state

- GIVEN a found cover was just staged for removal and _Undo_ is showing
- WHEN the owner selects _Undo_
- THEN the staged cover state returns to `found`, exactly as before the removal

#### Scenario: Removing a picked cover also acts at once

- GIVEN the form's current cover state is `picked`
- WHEN the owner selects _Remove the cover_
- THEN the staged state becomes `removed` immediately, with no confirmation dialog

### Requirement: Cover Upload Entry Points (P12)

The item form (capture and shelf edit) MUST offer an _Upload a cover_ action that opens the native file chooser with `accept="image/*"` (offering camera or gallery on a phone). The cover plate MUST also accept a dropped image file on desktop, and MUST accept a pasted image while the form holds focus.

#### Scenario: Upload via file chooser

- GIVEN the item form is open
- WHEN the owner selects _Upload a cover_ and picks an image file
- THEN the image begins processing as a staged upload

#### Scenario: Upload via drag-and-drop

- GIVEN the item form is open on a desktop browser
- WHEN the owner drags an image file onto the cover plate and drops it
- THEN the image begins processing as a staged upload

#### Scenario: Upload via paste

- GIVEN the item form has focus
- WHEN the owner pastes an image from the clipboard
- THEN the image begins processing as a staged upload

### Requirement: Local Preview Before Processing (P12)

Immediately after a file is chosen, dropped, or pasted, the system MUST show a local preview of it in the cover plate (an in-memory object URL, never browser storage) with the busy stroke, replaced by the processed image once ready.

#### Scenario: Immediate preview

- GIVEN the owner selects an image file for upload
- WHEN the file is accepted client-side
- THEN the cover plate shows the local preview with the busy stroke before the processed image is ready

#### Scenario: Preview never persists client-side

- GIVEN a local preview is showing
- WHEN the page is inspected for storage use
- THEN no browser storage (`localStorage`, `IndexedDB`) holds the preview or the image bytes

### Requirement: Upload Validation — File Type

The system MUST accept JPEG, PNG, GIF, and WebP images for upload. Any other file type, including HEIC and AVIF, MUST be refused with a plain line ("That file isn't an image this can read. Use JPEG, PNG or WebP.").

#### Scenario: Accepted format

- GIVEN the owner uploads a valid JPEG, PNG, GIF, or WebP file
- WHEN it is validated
- THEN it proceeds to decoding and processing

#### Scenario: HEIC refused

- GIVEN the owner uploads a HEIC image (as phones commonly produce by default)
- WHEN it is validated
- THEN the upload is refused with "That file isn't an image this can read. Use JPEG, PNG or WebP." and no staged cover is created

#### Scenario: Unrecognized file

- GIVEN the owner uploads a non-image file
- WHEN it is validated
- THEN the same plain refusal is shown

### Requirement: AVIF Is Never Accepted or Kept as a Cover

The system MUST NOT accept an uploaded AVIF image, and MUST NOT keep an AVIF image obtained automatically from a lookup or from a linked page's own image (for example, an article's preview image), because AVIF cannot be decoded without cgo. When the only image available for an item is AVIF, the system MUST behave as though no image was found: the upload is refused (see "Upload Validation — File Type"), and an automatic fetch that only yields an AVIF image is treated the same as a failed fetch, so the item's plate shows its plain, non-generated presentation.

#### Scenario: AVIF upload refused

- GIVEN the owner uploads an AVIF file
- WHEN it is validated
- THEN it is refused the same way as any other unreadable format

#### Scenario: Automatically fetched AVIF is not kept

- GIVEN an item's only discoverable image from an automatic lookup is AVIF
- WHEN the fetch runs
- THEN no cover is stored for it, exactly as if the fetch had failed

#### Scenario: Item falls back to its plain plate

- GIVEN an item whose only available image is AVIF and therefore holds no cover
- WHEN its plate renders
- THEN it shows the flat banded-cloth plate for its format, or the generated cover if it is a coverless book

### Requirement: Upload Validation — File Size

The system MUST cap the accepted upload at 15 MB. A file exceeding this cap MUST be refused before decoding, with a plain line ("That image is over 15 MB.").

#### Scenario: Oversized file rejected before decode

- GIVEN the owner uploads a 20 MB image file
- WHEN the upload is received
- THEN it is refused with "That image is over 15 MB." without attempting to decode it

#### Scenario: File within the cap proceeds

- GIVEN the owner uploads a 3 MB image file
- WHEN the upload is received
- THEN it proceeds to decoding

### Requirement: Decompression Bomb Guard

Before fully decoding an uploaded image, the system MUST check its declared dimensions and refuse images whose declared pixel dimensions would produce an unreasonably large decoded buffer, and MUST NOT allocate a full decode buffer for such a file.

#### Scenario: Small file declaring enormous dimensions

- GIVEN an uploaded file within the file-size cap but whose header declares dimensions that would decode to gigabytes of pixel data
- WHEN the dimension check runs
- THEN the file is refused before a full decode is attempted

### Requirement: EXIF Orientation Correction

The system MUST read the EXIF orientation of an uploaded JPEG and rotate/flip the decoded image accordingly, so a phone photo appears upright regardless of how the camera stored it. A missing or unparseable orientation tag MUST be treated as upright (no rotation applied), never as an error that blocks the upload.

#### Scenario: Rotated phone photo

- GIVEN an uploaded JPEG whose EXIF orientation tag indicates a 90-degree rotation
- WHEN it is processed
- THEN the stored cover image is upright, not rotated

#### Scenario: No EXIF orientation present

- GIVEN an uploaded image with no EXIF orientation tag (e.g. PNG, GIF, WebP, or a JPEG without the tag)
- WHEN it is processed
- THEN the image is stored as decoded, with no rotation applied

#### Scenario: Malformed EXIF data

- GIVEN an uploaded JPEG with a malformed or unparseable EXIF orientation segment
- WHEN it is processed
- THEN the image is treated as upright rather than the upload being refused

### Requirement: Cover Normalization on Write

Every held cover, regardless of how it was obtained (found and fetched, picked, or uploaded), MUST be normalized on write to one bounded maximum size (approximately 600px on the long side) and re-encoded as a compact image format, so the export and the served page never carry multi-megabyte images. An already-small, already-compact cover MAY be stored unchanged rather than needlessly re-processed, so long as the result meets the same bound.

#### Scenario: Large fetched cover normalized

- GIVEN a fetched cover image larger than the bounded maximum
- WHEN it is stored
- THEN it is resized to the bounded maximum

#### Scenario: Uploaded cover normalized identically

- GIVEN an owner-uploaded image larger than the bounded maximum
- WHEN it is stored
- THEN the same normalization applies, regardless of its upload origin

#### Scenario: Already-small cover is not upscaled

- GIVEN a cover already at or below the bounded maximum
- WHEN it is normalized
- THEN its dimensions are not upscaled

#### Scenario: Re-normalizing an already-normalized cover is a no-op

- GIVEN a cover that has already been normalized to the bounded size and format
- WHEN it is normalized again (for example, on a repeated startup pass or re-import)
- THEN the stored bytes are unchanged, so repeated normalization causes no generational quality loss

### Requirement: One-Time Normalization Pass for Existing Covers

At startup, the system MUST run one pass that normalizes every existing held cover stored before this change to the same bounded size used for new covers. This pass MUST be idempotent: running it again on already-normalized covers MUST NOT re-process them.

#### Scenario: First startup after upgrade

- GIVEN existing covers stored before this change, none normalized
- WHEN the app starts for the first time after this change
- THEN every existing cover is normalized to the bounded size

#### Scenario: Idempotent on subsequent startups

- GIVEN covers already normalized by a previous startup pass
- WHEN the app restarts
- THEN the pass does not re-process those covers

#### Scenario: A cover that fails to normalize is left as-is

- GIVEN an existing held cover whose bytes cannot be normalized (for example, corrupted data)
- WHEN the startup pass runs
- THEN that cover is left unchanged and the pass continues with the rest, without stopping startup

### Requirement: Cover Export Includes Choice and Every Held Cover

The JSON export MUST include, for every item, its cover choice, and MUST include every held cover (for choices `picked` and `uploaded`, and the found cover cache when appropriate) with enough information to restore it exactly: which item it belongs to, its origin link (when it has one), its media type, when it was fetched, and its image bytes. A remembered lookup failure (no bytes held) MUST NOT be exported as a cover entry.

#### Scenario: Held cover included in export

- GIVEN an item with a stored cover of any choice that holds bytes
- WHEN the library is exported
- THEN the export includes that cover's item reference, origin link (if any), media type, fetched time, and image bytes

#### Scenario: Cover choice included per item

- GIVEN an item with cover choice `removed`
- WHEN the library is exported
- THEN the export records that item's cover choice as `removed`

#### Scenario: Remembered failure excluded from export

- GIVEN an item whose automatic lookup failed and is remembered (no bytes held)
- WHEN the library is exported
- THEN no cover entry is written for that item

### Requirement: Export Memory Stays Flat Regardless of Cover Count

Producing the export MUST NOT require memory proportional to the total size of all held covers combined; the export process's memory use MUST stay flat (bounded, not growing) whether the library holds ten covers or many thousands.

#### Scenario: Export of a library with many large covers

- GIVEN a library holding a very large number of covers, each near the normalized size bound
- WHEN the library is exported
- THEN the export completes without the process's memory usage growing in proportion to the total cover count or total cover byte size

### Requirement: Cover Import Restore

Import MUST restore every exported item's cover choice and every exported cover's held bytes, so an imported library shows its covers and lock states without needing to refetch anything from either metadata source.

#### Scenario: Round-trip of an uploaded, locked cover

- GIVEN an export containing an item with cover choice `uploaded`
- WHEN that export is imported into an empty database
- THEN the item is restored with its uploaded cover already held and cover choice `uploaded`

#### Scenario: Round-trip of a found, unlocked cover

- GIVEN an export containing an item with cover choice `found`
- WHEN imported
- THEN the item is restored with cover choice `found`, following lookups as normal afterward

#### Scenario: Round-trip of a removed cover

- GIVEN an export containing an item with cover choice `removed`
- WHEN imported
- THEN the item is restored with cover choice `removed` and no real cover, and stays removed through subsequent lookups

#### Scenario: Import inserts covers within the item transaction

- GIVEN an export with items and their covers
- WHEN imported
- THEN each item and its cover(s) are inserted as one atomic unit, so a partial failure leaves no orphaned cover row

### Requirement: Import Refuses an Export From a Newer Version

Import MUST recognize the export's format version and MUST refuse, with a clear plain-language error, an export produced by a newer version of the app than the running one supports — rather than silently dropping fields it does not recognize (such as cover choices or covers) or partially importing. An export from an equal or older recognized version MUST import successfully, applying sensible defaults for fields the older format did not have (for example, treating every item as cover choice `found` when the export predates cover choices entirely).

#### Scenario: Newer export refused

- GIVEN an export file produced by a version of the app newer than the one running import
- WHEN the import is attempted
- THEN it is refused with a clear error naming the version mismatch, and nothing is imported

#### Scenario: Older export still imports with defaults

- GIVEN an export file from before cover choices existed
- WHEN it is imported
- THEN every item is restored with cover choice `found`, and import succeeds

#### Scenario: Same-version export imports cleanly

- GIVEN an export produced by the same version as the running app
- WHEN it is imported
- THEN it imports completely, including every cover choice and held cover

### Requirement: Edition Candidate Sourcing (Step 31)

From a picked work, the system MUST gather candidate editions from Open Library editions of that work and from Google Books volumes matching the same title and author, reduce them to candidates that carry a cover, and deduplicate them using the same merge rule as book search (shared ISBN-13, else folded main title + first-author surname).

#### Scenario: Editions from both sources with covers

- GIVEN a picked work with multiple Open Library editions and matching Google Books volumes, several carrying covers
- WHEN edition candidates are gathered
- THEN the candidate list includes editions from both sources that have a cover, deduplicated by the shared merge rule

#### Scenario: Editions without a cover excluded

- GIVEN an edition record with no cover image available from its source
- WHEN candidates are reduced
- THEN that edition is excluded from the picker's candidate list

### Requirement: Choosing an Edition Stages a Picked Cover and Fills Fields (P14)

Choosing a candidate in the edition picker MUST stage the item's cover as `picked` with that edition's cover (applied on File it / Save, per "Cover Changes Are Staged in the Form"), and MUST fill that edition's ISBN, publisher, and page count as unconfirmed (pencil) values on the item, following the Pencil Until Confirmed Rule, until the owner confirms them by touching the field or saving.

#### Scenario: Choosing an edition stages the pick and fills fields

- GIVEN the edition picker is open with candidates
- WHEN the owner chooses one
- THEN the cover choice is staged as `picked` with that edition's cover, and the edition's ISBN, publisher, and page count appear as pencil (unconfirmed) values

#### Scenario: Page count from a chosen edition affects sizing

- GIVEN the owner chooses an edition with a different page count than the item currently holds
- WHEN the choice is confirmed and the form is saved
- THEN the item's `size_value` reflects the chosen edition's page count, which subsequently participates in pace, time-left, and campaign-average calculations (SPEC §7, §8.1)

### Requirement: Edition Candidate Labels (P14)

Each edition candidate MUST be labelled with its publisher, year, language, and page count (where available), and the item's current cover, if one matches a candidate, MUST be marked as selected in the list.

#### Scenario: Candidate metadata shown

- GIVEN an edition candidate with a known publisher, year, language, and page count
- WHEN it renders in the picker
- THEN all four are shown in its label

#### Scenario: Current cover marked selected

- GIVEN the item's current cover matches one of the rendered candidates
- WHEN the picker opens
- THEN that candidate is visually marked as the current selection

### Requirement: Phone Layout for Cover Controls (P15)

At phone width, edition candidates MUST render as a horizontally scrolling row with snap points and touch targets of at least 2.75rem, and the upload and revert actions MUST sit full-width beneath the cover plate. Neither MUST cause the page itself to scroll sideways.

#### Scenario: Edition candidates on a phone

- GIVEN the item form is viewed at phone width
- WHEN the edition picker is open
- THEN candidates appear in a horizontally scrolling, snap-pointed row with targets at least 2.75rem, and the page does not scroll sideways

#### Scenario: Upload and revert actions on a phone

- GIVEN the item form is viewed at phone width with a cover present
- WHEN the form renders
- THEN upload and revert actions are full-width controls beneath the cover plate

### Requirement: Screen Reader and Focus for Cover Actions (P11)

Every informative cover image MUST carry `alt="Cover of <title>"`. Focus MUST land predictably after each cover action within the form: after an upload's local preview appears, after a revert or removal is staged (or its confirmation dialog closes), and after choosing an edition candidate.

#### Scenario: Alt text on a filed cover

- GIVEN an item with a stored cover
- WHEN its cover image renders anywhere in the app
- THEN it carries `alt="Cover of <title>"`

#### Scenario: Focus after upload

- GIVEN the owner uploads a cover successfully
- WHEN its local preview is ready
- THEN focus lands on a predictable, documented element (not lost to the document body)

#### Scenario: Focus after removal confirmation

- GIVEN the owner confirms discarding an uploaded cover in the confirmation dialog
- WHEN the dialog closes
- THEN focus returns to a predictable element in the item form
