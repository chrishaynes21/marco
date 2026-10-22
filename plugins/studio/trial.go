package main

// WHAT A TRIAL OF STUDIO LEFT BEHIND, WRITTEN BY STUDIO AS IT HAPPENS.
//
// # Why this exists
//
// Every defect this surface has had was found the same way: the owner did something, said what
// they saw, and engineering worked backwards from the sentence. "Settling is ass, it doesn't
// finish in time." "The images blink in the ui." "The pictures are gone." Each one was true,
// each one took a round trip to turn into a measurement, and one of them — a window and a
// recorder built a day apart — was only visible by comparing file timestamps. The instruction
// that closed that way of working:
//
//	Engineering should collect build identity, recording states, preview persistence, and
//	the canonical result automatically — not turn them into another owner checklist.
//
// So Studio writes those four down itself, as it goes. One ordinary flow — learn a Scene, review
// its pictures, save an Act, press Play — leaves one file that answers all four, and `studio
// -trial` reads it back. Nobody is asked to watch for anything, and nobody has to remember what
// they saw.
//
// # What it is not
//
// Not a second account of a run. The run record is the canonical one and this cites its
// reference and reads its numbers back out of it, exactly as history.go does and for the same
// reason: a window keeping its own verdict about a run would be a second answer to "what
// happened".
//
// Not a log, either. Nothing here is written to be skimmed as it arrives — the window already
// shows a person what is happening, in better words than these. This is evidence of ONE TRIAL,
// in the order the window learned it, and its only reader is the diagnostic in trialcheck.go.
//
// # Snapshots, and only when something changed
//
// The recording state is written when the sentence the window is SHOWING changes, not on every
// poll. The page polls four times every two seconds for as long as it is open, so a line per
// poll would make a file whose size measured how long somebody left the window open — and the
// one thing worth knowing, the sequence of states a demonstration actually passed through,
// would be buried in a thousand copies of each.
//
// That the state is taken from the value handed to the page, rather than read a second time
// from the recorder's stream, is the whole point of where it is wired: this file's claim is
// about what the owner was shown. A second reader of the same stream would be a record of what
// the window could have shown.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
)

// trialVersion is the shape of a note.
//
// Moved whenever `trialNote` gains or loses a field, and `TestTheTrialNoteShapeMovesWithItsVersion`
// makes that happen rather than hoping somebody remembers. The handshake earns its keep for the
// same reason learnstate's does: `omitempty` makes a writer that has never heard of a field
// indistinguishable from one whose value is false, so a reader of an older journal cannot tell
// a missing fact from a negative one unless the version says which it is reading.
const trialVersion = 1

// trialRefetch is how many times one picture may be asked for before the tally is a fault rather
// than a number.
//
// The page caches pictures by recording revision and label in a Map that lives as long as the
// window, so the expected count per label is ONE — one fetch, one `img`, kept across every poll
// and every page switch. Three leaves room for the two honest ways it can be more: a
// demonstration recorded again under the same name is a new revision and a new fetch, and a
// window reopened starts with an empty cache.
//
// The defect this measures asked for 66 pictures in 4.2 seconds with the finished review sitting
// open, and 108 while a run was in flight. Nothing near three happens by accident.
const trialRefetch = 3

// trialTally is how long the picture tally may go unwritten while pictures are being asked for.
//
// A tally that was written on every fetch would turn the blinking defect into a megabyte a
// minute, which is the wrong way to report it. A tally that was written only at the end would
// miss a Studio somebody closed from the task manager. So it is written at most this often while
// pictures are arriving, AND at every seam a reader cares about — see `flush`.
const trialTally = 2 * time.Second

// trialKind is what one note is about. The reader switches on it and ignores what it does not
// know, so a journal written by a later build is still partly readable.
type trialKind string

const (
	// trialOpened is the first line of every journal: which build wrote the rest of it.
	trialOpened trialKind = "opened"
	// trialRecording is a demonstration starting, and trialState each distinct account
	// of itself the window showed while it ran.
	trialRecording trialKind = "recording"
	trialState     trialKind = "state"
	// trialRecorded is what came of it, read from the store after the child exited.
	trialRecorded trialKind = "recorded"
	// trialPictures is the running tally of pictures handed to the page.
	trialPictures trialKind = "pictures"
	// trialAct is an Act composed — saved, or refused and why.
	trialAct trialKind = "act"
	// trialPlay is a performance started, and trialRan its canonical result.
	trialPlay trialKind = "play"
	trialRan  trialKind = "ran"
)

// trialNote is one thing that happened, as a line of the journal.
//
// One struct for every kind rather than one per kind, as `learnstate.Event` and the run record
// both are: the fields a kind does not use are omitted, and a reader that wants "every build
// named anywhere in this trial" asks one question of one field instead of knowing which kinds
// carry it.
type trialNote struct {
	Version int       `json:"version"`
	Seq     int       `json:"seq"`
	At      time.Time `json:"at"`
	Kind    trialKind `json:"kind"`

	// ── WHICH BUILDS TOOK PART ──
	//
	// Build is the window's, on every note. Recorder is the recorder's, as its own
	// stream reported it, and Ran is the director's that performed a run, read from the
	// record it wrote. Three fields because they are three claims: this is what the
	// owner lost a day to, and "the build" is not one thing.
	Build      string `json:"build,omitempty"`
	Recorder   string `json:"recorder,omitempty"`
	Ran        string `json:"ran,omitempty"`
	Mismatched bool   `json:"mismatched,omitempty"`
	// Director, Store and Routes are on the opening note: which executable would be
	// spawned, and which trees were read. The stale sibling that cost a day was not a
	// wrong revision in a file — it was the wrong FILE, beside the wrong binary.
	Director string `json:"director,omitempty"`
	Store    string `json:"store,omitempty"`
	Routes   string `json:"routes,omitempty"`

	// ── WHAT IT IS ABOUT ──
	Name        string `json:"name,omitempty"`
	Application string `json:"application,omitempty"`
	Seconds     int    `json:"seconds,omitempty"`

	// ── THE RECORDER'S ACCOUNT, AS THE WINDOW SHOWED IT ──
	Phase      string `json:"phase,omitempty"`
	Doing      string `json:"doing,omitempty"`
	Presses    int    `json:"presses,omitempty"`
	Watched    int    `json:"watched,omitempty"`
	Captured   int    `json:"captured,omitempty"`
	Unchecked  int    `json:"unchecked,omitempty"`
	Missing    string `json:"missing,omitempty"`
	StartKnown bool   `json:"start_known,omitempty"`
	ScreenRead bool   `json:"screen_read,omitempty"`

	// ── AND WHAT CAME OF IT ──
	Said     string   `json:"said,omitempty"`
	Next     string   `json:"next,omitempty"`
	Failed   bool     `json:"failed,omitempty"`
	Screens  []string `json:"screens,omitempty"`
	Controls []string `json:"controls,omitempty"`

	// Asked is how many times each picture has been handed to the page, by
	// "<routine>/<label>". The whole tally every time, so a reader takes the last one
	// and is correct by construction.
	Asked map[string]int `json:"asked,omitempty"`

	// ── AN ACT, AND A PERFORMANCE OF ONE ──
	Act   string   `json:"act,omitempty"`
	From  string   `json:"from,omitempty"`
	Steps []string `json:"steps,omitempty"`
	Error string   `json:"error,omitempty"`

	Live      bool   `json:"live,omitempty"`
	Reference string `json:"reference,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
	Because   string `json:"because,omitempty"`
	Sent      int    `json:"actions_sent,omitempty"`
	Verified  int    `json:"verified,omitempty"`
}

// trials is one Studio's journal, created the first time there is something to write in it.
//
// Lazily, because `-acts` builds the same surface to read somebody's store and report what it
// would show. A diagnostic that left a file behind in the store it was pointed at would be a
// diagnostic that changed the thing it was asked about.
type trials struct {
	dir                     string
	director, store, routes string
	build                   func() string

	mu   sync.Mutex
	path string
	seq  int
	// asked is the picture tally, and told the last state note written — both kept here
	// so a note is only written when it would say something new.
	asked map[string]int
	told  trialNote
	// tallied is when the tally was last written, and owing whether it has changed
	// since. See trialTally.
	tallied time.Time
	owing   bool
	// broken says writing has already failed once, so it is reported once.
	broken bool
}

// newTrials prepares a journal. Nothing is created until the first note.
func newTrials(home, director, routes string, build func() string) *trials {
	return &trials{dir: filepath.Join(home, "trials"), director: director,
		store: home, routes: routes, build: build, asked: map[string]int{}}
}

// note stamps one note and appends it, opening the journal if this is the first.
//
// A failure to write is NOT a failure of whatever was being done. Nothing in a flow depends on
// this file existing — a recording whose journal could not be written is a recording, and
// refusing it to protect the evidence would be the evidence deciding what the product does. Said
// once on stderr, which the console build shows.
func (t *trials) note(n trialNote) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.write(n)
}

// write is note's body, with the lock already held.
func (t *trials) write(n trialNote) {
	if t.broken {
		return
	}
	at := time.Now().UTC()
	if t.path == "" {
		if err := os.MkdirAll(t.dir, 0o755); err != nil {
			t.fail(err)
			return
		}
		// NAMED BY WHEN IT STARTED, so the newest sorts last by name and a
		// reader does not depend on modification times — a journal is appended
		// to for as long as the window is open, which makes mtime say when
		// somebody closed it.
		t.path = filepath.Join(t.dir, at.Format("20060102-150405")+".jsonl")
		// AND WHICH BUILD IS WRITING THE REST OF IT, as the first line, always.
		t.seq++
		t.put(trialNote{Seq: t.seq, At: at, Kind: trialOpened,
			Build: t.revision(), Director: t.director, Store: t.store,
			Routes: t.routes})
	}
	t.seq++
	n.Seq, n.At = t.seq, at
	if n.Build == "" {
		n.Build = t.revision()
	}
	t.put(n)
}

// put appends one line. The caller holds the lock and has opened the journal.
func (t *trials) put(n trialNote) {
	n.Version = trialVersion
	raw, err := json.Marshal(n)
	if err != nil {
		t.fail(err)
		return
	}
	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.fail(err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		t.fail(err)
	}
}

// fail says so once and stops trying.
func (t *trials) fail(err error) {
	if t.broken {
		return
	}
	t.broken = true
	fmt.Fprintf(os.Stderr, "studio: the trial journal could not be written, so this "+
		"flow will leave no evidence of itself: %v\n", err)
}

func (t *trials) revision() string {
	if t.build == nil {
		return ""
	}
	return t.build()
}

// recording is a demonstration starting.
func (t *trials) recording(name, application string, seconds int) {
	t.note(trialNote{Kind: trialRecording, Name: name,
		Application: application, Seconds: seconds})
}

// saw records the recorder's account of itself, when it differs from the last one shown.
//
// Called with the value handed to the page, so what is written is what the owner was shown. A
// `Learning` with no recording in it is not a state: the page polls this verb whether or not
// anything has ever been recorded, and a journal full of nothing would be a file saying how long
// Studio was left open.
func (t *trials) saw(l Learning) {
	if l.Name == "" || l.Phase == "" {
		return
	}
	n := trialNote{Kind: trialState, Name: l.Name, Application: l.Application,
		Phase: l.Phase, Doing: l.Doing, Presses: l.Presses,
		Watched: l.Watched, Captured: l.Captured, Unchecked: l.Unchecked,
		Missing: l.Missing, StartKnown: l.StartKnown, ScreenRead: l.ScreenRead,
		Recorder: l.Recorder, Mismatched: l.Mismatched}
	t.mu.Lock()
	defer t.mu.Unlock()
	// EVERYTHING BUT THE STAMP, which is the only thing that differs between two polls
	// of a recording that has not moved.
	if saysTheSame(t.told, n) {
		return
	}
	t.told = n
	t.write(n)
}

// recorded is what came of a demonstration, with the pictures the store kept of it.
func (t *trials) recorded(name string, said, next string, failed bool, kept Pictured) {
	t.note(trialNote{Kind: trialRecorded, Name: name, Said: said, Next: next,
		Failed: failed, Screens: kept.Screens, Controls: kept.Controls})
	t.flush()
}

// pictured counts one picture handed to the page.
//
// The count is the measurement: the page is supposed to ask for each picture once and keep the
// element, and the defect that made the owner say "the images blink in the ui" was nothing but
// this number being large. See trialRefetch and trialTally.
func (t *trials) pictured(routine, label string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.asked[routine+"/"+label]++
	t.owing = true
	if time.Since(t.tallied) < trialTally {
		return
	}
	t.tally()
}

// flush writes the tally if it owes one, at a seam where a reader will want it.
func (t *trials) flush() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.owing {
		return
	}
	t.tally()
}

// tally writes the whole current count. The caller holds the lock.
func (t *trials) tally() {
	asked := make(map[string]int, len(t.asked))
	for k, v := range t.asked {
		asked[k] = v
	}
	t.tallied, t.owing = time.Now(), false
	t.write(trialNote{Kind: trialPictures, Asked: asked})
}

// composed is an Act saved, or refused and why.
func (t *trials) composed(from, act string, steps []string, why string) {
	t.note(trialNote{Kind: trialAct, Act: act, From: from,
		Steps: append([]string(nil), steps...), Error: why})
	t.flush()
}

// playing is a performance started.
func (t *trials) playing(act string, live bool) {
	t.note(trialNote{Kind: trialPlay, Act: act, Live: live})
	t.flush()
}

// performed is the canonical result, from the record the run wrote.
//
// The reference comes from the run's own citation, and everything else from the record it is
// named by — not from anything this window concluded. `rec` is empty when there is no record,
// which is itself the answer to "what happened": a run that cited a reference and wrote no
// record is a run that did not finish.
func (t *trials) performed(act, reference string, live bool, rec record, had bool) {
	n := trialNote{Kind: trialRan, Act: act, Reference: reference, Live: live}
	if had {
		n.Outcome, n.Because = rec.Outcome, rec.Because
		n.Sent, n.Verified = rec.ActionsSent, rec.Verified
		n.Ran, n.Application = rec.Build, rec.Application
	}
	t.note(n)
	t.flush()
}

// saysTheSame says whether two notes would read identically, ignoring their stamps.
//
// Compared BY THEIR JSON, deliberately: what is being asked is whether the note would read
// differently, and the note is JSON. A field added to `trialNote` is therefore compared from the
// moment it exists, which a hand-written list of comparisons would not be — and a hand-written
// list is exactly the thing that goes stale quietly, leaving a journal that repeats a state
// which had changed in the one field nobody added to the list.
func saysTheSame(a, b trialNote) bool {
	bare := func(n trialNote) string {
		n.Seq, n.At, n.Version, n.Build = 0, time.Time{}, 0, ""
		// AND THE SECONDS LEFT ARE NOT A CHANGE, which is the same sentence
		// `learnstate.Say` makes about its own `Left` and is here for the same
		// reason. The preparation delay is the one stretch the recorder cannot
		// report on, so the window counts it down itself — "Switch to
		// ArkAscended.exe. Recording starts in 3s." — and that sentence differs
		// every second by construction. Comparing it would write a note per
		// second of a state that has not moved.
		if n.Phase == string(learnstate.Preparing) {
			n.Doing = ""
		}
		raw, _ := json.Marshal(n)
		return string(raw)
	}
	return bare(a) == bare(b)
}

// recordFor is the run record one reference names, as it sits on disk.
//
// The same file `History` lists and `Inspect` prints whole, read into the same struct, so the
// trial's numbers and the window's cannot disagree about one run.
func (a *Acts) recordFor(reference string) (record, bool) {
	if strings.TrimSpace(reference) == "" {
		return record{}, false
	}
	raw, err := os.ReadFile(filepath.Join(a.home, "runs", reference+".json"))
	if err != nil {
		return record{}, false
	}
	var r record
	if json.Unmarshal(raw, &r) != nil {
		return record{}, false
	}
	return r, true
}

// ── READING THEM BACK ─────────────────────────────────────────────────────────

// trial is one journal, parsed.
type trial struct {
	Path  string
	Notes []trialNote
	// Unreadable is how many lines could not be read, which is a fact about the
	// journal and not a reason to discard what could.
	Unreadable int
}

// trialsIn is the journals in a store, oldest first.
func trialsIn(home string) ([]trial, error) {
	dir := filepath.Join(home, "trials")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	out := make([]trial, 0, len(names))
	for _, name := range names {
		out = append(out, readTrial(filepath.Join(dir, name)))
	}
	return out, nil
}

// readTrial parses one journal, keeping the lines it can vouch for.
func readTrial(path string) trial {
	t := trial{Path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Unreadable++
		return t
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var n trialNote
		// A LINE THIS BUILD CANNOT VOUCH FOR IS COUNTED, NOT GUESSED AT. The
		// shape is the only thing that says what a missing field means; see
		// trialVersion.
		if json.Unmarshal([]byte(line), &n) != nil || n.Version != trialVersion {
			t.Unreadable++
			continue
		}
		t.Notes = append(t.Notes, n)
	}
	return t
}

// last is the newest note of a kind, and whether there was one.
func (t trial) last(k trialKind) (trialNote, bool) {
	for i := len(t.Notes) - 1; i >= 0; i-- {
		if t.Notes[i].Kind == k {
			return t.Notes[i], true
		}
	}
	return trialNote{}, false
}

// first is the oldest note of a kind, and whether there was one.
func (t trial) first(k trialKind) (trialNote, bool) {
	for _, n := range t.Notes {
		if n.Kind == k {
			return n, true
		}
	}
	return trialNote{}, false
}

// states is the recording states this trial passed through, in order.
func (t trial) states() []trialNote {
	var out []trialNote
	for _, n := range t.Notes {
		if n.Kind == trialState {
			out = append(out, n)
		}
	}
	return out
}

// builds is every build named anywhere in this trial, which parts of it never said, and whether
// the ones that did are one build.
//
// The question the owner should never have to answer by comparing file timestamps, asked of the
// whole trial rather than of one pair: the window, the recorder and the director that performed
// the run are three binaries, and any two of them can differ.
//
// An UNSTAMPED build is not a build identity, and it is not evidence of agreement either. A
// binary made without VCS information says "unknown", and two of those are not a match — which
// is `buildid.Same`'s own rule and the reason it is not the negation of `buildid.Differ`. So an
// unknown is reported as a part of the trial that did not say what it was, never folded in as a
// revision shared with another unknown.
func (t trial) builds() (named map[string][]string, silent []string, agree bool) {
	named = map[string][]string{}
	took := map[string]bool{}
	add := func(who, rev string) {
		took[who] = true
		if rev == "" || rev == "unknown" {
			return
		}
		for _, had := range named[rev] {
			if had == who {
				return
			}
		}
		named[rev] = append(named[rev], who)
	}
	for _, n := range t.Notes {
		// WHO TOOK PART IS DECIDED BY THE KIND OF NOTE, not by whether a
		// revision arrived: a recorder that said nothing about itself is the
		// case worth reporting, and a trial with no recording in it has no
		// recorder to report.
		add("the window", n.Build)
		if n.Kind == trialState {
			add("the recorder", n.Recorder)
		}
		if n.Kind == trialRan {
			add("the director that performed the run", n.Ran)
		}
	}
	for _, who := range named {
		sort.Strings(who)
	}
	for who := range took {
		stamped := false
		for _, these := range named {
			for _, had := range these {
				if had == who {
					stamped = true
				}
			}
		}
		if !stamped {
			silent = append(silent, who)
		}
	}
	sort.Strings(silent)
	return named, silent, len(named) <= 1
}

// refetched is the pictures asked for more than trialRefetch times, worst first.
func (t trial) refetched() (worst []string, most int, labels, total int) {
	n, ok := t.last(trialPictures)
	if !ok {
		return nil, 0, 0, 0
	}
	keys := make([]string, 0, len(n.Asked))
	for k := range n.Asked {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if n.Asked[keys[i]] != n.Asked[keys[j]] {
			return n.Asked[keys[i]] > n.Asked[keys[j]]
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		labels++
		total += n.Asked[k]
		if n.Asked[k] > most {
			most = n.Asked[k]
		}
		if n.Asked[k] > trialRefetch {
			worst = append(worst, fmt.Sprintf("%s asked for %d times",
				k, n.Asked[k]))
		}
	}
	return worst, most, labels, total
}
