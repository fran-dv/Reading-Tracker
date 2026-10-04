package web

import (
	"errors"
	"testing"

	"github.com/fran-dv/reading-tracker/internal/covers"
	"github.com/fran-dv/reading-tracker/internal/library"
)

// coverImage validates every shape Service.SetCover would otherwise
// refuse, before the item itself is created or updated, so a filing or
// a save never writes the item first and only then discovers the
// cover cannot follow.
func TestItemFormCoverImage(t *testing.T) {
	drafts := covers.NewDrafts()
	uploadToken := drafts.Put(covers.Image{Bytes: []byte("jpeg bytes")})
	pickToken := drafts.Put(covers.Image{Bytes: []byte("jpeg bytes"), SourceURL: "https://covers.openlibrary.org/b/id/1.jpg"})
	pickNoSourceToken := drafts.Put(covers.Image{Bytes: []byte("jpeg bytes")})

	tests := []struct {
		name      string
		choice    string
		draft     string
		wantImage bool
		wantErr   bool
	}{
		{"found needs nothing", "found", "", false, false},
		{"removed needs nothing", "removed", "", false, false},
		{"uploaded with a live draft", "uploaded", uploadToken, true, false},
		{"uploaded with no draft at all", "uploaded", "", false, true},
		{"uploaded with an unknown or expired draft", "uploaded", "does-not-exist", false, true},
		{"picked with its edition's source link", "picked", pickToken, true, false},
		{"picked with no source link", "picked", pickNoSourceToken, false, true},
		{"unrecognized choice", "sideways", "", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := itemForm{CoverChoice: tc.choice, CoverDraft: tc.draft}
			img, err := in.coverImage(drafts)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tc.wantErr)
			}
			if (img != nil) != tc.wantImage {
				t.Fatalf("image = %v, want present: %v", img, tc.wantImage)
			}
			if tc.wantErr {
				var verr *library.ValidationError
				if !errors.As(err, &verr) || verr.Field != "cover" {
					t.Fatalf("want a cover field error, got %v", err)
				}
			}
		})
	}
}

// coverChanged is the one check both coverImage's caller and applyCover
// go by, so they can never disagree about whether the cover is really
// being touched.
func TestItemFormCoverChanged(t *testing.T) {
	tests := []struct {
		name    string
		choice  string
		draft   string
		current library.CoverChoice
		want    bool
	}{
		{"same choice, no new draft", "found", "", library.CoverFound, false},
		{"a different choice", "removed", "", library.CoverFound, true},
		{"same choice, a fresh draft replaces the old one", "uploaded", "tok", library.CoverUploaded, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := itemForm{CoverChoice: tc.choice, CoverDraft: tc.draft}
			if got := in.coverChanged(tc.current); got != tc.want {
				t.Fatalf("coverChanged = %v, want %v", got, tc.want)
			}
		})
	}
}
