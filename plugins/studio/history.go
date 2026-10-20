package main

// WHAT HAS HAPPENED BEFORE, AND A FILE SOMEBODY CAN SEND WHEN IT WENT WRONG.
//
// # Why the records and not a log
//
// Because the run record is the canonical account of a run — the thing `director` wrote when it
// ended, named by the same reference both binaries parse — and a second account kept by this
// window would be a second answer to "what happened". So this reads the records, newest first,
// and says what each one says.
//
// Nothing here runs, cancels, or changes anything. It reads files and writes one.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
)

// historyShown is how many past runs the window lists.
//
// A window is for the ones somebody might still be asking about; the directory is the archive,
// and `Inspect` reaches any of them by reference.
const historyShown = 25

// PastRun is one saved run, as somebody asking "what happened" needs it.
//
// A subset of the record, by name, read as JSON rather than through the writer's own type:
// `RunRecord` lives in cmd/director's main package and nothing can import it. The fields are
// therefore matched by their JSON names, and a record carrying a field this does not know is
// still listed — `Inspect` shows the whole of it.
type PastRun struct {
	Reference   string `json:"reference"`
	Act         string `json:"act,omitempty"`
	Routine     string `json:"routine,omitempty"`
	Application string `json:"application,omitempty"`
	Outcome     string `json:"outcome"`
	// Said is what the outcome means for what the person asked for, and Next what to do
	// about it. Both from the same functions the live run screen uses, so a run reads
	// the same way an hour later as it did while it was in front of somebody.
	Said string `json:"said,omitempty"`
	Next string `json:"next,omitempty"`
	// When is the local time it started, written to be read rather than parsed.
	When string `json:"when,omitempty"`
	Live bool   `json:"live"`
	// Sent is how many presses actually went out, and Verified how many were checked.
	// The first is the one that matters after something went wrong: it is the answer to
	// "did it touch my machine".
	Sent     int `json:"sent"`
	Verified int `json:"verified"`
	// Authority is who authorised it, in words — "the person who pressed Play in Studio
	// at 16:05:44, authorising …" — which is the difference between a granted run and
	// one answered by key.
	Authority string `json:"authority,omitempty"`
	// Unfinished is a record that says a run began and never says how it ended.
	Unfinished bool `json:"unfinished,omitempty"`

	started time.Time
}

// record is the run record as it sits on disk, by its JSON names.
type record struct {
	Run         string `json:"run"`
	Workflow    string `json:"workflow"`
	Act         string `json:"act"`
	Application string `json:"application"`
	Started     string `json:"started"`
	Ended       string `json:"ended"`
	Outcome     string `json:"outcome"`
	Because     string `json:"because"`
	ActionsSent int    `json:"actions_sent"`
	Verified    int    `json:"verified"`
	Live        bool   `json:"live"`
	Authority   string `json:"authority"`
}

// History is the runs this machine has records of, newest first.
func (a *Acts) History() []PastRun {
	dir := filepath.Join(a.home, "runs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []PastRun{}
	}
	out := make([]PastRun, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			continue
		}
		var r record
		if jerr := json.Unmarshal(raw, &r); jerr != nil {
			// A RECORD THAT WILL NOT PARSE IS STILL A RUN THAT HAPPENED.
			// Listed by its filename with nothing claimed about it, because
			// the alternative is a window that silently has fewer runs in it
			// than the machine does.
			out = append(out, PastRun{
				Reference: strings.TrimSuffix(e.Name(), ".json"),
				Outcome:   "unreadable", Said: jerr.Error()})
			continue
		}
		out = append(out, aPastRun(r))
	}
	// NEWEST FIRST, by the time the RUN says it started rather than by the file's own
	// modification time: a record is written twice, once when the reference is reserved
	// and once when it ends, so mtime orders by when runs finished.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].started.After(out[j].started)
	})
	if len(out) > historyShown {
		out = out[:historyShown]
	}
	return out
}

func aPastRun(r record) PastRun {
	p := PastRun{
		Reference: r.Run, Act: r.Act, Routine: r.Workflow,
		Application: r.Application, Outcome: r.Outcome,
		Live: r.Live, Sent: r.ActionsSent, Verified: r.Verified,
		Authority: r.Authority,
	}
	name := r.Act
	if name == "" {
		name = r.Workflow
	}
	p.Said = saidOf(name, runref.Outcome(r.Outcome), r.Because)
	// NO `waits` HERE. What an Act would wait for is a fact about the Act as it stands
	// now, and this is a run that happened; offering today's remedy for a run from last
	// week would be attributing one to the other. The Act's own listing says it.
	p.Next = nextOf(runref.Outcome(r.Outcome), "")
	if at, err := time.Parse(time.RFC3339, r.Started); err == nil {
		p.started = at
		p.When = at.Local().Format("2006-01-02 15:04")
	}
	p.Unfinished = r.Ended == ""
	return p
}

// Reported is where the problem report went, and what is in it.
type Reported struct {
	Path  string `json:"path,omitempty"`
	Holds string `json:"holds,omitempty"`
	Error string `json:"error,omitempty"`
}

// Report writes everything about one run into a file somebody can attach, and says where.
//
// # Why a file and not a submission
//
// Because this window has no business sending anything anywhere. It reads the owner's own store,
// and a button that posted their run — the application they were using, the words read off their
// screen, the reference and the authority — to a service would be a surface deciding to publish
// somebody's desktop. So it gathers, and the person sends it if they want to.
//
// What it holds is the canonical record, the transcript of the run if this window still has one,
// and the Act as it stands now. That last one matters: a run refused for a reason about the
// demonstration is explained by the Act rather than by the run.
func (a *Acts) Report(reference string) Reported {
	if strings.TrimSpace(reference) == "" {
		return Reported{Error: "there is no run reference to report"}
	}
	rec := a.Inspect(reference)
	raw, _ := json.MarshalIndent(rec, "", "  ")

	var b strings.Builder
	fmt.Fprintf(&b, "MARCO — a problem with run %s\n\n", reference)
	fmt.Fprintf(&b, "written %s\n\n", time.Now().Local().Format(time.RFC1123))
	b.WriteString("WHAT YOU WERE DOING\n  (say what you expected here)\n\n")
	b.WriteString("THE RUN'S OWN RECORD\n")
	b.WriteString(indented(string(raw)))

	a.mu.Lock()
	lines := append([]string(nil), a.transcript...)
	act := a.act
	a.mu.Unlock()
	if len(lines) > 0 {
		b.WriteString("\nWHAT IT SAID AS IT RAN\n")
		b.WriteString(indented(strings.Join(lines, "\n")))
	}
	if act != "" {
		for _, c := range a.Saved() {
			if !strings.EqualFold(c.Name, act) {
				continue
			}
			shown, _ := json.MarshalIndent(c, "", "  ")
			b.WriteString("\nTHE ACT, AS IT STANDS NOW\n")
			b.WriteString(indented(string(shown)))
		}
	}

	dir := filepath.Join(a.home, "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Reported{Error: err.Error()}
	}
	p := filepath.Join(dir, "problem-"+reference+".txt")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		return Reported{Error: err.Error()}
	}
	return Reported{Path: p, Holds: "the run's record, what it said as it ran, and " +
		"the Act as it stands now. Nothing has been sent anywhere — open it, add " +
		"what you expected to happen, and send it yourself."}
}

func indented(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n") + "\n"
}
