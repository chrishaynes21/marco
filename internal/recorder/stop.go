package recorder

import "strings"

// DefaultStopKey is the gesture that ends a recording / aborts a running route
// when $MARCO_STOP_KEY isn't set. It deliberately isn't Esc, so Esc (and the
// other common keys) can be recorded as ordinary keystrokes inside a macro. F12
// is present on every keyboard and almost never part of an automation.
const DefaultStopKey = "f12"

// StopKey matches a stop gesture against a live event stream. The gesture is one
// key, or a chord of keys held at once ("ctrl+f12"), specified lowercased and
// joined with '+'. Build with ParseStopKey; the zero value is unusable.
type StopKey struct {
	names []string        // ordered, deduped, lowercased
	set   map[string]bool // membership test
	held  map[string]bool // currently-down subset (mutated by Triggered)
}

// ParseStopKey parses a spec like "f12", "esc", or "ctrl+f12". An empty spec
// uses DefaultStopKey. Unknown key names are kept verbatim — they simply never
// match what the recorder emits, which surfaces as "the stop key never fires"
// rather than a silent wrong-key.
func ParseStopKey(spec string) StopKey {
	if strings.TrimSpace(spec) == "" {
		spec = DefaultStopKey
	}
	sk := StopKey{set: map[string]bool{}, held: map[string]bool{}}
	for part := range strings.SplitSeq(spec, "+") {
		k := strings.ToLower(strings.TrimSpace(part))
		if k == "" || sk.set[k] {
			continue
		}
		sk.set[k] = true
		sk.names = append(sk.names, k)
	}
	return sk
}

// Has reports whether name is one of the stop gesture's keys (used to strip the
// gesture from the tail of a recording).
func (s StopKey) Has(name string) bool { return s.set[strings.ToLower(name)] }

// Triggered updates the held-key state from ev and reports whether the full stop
// gesture is now pressed (all of its keys down at once). Non-key events and keys
// outside the gesture are ignored (they don't disturb the chord state).
func (s *StopKey) Triggered(ev RecordedEvent) bool {
	if ev.Kind != EvKey {
		return false
	}
	name := strings.ToLower(ev.KeyName)
	if !s.set[name] {
		return false
	}
	if ev.Down {
		s.held[name] = true
	} else {
		delete(s.held, name)
	}
	return len(s.held) == len(s.names)
}

// Label renders the gesture for prompts, e.g. "F12" or "Ctrl+F12".
func (s StopKey) Label() string {
	if len(s.names) == 0 {
		return strings.ToUpper(DefaultStopKey)
	}
	parts := make([]string, len(s.names))
	for i, n := range s.names {
		parts[i] = title(n)
	}
	return strings.Join(parts, "+")
}

// title capitalizes the first letter (ASCII), leaving the rest — so "f12"→"F12",
// "esc"→"Esc", "pagedown"→"Pagedown".
func title(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 32
	}
	return string(b)
}
