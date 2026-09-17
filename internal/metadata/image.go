package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"strings"
)

// maxImage caps the bytes kept for one cover. Open Library's large covers
// and YouTube thumbnails sit well under it; anything bigger is refused
// rather than truncated, because half an image is not an image.
const maxImage = 2 << 20

// ErrNotAnImage is returned when a URL answers with something that is not
// one of the image types a browser can draw.
var ErrNotAnImage = errors.New("not an image")

// imageTypes are the types worth keeping. SVG is left out: a cover is a
// photograph, and an SVG from a stranger's site is a script.
var imageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/avif": true,
	"image/gif":  true,
}

// Image fetches a cover. It returns the bytes and the media type to serve
// them as. Like every lookup here it is best effort: callers draw the
// item's blank plate when it fails.
func (c *Client) Image(ctx context.Context, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, "", ErrInvalidURL
	}
	resp, err := c.get(ctx, u.String())
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if !imageTypes[mediaType] {
		return nil, "", fmt.Errorf("%s: %w", mediaType, ErrNotAnImage)
	}
	// One byte past the cap tells an oversized image from an exact fit.
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImage+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxImage {
		return nil, "", fmt.Errorf("cover is over %d bytes", maxImage)
	}
	if len(data) == 0 {
		return nil, "", ErrNotAnImage
	}
	return data, mediaType, nil
}
