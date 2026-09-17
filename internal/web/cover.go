package web

import (
	"bytes"
	"context"
	"net/http"
	"strconv"

	"github.com/fran-dv/reading-tracker/internal/covers"
)

// coverCache is what the cover route needs from the covers package. Tests
// substitute a fake.
type coverCache interface {
	Cover(ctx context.Context, itemID, url string) (*covers.Cover, error)
}

// getCover serves an item's cover from this machine. A cover that cannot be
// had answers 404 and the entry keeps its blank plate: an image with an
// empty alt draws nothing over it.
func (h *handler) getCover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	item, err := h.svc.GetItem(ctx, r.PathValue("id"))
	if err != nil {
		h.httpError(w, r, err)
		return
	}
	c, err := h.covers.Cover(ctx, item.ID, item.CoverURL)
	if err != nil {
		// A cover is never worth an error page: it is a picture over a
		// plate that is already drawn.
		h.log.Warn("cover", "item", item.ID, "err", err)
		http.NotFound(w, r)
		return
	}
	if c.Missing() {
		http.NotFound(w, r)
		return
	}

	// The bytes change only when the item's cover link does, so they are
	// held for a year and the tag catches a change before then.
	w.Header().Set("Content-Type", c.MediaType)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+strconv.FormatInt(c.FetchedAt.UnixMilli(), 36)+`"`)
	http.ServeContent(w, r, "", c.FetchedAt, bytes.NewReader(c.Bytes))
}
