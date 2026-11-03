// Package recordingnow is the one fact every part of Marco can read: a recording is in progress.
//
// # Why it exists
//
// A leader shortcut injects real input. Pressed while somebody is demonstrating a routine, it would
// type into the window being recorded — and the widget, which owns the leader, has no idea that
// Studio (or a terminal's `marco learn`, or a Director learn session) is recording, because they are
// different processes. The owner's ruling is that a recording in progress BLOCKS routine shortcuts.
// That needs a fact both sides can see without a service, the same constraint `internal/stopsignal`
// was written under.
//
// # It fails CLOSED, in both directions
//
//   - A recording that cannot be announced does not start. The announcement is written BEFORE the
//     hooks go on, so there is no instant at which input is being captured and nothing says so.
//   - A reader that cannot tell answers `Unknown`, never "no recording". An unreadable directory,
//     a marker it cannot parse, a process whose liveness cannot be established — each is Unknown,
//     and a shortcut refuses on Unknown exactly as it refuses on Recording.
//
// Only a marker whose process is CONFIRMED gone — it does not exist, or it exited, or its pid now
// belongs to a process that started at another time — is stale, and only a stale marker is swept.
//
// # Why a file per recorder
//
// One file per recording process under `<MARCO_HOME>/recording/`, named by its pid and that
// process's START TIME, and holding both. A pid on its own is not a process — Windows hands the
// number back out — so a marker is live only while a process with that pid AND that start time
// exists. A recorder killed mid-demonstration leaves its file behind; the file then names a process
// that is confirmed gone and is swept, so a crash cannot block shortcuts for ever.
//
// # Off Windows
//
// A process's start time cannot be read, so a recording cannot be announced and therefore does not
// start, and a marker present there is Unknown. The product does not ship there; it fails closed
// rather than pretending.
package recordingnow

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Marker is one recording in progress.
type Marker struct {
	PID     uint32    `json:"pid"`
	Since   time.Time `json:"since"`   // when that process started — what makes the pid an identity
	Started time.Time `json:"started"` // when the recording started
	What    string    `json:"what"`    // the routine being recorded, for the sentence
	path    string
}

// State is what a reader knows.
type State int

const (
	// None: confirmed that no recording is in progress.
	None State = iota
	// Recording: a recording is in progress.
	Recording
	// Unknown: it could not be established either way. Treated like Recording by anything that
	// would inject input.
	Unknown
)

// liveness is what is known about one process.
type liveness int

const (
	alive liveness = iota
	gone
	unsure
)

// processAt is how a pid is identified: whether it is alive and when it started. A variable so a
// test can state the processes that exist.
var processAt = processState

func dir(home string) string { return filepath.Join(home, "recording") }

// Mark records that THIS process is about to record, and returns what Clear needs.
//
// Called BEFORE the recorder's hooks go on; an error means the recording must not start.
//
// An empty home writes nothing and returns the zero marker with no error: the caller stated it
// announces nothing, which is every test that never asked for this. Every production constructor
// states a home, and `TestTheTerminalAnnouncesItsRecordings` / Studio's equivalent hold that.
func Mark(home, what string) (Marker, error) {
	if strings.TrimSpace(home) == "" {
		return Marker{}, nil
	}
	pid := uint32(os.Getpid())
	state, since := processAt(pid)
	if state != alive || since.IsZero() {
		return Marker{}, fmt.Errorf("this recording could not be announced to the rest of "+
			"Marco (process %d could not be identified), so it was not started — a "+
			"recording nothing knows about could have shortcuts typed into it", pid)
	}
	m := Marker{PID: pid, Since: since.UTC(), Started: time.Now().UTC(), What: what}
	if err := os.MkdirAll(dir(home), 0o755); err != nil {
		return Marker{}, fmt.Errorf("this recording could not be announced to the rest of "+
			"Marco, so it was not started: %w", err)
	}
	m.path = filepath.Join(dir(home), fmt.Sprintf("%d-%d.json", pid, m.Since.UnixNano()))
	raw, _ := json.Marshal(m)
	if err := writeAtomic(m.path, raw); err != nil {
		return Marker{}, fmt.Errorf("this recording could not be announced to the rest of "+
			"Marco, so it was not started: %w", err)
	}
	return m, nil
}

// Clear removes a marker Mark returned. Safe on the zero marker.
func Clear(m Marker) {
	if m.path != "" {
		_ = os.Remove(m.path)
	}
}

// InProgress is whether a recording is going on now, and which, when one is.
func InProgress(home string) (Marker, State) {
	if strings.TrimSpace(home) == "" {
		// No home is no shared state to read, which is not evidence of absence.
		return Marker{}, Unknown
	}
	entries, err := os.ReadDir(dir(home))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// CONFIRMED, not assumed: Windows reports a path that exists but is not a
		// directory as not found too, and that is a broken state, not an absent one.
		if _, lerr := os.Lstat(dir(home)); errors.Is(lerr, fs.ErrNotExist) {
			return Marker{}, None
		}
		return Marker{}, Unknown
	case err != nil:
		return Marker{}, Unknown
	}
	verdict := None
	var unsureOf Marker
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir(home), e.Name())
		raw, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed between the listing and the read: that recording ended
		}
		var m Marker
		if err != nil || json.Unmarshal(raw, &m) != nil || m.PID == 0 {
			verdict, unsureOf = Unknown, Marker{path: p}
			continue
		}
		m.path = p
		state, since := processAt(m.PID)
		switch {
		case state == alive && since.UTC().Equal(m.Since.UTC()):
			return m, Recording
		case state == gone, state == alive && !since.IsZero():
			// Confirmed gone, or the pid belongs to a process that started at another
			// time. Stale: swept.
			_ = os.Remove(p)
		default:
			verdict, unsureOf = Unknown, m
		}
	}
	return unsureOf, verdict
}

// Says is the sentence for a shortcut held off by this recording.
func (m Marker) Says() string {
	what := strings.TrimSpace(m.What)
	if what == "" {
		what = "a routine"
	} else {
		what = strconv.Quote(what)
	}
	return fmt.Sprintf("a recording of %s is in progress, so shortcuts are held off until "+
		"it ends — nothing was performed", what)
}

// UnknownSays is the sentence for a shortcut held off because nobody could tell.
func UnknownSays() string {
	return "Marco could not tell whether a recording is in progress, so this shortcut was " +
		"held off — nothing was performed"
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".marker-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
