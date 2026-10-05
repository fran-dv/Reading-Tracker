package covers

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// draftTTL is how long a staged cover is held before it is swept: long
// enough to fill out and submit a form, short enough that an abandoned
// draft, or one orphaned by a restart, costs nothing to forget.
const draftTTL = time.Hour

// Image is a staged cover: the bytes a draft holds, and the edition link
// that goes with a pick (empty for an upload).
type Image struct {
	Bytes     []byte
	SourceURL string
}

type draftEntry struct {
	img Image
	at  time.Time
}

// Drafts holds cover changes staged in an open form until it is filed or
// saved (cover-management: Cover Changes Are Staged in the Form Until
// Filed or Saved). It is server memory, not a database table: losing a
// draft on restart is honest, and the form says so when a filing finds
// its token gone (design.md ADR-10).
type Drafts struct {
	mu   sync.Mutex
	byID map[string]draftEntry
	now  func() time.Time
}

// NewDrafts returns an empty draft store.
func NewDrafts() *Drafts {
	return &Drafts{byID: make(map[string]draftEntry), now: time.Now}
}

// Put stores img under a fresh random token, sweeping every draft older
// than draftTTL first, and returns the token.
func (d *Drafts) Put(img Image) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	for token, entry := range d.byID {
		if now.Sub(entry.at) > draftTTL {
			delete(d.byID, token)
		}
	}
	token := randomToken()
	d.byID[token] = draftEntry{img: img, at: now}
	return token
}

// Get returns the draft under token, if one exists and has not expired.
func (d *Drafts) Get(token string) (Image, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	entry, ok := d.byID[token]
	if !ok || d.now().Sub(entry.at) > draftTTL {
		return Image{}, false
	}
	return entry.img, true
}

// randomToken returns a URL-safe token good enough that nobody guesses
// another owner's in-flight upload. crypto/rand failing means the
// platform itself is broken, which nothing here can recover from.
func randomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("covers: crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
