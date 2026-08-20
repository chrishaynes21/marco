package service

import (
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Accessibility lifecycle, tracked per application.
//
// This is the diagnostic half of the milestone's central win. Under the old
// spawn-per-command model the accessibility bridge was created and destroyed for
// every request, so Chrome and VS Code — which only expose their interiors after
// sustained client presence — were re-measured from cold every single time. Chrome
// went from 65 elements to 2248 over minutes of continuous attachment; a process
// that lived for one command never got past the first number.
//
// Keeping the process alive fixes that on its own. What this adds is the ability to
// SEE that it is fixed: how long a provider has been attached, whether its tree has
// settled, and how much of it is actually usable.
//
// Nothing here records labels or content. The counts describe how much structure an
// application exposes; a status command must not leak what is on the user's screen.

// ProviderStatus is what the service knows about one application's accessibility.
type ProviderStatus struct {
	App string `json:"app"`
	// Status is "ready", "shallow" (well observed but nothing seen into),
	// "unobservable" (nothing operable) or "stale".
	Status string `json:"status"`

	AttachedAt time.Time `json:"attached_at"`
	LastSeen   time.Time `json:"last_seen"`

	Elements   int `json:"elements"`
	Content    int `json:"content"`
	Actionable int `json:"actionable"`

	// LastStructuralChange is when the element count last moved. An application
	// whose tree is still growing has not finished hydrating.
	LastStructuralChange time.Time `json:"last_structural_change"`
	// StableFor is how long the tree has been unchanged — the practical signal that
	// hydration has finished.
	StableFor    time.Duration `json:"stable_for"`
	StableForStr string        `json:"stable_for_human"`

	// Observations is how many times this application has been snapshotted since the
	// service started. High counts with a stable tree are what provider reuse looks
	// like from outside.
	Observations int `json:"observations"`

	Coverage      float64 `json:"coverage"`
	Actionability float64 `json:"actionability"`
}

// ProviderTracker records accessibility lifecycle across commands.
type ProviderTracker struct {
	mu    sync.RWMutex
	byApp map[string]*ProviderStatus
	// attachedAt is when the service brought its accessibility client up.
	attachedAt time.Time
}

// NewProviderTracker returns a tracker marked as attached now.
func NewProviderTracker() *ProviderTracker {
	return &ProviderTracker{byApp: map[string]*ProviderStatus{}, attachedAt: time.Now()}
}

// AttachedAt is when the accessibility client came up.
func (t *ProviderTracker) AttachedAt() time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.attachedAt
}

// Observe records what a snapshot showed about an application.
func (t *ProviderTracker) Observe(w directorapi.WorldState) {
	app := ""
	if w.ActiveApp != nil {
		app = w.ActiveApp.ID
	}
	if app == "" {
		return
	}

	elements, content, actionable := 0, 0, 0
	for _, el := range w.Elements {
		elements++
		if el.Role.Content() {
			content++
		}
		if el.Addressable() {
			actionable++
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	p, known := t.byApp[app]
	if !known {
		p = &ProviderStatus{App: app, AttachedAt: now, LastStructuralChange: now}
		t.byApp[app] = p
	}

	// A changed element count means the tree is still moving. Tracking WHEN it last
	// moved is what distinguishes "hydrated and settled" from "still filling in",
	// which is exactly the difference a cold-started provider could never show.
	if p.Elements != elements {
		p.LastStructuralChange = now
	}

	p.LastSeen = now
	p.Elements = elements
	p.Content = content
	p.Actionable = actionable
	p.Observations++
	p.Coverage = w.Confidence.Coverage
	p.Actionability = w.Confidence.Actionability
	p.StableFor = now.Sub(p.LastStructuralChange)

	switch {
	case w.Confidence.Blind():
		p.Status = "unobservable"
	case w.Confidence.Shallow():
		p.Status = "shallow"
	default:
		p.Status = "ready"
	}
}

// Status returns every tracked provider, most recently seen first.
func (t *ProviderTracker) Status() []ProviderStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make([]ProviderStatus, 0, len(t.byApp))
	now := time.Now()
	for _, p := range t.byApp {
		snapshot := *p
		snapshot.StableFor = now.Sub(p.LastStructuralChange).Round(time.Second)
		snapshot.StableForStr = humanDuration(snapshot.StableFor)
		// An application not seen for a while may have closed. Reporting it as still
		// ready would be a claim the service cannot support.
		if now.Sub(p.LastSeen) > staleProviderAfter {
			snapshot.Status = "stale"
		}
		out = append(out, snapshot)
	}
	// Most recently seen first: the application in front is what a person is asking
	// about.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].LastSeen.After(out[j-1].LastSeen); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// staleProviderAfter is how long an application may go unobserved before its
// reported state is treated as no longer current.
const staleProviderAfter = 5 * time.Minute

// humanDuration renders a duration the way a status line should read.
func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return itoa(h) + "h " + itoa(m) + "m"
	case m > 0:
		return itoa(m) + "m " + itoa(s) + "s"
	default:
		return itoa(s) + "s"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// AccessibilityReporter is a Runtime that can say why its Accessibility Actor cannot act.
//
// # Why an optional interface rather than a widening of Runtime
//
// The same reason Performer is one, and it is not a stylistic preference. A Director that cannot
// answer this is a legitimate Director — `cmd/marco` has a stub Runtime whose whole purpose is to
// answer the Director protocol without being one — and putting the method on `Runtime` would make
// every implementer, present and future, write a line claiming to know something about a bridge it
// has never heard of, purely in order to keep compiling.
//
// Asked for by name, a Director either offers the answer or honestly does not.
type AccessibilityReporter interface {
	// AccessibilityUnavailable is why the Accessibility Actor cannot act, empty when it can.
	AccessibilityUnavailable() string
}

// accessibilityReason is what to tell a client about a missing Accessibility Actor.
//
// Empty from a Runtime that does not report on itself, which reads as "nothing to say" — the same
// thing it means from one that reports and finds nothing wrong. That collapse is deliberate: this
// is a diagnostic, and a diagnostic that distinguished "fine" from "cannot tell" would put a
// sentence about Marco's own plumbing in front of somebody asking about their computer.
//
// It exists because the Director now BOOTS without an accessibility bridge instead of refusing to
// start over one. Degrading is only honest if the degradation can be reported: "Accessibility
// clients: 0" reads identically for "nothing observed yet" and "there is nothing to observe
// through", and only one of those is something the person can fix.
func accessibilityReason(rt Runtime) string {
	r, ok := rt.(AccessibilityReporter)
	if !ok {
		return ""
	}
	return r.AccessibilityUnavailable()
}
