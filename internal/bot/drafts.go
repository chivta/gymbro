package bot

import (
	"context"
	"sync"
	"time"
)

// draftTTL is how long a draft survives without being touched.
const draftTTL = 48 * time.Hour

// draftSweepInterval is how often the sweeper evicts expired drafts.
const draftSweepInterval = time.Hour

// draftKey identifies a draft: the user's message in a chat.
type draftKey struct {
	ChatID int64
	MsgID  int
}

// suspectRef is one suspected misspelling as rendered; buttons refer to it by index.
type suspectRef struct {
	Key  string
	Name string
}

// draft is the in-memory state of one user message being previewed. Its mutex
// serializes handlers working on the same draft (held across API calls); the
// cache's own mutex only protects the map.
type draft struct {
	mu sync.Mutex

	Text   string
	Posted time.Time // message date, supplies the year of a header date
	// Kept holds name keys the user chose to keep as new.
	Kept map[string]bool
	// PreviewIDs are the bot's preview message IDs, in order.
	PreviewIDs []int
	// Suspects is the ordered list of suspected names of the last render.
	Suspects []suspectRef
	// SavedWorkoutID is set after a save and cleared by an edit; 0 means not saved.
	SavedWorkoutID int64
}

type draftEntry struct {
	d       *draft
	touched time.Time
}

// draftCache holds drafts for draftTTL since last touch. Expired entries are
// evicted lazily on get and by sweep.
type draftCache struct {
	mu    sync.Mutex
	items map[draftKey]draftEntry
	ttl   time.Duration
	now   func() time.Time
}

func newDraftCache(ttl time.Duration, now func() time.Time) *draftCache {
	return &draftCache{items: make(map[draftKey]draftEntry), ttl: ttl, now: now}
}

// get returns the live draft for key and refreshes its TTL.
func (c *draftCache) get(key draftKey) (*draft, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return nil, false
	}
	now := c.now()
	if now.Sub(e.touched) > c.ttl {
		delete(c.items, key)
		return nil, false
	}
	e.touched = now
	c.items[key] = e
	return e.d, true
}

// latest returns the key and draft touched most recently in a chat, refreshing
// nothing. ok is false when the chat has no live draft.
func (c *draftCache) latest(chatID int64) (draftKey, *draft, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	var bestKey draftKey
	var best draftEntry
	found := false
	for k, e := range c.items {
		if k.ChatID != chatID || now.Sub(e.touched) > c.ttl {
			continue
		}
		if !found || e.touched.After(best.touched) {
			bestKey, best, found = k, e, true
		}
	}
	return bestKey, best.d, found
}

func (c *draftCache) put(key draftKey, d *draft) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = draftEntry{d: d, touched: c.now()}
}

// sweep evicts every expired draft.
func (c *draftCache) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	for k, e := range c.items {
		if now.Sub(e.touched) > c.ttl {
			delete(c.items, k)
		}
	}
}

func (c *draftCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// runSweeper sweeps every interval until ctx is cancelled.
func (c *draftCache) runSweeper(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sweep()
		}
	}
}
