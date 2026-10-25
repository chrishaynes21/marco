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
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
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
	// Live is whether the run was allowed to touch the machine, WHEN THE RECORD SAYS SO.
	//
	// A pointer, and the defect that made it one: a routine's record states no liveness at
	// all — `marco do` has no "write it down instead" mode to distinguish, so there was
	// nothing to state — and a plain bool read `false` for it. The page then labelled a
	// routine that really pressed things `input disabled`, which is not a missing field
	// rendered badly but a false claim about somebody's machine. Absent is not "no".
	//
	// A supervised record always states it, so an Act's row is unchanged.
	Live *bool `json:"live,omitempty"`
	// Sent is how many presses actually went out, or ABSENT when the record does not say,
	// and Verified how many were checked. The first is the one that matters after
	// something went wrong: it is the answer to "did it touch my machine".
	//
	// A pointer for the same reason `routineRun.Asked` is one where it is written: an
	// attempt that was interrupted may already have sent input and nobody wrote the number
	// down. Zero there is not a count, it is the claim that nothing was pressed — and it
	// would be shown for exactly the run a person is most likely to be asking about.
	Sent     *int `json:"sent,omitempty"`
	Verified int  `json:"verified"`
	// Authority is who authorised it, in words — "the person who pressed Play in Studio
	// at 16:05:44, authorising …" — which is the difference between a granted run and
	// one answered by key.
	Authority string `json:"authority,omitempty"`
	// Unfinished is a record that says a run began and never says how it ended.
	Unfinished bool `json:"unfinished,omitempty"`
	// Kind is which sort of run this was, when the record says: `routine` for a saved
	// routine performed by `marco do`, empty for a supervised Act.
	//
	// Carried to the page because the two answer different questions and a row that
	// looked the same for both would invite a reader to compare them. See
	// [[ADR-253-the-claim-is-shared-and-the-record-is-not]].
	Kind string `json:"kind,omitempty"`

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
	// Build is the revision of the director that performed the run.
	//
	// Not shown in the window's own listing — a person reading what happened last week
	// is not asking which binary it was — and read by the trial journal, which is
	// exactly that question. See trial.go.
	Build string `json:"build,omitempty"`
	// ActionsSent and Live are POINTERS because a record that does not carry them is a
	// record that does not say, and the zero value of each is a claim: no presses went
	// out, and the machine was never touched. See PastRun.Sent and PastRun.Live.
	ActionsSent *int   `json:"actions_sent"`
	Verified    int    `json:"verified"`
	Live        *bool  `json:"live"`
	Authority   string `json:"authority"`
	// Routine is the slug a ROUTINE's record names itself by.
	//
	// Its own key beside `workflow`, which is the supervised record's name for the
	// DEMONSTRATION a run's interactions came from. `PastRun` has carried a `routine`
	// field since before anything wrote one, and this is what fills it.
	RoutineName string `json:"routine,omitempty"`
	// Kind is the discriminator a routine's record carries and a supervised one does
	// not, because the supervised record was written before there were two kinds.
	// Absent therefore MEANS supervised, and that asymmetry is deliberate: inventing a
	// kind for the records already on somebody's machine would be rewriting them.
	Kind string `json:"kind,omitempty"`
	// InputActions is how many input actions a ROUTINE's run asked the machine for, or
	// nil when the record does not say.
	//
	// Its own key, not `actions_sent`. The supervised record's number is how many
	// programs reached a computer and this one is how many input actions were asked
	// for; they are close enough to be added up by a reader and different enough that
	// the sum would mean nothing. One key per meaning. See oshost.Host.InputActions.
	//
	// A pointer because it is WRITTEN as one: `marco` omits it when no host could be
	// asked, and an interrupted attempt's provisional record has not reached the count
	// yet. Reading that absence as an int put the three-zeroes claim back at the one end
	// of the wire that had been careful not to make it.
	InputActions *int `json:"input_actions"`
}

// routine reports whether a record was written by a routine's run.
//
// Asked of the record's own `kind` and nowhere else. Guessing from which fields are filled in
// would be this window deciding what kind of run somebody had, and the first record that broke
// the guess would be rendered in the wrong vocabulary — which is the one thing
// [[ADR-253-the-claim-is-shared-and-the-record-is-not]] exists to prevent.
func (r record) routine() bool { return r.Kind == runs.KindRoutine }

// History is the runs this machine has records of, newest first.
func (a *Acts) History() []PastRun {
	dir := filepath.Join(a.home, marcopaths.RunsDir)
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
		Reference: r.Run, Act: r.Act,
		Application: r.Application, Outcome: r.Outcome,
		Live: r.Live, Verified: r.Verified,
		Authority: r.Authority, Kind: r.Kind,
	}
	if at, err := time.Parse(time.RFC3339, r.Started); err == nil {
		p.started = at
		p.When = at.Local().Format("2006-01-02 15:04")
	}
	p.Unfinished = r.Ended == ""

	// ── AND EACH KIND IS READ IN ITS OWN VOCABULARY ──
	//
	// The defect this closes. Every record used to go through `saidOf` and `nextOf`,
	// which are the SUPERVISED seven words — so a routine's `performed` fell to their
	// default and a routine's run was described, and given a remedy, in the language of
	// evidence it never gathered. Not a conversion into `verified`, but the same mistake
	// one step quieter: an Act's sentence about a run that was not one.
	//
	// `saidOfRoutine` already exists and already says the load-bearing thing — every
	// press happened, and nothing here checked that the application did what you wanted.
	// So the only new thing is asking which kind this was. See ADR-253.
	//
	// AND NEITHER KIND IS GIVEN A NUMBER OR A MODE IT DID NOT STATE. `Sent` and `Live`
	// are carried across as the record wrote them — present or absent — because the page
	// renders the absence as absence. See PastRun.Live, which is the field a routine's
	// run was being labelled `input disabled` by.
	if r.routine() {
		p.Routine = r.RoutineName
		p.Sent = r.InputActions
		p.Said = saidOfRoutine(r.RoutineName, r.Outcome, r.Because, false)
		p.Next = nextOfRoutine(r.Outcome)
		return p
	}
	p.Routine = r.Workflow
	p.Sent = r.ActionsSent
	name := r.Act
	if name == "" {
		name = r.Workflow
	}
	p.Said = saidOf(name, runref.Outcome(r.Outcome), r.Because)
	// NO `waits` HERE. What an Act would wait for is a fact about the Act as it stands
	// now, and this is a run that happened; offering today's remedy for a run from last
	// week would be attributing one to the other. The Act's own listing says it.
	p.Next = nextOf(runref.Outcome(r.Outcome), "")
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

	// ── AND WHAT IT SAID, ONLY WHEN IT WAS THIS RUN THAT SAID IT ──
	//
	// The transcript is this window's memory of ONE child, cleared and refilled every time
	// something runs. Copying it in unconditionally meant that playing B and then reporting
	// A wrote a file holding A's reference and A's record with B's output underneath,
	// headed as what A said — and the person attaching it to an email has no way to tell.
	// A wrong transcript is worse than none: it is evidence about the wrong run, and this
	// is the file somebody sends when they cannot explain what happened.
	//
	// So the lines have an owner and it is asked. See `said`.
	lines, owner := a.said()
	b.WriteString("\nWHAT IT SAID AS IT RAN\n")
	if len(lines) > 0 && owner == reference {
		b.WriteString(indented(strings.Join(lines, "\n")))
	} else {
		// SAID, AND NOT LEFT OUT. A section silently missing reads as a report that
		// forgot, and the reason is worth a line: a run older than this window has
		// no transcript here, and one that has been replaced is somebody else's.
		b.WriteString("  not available. This window keeps only what the run in " +
			"front of it said, and that is not this run's output — either " +
			"something has run since, or this run was never watched from " +
			"here. The record above is the durable account.\n")
	}
	// ── AND THE THING IT WAS A RUN OF, AS IT STANDS NOW ──
	//
	// For an Act that is the composition; for a routine it is the SOURCE, which is the
	// executable truth about a routine and the only thing a reader can judge the run
	// against. Each is found by the record's own kind rather than by whichever of them
	// this window happens to have open.
	if kind, said := rec["kind"].(string); said && kind == runs.KindRoutine {
		a.describeRoutine(&b, rec)
	} else {
		a.describeAct(&b, rec)
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
		"the routine or Act as it stands now. Nothing has been sent anywhere — open it, add " +
		"what you expected to happen, and send it yourself."}
}

// said is this window's live transcript and the reference it belongs to.
//
// # Why the owner comes back with the lines
//
// Because the pair is the whole answer. A transcript read now and a reference read a moment later
// could be two different runs', and the question `Report` is asking — "is this still the run whose
// output I am holding" — cannot be answered with half of it.
//
// Each kind of run keeps its reference in its own place: an Act's is read off the child's citation
// into `a.reference`, a routine's into `a.routine.reference`, and `reserve` clears both when a new
// run claims the slot. Whichever is set owns whatever is in `a.transcript`.
func (a *Acts) said() (lines []string, owner string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	lines = append([]string(nil), a.transcript...)
	owner = a.reference
	if owner == "" && a.routine != nil {
		owner = a.routine.reference
	}
	return lines, owner
}

// describeAct writes the Act a run was a run OF into the report, as it stands now.
//
// Named by the RECORD and not by whichever Act this window has open — the same correction the
// transcript needed one section up, for the same reason: a report written an hour later about a
// run from this morning has to describe the Act that ran. Reporting an older run while a different
// Act was loaded printed that other Act's composition under this heading.
//
// A record with no Act in it was not a run of one, and nothing is printed for it.
func (a *Acts) describeAct(b *strings.Builder, rec map[string]any) {
	name, _ := rec["act"].(string)
	if name == "" {
		return
	}
	for _, c := range a.Saved() {
		if !strings.EqualFold(c.Name, name) {
			continue
		}
		shown, _ := json.MarshalIndent(c, "", "  ")
		b.WriteString("\nTHE ACT, AS IT STANDS NOW\n")
		b.WriteString(indented(string(shown)))
		return
	}
	// GONE. Worth saying for the reason the routine's own section says it: a report with
	// the section silently missing looks like a report that forgot to gather it.
	fmt.Fprintf(b, "\nTHE ACT, AS IT STANDS NOW\n  %q is not composed in this library "+
		"any more, so there is nothing to show beside the run.\n", name)
}

func indented(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n") + "\n"
}

// Forgotten is what a deletion did, or what pressing again would do.
type Forgotten struct {
	// Asks is what would be removed, when nothing has been removed yet. Empty once
	// something has: a sentence in the future tense beside a count in the past tense
	// would be the window describing two different moments at once.
	Asks string `json:"asks,omitempty"`
	// Deleted is how many records went, and Kept how many are left in the directory.
	Deleted int    `json:"deleted"`
	Kept    int    `json:"kept"`
	Error   string `json:"error,omitempty"`
}

// Forget removes run records: one by reference, or every one when the reference is empty.
//
// # Why it asks first
//
// Because a run record is the only account of what Marco did to somebody's machine, and the
// one thing `Report` has to gather from. Deleting it cannot be undone and nothing else holds
// what it held. So the first call removes nothing and says what would go, and the second —
// `anyway` — does it. That is the shape `Done` already uses for the other irreversible thing
// this window offers, and one idiom for both is one thing to learn.
//
// # Why the reference is matched and never joined
//
// Because it arrives from the page, and a path built from it would be a path somebody could
// aim. `filepath.Join(dir, reference + ".json")` with a reference of `../../routes/os` is a
// deletion outside the store, and no amount of checking the string afterwards is as good as
// not building the path at all. So this reads the directory and compares NAMES: a record is
// removed only if the directory actually offered it, which makes the traversal impossible
// rather than guarded against.
//
// # What it does not touch
//
// A problem report. `Report` writes a file for the person to send, and it is theirs — deleting
// the run it describes is not a reason to take back something they asked for and may be about
// to attach to an email. Orphaned on purpose, and said so in `Asks`.
func (a *Acts) Forget(reference string, anyway bool) Forgotten {
	reference = strings.TrimSpace(reference)

	// ── A RUN IN FLIGHT IS NOT A PAST RUN ──
	//
	// Its record is being written while this reads the directory, and the runner owns it
	// until it has said how it ended. Removing it would leave a live child writing to a
	// path nothing lists, and the history would then be missing the one run somebody is
	// most likely to ask about.
	a.mu.Lock()
	busy, running := a.busy, a.reference
	a.mu.Unlock()
	if busy && (reference == "" || reference == running) {
		return Forgotten{Error: "a run is in flight; stop it first, or wait for it " +
			"to say how it ended"}
	}

	dir := filepath.Join(a.home, marcopaths.RunsDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Forgotten{Error: "there are no runs on this machine"}
		}
		return Forgotten{Error: err.Error()}
	}
	// THE RECORDS THE DIRECTORY ACTUALLY OFFERS, and the ones this call is about.
	var mine []string
	kept := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		kept++
		if reference != "" && strings.TrimSuffix(e.Name(), ".json") != reference {
			continue
		}
		mine = append(mine, e.Name())
	}
	if len(mine) == 0 {
		if reference == "" {
			return Forgotten{Error: "there are no runs on this machine"}
		}
		return Forgotten{Error: fmt.Sprintf("no record of %q is kept here", reference)}
	}

	if !anyway {
		what := fmt.Sprintf("%d run record(s)", len(mine))
		if reference != "" {
			what = "the record of " + reference
		}
		return Forgotten{Kept: kept, Asks: "This removes " + what + " for good, and " +
			"a run record is the only account of what Marco did — it is what " +
			"Report gathers. Any problem report you have already written is " +
			"left where it is."}
	}

	gone, failed := 0, ""
	for _, name := range mine {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			failed = err.Error()
			continue
		}
		gone++
	}
	return Forgotten{Deleted: gone, Kept: kept - gone, Error: failed}
}

// describeRoutine writes the routine a run was a run OF into the report, as it stands now.
//
// # And whether it is still what ran
//
// The one thing a routine's problem report can say that nothing else can. The record carries the
// digest of the source that actually ran; the library carries the file. If they differ, the
// routine has been saved again since that attempt — so the source printed below is NOT the source
// that failed, and a person comparing the two without being told would be reading the wrong
// program. Re-recording is exactly what somebody does between a failure and reporting it, which
// makes this the likely case rather than the unlikely one.
//
// Found by the identity in the record rather than by whichever routine this window has selected:
// a report written an hour later, about a run from this morning, must describe the routine that
// ran and not the one somebody is looking at.
func (a *Acts) describeRoutine(b *strings.Builder, rec map[string]any) {
	text := func(key string) string {
		s, _ := rec[key].(string)
		return s
	}
	slug, app := text("routine"), text("application")
	if slug == "" {
		return
	}
	rt := routes.Route{Slug: slug, App: app, Focus: text("scope") == routes.FocusDir}
	raw, err := os.ReadFile(a.reg().Path(rt))
	if err != nil {
		// GONE, OR SOMEWHERE ELSE. Worth saying: a run whose routine cannot be found
		// is a different finding from one whose routine still reads oddly, and a
		// report with the section silently missing looks like a report that forgot.
		fmt.Fprintf(b, "\nTHE ROUTINE, AS IT STANDS NOW\n  %s is not in this "+
			"library (%s) any more: %v\n", routeID(rt), a.routesDir, err)
		return
	}
	src := string(raw)
	b.WriteString("\nTHE ROUTINE, AS IT STANDS NOW\n")
	if ran, now := text("source_digest"), routes.DigestOf(src); ran != "" && ran != now {
		fmt.Fprintf(b, "  THIS IS NOT THE SOURCE THAT RAN. The run recorded source "+
			"%s and the file now reads as %s, so it has been saved again since "+
			"that attempt — read what follows as the routine as it is today, not "+
			"as the one that failed.\n\n", ran, now)
	}
	fmt.Fprintf(b, "  %s (%s)\n", routeID(rt), a.reg().Path(rt))
	b.WriteString(indented(src))
}
