package library

import (
	"context"
	"time"
)

// CoverChoice is the one source of truth for what cover an item shows and
// whether that cover is locked against automatic replacement. found is the
// only choice a lookup may still update; the other three are owner
// decisions that a fresh lookup must never undo (cover-management: One
// Cover Choice Per Item, Automatic Lookups Never Override a Locked Choice).
type CoverChoice string

const (
	// CoverFound is the default: the item shows whatever cover.cover_url
	// resolves to, if anything, and keeps following fresh lookups.
	CoverFound CoverChoice = "found"
	// CoverPicked is an edition cover the owner chose from the picker.
	CoverPicked CoverChoice = "picked"
	// CoverUploaded is an image file the owner supplied.
	CoverUploaded CoverChoice = "uploaded"
	// CoverRemoved means the owner explicitly chose to show no real cover.
	CoverRemoved CoverChoice = "removed"
)

// CoverImage is a held cover's bytes, independent of how they were
// obtained: a picked edition or an owner's upload. SourceURL is the
// edition link for a pick, and empty for an upload.
type CoverImage struct {
	ItemID    string    `json:"item_id"`
	SourceURL string    `json:"source_url"`
	MediaType string    `json:"media_type"`
	FetchedAt time.Time `json:"fetched_at"`
	Bytes     []byte    `json:"bytes"`
}

// SetCover applies the owner's cover decision — a pick, an upload, a
// revert to the found cover, or a removal — in one transaction, bumping
// updated_at (cover-management: One Cover Choice Per Item, all four
// scenarios). picked and uploaded require img with image bytes; picked
// also requires img.SourceURL, the edition link kept for a picked cover
// (ADR-7). found and removed discard any held cover, even when img is nil.
func (s *Service) SetCover(ctx context.Context, id string, choice CoverChoice, img *CoverImage) (*Item, error) {
	return s.transition(ctx, id, func(r Repo, it *Item) error {
		switch choice {
		case CoverPicked:
			if img == nil || len(img.Bytes) == 0 || img.SourceURL == "" {
				return &ValidationError{"cover", "a picked cover needs its image and the edition's link"}
			}
		case CoverUploaded:
			if img == nil || len(img.Bytes) == 0 {
				return &ValidationError{"cover", "an uploaded cover needs its image"}
			}
		case CoverFound, CoverRemoved:
			// Nothing to hold; any existing cover is discarded below.
		default:
			return &ValidationError{"cover_choice", "unknown value"}
		}

		it.CoverChoice = choice
		if choice != CoverPicked && choice != CoverUploaded {
			return r.DeleteCoverImage(it.ID)
		}
		img.ItemID = it.ID
		img.FetchedAt = s.now()
		return r.PutCoverImage(img)
	})
}
