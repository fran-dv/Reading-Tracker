package covers

import (
	"testing"
	"time"
)

// fixedClock returns a now func that can be advanced by the test.
func fixedClock(start time.Time) (func() time.Time, *time.Time) {
	t := start
	return func() time.Time { return t }, &t
}

func TestDraftsPutGetRoundTrip(t *testing.T) {
	d := NewDrafts()
	want := Image{Bytes: []byte("jpeg bytes"), SourceURL: "https://covers.openlibrary.org/b/id/1.jpg"}
	token := d.Put(want)
	if token == "" {
		t.Fatal("Put returned an empty token")
	}
	got, ok := d.Get(token)
	if !ok {
		t.Fatal("Get reported the token missing right after Put")
	}
	if string(got.Bytes) != string(want.Bytes) || got.SourceURL != want.SourceURL {
		t.Fatalf("Get = %+v, want %+v", got, want)
	}
}

func TestDraftsGetUnknownToken(t *testing.T) {
	d := NewDrafts()
	if _, ok := d.Get("does-not-exist"); ok {
		t.Fatal("Get reported an unknown token as present")
	}
}

func TestDraftsExpiryExactlyAtTTL(t *testing.T) {
	now, clock := fixedClock(time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	d := NewDrafts()
	d.now = now
	token := d.Put(Image{Bytes: []byte("x")})

	*clock = clock.Add(draftTTL) // exactly at the TTL: "older than" has not yet fired
	if _, ok := d.Get(token); !ok {
		t.Fatal("Get expired a draft exactly at the TTL, not strictly past it")
	}

	*clock = clock.Add(time.Nanosecond) // one tick past: now strictly older than the TTL
	if _, ok := d.Get(token); ok {
		t.Fatal("Get kept a draft one tick past the TTL")
	}
}

func TestDraftsSweepOnPut(t *testing.T) {
	now, clock := fixedClock(time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	d := NewDrafts()
	d.now = now
	stale := d.Put(Image{Bytes: []byte("stale")})

	*clock = clock.Add(draftTTL + time.Second) // stale is now past its TTL
	fresh := d.Put(Image{Bytes: []byte("fresh")})

	if _, ok := d.Get(fresh); !ok {
		t.Fatal("the fresh draft should still be there")
	}
	d.mu.Lock()
	_, stillThere := d.byID[stale]
	d.mu.Unlock()
	if stillThere {
		t.Fatal("Put did not sweep a draft already past the TTL")
	}
}
