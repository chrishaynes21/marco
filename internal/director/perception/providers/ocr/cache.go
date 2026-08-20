package ocr

import (
	"fmt"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// defaultFreshness is how long an OCR pass may be reused.
//
// Short on purpose. The cache exists so that `director ocr` followed immediately by
// `director fusion` does not capture the screen twice; it is NOT a way to avoid
// looking. A desktop changes continuously, and reusing text after the window it came
// from has visibly changed would place old words on a new screen — the same failure as
// a stale capture, arrived at more slowly.
const defaultFreshness = 3 * time.Second

// cache holds the most recent OCR pass for one window.
//
// One entry, not a map. The provider reads the active window, there is one of those,
// and a multi-entry cache would mostly hold results for windows nobody is looking at
// while making the eviction rules something to get wrong.
type cache struct {
	mu        sync.Mutex
	freshness time.Duration

	key   string
	at    time.Time
	obs   []observation.Observation
	diag  Diagnostics
	valid bool
}

func newCache(freshness time.Duration) *cache {
	if freshness <= 0 {
		freshness = defaultFreshness
	}
	return &cache{freshness: freshness}
}

type cached struct {
	obs  []observation.Observation
	diag Diagnostics
}

// cacheKey identifies what a cached pass was of.
//
// Everything that would change the answer is in the key. Window identity alone is not
// enough: the same window at different BOUNDS shows different content, and a region
// request reads a different part of it. A key that omitted either would serve text
// from the wrong place.
func cacheKey(w directorapi.Window, region *directorapi.Rect) string {
	k := fmt.Sprintf("%s|%d,%d,%dx%d|%s|%s",
		w.ID, w.Bounds.X, w.Bounds.Y, w.Bounds.Width, w.Bounds.Height,
		w.Title, w.Application)
	if region != nil {
		k += fmt.Sprintf("|r%d,%d,%dx%d", region.X, region.Y, region.Width, region.Height)
	}
	return k
}

func (c *cache) get(w directorapi.Window, region *directorapi.Rect) (cached, bool) {
	if c == nil {
		return cached{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.valid || c.key != cacheKey(w, region) {
		return cached{}, false
	}
	if time.Since(c.at) > c.freshness {
		// Expired. Dropped rather than returned with a warning: a caller that received
		// stale text would have to decide what to do with it, and the only correct
		// answer is to look again.
		c.valid = false
		return cached{}, false
	}
	return cached{obs: c.obs, diag: c.diag}, true
}

func (c *cache) put(w directorapi.Window, region *directorapi.Rect,
	obs []observation.Observation, diag Diagnostics) {

	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.key, c.at, c.obs, c.diag, c.valid = cacheKey(w, region), time.Now(), obs, diag, true
}

// Invalidate drops any cached pass. Called when something is known to have changed.
func (p *Provider) Invalidate() {
	if p.cache == nil {
		return
	}
	p.cache.mu.Lock()
	p.cache.valid = false
	p.cache.mu.Unlock()
}

// SetFreshness configures how long a pass may be reused. Zero restores the default.
func (p *Provider) SetFreshness(d time.Duration) {
	if p.cache == nil {
		p.cache = newCache(d)
		return
	}
	p.cache.mu.Lock()
	if d <= 0 {
		d = defaultFreshness
	}
	p.cache.freshness = d
	p.cache.valid = false
	p.cache.mu.Unlock()
}
