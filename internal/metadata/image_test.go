package metadata

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
)

func TestImageAcceptedAndRefusedTypes(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantErr     bool
	}{
		{"jpeg accepted", "image/jpeg", false},
		{"png accepted", "image/png", false},
		{"webp accepted", "image/webp", false},
		{"gif accepted", "image/gif", false},
		// AVIF is never accepted or kept as a cover (cover-management):
		// Go cannot decode it without cgo, so a fetch that only yields an
		// AVIF image must be treated the same as a failed fetch.
		{"avif refused", "image/avif", true},
		{"svg refused", "image/svg+xml", true},
		{"html refused", "text/html", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/cover.img", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.Write([]byte("pretend-image-bytes"))
			})
			c := newTestClient(t, mux)

			data, mediaType, err := c.Image(ctx, "https://covers.example/cover.img")
			if tt.wantErr {
				if !errors.Is(err, ErrNotAnImage) {
					t.Fatalf("Image() error = %v, want ErrNotAnImage", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Image() error = %v", err)
			}
			if mediaType != tt.contentType {
				t.Fatalf("mediaType = %q, want %q", mediaType, tt.contentType)
			}
			if string(data) != "pretend-image-bytes" {
				t.Fatalf("data = %q, want the response body", data)
			}
		})
	}
}

func TestImageRefusesOversizedBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/big.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(bytes.Repeat([]byte{0}, maxImage+1))
	})
	c := newTestClient(t, mux)

	if _, _, err := c.Image(ctx, "https://covers.example/big.jpg"); err == nil {
		t.Fatal("Image() = nil error, want a size-cap error")
	}
}

func TestImageRefusesInvalidURL(t *testing.T) {
	c := newTestClient(t, http.NewServeMux())
	tests := []string{"", "not a url", "ftp://covers.example/cover.jpg"}
	for _, raw := range tests {
		if _, _, err := c.Image(ctx, raw); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("Image(%q) error = %v, want ErrInvalidURL", raw, err)
		}
	}
}
