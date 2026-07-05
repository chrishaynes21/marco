package oshost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// The anchor cache persists where each anchor last RESOLVED, so the next run can recenter
// its search there and FOLLOW a UI that drifts between runs (a window that creeps across
// the screen, a list that grows, a panel that reflows) — drift the in-frame matchers can't
// track because the target eventually leaves the fixed search box around the recorded
// point. Best-effort: a missing/corrupt cache or a lost concurrent update just means no
// last-known hint (the recorded point is always still searched, and the scorer still gates
// every click). Keyed by the anchor's template path. Disable with MARCO_ANCHOR_CACHE=0.

var cacheMu sync.Mutex

func anchorCacheEnabled() bool {
	return strings.TrimSpace(os.Getenv("MARCO_ANCHOR_CACHE")) != "0"
}

func anchorCachePath() string {
	if p := strings.TrimSpace(os.Getenv("MARCO_ANCHOR_CACHE_FILE")); p != "" {
		return p
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "marco", "anchors.json")
}

func readCache() map[string][2]int {
	m := map[string][2]int{}
	if data, err := os.ReadFile(anchorCachePath()); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

// anchorLastKnown returns the last location this anchor (keyed by its template path)
// resolved to, if cached and enabled.
func anchorLastKnown(imgPath string) ([2]int, bool) {
	if imgPath == "" || !anchorCacheEnabled() {
		return [2]int{}, false
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	p, ok := readCache()[imgPath]
	return p, ok
}

// rememberLocation records where an anchor resolved, so the next run searches there too.
func rememberLocation(imgPath string, x, y int) {
	if imgPath == "" || !anchorCacheEnabled() {
		return
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	m := readCache()
	if cur, ok := m[imgPath]; ok && cur[0] == x && cur[1] == y {
		return // unchanged — skip the write
	}
	m[imgPath] = [2]int{x, y}
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	path := anchorCachePath()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path) // atomic-ish replace
	}
}
