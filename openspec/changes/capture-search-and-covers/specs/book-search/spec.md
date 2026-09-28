# Book Search Specification

## Purpose

Capture's book lookup finds the book the owner is holding in one search, whether by words or by ISBN, without quietly hiding matches that exist or pretending a source answered when it did not (SPEC §0, third principle). This capability covers the Open Library relevance query, the merged Google Books source, source-degradation and disclosure, ISBN entry and validation, and the result-list behaviors that make search fast and honest (capture UX items P1–P11, SPEC §12 step 29).

All requirements below are scoped to step 29 unless noted otherwise. Pure logic (merge/dedup, ISBN normalization and checksum) MUST be implementable and testable with `go test` alone, with no HTTP, template, or Datastar imports (CLAUDE.md).

## Requirements

### Requirement: Relevance-Based Open Library Search

The system MUST query Open Library's search endpoint using the relevance parameter (`q=`) instead of a title-only field match, and MUST request `key`, `title`, `author_name`, `first_publish_year`, `number_of_pages_median`, `cover_i`, `isbn`, and `publisher` for each result.

The system MUST request more results per query than the previous fixed cap of 5, and MUST retrieve the total number of matches Open Library reports, independent of how many are actually fetched.

#### Scenario: Book findable only by subtitle or alternate title

- GIVEN a book that openlibrary.org's own search finds by a subtitle, alternate title, or series name, but not by its canonical title alone
- WHEN the owner types those words into capture's search field
- THEN the book appears among the results

#### Scenario: A relevant result ranked below the old five-result cap

- GIVEN a query whose correct match is Open Library's 6th–20th ranked result by relevance
- WHEN the query is searched with the new limit
- THEN the match appears in the returned results without the owner needing to narrow the query further

### Requirement: Honest "More Not Shown" Count

When Open Library reports more total matches than the number of results fetched and shown, the system MUST tell the owner plainly how many are not shown (for example, "12 more not shown. Add a word to narrow it."), so "keep typing" is never confused with "there is nothing more" (SPEC §0).

#### Scenario: More matches exist than are shown

- GIVEN a query for which Open Library reports 20 total matches and the system fetches and shows 8
- WHEN the results render
- THEN the list states that 12 more exist and are not shown, with guidance to narrow the query

#### Scenario: Every match is already shown

- GIVEN a query for which Open Library reports 3 total matches and all 3 are fetched and shown
- WHEN the results render
- THEN no "more not shown" line appears

### Requirement: Google Books Merged Source

The system MUST query Google Books' keyless volumes endpoint alongside Open Library for the same search words (or the same ISBN, for the ISBN field), and MUST run both queries concurrently under one short shared deadline (approximately 4 seconds), so capture never waits on a slow source.

#### Scenario: Both sources answer within the deadline

- GIVEN a search query
- WHEN both Open Library and Google Books respond before the deadline
- THEN results from both sources are merged into one result list

#### Scenario: One source exceeds the deadline

- GIVEN a search query
- WHEN Google Books has not responded by the shared deadline but Open Library has
- THEN the system returns Open Library's results without waiting further for Google Books

### Requirement: Result Merge and Deduplication

The system MUST merge results from Open Library and Google Books into one deduplicated list using a pure, table-tested function with no HTTP or template imports. Two results MUST be treated as the same edition/work when they share an ISBN-13; otherwise they MUST be treated as the same when their folded main title (the text before the first colon, lower-cased, letters and digits only) and folded first-author surname match. Open Library results MUST lead the merged order; Google-only results (no Open Library match) MUST follow. When two results are merged into one, the kept result MUST be filled with any field the Open Library side lacks but the Google Books side has (cover, page count, publisher, ISBN), so a merge never discards information one side alone provided.

#### Scenario: Same book found by both sources via shared ISBN

- GIVEN an Open Library result and a Google Books result that share an ISBN-13
- WHEN the results are merged
- THEN they collapse into a single merged result, and it appears in Open-Library-leading order

#### Scenario: Same book found by both sources with no shared ISBN

- GIVEN an Open Library result and a Google Books result with no ISBN in common but the same folded main title and first-author surname
- WHEN the results are merged
- THEN they collapse into a single merged result

#### Scenario: Merge fills in a field the leading result lacks

- GIVEN an Open Library result with no cover and a matching Google Books result that has one
- WHEN the results are merged
- THEN the merged result carries the Google Books cover, while keeping Open Library's lead position and its other fields

#### Scenario: Distinct books with similar titles are not merged

- GIVEN two results with different first-author surnames and no shared ISBN
- WHEN the results are merged
- THEN they remain as two separate results

#### Scenario: Subtitle differences do not prevent a merge

- GIVEN an Open Library result titled "Example: A Long Subtitle" and a Google Books result titled "Example" by the same first author, with no shared ISBN
- WHEN the results are merged
- THEN they collapse into a single merged result, because their folded main title (before the colon) and first-author surname match

#### Scenario: Google-only result

- GIVEN a Google Books result with no matching Open Library result by ISBN-13 or by folded main title and first-author surname
- WHEN the results are merged
- THEN it appears in the merged list after every Open-Library-led result

### Requirement: Graceful Single-Source Degradation

If Google Books fails, times out, or is refused (rate limit or quota response), the system MUST degrade to returning Open Library's results alone rather than failing the search. If Open Library fails or times out, the system MUST still return Google Books' results alone when available. If both sources fail, the search MUST return an empty result set with the failure disclosed (see "Plain Failure States with Retry"); capture MUST remain usable for manual entry regardless.

#### Scenario: Google Books quota refusal (429/403)

- GIVEN Google Books responds with a rate-limit or quota-refusal status
- WHEN the merged search runs
- THEN the system returns Open Library's results alone and does not surface an error that blocks capture

#### Scenario: Google Books network timeout

- GIVEN Google Books does not respond before the shared deadline
- WHEN the merged search runs
- THEN the system returns Open Library's results alone

#### Scenario: Open Library failure with Google Books available

- GIVEN Open Library returns an error or times out and Google Books responds successfully
- WHEN the merged search runs
- THEN the system returns Google Books' results alone

#### Scenario: Both sources fail

- GIVEN both Open Library and Google Books fail or time out
- WHEN the merged search runs
- THEN the system returns no results and reports that neither source answered

### Requirement: Source Status Disclosure (P2)

The system MUST record and expose which of the two sources answered a given search, so the UI can tell the owner in one line when a source did not answer (for example, "Only Open Library answered."). When results are empty, the empty state MUST name both sources.

#### Scenario: One source silent

- GIVEN Google Books failed or timed out for a search and Open Library answered
- WHEN the results render
- THEN one line states that only Open Library answered

#### Scenario: Both sources answered

- GIVEN both sources answered
- WHEN the results render
- THEN no "only one source" disclosure line is shown

#### Scenario: Empty results name both sources

- GIVEN a search that returns zero results because neither source has a match
- WHEN the empty state renders
- THEN it names both Open Library and Google Books, not a single generic source

### Requirement: ISBN Field Entry and Validation (P9)

Capture MUST offer a field dedicated to ISBN entry, separate from the words search field, for book-format items. The field MUST accept ISBN-10 or ISBN-13 written with or without hyphens or spaces, MUST use a numeric input mode on phones, and MUST validate the checksum live as the owner types, showing a plain error ("Those digits don't make an ISBN. Check them against the book.") when the digits are the right length but fail the checksum. When the entered value is a complete, checksum-valid ISBN, the system MUST look it up at once, without the debounce used for the words field.

#### Scenario: Valid ISBN-13 with hyphens

- GIVEN the owner types an ISBN-13 with hyphens (e.g. "978-0-13-468599-1")
- WHEN the value completes and its checksum validates
- THEN the system normalizes it and looks it up immediately, with no debounce delay

#### Scenario: Valid ISBN-10 without hyphens

- GIVEN the owner types a bare 10-digit ISBN-10 whose checksum is valid
- WHEN the value completes
- THEN the system converts it to its ISBN-13 form for lookup and storage

#### Scenario: Checksum failure

- GIVEN the owner types a 10- or 13-digit sequence whose checksum digit does not match
- WHEN validation runs
- THEN the field shows "Those digits don't make an ISBN. Check them against the book." and no lookup is issued

#### Scenario: Incomplete entry

- GIVEN the owner has typed fewer digits than a complete ISBN
- WHEN validation runs
- THEN no error is shown and no lookup is issued yet

### Requirement: ISBN Normalization (Pure Function)

The system MUST provide a pure function, testable with `go test` alone and free of HTTP or template imports, that normalizes an ISBN-10 or ISBN-13 string (with any mix of hyphens and spaces, and an ISBN-10 that may end in the check character `X`) into its canonical ISBN-13 form with hyphens and spaces removed, converting ISBN-10 to ISBN-13 using the standard prefix and recomputed check digit. This normalized form is the one persisted on the item.

#### Scenario: ISBN-10 to ISBN-13 conversion

- GIVEN a valid ISBN-10 string
- WHEN it is normalized
- THEN the result is its equivalent 13-digit ISBN-13 with the correct recomputed check digit

#### Scenario: ISBN-10 with an X check character

- GIVEN a valid ISBN-10 string whose check character is `X`
- WHEN it is normalized
- THEN it converts correctly to its ISBN-13 form

#### Scenario: Hyphens and spaces stripped

- GIVEN an ISBN-13 string containing hyphens and spaces in arbitrary positions
- WHEN it is normalized
- THEN the result contains only the 13 digits, no separators

#### Scenario: Checksum validation rejects invalid input

- GIVEN a 10- or 13-character digit string with an incorrect check digit
- WHEN checksum validation runs
- THEN the function reports it as invalid and performs no normalization

### Requirement: ISBN Lookup Across Sources

A complete, valid ISBN MUST be looked up directly against both Open Library and Google Books (using each source's ISBN-specific query), applying the same merge, deduplication, and degradation rules as a words search.

#### Scenario: ISBN found on one source only

- GIVEN a valid ISBN present in Open Library's catalog but not Google Books'
- WHEN the ISBN lookup runs
- THEN the Open Library edition is returned and used

#### Scenario: ISBN with no match on either source

- GIVEN a valid, checksum-correct ISBN that neither source recognizes
- WHEN the lookup completes
- THEN the field keeps the entered ISBN and the "none of these" manual-entry path applies (see "None of These Path"), moving focus to Title

### Requirement: Barcode Scan Availability (P10)

Capture MUST offer a _Scan_ action beside the ISBN field that reads a book's back-cover barcode using the browser's camera and barcode-detection capability, but only when the browser exposes barcode detection AND the page is loaded in a secure context (HTTPS or `localhost`). Outside that condition, the action MUST be absent from the page rather than rendered disabled or broken.

#### Scenario: Secure context with barcode detection supported

- GIVEN the app is served over HTTPS (or accessed via `localhost`) and the browser supports barcode detection
- WHEN capture's ISBN field renders
- THEN the _Scan_ action is present and, when used, fills the ISBN field with the decoded value and triggers its lookup

#### Scenario: Insecure context (plain HTTP on the local network)

- GIVEN the app is reached over plain HTTP from a phone on the local network (not `localhost`)
- WHEN capture's ISBN field renders
- THEN the _Scan_ action is not present anywhere on the page

#### Scenario: Browser without barcode detection

- GIVEN a secure context but a browser that does not implement barcode detection
- WHEN capture's ISBN field renders
- THEN the _Scan_ action is not present

### Requirement: Publisher and ISBN Item Fields

`library.Item` MUST gain two new optional fields: `publisher` (string, any format) and `isbn` (string, book format only, stored in normalized ISBN-13 form). Both MUST be filled automatically from a search or ISBN lookup result when the source provides them, MUST remain editable by the owner in the item form afterward, and MUST be included in JSON export and restored on import (SPEC §10). The system MUST refuse a non-empty `isbn` on any format other than `book`.

#### Scenario: Publisher and ISBN filled from a picked result

- GIVEN a search result carries a publisher and an ISBN
- WHEN the owner picks that result
- THEN the item's `publisher` and `isbn` fields are filled with those values, editable afterward

#### Scenario: Publisher stays empty for a manual entry

- GIVEN the owner uses the "none of these" manual-entry path
- WHEN the item is filed
- THEN `publisher` and `isbn` are empty, and filing is not blocked by their absence

#### Scenario: ISBN refused on a non-book format

- GIVEN an item with `format` other than `book`
- WHEN an `isbn` value is submitted for it
- THEN the system refuses the value with a plain reason and does not persist it

### Requirement: Keyboard Navigation Through Results (P1)

The result list MUST behave as a combobox/listbox pair tied to the search field: Arrow Up/Down MUST move a visible active-result indicator through the list, Enter MUST pick the active result, and the field MUST expose `aria-activedescendant` pointing at the active result's id. This reuses the existing picker's keyboard pattern (DESIGN.md "Picker").

#### Scenario: Arrow-key navigation and Enter to pick

- GIVEN a rendered result list with focus in the search field
- WHEN the owner presses Arrow Down twice, then Enter
- THEN the third result (if present) is picked, filling its fields as confirmed

#### Scenario: No mouse needed

- GIVEN a rendered result list
- WHEN the owner never touches a pointing device
- THEN a book can be found and picked entirely via keyboard, from typing the query through filing the item

### Requirement: Matched Word Highlighting (P6)

When a relevance search matches a result on a subtitle, alternate title, or series rather than the canonical title alone, the system MUST visually set apart the matched words within the displayed title, using DESIGN's existing ink rules (no new colour introduced).

#### Scenario: Match on subtitle

- GIVEN a result whose canonical title does not contain the searched words, but whose subtitle does
- WHEN the result renders
- THEN the words that matched are set apart within the shown title text by weight or ink, distinguishable from the rest of the title

#### Scenario: Match on canonical title only

- GIVEN a result whose canonical title itself contains all the searched words
- WHEN the result renders
- THEN the matched words are set apart in the same way, or the whole title is treated as matched — the mechanism MUST NOT introduce a new ink colour beyond DESIGN's existing rules

### Requirement: Publisher and Year on Each Result (P3)

Each result MUST show its publisher and first-publish year (when available) alongside author and page count, so the owner can tell editions apart before picking.

An Open Library work-level result MUST take its publisher, its edition-precise ISBN, and, when the work itself reports none, its year from the one best-matching edition Open Library's search returns for that work (design.md ADR-1, "Publisher, ISBN and year on an Open Library work result"), never from the work's own (potentially large) list of every edition's publisher.

#### Scenario: Result with publisher and year available

- GIVEN a merged result that carries a publisher name and a first-publish year
- WHEN it renders in the result list
- THEN both are shown in the result's metadata line

#### Scenario: Result missing publisher

- GIVEN a merged result with no publisher information from either source
- WHEN it renders
- THEN the metadata line omits the publisher without showing a placeholder or error

#### Scenario: Work-level result with a best-matching edition

- GIVEN an Open Library work-level result for which Open Library's search reports a best-matching edition carrying a publisher
- WHEN the result is prepared
- THEN that edition's publisher is shown, regardless of how many publishers the work's editions carry in total

#### Scenario: Work-level result with no edition on offer

- GIVEN an Open Library work-level result for which Open Library's search reports no edition
- WHEN the result is prepared
- THEN no publisher is shown for it, since none can be attributed to a specific edition

### Requirement: Pending State Without Flicker (P4)

While a refined query is in flight, the system MUST keep the previous results visible (in pencil, per DESIGN's Pencil Until Confirmed styling) and show a pending hint in the field's hint slot ("Looking it up") using the system's existing pending stroke idiom. The result list MUST NOT blank or reflow between keystrokes.

#### Scenario: Typing a second keyword while results are showing

- GIVEN a visible result list from a prior query
- WHEN the owner types another character, triggering a new search
- THEN the existing results remain visible (in pencil) with the "Looking it up" pending hint shown, and the list does not blank or collapse before the new results arrive

### Requirement: Superseded Request Cancellation (P5)

Each new keystroke that triggers a new search MUST cancel the previous in-flight search request. The server MUST drop cancelled work rather than completing it needlessly. A response that no longer matches the field's current query value MUST never be drawn into the result list.

#### Scenario: Fast typing cancels stale requests

- GIVEN the owner types quickly, triggering several searches in succession
- WHEN an earlier search's response arrives after a later keystroke has already changed the query
- THEN that stale response is discarded and never rendered

#### Scenario: Server-side cancellation

- GIVEN a search request is in flight
- WHEN the client cancels it because the query changed
- THEN the server stops processing that request rather than completing an unused search

### Requirement: "None of These" Path (P7)

The result list MUST include a final row that, when chosen, closes the list, keeps the typed words as a confirmed title (inked white, not pencil), sets the item's format to `book`, and moves focus to the Author field. When the ISBN field's lookup finds no match, choosing to proceed keeps the entered ISBN and moves focus to the Title field instead. In both cases, manual entry MUST start from the fields already filled by what the owner typed, never from a blank form.

#### Scenario: "None of these" from a words search

- GIVEN a rendered result list for a words search
- WHEN the owner selects the "None of these" row
- THEN the list closes, the typed words become the confirmed title, format is set to book, and focus moves to Author

#### Scenario: ISBN lookup finds nothing

- GIVEN a valid, checksum-correct ISBN that returns no match from either source
- WHEN the owner proceeds past that empty result
- THEN the entered ISBN is kept on the item, and focus moves to Title

### Requirement: Plain Failure States with Retry (P8)

Each source's individual failure MUST be stated per the Source Status Disclosure requirement. When neither source answers, the system MUST show one rubric line ("Neither Open Library nor Google Books answered. Type the details in.") with a quiet _Try again_ action. Filing an item MUST NOT be blocked by a lookup failure of any kind.

#### Scenario: Both sources fail

- GIVEN both Open Library and Google Books fail or time out for a search
- WHEN the results render
- THEN the rubric failure line and a quiet _Try again_ action are shown, and the owner can still fill in the details manually and file the item

#### Scenario: Retry after failure

- GIVEN the failure state is shown
- WHEN the owner selects _Try again_
- THEN the search re-runs against both sources with the same query

### Requirement: Screen Reader and Focus for Search (P11)

A polite live region MUST announce the result count, the "more not shown" count when present, and which sources answered, whenever results update. Every result's cover image MUST carry an `alt` attribute naming the book ("Cover of <title>"). Focus MUST land predictably: on the Why field after picking a result, on Author after "none of these" from a words search, and on Title after an unmatched ISBN.

#### Scenario: Live region announces result update

- GIVEN a search completes
- WHEN the results render
- THEN a polite live region announces the result count, any "more not shown" count, and which sources answered

#### Scenario: Cover alt text

- GIVEN a result with a cover thumbnail
- WHEN it renders
- THEN the image carries `alt="Cover of <title>"`

#### Scenario: Focus after picking a result

- GIVEN the owner picks a result via keyboard or pointer
- WHEN the pick completes
- THEN focus moves to the Why field

### Requirement: Thumbnail URL Upgrade

Google Books thumbnail URLs returned as `http://` MUST be rewritten to `https://` before being used in a result, since results are drawn client-side from the source's own URL until a cover is filed.

#### Scenario: Google Books returns an http thumbnail link

- GIVEN a Google Books result whose thumbnail URL scheme is `http://`
- WHEN the result is prepared for display
- THEN the thumbnail URL is rewritten to `https://` before rendering
