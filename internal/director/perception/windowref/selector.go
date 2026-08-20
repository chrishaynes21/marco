package windowref

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Choosing a window on purpose.
//
// Until now the Director looked at whatever happened to be in front. That is the right
// default for a command a person types about the thing they are looking at, and completely
// wrong for watching a game: running any diagnostic from a terminal gives the terminal
// focus, so three separate live attempts at Rocket League ended up describing VS Code
// instead.
//
// A selector says which window, once, and keeps saying it. Focus stops being an input.
//
// # Ephemeral identifiers
//
// A selector may name a window by an id like "window_7". Those ids are issued by the
// Director, are valid only for the generation they were issued in, and mean nothing
// afterwards. That is deliberate: the whole lesson of the stale-capture incident is that a
// window reference is not durable, and an id a person could write down and reuse tomorrow
// would smuggle the same mistake back in through the front door. A raw HWND is never
// accepted from a user for the same reason.

// Selector names the window to observe.
//
// Exactly one PRIMARY selector — id, title, application or process — must be set. Several
// would be a query, and a query that matched two windows would need a tie-break nobody
// asked for.
type Selector struct {
	// EphemeralID is a Director-issued id from `director windows`, valid only for the
	// generation it was listed in.
	EphemeralID string
	// Title matches a window's caption, case-insensitively, as a substring. Convenient
	// and weak: titles change while a window lives.
	Title string
	// Application matches the executable name — the identity that survives a restart.
	Application string
	// ProcessID matches one running process exactly.
	ProcessID uint32
}

// Zero reports whether nothing was selected.
func (s Selector) Zero() bool {
	return s.EphemeralID == "" && s.Title == "" && s.Application == "" && s.ProcessID == 0
}

// Describe renders the selector for a person, without leaking a handle.
func (s Selector) Describe() string {
	switch {
	case s.EphemeralID != "":
		return "window " + s.EphemeralID
	case s.Application != "":
		return "application " + s.Application
	case s.Title != "":
		return fmt.Sprintf("title %q", s.Title)
	case s.ProcessID != 0:
		return fmt.Sprintf("process %d", s.ProcessID)
	}
	return "nothing"
}

// Validate reports whether exactly one primary selector was given.
func (s Selector) Validate() error {
	n := 0
	for _, set := range []bool{
		s.EphemeralID != "", s.Title != "", s.Application != "", s.ProcessID != 0,
	} {
		if set {
			n++
		}
	}
	switch n {
	case 1:
		return nil
	case 0:
		return fmt.Errorf("no window was selected: give one of " +
			"--window-id, --window-title, --application or --process")
	default:
		return fmt.Errorf("more than one window selector was given; " +
			"use exactly one so there is nothing to disambiguate")
	}
}

// Resolution is the outcome of resolving a selector against the live desktop.
type Resolution string

const (
	// Resolved: exactly one current window matches.
	Resolved Resolution = "resolved"
	// NotFound: nothing currently matches.
	NotFound Resolution = "not_found"
	// AmbiguousSelector: several windows match and the selector cannot tell them apart.
	AmbiguousSelector Resolution = "ambiguous"
	// StaleEphemeralID: the id was issued for a generation that has since ended.
	StaleEphemeralID Resolution = "stale_id"
	// UnusableSelector: nothing was selected, or several primaries were.
	UnusableSelector Resolution = "unusable_selector"
)

// OK reports whether resolution produced exactly one window.
func (r Resolution) OK() bool { return r == Resolved }

// Listing is one window as `director windows` shows it.
type Listing struct {
	// ID is the ephemeral, Director-issued reference. Valid for this generation only.
	ID string `json:"id"`
	// Application is the executable name.
	Application string `json:"application"`
	// Title is the caption, truncated by the renderer rather than here.
	Title string `json:"title"`
	// ProcessID owns it.
	ProcessID uint32 `json:"process_id"`
	// State is a short word: foreground, visible, minimized, off-screen.
	State string `json:"state"`
	// Bounds is where it is, for a reader deciding which of two it means.
	Bounds directorapi.Rect `json:"bounds"`
}

// Directory issues and resolves ephemeral window ids.
//
// One per Director. Ids are handed out by `director windows` and consumed by a selector
// moments later; they are intentionally forgettable, and an id from a previous listing
// whose window has since changed generation resolves to StaleEphemeralID rather than to
// something plausible.
type Directory struct {
	mu     sync.Mutex
	next   uint64
	issued map[string]issuedID
}

type issuedID struct {
	handle    uintptr
	processID uint32
	// application and title are kept so a stale id can explain what it USED to mean.
	application string
}

// NewDirectory returns an empty directory.
func NewDirectory() *Directory {
	return &Directory{issued: map[string]issuedID{}}
}

// List enumerates the current windows and issues an id for each.
//
// Enumerated fresh, always. A cached listing would be a list of windows that existed once,
// which is the category of thing this package exists to stop anyone acting on.
func (d *Directory) List(ctx context.Context, p Platform, application string) []Listing {
	candidates := p.Candidates(ctx, application)
	if application == "" {
		candidates = allWindows(ctx, p)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Application != candidates[j].Application {
			return candidates[i].Application < candidates[j].Application
		}
		return candidates[i].Handle < candidates[j].Handle
	})

	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]Listing, 0, len(candidates))
	for _, c := range candidates {
		d.next++
		id := fmt.Sprintf("window_%d", d.next)
		d.issued[id] = issuedID{
			handle: c.Handle, processID: c.ProcessID, application: c.Application,
		}
		out = append(out, Listing{
			ID: id, Application: c.Application, Title: c.Title,
			ProcessID: c.ProcessID, State: describeState(c), Bounds: c.Bounds,
		})
	}
	return out
}

// Adopt issues an ephemeral id for one candidate the Director chose itself.
//
// The same issuing path `List` uses, for the case where nobody typed a selector: Marco resolved
// the window on the user's behalf and still has to name it in the one vocabulary everything
// downstream understands. Going through the directory rather than fabricating a `Selector` from a
// process id is what keeps the generation pinned — an id names one window of one process, and
// refuses to follow a replacement, which is exactly the guarantee an automatically-chosen target
// needs most.
func (d *Directory) Adopt(c Candidate) string {
	if c.Handle == 0 {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	id := fmt.Sprintf("window_%d", d.next)
	d.issued[id] = issuedID{
		handle: c.Handle, processID: c.ProcessID, application: c.Application,
	}
	return id
}

// Foreground is the window the user is currently working in.
//
// # Why this exists, given that the whole package is about NOT following focus
//
// Because "do not follow focus" and "never ask what the user is looking at" are different rules,
// and only the first one is real. A session must not drift to whatever comes forward while it
// runs — that is the stale-capture lesson and it stands. Choosing a starting target once, on the
// user's behalf, is the opposite situation: the alternative is making somebody run
// `director windows`, read a table of ephemeral ids, and type one back, which is the product
// asking a person to do the Director's job.
//
// Resolved ONCE and then pinned by the caller through [Directory.Adopt], so the answer cannot
// change underneath a running session. Nothing here is a live selector.
//
// Marco's own presentation surfaces cannot be returned: they are excluded from candidacy at the
// platform chokepoint, so a laser pointer can never become the thing being pointed at.
func Foreground(ctx context.Context, p Platform) (Candidate, Resolution, string) {
	for _, c := range allWindows(ctx, p) {
		if c.Foreground {
			return c, Resolved, ""
		}
	}
	return Candidate{}, NotFound, "nothing is in the foreground that I can observe — " +
		"the window in front may be minimized, off-screen, or one I'm not allowed to look at"
}

// lookUp returns what an id was issued for.
func (d *Directory) lookUp(id string) (issuedID, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.issued[id]
	return v, ok
}

// describeState is the one-word state a listing shows.
func describeState(c Candidate) string {
	switch {
	case c.Foreground:
		return "foreground"
	case c.Minimized:
		return "minimized"
	case !c.OnScreen:
		return "off-screen"
	case c.Visible:
		return "visible"
	}
	return "hidden"
}

// Resolve turns a selector into exactly one current window, or explains why it cannot.
//
// Never chooses by enumeration order. Two windows a selector cannot tell apart is a
// question for the person who wrote the selector, and the honest answer is to ask for a
// more specific one.
func Resolve(ctx context.Context, p Platform, dir *Directory, s Selector) (Candidate, Resolution, string) {
	if err := s.Validate(); err != nil {
		return Candidate{}, UnusableSelector, err.Error()
	}

	switch {
	case s.EphemeralID != "":
		if dir == nil {
			return Candidate{}, UnusableSelector, "no window listing has been taken yet; run `director windows` first"
		}
		issued, ok := dir.lookUp(s.EphemeralID)
		if !ok {
			return Candidate{}, NotFound, fmt.Sprintf(
				"%s is not an id this Director issued; run `director windows` for current ids",
				s.EphemeralID)
		}
		live, exists := p.Live(ctx, issued.handle)
		if !exists {
			return Candidate{}, StaleEphemeralID, fmt.Sprintf(
				"%s referred to a %s window that no longer exists; ids are valid only for "+
					"the generation they were listed in — run `director windows` again",
				s.EphemeralID, orUnknown(issued.application))
		}
		if issued.processID != 0 && live.ProcessID != issued.processID {
			return Candidate{}, StaleEphemeralID, fmt.Sprintf(
				"%s now refers to a different process; the id is stale — run `director windows` again",
				s.EphemeralID)
		}
		return live, Resolved, ""

	case s.ProcessID != 0:
		matches := filter(allWindows(ctx, p), func(c Candidate) bool {
			return c.ProcessID == s.ProcessID
		})
		return one(matches, fmt.Sprintf("process %d", s.ProcessID),
			"that process has more than one window; select it by --window-id instead")

	case s.Application != "":
		matches := usable(p.Candidates(ctx, s.Application))
		return one(matches, "application "+s.Application,
			fmt.Sprintf("%s has more than one window; run `director windows --application %s` "+
				"and select one by --window-id", s.Application, s.Application))

	default: // title
		want := strings.ToLower(strings.TrimSpace(s.Title))
		matches := filter(allWindows(ctx, p), func(c Candidate) bool {
			return strings.Contains(strings.ToLower(c.Title), want)
		})
		return one(matches, fmt.Sprintf("title %q", s.Title),
			"more than one window's title matches; use a longer title or --window-id")
	}
}

// one insists on exactly one match.
func one(matches []Candidate, what, ambiguous string) (Candidate, Resolution, string) {
	switch len(matches) {
	case 1:
		return matches[0], Resolved, ""
	case 0:
		return Candidate{}, NotFound, "no current window matches " + what
	default:
		return Candidate{}, AmbiguousSelector, ambiguous
	}
}

// allWindows lists every capturable window, whatever it belongs to.
func allWindows(ctx context.Context, p Platform) []Candidate {
	if l, ok := p.(interface {
		AllCandidates(context.Context) []Candidate
	}); ok {
		return usable(l.AllCandidates(ctx))
	}
	return nil
}

func filter(in []Candidate, keep func(Candidate) bool) []Candidate {
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

func orUnknown(s string) string {
	if s == "" {
		return "an unknown"
	}
	return s
}

// AcquireBy validates and, where necessary, reacquires the window a selector names.
//
// The focus-independent entry point. Nothing here consults the foreground window, and
// nothing about the calling process's own focus can change which window comes back.
//
// Reacquisition rules differ by selector kind, and the difference is deliberate:
//
//   - --application MAY follow a restart. The executable name is exactly the identity that
//     survives one, so finding the new window is what the user asked for.
//   - --window-id MAY NOT. An id names one generation; silently attaching to a replacement
//     would make an explicitly-chosen window mean "whatever came next", which is the
//     durable-identity mistake in a new costume.
//   - --process cannot outlive its process, so a restart is NotFound.
//   - --title MAY follow, because a title is a description rather than a reference — but
//     only when it still matches exactly one window.
func (t *Tracker) AcquireBy(ctx context.Context, p Platform, dir *Directory, s Selector) Validation {
	if err := s.Validate(); err != nil {
		return Validation{State: Unavailable, Reason: err.Error()}
	}

	candidate, res, why := Resolve(ctx, p, dir, s)
	if !res.OK() {
		t.mu.Lock()
		previous := t.invalidateLocked(why)
		t.lastState, t.lastWhy = stateFor(res), why
		t.mu.Unlock()
		return Validation{State: stateFor(res), Reason: why, Previous: previous}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	previousProcess := t.current.ProcessID
	previousGeneration := t.current.Generation
	t.adopt(Ref{
		ID: candidate.ID, Handle: candidate.Handle, ProcessID: candidate.ProcessID,
		Application: candidate.Application, Title: candidate.Title,
		Bounds: candidate.Bounds, ValidatedAt: t.now(),
	})
	t.proposed = Ref{}
	t.lastState, t.lastWhy = Valid, ""

	return Validation{
		State: Valid, Ref: t.current,
		Reacquired:     previousGeneration != t.current.Generation,
		ProcessChanged: previousProcess != 0 && previousProcess != candidate.ProcessID,
		Previous:       previousGeneration,
	}
}

// stateFor maps a resolution failure onto the validation state a caller acts on.
func stateFor(r Resolution) State {
	switch r {
	case AmbiguousSelector:
		return Ambiguous
	case StaleEphemeralID:
		return Replaced
	default:
		return Unavailable
	}
}
