package main

// THE ACT SURFACE STUDIO IS FOR: compose, select, prepare, run, cancel, inspect.
//
// A vertical slice, deliberately thin, over the path the Architect approved — and over the SAME
// functions the command-line path uses, not a second implementation of any of them:
//
//	compose   demonstrated.Compose + demonstrated.Save   the one generator, the one save
//	select    routes.Registry + demonstrated.Load        the plays on this machine
//	prepare   demo.Store.Sequences                        the demonstrations and their steps
//	run       director perform-workflow --play …          the production supervised runner
//	cancel    stopsignal.Raise                            the one cross-process stop
//	inspect   the run record this run wrote               by its canonical reference
//
// # Why running is a child process rather than a library call
//
// Because a supervised run needs a PERSON at the consent desk, and the desk reads F7/F8/F9 from a
// global low-level keyboard hook and reports on a transcript. That is `director
// perform-workflow`'s own composition root: the live observer, the person who grounds a screen,
// the person who permits a press, the executor, the stop key, the run record. Reaching into it
// from here would be a second place where all of that is assembled, and a second place is how two
// answers to "may this be pressed" come to exist.
//
// So Studio spawns it and shows what it says, which is exactly what `cmd/marco` already does for
// a demonstrated name (`runDirectorCommand`, one pipe for both streams, `runref.Parse` over the
// lines). The keyboard answers go to the real desk on the real hook; Studio is the window that
// shows the person what they are answering.
//
// # What it will not do
//
//   - press anything on its own. `--live` is a toggle that defaults OFF and says what off means.
//   - decide anything. Every refusal shown here came from the production code that refused.
//   - hold a question. The desk's prompts are the desk's; this renders the transcript.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// Acts is everything the page can ask about Acts and demonstrations.
//
// Separate from `Backend` because they answer to different things: `Backend` speaks to the
// Director SERVICE for the feasibility proof, and this speaks to the stores and to the supervised
// runner. Nothing here uses the service at all.
type Acts struct {
	// home is the Marco store. Resolved once; see storeDir.
	home string
	// routes is the library: $MARCO_ROUTES, else `routes` inside the store, resolved by
	// the one place that answers that for every binary. See internal/marcopaths.
	routesDir string

	mu sync.Mutex
	// busy is the RESERVATION, taken before a child is started and released only once
	// that child has exited AND its transcript has been drained.
	//
	// # Why a reservation rather than a pointer to the child
	//
	// Because the first version checked `running != nil`, released the lock, started the
	// process, and only then stored it. Two calls arriving together both passed the check
	// and both started a supervised runner — two sets of consent questions on one
	// keyboard, where F7 answers whichever desk happens to be listening. The page's Run
	// button is disabled from the next poll, which is up to 400ms of open window.
	//
	// So the slot is claimed under the same lock that tested it, and it stays claimed
	// until there is nothing left of the run: a child that has exited with output still
	// in the pipe is a run whose transcript is still arriving.
	busy bool
	// running is the child, while there is one. Nil between runs and after shutdown.
	running *exec.Cmd
	// saying is where this run writes its structured state, and said whether anything
	// was ever read from it. One file per run, in a temporary directory: it is a
	// conversation between two processes and not something a person keeps.
	saying string
	// stopRun cancels the child's context, and done closes when the run has fully
	// finished — exited and drained. Both nil between runs. See Close.
	stopRun func()
	done    chan struct{}
	// act and live are what THIS run is, kept because the page's controls are not a
	// description of a run already in flight: a person who ticks the live box while
	// something is running must not be shown a dry run as live, or the other way about.
	act  string
	live bool
	// kind is WHICH KIND of run the slot is holding — an Act's supervised performance or
	// a saved routine's — and routine is the second one's account of itself, nil for the
	// first. Both outlive the run, because the window still has to describe one that has
	// ended.
	//
	// The page needs the distinction because the two can say different amounts about
	// themselves: an Act's run cites a canonical reference, publishes its own state and
	// can be asked a question, and a routine's run has a transcript and an answer. A
	// window that rendered the second as the first would report "the run cited no
	// reference, so there is no canonical account of it" about a routine that behaved
	// perfectly. See routines.go.
	kind    string
	routine *routineRun
	// closing says shutdown has begun, and it is the gate on STARTING anything.
	//
	// # The race this closes
	//
	// `Run` claimed the slot, released the lock, built a pipe, started the process, and
	// only then published the child. `Close` returned at once when it saw no child — so a
	// window closed in that interval let the launch publish a runner afterwards, with
	// nothing left to stop it and cleanup already proceeding. A supervised runner alive
	// after the window that was asking its questions is gone.
	//
	// So shutdown covers the slot's WHOLE lifetime. Once this is set no claim may be
	// taken, a claim already taken aborts before anything is started, and `Close` does
	// not return while a launch could still publish a child.
	closing bool
	// afterReserve runs between claiming the slot and starting the child, and exists
	// only so a test can stand inside that interval.
	//
	// A seam for a timing window rather than a seam for behaviour: the interval is real,
	// it is where the race lived, and there is no way to be inside it from outside. Nil
	// in every production path.
	afterReserve func()
	// raising stands in for bringing the application forward, which needs a real one
	// running on a real desktop. Nil in every production path; see play.go.
	raising func(app string) string
	// listing and lifting stand in for the two halves of bringing an application
	// forward: what windows there are, and raising one. Nil in every production path;
	// see play.go.
	listing func() []windowref.Candidate
	lifting func(h uintptr, pid uint32) (leading, checked bool)
	// revision is this window's own build, substitutable for the same reason the
	// three above are: a test binary carries no VCS stamp, so the one pair this
	// has to get right — two builds that genuinely differ — cannot otherwise be
	// made to happen at all. Nil means ask the binary.
	revision func() string
	// trial is this Studio's own account of what a trial of it showed: which builds
	// took part, the states a recording passed through, how often the page had to ask
	// for a picture, and the canonical result of a run. Written as it happens, because
	// the alternative is the owner being asked to watch for four things while they are
	// demonstrating a fifth. See trial.go.
	trial *trials
	// learn is the demonstration being recorded, or the last one. Nil until something
	// has been. A recording and a run are both one-at-a-time and exclude each other:
	// there is one screen, and a recorder watching it while a runner presses on it
	// would be Marco demonstrating to itself. See learn.go.
	learn *learning
	// capture is the recorder-first recording in flight, or the draft waiting to be
	// looked at. Nil until something has been recorded that way and again once it is
	// saved or cancelled.
	//
	// A SECOND KIND OF RECORDING, and the reason `recordingLocked` exists: everything
	// that must not happen while the screen is being watched used to ask about `learn`
	// by hand. See recording.go.
	capture *capture
	// recording stands in for the input recorder, substitutable for the same reason
	// `raising` and `listing` are: the real one installs low-level hooks and needs a
	// person's hands on a real desktop, and the claim worth testing — that pressing
	// Stop in this window does not become a step — cannot otherwise be made to happen
	// at all. Nil in every production path; see recording.go.
	recording func() recorder.Recorder
	// transcript is what the run has said so far, which is the whole of what a person
	// watching it has to go on.
	transcript []string
	// reference is this run's canonical identity, as the runner itself printed it.
	reference string
	// outcome is how it ended, once it has.
	outcome string
}

// marcoChecks answers `demonstrated.Plays` over the production checker, exactly as cmd/director
// does. One line, so the question a saved play is checked with is the one the runtime asks.
type marcoChecks struct{}

func (marcoChecks) Check(src string) error { return driver.CheckSource(src) }

// newActs resolves the stores this Studio reads.
func newActs(home string) *Acts {
	// THE LIBRARY, AND NOT A DIRECTORY BESIDE WHOEVER LAUNCHED THIS.
	//
	// This used to resolve `$MARCO_ROUTES, else "routes"` in its own words, relative to the
	// working directory — so a Studio double-clicked in `plugins\studio` kept its routines
	// in `plugins\studio\routes`, where nothing else in the product would ever look, and
	// the instruction for it was "launch both from the repo root". A library is a place.
	// See internal/marcopaths.
	a := &Acts{home: home, routesDir: marcopaths.Routes()}
	// AND IT WRITES DOWN WHAT A TRIAL OF ITSELF SHOWED, which is the alternative to
	// asking the owner to watch for four things at once. Nothing is created until there
	// is something to record; `a.myBuild` rather than `buildid.Revision` so a test's
	// substituted revision is the one written. See trial.go.
	a.trial = newTrials(a.home, directorBin(), a.routesDir, a.myBuild)
	return a
}

// ── PREPARE: what there is to compose from ────────────────────────────────────

// Interaction is one step of a demonstration, as a person choosing has to see it.
type Interaction struct {
	Position int    `json:"position"`
	Says     string `json:"says"`
	// ControlLabel, FromLabel and ToLabel are the artifact’s own labels for what this
	// step presses and the screens on either side of it.
	//
	// # Why the labels are here after all, having been deliberately left out
	//
	// Because they are KEYS and not text. `control_2` is a name this machine gave
	// itself and showing it to somebody tells them nothing — that is why `Reads` and
	// `On` exist and why those are what the page displays. But the picture of that
	// control is filed under exactly that label, so asking for it needs the label.
	//
	// They are never rendered. A page that printed one would be back to showing somebody
	// the machine’s word for a thing they have a word of their own for. See Picture.
	ControlLabel string `json:"control_label,omitempty"`
	FromLabel    string `json:"from_label,omitempty"`
	ToLabel      string `json:"to_label,omitempty"`
	// Pressable says this step presses something. A step that pressed nothing cannot be
	// an interaction in an Act, and a person needs to see which is which rather than
	// find out from a refusal.
	Pressable bool `json:"pressable"`
	// Verifiable says the demonstration recorded an outcome for it, which is what makes
	// a verdict possible at all. Without one the interaction can never be verified, so an
	// Act cannot continue past it — see ADR-243. Shown because it is the single most
	// surprising thing about running an Act, and the one a person should know BEFORE
	// composing rather than after a run stops.
	Verifiable bool `json:"verifiable"`
	// Recognisable says the screen this step STARTS on has enough words for a run to
	// identify it — `observe.ScreenText.Identifies`, the same call the run makes.
	//
	// Separate from `On`, which is a SAMPLE of those words for a person to read: six of
	// them at most, so its length says nothing about whether the screen has the three a
	// run needs. This surface asked `len(On) > 0` and so called a two-word screen ready,
	// which is a demonstration presented as ready whose Act can only ever wait.
	Recognisable bool `json:"recognisable"`
	// Reads is the words the demonstration read ON the control it pressed.
	//
	// # Why this is here and `control_2` is not
	//
	// Because `control_2` is a name this machine gave itself. A person choosing which
	// interaction they want performed cannot recognise it, cannot tell it from
	// `control_1`, and has no way to know that one of them is the row of tabs across the
	// top of their own settings screen. The words are the only thing in the artifact that
	// exists on their screen as well as in the file.
	//
	// Empty when the demonstration read nothing there, which is honest and used to be
	// the only case: `Control.Text` went unwritten by every recorder for a while, so an
	// older artifact says `control_2` and nothing else. A surface that only ever showed
	// the label would have hidden the difference between the two.
	Reads []string `json:"reads,omitempty"`
	// On is some of the words read on the screen this step STARTS on, the steadiest
	// first, and empty when the demonstration read none there.
	//
	// It is the only recognisable name that screen has, and its absence is a warning.
	// `screen_a` is a name this machine gave itself — the owner's verdict on reading a
	// list of it was "it's screen_a to screen_b, how am I supposed to know what that
	// means" — and Marco is in the same position: an Act's starting screen is
	// established by matching the words on it, so a saved screen with none cannot be
	// positively identified and a run over it pauses rather than pressing. See onWordsOf.
	On []string `json:"on,omitempty"`
	// OutcomeWords is how many words the demonstration saw appear and STAY after this
	// press, and OutcomeSeen a few of them.
	//
	// The count is the evidence — five is the floor a verdict needs — and the sample is
	// what makes it recognisable: "61 words, among them advanced, bloom, brightness" is a
	// thing somebody can look at their own screen and agree with. `Verifiable` is the
	// same fact as a yes or no, kept because a page needs both.
	OutcomeWords int      `json:"outcome_words"`
	OutcomeSeen  []string `json:"outcome_seen,omitempty"`
}

// Demonstration is one saved routine and its interactions.
type Demonstration struct {
	Name        string        `json:"name"`
	Application string        `json:"application"`
	SavedAt     string        `json:"saved_at"`
	Steps       []Interaction `json:"steps"`
	// Problem is why it cannot be read, when it cannot. A routine that will not load is
	// a file to look at; one that was never saved is a demonstration to give.
	Problem string `json:"problem,omitempty"`
}

// Demonstrations is everything somebody has shown Marco, with the interactions of each.
func (a *Acts) Demonstrations() []Demonstration {
	store, err := demo.Open(a.home)
	if err != nil {
		return []Demonstration{{Name: "(the store could not be opened)",
			Problem: err.Error()}}
	}
	saved := store.Sequences()
	out := make([]Demonstration, 0, len(saved))
	for _, art := range saved {
		d := Demonstration{Name: art.Name,
			Application: string(art.Application),
			SavedAt:     art.SavedAt.Local().Format("2006-01-02 15:04"),
			Problem:     store.SequenceProblem(art.Name)}
		for _, s := range art.Steps {
			in := Interaction{
				Position:   s.Position,
				Says:       saysOf(s),
				Pressable:  s.Control != "",
				Verifiable: len(s.Outcome) > 0,
				Reads:      readsOf(art, s),
				On:         onWordsOf(art, s),
				// WHETHER A RUN COULD NAME THE SCREEN IT STARTS ON,
				// which `On` cannot answer: it is six words at most.
				Recognisable: recognisableFrom(art, s),
				// THE KEYS ITS PICTURES ARE FILED UNDER, not anything
				// to show. See Interaction.ControlLabel.
				ControlLabel: s.Control,
				FromLabel:    s.From,
				ToLabel:      s.To,
			}
			in.OutcomeWords = len(s.Outcome)
			in.OutcomeSeen = someOf(s.Outcome, outcomeShown)
			d.Steps = append(d.Steps, in)
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// readsOf is the words the demonstration read on the control this step pressed.
//
// Capped, and in the artifact's own order: the first few are what a person reads first on their
// own screen, and a row of tabs can carry a dozen. Empty for a step that pressed nothing, which
// is the one case where there is no control to have read anything on.
func readsOf(art workflowstore.Artifact, s workflowstore.Step) []string {
	if s.Control == "" {
		return nil
	}
	for _, c := range art.Controls {
		if c.Label == s.Control {
			return wordsToShow(c.Text, controlWordsShown)
		}
	}
	return nil
}

// recognisableFrom reports whether a run could identify the screen a step starts on.
//
// The artifact's own accumulated vocabulary for that screen, through the predicate the run
// itself uses. Deliberately NOT a count of `On`: that is a sample for somebody to read, cut to
// `screenWordsShown`, and a surface which inferred a judgement from the length of a display
// sample would be a surface whose verdict changed when the sample did.
func recognisableFrom(art workflowstore.Artifact, s workflowstore.Step) bool {
	for _, sc := range art.Screens {
		if sc.Label == s.From {
			return sc.Said().Identifies()
		}
	}
	return false
}

// onWordsOf is some of the words the demonstration read on the screen a step starts on.
//
// Empty means two things at once, and they are the same thing: nothing to show a person, and
// nothing for Marco to match. See the field it fills.
func onWordsOf(art workflowstore.Artifact, s workflowstore.Step) []string {
	for _, sc := range art.Screens {
		if sc.Label != s.From {
			continue
		}
		said := sc.Said()
		if !said.Said() {
			return nil
		}
		return wordsToShow(byHowOftenSeen(said.Words), screenWordsShown)
	}
	return nil
}

// byHowOftenSeen orders a screen's words by how many looks read them, the steadiest first.
//
// A stored screen's words are accumulated over every look that reached it, so the count is how
// reliably each one was there: a word read on every look is one the person will see too, and a
// word read once is as likely to be an OCR ghost. Ties go alphabetically, because a label that
// reshuffles itself on every Refresh is a label nobody can use to recognise anything.
func byHowOftenSeen(words map[string]int) []string {
	out := make([]string, 0, len(words))
	for w := range words {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if words[out[i]] != words[out[j]] {
			return words[out[i]] > words[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// wordsToShow is the first n words worth showing somebody.
//
// Words of one or two characters are dropped while there are longer ones to show: reading a
// game's own UI produces `od` and `ul` beside `cosmetics` and `graphics`, and a label that says
// "audio · camera · cosmetics · graphics · od · ul" has spent two of its six on noise and
// reads like noise entirely. Not a wordlist and not per-application — a length, applied to every
// screen and control alike.
//
// If nothing survives it, the short words are shown anyway: a control whose only word is "OK"
// reads "OK", and saying nothing about it would be worse than two characters.
func wordsToShow(all []string, n int) []string {
	kept := make([]string, 0, n)
	for _, w := range all {
		if len([]rune(w)) > 2 {
			kept = append(kept, w)
		}
		if len(kept) == n {
			return kept
		}
	}
	if len(kept) > 0 {
		return kept
	}
	return someOf(all, n)
}

// controlWordsShown and outcomeShown are how many words of each a person is shown.
//
// Enough to recognise the thing and not enough to be a wall of OCR. The outcome's COUNT is shown
// in full beside its sample, so nothing is hidden by the cap — a person sees "61 words" and three
// of them, rather than three words and no idea how much was behind them.
const (
	controlWordsShown = 6
	screenWordsShown  = 6
	outcomeShown      = 3
)

// someOf is the first n of a list, copied.
//
// Copied because these are the artifact's own slices and a surface that handed its caller a
// window onto the store's memory would be one edit away from changing it.
func someOf(all []string, n int) []string {
	if len(all) > n {
		all = all[:n]
	}
	return append([]string(nil), all...)
}

// saysOf is a step in words, preferring the demonstration's own sentence.
func saysOf(s workflowstore.Step) string {
	if s.Says != "" {
		return s.Says
	}
	if s.Control == "" {
		return fmt.Sprintf("%s to %s — nothing was pressed", orElse(s.From, "somewhere"),
			orElse(s.To, "somewhere"))
	}
	return fmt.Sprintf("press %s on %s", s.Control, orElse(s.From, "the screen"))
}

func orElse(s, then string) string {
	if s == "" {
		return then
	}
	return s
}

// ── COMPOSE: an Act from interactions somebody demonstrated ───────────────────

// Composed is what came of composing, for the page to render.
type Composed struct {
	Act string `json:"act"`
	// Play is the generated source. Shown, always: an Act is legal Marco so that a person
	// can read what they saved and recognise it.
	Play     string `json:"play"`
	Bindings string `json:"bindings"`
	Path     string `json:"path"`
	Error    string `json:"error,omitempty"`
}

// Compose writes an Act from a demonstration, in the order the interactions are given.
//
// `picked` is `"<position>=<name>"` per interaction, in run order — the same shape the command
// takes, for the same reason: the position says which demonstrated interaction is meant and the
// name is the person's own words, which is what the play will say.
// AND WHAT WAS COMPOSED IS WRITTEN DOWN, EITHER WAY.
//
// A refusal is the more interesting of the two: an Act that could not be saved from a
// demonstration which looked fine is the shape of the one limitation this surface knows it has —
// a press whose destination has nothing readable on it can be demonstrated and cannot anchor an
// Act. So the note is taken out here, around every path, rather than at the points that refuse:
// a refusal nobody wrote down is the half of a trial that turns into a conversation. See trial.go.
func (a *Acts) Compose(from, act string, picked []string) Composed {
	out := a.compose(from, act, picked)
	names := make([]string, 0, len(picked))
	for _, p := range picked {
		_, name, cut := strings.Cut(p, "=")
		if !cut {
			// MALFORMED, AND KEPT AS IT ARRIVED. What the page sent is the
			// evidence; tidying it here would hide the thing being reported.
			name = p
		}
		names = append(names, strings.TrimSpace(name))
	}
	a.trial.composed(from, act, names, out.Error)
	return out
}

// compose is Compose's answer.
func (a *Acts) compose(from, act string, picked []string) Composed {
	store, err := demo.Open(a.home)
	if err != nil {
		return Composed{Error: err.Error()}
	}
	art, ok := store.Sequence(from)
	if !ok {
		if why := store.SequenceProblem(from); why != "" {
			return Composed{Error: fmt.Sprintf("%q cannot be read: %s", from, why)}
		}
		return Composed{Error: fmt.Sprintf("nothing called %q has been saved", from)}
	}
	named := make([]demonstrated.Named, 0, len(picked))
	for _, p := range picked {
		at, name, cut := strings.Cut(p, "=")
		if !cut || strings.TrimSpace(name) == "" {
			return Composed{Error: fmt.Sprintf("%q needs a name: write it as "+
				"<interaction>=<what you call it>", p)}
		}
		var n int
		if _, perr := fmt.Sscanf(strings.TrimSpace(at), "%d", &n); perr != nil {
			return Composed{Error: fmt.Sprintf("%q is not an interaction number", at)}
		}
		named = append(named, demonstrated.Named{Position: n,
			Name: strings.TrimSpace(name)})
	}
	// AND AN ACT IS NOT ONE OF ITS OWN INTERACTIONS. Both become declarations in the play
	// this writes, so one name for two things is a compile error that reads `graph: 16:1:
	// duplicate declaration: AudioTab` — true, and not a sentence to hand somebody
	// composing their first Act.
	for _, n := range named {
		if strings.EqualFold(strings.TrimSpace(n.Name), strings.TrimSpace(act)) {
			return Composed{Error: fmt.Sprintf("this Act and one of its "+
				"interactions are both called %q — give the Act a name of "+
				"its own", n.Name)}
		}
	}
	src, binds, cerr := demonstrated.Compose(art, act, named, time.Now(), marcoChecks{})
	if cerr != nil {
		return Composed{Error: cerr.Error()}
	}
	rt := routes.Route{Slug: routes.Slug(act)}
	if serr := demonstrated.Save(a.reg(), rt, src, binds, time.Now()); serr != nil {
		return Composed{Error: serr.Error()}
	}
	raw, _ := binds.Marshal()
	return Composed{Act: act, Play: src, Bindings: string(raw),
		Path: a.reg().Path(rt)}
}

// applicationFor is the executable an Act was demonstrated against, or "".
//
// From the Act's own bindings, which is where the demonstration's selector was recorded when it
// was composed. Asking a person to retype it is asking for something already written down, and
// a typo in it is a run that grounds against the wrong window or refuses for a reason that is
// about a selector rather than about their Act.
func (a *Acts) applicationFor(act string) string {
	for _, rt := range a.reg().List() {
		_, binds, err := demonstrated.Load(a.reg(), rt)
		if err != nil {
			continue
		}
		if binds.Act == act || rt.Slug == act {
			return binds.Application
		}
	}
	return ""
}

// waitsFor is why a run of this Act would wait rather than press, or "".
//
// Asked of the Act as it stands now, which is the only honest time to ask it: the remedy for a
// run that waited is a change to the demonstration, and whether that change has been made is a
// question about today.
// startsOnFor is what the screen an Act starts on was read to say, for the run screen.
//
// The same reading the Act’s own detail shows, read from the same place, because a person
// checking where they are standing mid-run must not be shown a different answer from the one
// they read before they pressed Play.
func (a *Acts) startsOnFor(act string) []string {
	if act == "" {
		return nil
	}
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, act) {
			return c.StartsOn
		}
	}
	return nil
}

func (a *Acts) waitsFor(act string) string {
	if act == "" {
		return ""
	}
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, act) {
			return c.Waits
		}
	}
	return ""
}

// reg is the route store, built the same way director's is.
func (a *Acts) reg() routes.Registry { return routes.Registry{Dir: a.routesDir} }

// ── SELECT and INSPECT: the Acts on this machine, and whether they will run ───

// Act is one saved play, as a person choosing one has to see it.
type Act struct {
	Name string `json:"name"`
	// From and Revision are the demonstration it performs and the revision it is pinned
	// to. Both, because a record with only the first cannot tell two Acts over one
	// demonstration apart.
	From     string `json:"from"`
	Revision string `json:"revision"`
	// Interactions is what it performs, in the order it performs them, and Steps is
	// the same list with what each one does.
	//
	// Both, because they answer different questions: the names are the Act, and the
	// steps are what a person is about to let happen — the control each name presses,
	// the screen it is on, and what the demonstration saw change afterwards. An ordered
	// preview of that is the whole of "what will this do", and it can be shown without
	// running anything, which a dry run cannot claim: a dry run of two interactions
	// never performs the first, so it cannot reach the second.
	Interactions []string  `json:"interactions"`
	Steps        []ActStep `json:"steps,omitempty"`
	// StartsOn is the words read on the screen its first interaction starts from, and
	// empty when the demonstration read none there — which is what `Waits` is about.
	StartsOn    []string `json:"starts_on,omitempty"`
	Application string   `json:"application"`
	Play        string   `json:"play"`
	// Source is the digest of the play the bindings were written with, and Bound is what
	// each interaction NAME means — the demonstrated position it resolves to.
	//
	// Both travel into the authorisation, which the runner checks against the plan it
	// preflighted: the same Act can be composed again over the same demonstration with
	// the same names in the same order bound to different positions, and then everything
	// a person sees is unchanged and every control that would be pressed is different.
	Source string         `json:"source,omitempty"`
	Bound  []BoundNameFor `json:"bound,omitempty"`
	// Identity is all of that in one string, and it is the term the page hands back when
	// somebody presses Play on this Act.
	//
	// The digest alone was not enough, and it looked like it was — see identityOf.
	Identity string `json:"identity,omitempty"`
	// Waits is why a run of this Act would wait rather than press, when it would, and
	// empty when nothing here can say that it would.
	//
	// # The run this is for
	//
	// The owner composed an Act over an interaction whose starting screen the
	// demonstration had read no words on, pressed Play, and watched thirty seconds of
	// nothing end in "refused — 0 of 2 interactions verified". The runner behaved
	// correctly throughout: it paused, repeatedly, saying "screen_b was saved without
	// any words, so a reading cannot identify it". But the Act was listed as "ready",
	// and ready is what a person reads as "this will work".
	//
	// Separate from `Why` because the two are different claims and both are true at
	// once: the preflight says this Act CAN be performed, and this says a performance of
	// it will wait for something it cannot recognise. See waitsBecause.
	Waits string `json:"waits,omitempty"`
	// OnLayout says every interaction of this Act could be performed on its demonstrated
	// layout, and LayoutSays is the sentence a person reads either way.
	//
	// Geometry replay presses where somebody clicked, in a window of the size they
	// clicked it in, on a screen nothing could recognise. It needs two things from the
	// demonstration and neither can be added afterwards: the window it was made in, and
	// the point of each press. A routine recorded before those were kept has neither, so
	// the honest sentence names a fresh demonstration rather than offering a choice that
	// would be refused at the first press.
	//
	// It is offered rather than taken: the mode is a narrow exception to the rule that a
	// recorded coordinate is not a replay target, and the person pressing Play is who
	// takes it. See [[ADR-248-replaying-a-demonstrated-layout]].
	OnLayout   bool   `json:"on_layout"`
	LayoutSays string `json:"layout_says,omitempty"`
	// Runnable is whether it preflights RIGHT NOW, and Why the refusal when it does not.
	//
	// Asked here rather than guessed, through the production preflight: the bindings may
	// have been edited, the demonstration may have been re-saved, the play may have been
	// changed by hand. A person should see that before they start, not after.
	Runnable bool   `json:"runnable"`
	Why      string `json:"why,omitempty"`
}

// ActStep is one interaction of an Act, in the order it is performed.
//
// Everything in it was READ during the demonstration. `control_2` and `screen_a` are available
// here and deliberately absent: they are names this machine gave itself, and a person deciding
// whether to let something happen cannot recognise either of them.
type ActStep struct {
	// Name is what the person called it, which is also what the play says.
	Name string `json:"name"`
	// Reads is the words on the control it presses, On the words on the screen it
	// presses them from. Empty means the demonstration read nothing there.
	Reads []string `json:"reads,omitempty"`
	On    []string `json:"on,omitempty"`
	// Expect is how many words the demonstration saw appear and STAY afterwards, and
	// Expected a few of them. Zero is an interaction whose result cannot be checked, so
	// nothing after it will be performed.
	Expect   int      `json:"expect"`
	Expected []string `json:"expected,omitempty"`
	// Position is the demonstrated interaction this name resolves to. Evidence about
	// where it came from, never a label.
	Position int `json:"position"`
	// Routine is the demonstration this interaction belongs to, and ControlLabel,
	// FromLabel and ToLabel are its labels inside it — together, the key to the pictures
	// kept of it. Keys, never shown: see Interaction.ControlLabel.
	Routine      string `json:"routine,omitempty"`
	ControlLabel string `json:"control_label,omitempty"`
	FromLabel    string `json:"from_label,omitempty"`
	ToLabel      string `json:"to_label,omitempty"`
}

// BoundNameFor is one interaction name and the demonstrated position it resolves to.
type BoundNameFor struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

// identityOf is everything about an Act that decides what it would press, as one string.
//
// # Why not the digest of the play
//
// Because the play cannot see the thing this has to catch. Compose an Act over one
// demonstration with the same names in the same order bound to different positions and the
// generated Marco is BYTE-IDENTICAL, with the same bindings digest — proven, not argued, by
// cmd/director TestRecomposingTheSameNamesChangesNothingTheOldTermsCouldSee. So a page that
// handed back the digest it was displaying handed back a term that had already agreed, and the
// one change a person cannot see was the one change nothing checked.
//
// # What is in it
//
// The digest, the demonstration revision, the application, and every interaction name with the
// position it resolves to, in run order. Anything that moves one of those makes a different
// string, and a press of Play under the old one is refused.
//
// Plain text rather than a hash on purpose: when two of these disagree, they say WHICH part
// moved, to anybody reading them side by side.
func identityOf(c Act) string {
	parts := []string{"play=" + c.Source, "revision=" + c.Revision,
		"application=" + c.Application}
	for _, b := range c.Bound {
		parts = append(parts, fmt.Sprintf("%s=%d", b.Name, b.Position))
	}
	return strings.Join(parts, " ")
}

// Saved is every Act on this machine, with whether each one would run.
func (a *Acts) Saved() []Act {
	store, serr := demo.Open(a.home)
	out := []Act{}
	for _, rt := range a.reg().List() {
		src, binds, err := demonstrated.Load(a.reg(), rt)
		if err != nil {
			// NOT AN ACT. A play with no bindings is an ordinary play, which is
			// the right fail-closed direction and not something to report as
			// broken — see ADR-243. Skipped rather than listed.
			continue
		}
		act := Act{Name: binds.Act, From: routineOf(binds), Play: src,
			Application: binds.Application, Source: binds.Source}
		if act.Name == "" {
			act.Name = rt.Slug
		}
		for _, s := range binds.Scenes {
			act.Interactions = append(act.Interactions, s.Name)
			act.Revision = s.Revision
			// AND WHAT THE NAME MEANS, which is the only part of an Act's
			// identity that a person cannot see and a recompose can change.
			act.Bound = append(act.Bound, BoundNameFor{Name: s.Name,
				Position: s.Position})
		}
		// AND THE WHOLE OF IT AS ONE TERM, which is what a page can hand back.
		act.Identity = identityOf(act)
		if serr == nil {
			act.StartsOn, act.Steps, act.Waits = describeAct(store, binds)
			// AND WHETHER IT COULD BE PERFORMED ON ITS DEMONSTRATED
			// LAYOUT, which is a property of the DEMONSTRATION rather
			// than of the Act: what it needs is the window and the
			// click, and neither can be added to a recording after
			// the fact.
			act.OnLayout, act.LayoutSays = actOnLayout(store, binds)
		}
		if serr != nil {
			act.Why = serr.Error()
		} else if _, perr := demonstrated.Preflight(src, binds, store,
			marcoChecks{}); perr != nil {
			act.Why = perr.Error()
		} else {
			act.Runnable = true
		}
		out = append(out, act)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// describeAct is what an Act will do, where it has to start, and why it would wait instead.
//
// One pass over the demonstration behind it, because all three answers come from the same place
// and a second pass is a second chance to describe a different artifact.
//
// # The waiting
//
// An Act starts on the screen its FIRST interaction was demonstrated on, and that screen is
// established by matching the words read on it. A screen saved without words cannot be
// positively identified — the arrangement alone has been measured to confuse two screens of one
// menu — so the run pauses, every look, until its deadline. Not a refusal and not a preflight
// failure: the Act is composed correctly and would perform perfectly against a screen Marco
// could recognise. What is missing is in the DEMONSTRATION, so the sentence says that.
func describeAct(store *demo.Store, binds demonstrated.Bindings) (startsOn []string,
	steps []ActStep, waits string) {
	if store == nil {
		return nil, nil, ""
	}
	for i, sc := range binds.Scenes {
		step := ActStep{Name: sc.Name, Position: sc.Position}
		art, ok := store.Sequence(sc.Routine)
		if !ok {
			// THE DEMONSTRATION IS GONE. Said by the preflight in its own
			// words; this lists the interaction with nothing claimed about it
			// rather than leaving it out of the order.
			steps = append(steps, step)
			continue
		}
		for _, in := range art.Steps {
			if in.Position != sc.Position {
				continue
			}
			step.Reads = readsOf(art, in)
			step.On = onWordsOf(art, in)
			step.Expect = len(in.Outcome)
			step.Expected = someOf(in.Outcome, outcomeShown)
			// AND WHERE ITS PICTURES ARE. `sc.Routine` rather than the
			// Act’s name: the pictures were kept when the DEMONSTRATION
			// was recorded and are filed under its name.
			step.Routine = sc.Routine
			step.ControlLabel, step.FromLabel, step.ToLabel =
				in.Control, in.From, in.To
			break
		}
		if i == 0 {
			startsOn = step.On
			if len(step.On) == 0 {
				waits = "its first interaction starts on a screen the " +
					"demonstration read no words on, so Marco cannot " +
					"confirm you are there: a run will wait for it " +
					"instead of pressing, and end without doing " +
					"anything. Demonstrate it again, resting on that " +
					"screen long enough to be read, or build an Act " +
					"from an interaction whose screen has words."
			}
		}
		steps = append(steps, step)
	}
	return startsOn, steps, waits
}

func routineOf(b demonstrated.Bindings) string {
	for _, s := range b.Scenes {
		return s.Routine
	}
	return ""
}

// ── RUN: the production supervised runner, as a child, with its transcript ────

// readingPatience is how many observations a step of a reading run is given before the run
// stops rather than waiting indefinitely.
//
// A hundred and fifty: fifteen seconds at the ten-looks-a-second cadence, which is about thirty
// answers from a source that reads twice a second. See `start`, where it is passed and why.
const readingPatience = 150

// Running is what the page shows while a run is in flight and after it ends.
type Running struct {
	Act        string   `json:"act"`
	Live       bool     `json:"live"`
	Reference  string   `json:"reference"`
	Outcome    string   `json:"outcome"`
	Transcript []string `json:"transcript"`
	Active     bool     `json:"active"`
	// State is the RUNNER'S OWN latest account of itself, when there is one.
	//
	// Not derived from the transcript any more. Studio used to match on sentences —
	// "May Marco", "Is that the screen you are on?" — so improving the wording of a
	// consent prompt would have silently changed what a window showed somebody about a
	// press they were about to authorise. See internal/director/runstate.
	State *runstate.Event `json:"state,omitempty"`
	// Asking is the question somebody CAN STILL ANSWER, which is not every question the
	// state carries.
	//
	// A separate field because the two claims are different and only one of them is about
	// the keyboard. `State.Question` is what the run was waiting for when it last said
	// anything — evidence, kept. This is an invitation to press a key, and it is only
	// honest while there is a process on the other end of the hook to hear it: a child
	// that exited with a question open left a snapshot that still carried it, and the page
	// went on offering its keys. Somebody pressing F7 at that is pressing it at nothing,
	// in a window that just told them a press was waiting on their answer.
	Asking *runstate.Question `json:"asking,omitempty"`
	// Reason is why it ended, from the final event.
	Reason string `json:"reason,omitempty"`
	// Said is how it went, in a sentence. The outcome word beside it is still shown; see
	// said.go.
	Said string `json:"said,omitempty"`
	// Doing is what is happening NOW, in the words of the thing the person asked for:
	// "Preparing Ark…", "Step 1 of 2 · open the audio settings", "Checking that it
	// worked…". Empty once the run has ended, where `Said` is the headline. See doing.go.
	Doing string `json:"doing,omitempty"`
	// Next is what to do about how it ended, when there is something to do. See nextOf.
	Next string `json:"next,omitempty"`
	// StartsOn is what the screen this Act starts on was read to say, so a person
	// watching a paused run can check whether they are standing on it.
	//
	// # Why the words and not the label
	//
	// Because the run pauses about `screen_b`, which is a name this machine gave itself
	// while folding looks together, and the owner’s report of their first trial was that
	// they could tell which screen was being asked for “but not easily”. The words read
	// off that screen are the only part of its identity a person can compare against what
	// is in front of them. Carried beside the pause rather than substituted into it: the
	// sentence is the runner’s and this surface does not edit it.
	StartsOn []string `json:"starts_on,omitempty"`
	// Complete says the run ENDED — it said so itself, with a final event. Incomplete
	// says the child is gone and never did, which is not the same as having failed:
	// one is a run with an answer and the other is a run nobody can speak for.
	Complete   bool `json:"complete"`
	Incomplete bool `json:"incomplete"`
	// Routine says this is a saved routine running rather than an Act being performed,
	// and the page renders it with less in it for that reason: there is no canonical
	// reference, no phase, no pause and no question, because the thing performing it is
	// `marco do` and not the supervised runner. Said rather than inferred from the
	// absence of those, which would make a run that had not spoken yet look like the
	// other kind. See routines.go.
	Routine bool `json:"routine,omitempty"`
}

// Run performs an Act through `director perform-workflow --play`.
//
// Returns at once; the transcript arrives through `Watch`. A page that blocked on this would be a
// page that cannot show a prompt, which is the one thing a person has to see.
//
// `live` is the whole of the decision to touch the machine, and it is the caller's: without it
// the entire path runs and the host writes each program down instead of pressing anything.
func (a *Acts) Run(act, application string, live, reading bool) string {
	// THE ENGINEERING PATH, KEPT. Every press asked for by key, the application typed by
	// hand, reading optional: the shape the acceptance checklist in E2E.md drives, and
	// the only way to exercise the keyed consent desk from this surface. `Play` is what a
	// person presses; see play.go.
	return a.start(Performance{Act: act, Application: application, Live: live,
		Reading: reading})
}

// start claims the one run slot and spawns the supervised runner.
//
// One body for both contracts, because everything below the arguments is the same problem: one
// child at a time, one transcript, one structured stream, and a window that can stop it. What
// differs is four flags, and a second copy of this plumbing is how the two would drift.
func (a *Acts) start(p Performance) string {
	act, application, live, reading := p.Act, p.Application, p.Live, p.Reading
	// THE ONE SLOT, CLAIMED. See `reserve`, which is shared with the other kind of run
	// this window can start — a saved routine — because "may anything run now" has to
	// have one answer or the two kinds would both get a yes.
	if why := a.reserve(kindAct); why != "" {
		return why
	}
	a.mu.Lock()
	a.act, a.live, a.saying = act, live, ""
	a.mu.Unlock()
	// AND RELEASED AGAIN ON EVERY PATH THAT DOES NOT REACH A RUNNING CHILD. A slot
	// claimed and then abandoned is a Studio that refuses every later run.
	release := func() {
		a.mu.Lock()
		a.busy, a.running, a.stopRun, a.done = false, nil, nil, nil
		a.mu.Unlock()
	}
	// THE INTERVAL THE RACE LIVED IN. Nil in production; see afterReserve.
	a.mu.Lock()
	pause := a.afterReserve
	a.mu.Unlock()
	if pause != nil {
		pause()
	}

	// WHERE THIS RUN WILL SAY WHAT IT IS DOING. One file per run in a temporary
	// directory: a conversation between two processes, not something a person keeps.
	// A failure to make it is not a reason to refuse the run — the transcript still
	// arrives, and a window with no structured state is worse than no window only if
	// it pretends otherwise, which `Incomplete` prevents.
	saying := ""
	if f, ferr := os.CreateTemp("", "studio-run-*.jsonl"); ferr == nil {
		saying = f.Name()
		_ = f.Close()
	}
	args := []string{"perform-workflow", "--play", act, "--hotkeys"}
	// ── AND THE DEMONSTRATED LAYOUT, WHEN THE PERSON ASKED FOR IT ──
	//
	// Passed through rather than decided here. `Play` has already checked that the
	// demonstration behind this Act recorded the window and the clicks, so a run
	// that gets this far with the flag is one that can use it. Absent otherwise,
	// which is every run of every Act that did not ask.
	if p.FixedLayout {
		args = append(args, "--fixed-layout")
	}
	if saying != "" {
		args = append(args, "--events", saying)
	}
	// ── THE AUTHORISATION, WHEN THIS IS A PERSON PRESSING PLAY ──
	//
	// Written before the child starts and handed to it by path. `--hotkeys` is still
	// passed: the hook is what makes F9 stop a run from anywhere, and under a grant it
	// answers nothing else — the runner does not build a consent desk to ask with. See
	// cmd/director/actgrant.go.
	if p.Grant != nil {
		// THE ACT SOMEBODY CHOSE, NOT THE ONE ON DISK NOW.
		//
		// This used to read the store again here and authorise whatever it found,
		// which is a second chance to pick up a different Act: `Play` checked one
		// version and the grant was written from another. A replacement in that
		// interval was not refused by either of them — it was authorised, because
		// the grant and the plan the runner preflights would then agree about an Act
		// nobody had read. What arrives here is the snapshot `Play` checked, and
		// nothing between the two instants can change it.
		path, gerr := a.granted(*p.Grant)
		if gerr != nil {
			// NOT A RUN WITHOUT ONE. The authorisation IS the permission, and a
			// run started without it would be one asking questions nobody is
			// there to answer.
			release()
			return "the authorisation for this run could not be written, so " +
				"nothing was started: " + gerr.Error()
		}
		args = append(args, "--granted", path)
	}
	// THE APPLICATION THE ACT ALREADY KNOWS, when the caller did not name one.
	//
	// The demonstration recorded which executable it was shown against and the Act is
	// pinned to that demonstration, so asking a person to type `arkascended` is asking
	// them for something already written down — and getting it wrong is a run that
	// grounds against the wrong window, or refuses for a reason about a selector rather
	// than about their Act.
	if application == "" {
		if p.Grant != nil {
			// FROM THE SNAPSHOT, for the same reason as the authorisation
			// above: a read of the store here is another chance to aim this
			// run at a window belonging to a different version of the Act.
			application = p.Grant.Application
		} else {
			application = a.applicationFor(act)
		}
	}
	if application != "" {
		args = append(args, "--application", application)
	}
	if live {
		args = append(args, "--live")
	} else {
		// NO DWELL, AND ONLY FOR A DRY RUN.
		//
		// The dwell rests the pointer on the control and then CHECKS that the
		// operating system actually put it there — ADR-198, a move that reported
		// completed and did not happen. In a dry run the move is written down
		// instead of performed, so the pointer never arrives and that check refuses
		// the click, correctly, for a reason that has nothing to do with the Act.
		//
		// A LIVE run must inherit the production default. Passing `--dwell 0s`
		// always, as the first version did, took the pointer-rest, the
		// pointer-arrival measurement and the hover probe out of exactly the runs
		// that need them — the game-menu work those three were built for.
		args = append(args, "--dwell", "0s")
	}
	if reading {
		args = append(args, "--reading")
		// AND TIME FOR THE THING THAT READS IT.
		//
		// A granted run cannot ask anybody where they are, so it identifies the
		// screen from its words — and the words come from a source that answers
		// about twice a second against a loop that looks ten times, with a good
		// share of its readings thrown away because the observation moved on while
		// OCR was working. The runner’s own patience is forty observations: four
		// seconds, which is the fast loop’s budget and about eight chances at a
		// reading. The owner’s first live Act refused twice in a row inside it, on
		// the right screen, having never been read.
		//
		// So a run that depends on the slow source is given time measured in the
		// slow source’s answers rather than in the fast loop’s looks. Nothing is
		// pressed while it waits, the pause says what it is waiting for, Stop is
		// on screen throughout, and the pause resolves by itself the moment the
		// evidence arrives — waiting is the cheapest thing this run can do.
		args = append(args, "--patience", strconv.Itoa(readingPatience))
	}

	// A CANCELLABLE CONTEXT, so closing the window can end the run rather than orphan
	// it. `context.Background()` left a supervised runner alive after the window that
	// was asking its questions had gone.
	ctx, stop := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, directorBin(), args...)
	// AND NO CONSOLE WINDOW, which would take the foreground off the application the
	// run is about to press something in. See hideConsole.
	hideConsole(cmd)
	// THE SAME STORE, EXPLICITLY. Both this process and the runner read $MARCO_HOME, and
	// a child with a different answer would ground against one store while the window
	// listed another.
	cmd.Env = append(os.Environ(), "MARCO_HOME="+a.home)
	// ONE TRANSCRIPT, IN THE ORDER A PERSON AT A TERMINAL WOULD SEE IT. The runner says
	// some things on stderr — a provider that is missing, a record that could not be
	// written — and every one of them is part of the account of what happened.
	pr, pw, err := os.Pipe()
	if err != nil {
		stop()
		release()
		return err.Error()
	}
	cmd.Stdout, cmd.Stderr = pw, pw

	// STARTING AND PUBLISHING ARE ONE ATOMIC STEP, under the lock, with the closing check
	// inside it.
	//
	// That is the whole of the fix. Separately, there was always a moment when the slot
	// was claimed and no child existed, and `Close` returning in that moment let this
	// launch publish a runner nobody would stop. Now a shutdown that began before this
	// point is seen here and nothing is started; a shutdown that begins after it finds a
	// child already published. There is no third case.
	//
	// Holding the lock across `Start` costs a page poll a millisecond or two, which is
	// the entire price.
	drained := make(chan struct{})
	finished := make(chan struct{})
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		pw.Close()
		pr.Close()
		stop()
		release()
		return "Studio is closing, so nothing new was started"
	}
	if err := cmd.Start(); err != nil {
		a.mu.Unlock()
		pw.Close()
		pr.Close()
		stop()
		release()
		return err.Error()
	}
	a.running, a.stopRun, a.done, a.saying = cmd, stop, finished, saying
	a.mu.Unlock()
	pw.Close()
	// A PERFORMANCE HAS STARTED, noted once the child is running for the same reason a
	// recording is: what a trial records is what happened, not what was asked for.
	a.trial.playing(act, live, p.FixedLayout)

	go func() { a.read(pr); close(drained) }()
	go func() {
		_ = cmd.Wait()
		// EXITED IS NOT FINISHED. A child that has gone with output still in the
		// pipe is a run whose transcript is still arriving, and the slot stays
		// claimed until there is nothing left of it.
		<-drained
		stop()
		// AND THE CANONICAL RESULT, FROM THE RECORD THE RUN WROTE.
		//
		// After the transcript has drained, because the reference is read out of it
		// — and from the record rather than from this window's bookkeeping, so the
		// trial cites the same account `marco` and `director` would. A run that
		// cited no reference, or wrote no record, is recorded as exactly that.
		//
		// BEFORE the slot is released, which is what makes the journal complete by
		// the time anything can observe that the run is over: a window closing
		// waits on `finished`, and a reader watching for the run to end would
		// otherwise race the last line of the evidence.
		a.mu.Lock()
		ref := a.reference
		a.mu.Unlock()
		rec, had := a.recordFor(ref)
		// AND WHY IT STOPPED, which the record does not say: `because` is a
		// tally and the runner's last pause is the sentence. See lastPause.
		a.trial.performed(act, ref, live, rec, had, lastPause(saying))
		release()
		close(finished)
	}()
	return ""
}

// Close ends a run this Studio started and waits for it to finish.
//
// # Why the window closing has to do this
//
// Because the child was given `context.Background()`, so closing Studio left a supervised runner
// ALIVE — still grounding, still able to ask for a press, with the window that was showing the
// questions gone. A person would have had no way to see what it was asking and no way to answer
// it except the keys.
//
// The stop is the product's own: `stopsignal.Raise` is what F9 and `director stop` raise, and the
// runner notices at the last moment it could matter and stops at a safe point. Cancelling the
// context as well is the backstop for a child that is not listening.
//
// BOUNDED, and the order matters: the run is given time to write its record before anything is
// removed, because a temporary store deleted first would take the record with it. Past the bound
// the child is killed — a shell that waited forever on shutdown is a shell somebody has to kill.
func (a *Acts) Close() {
	// AND THE EVIDENCE IS COMPLETE BEFORE THE WINDOW IS GONE, on every path out of
	// here. The picture tally is written at most every couple of seconds while
	// pictures are arriving — see trialTally — so a window closed in that interval
	// would otherwise leave the last of it unwritten.
	defer a.trial.flush()
	// CLOSING IS DECLARED FIRST, so nothing new can be claimed and a launch already
	// inside its own reservation aborts rather than starting anything.
	a.mu.Lock()
	a.closing = true
	busy, cmd, stop, done := a.busy, a.running, a.stopRun, a.done
	rec := a.learn
	watching := a.capture
	a.mu.Unlock()
	// AND A RECORDER-FIRST CAPTURE ENDS WITH THE WINDOW, HOOKS AND ALL.
	//
	// The same rule as the recording below and a sharper reason: those hooks are in THIS
	// process, so leaving them installed past shutdown is leaving low-level keyboard and
	// mouse hooks on somebody's desktop with nothing left to unhook them. The draft is
	// abandoned, because a window being closed is not somebody saying "keep what you
	// have". Waited for, so the pump has returned before the process does.
	if watching != nil {
		watching.ended.Do(func() { close(watching.stop) })
		select {
		case <-watching.done:
		case <-time.After(3 * time.Second):
			fmt.Fprintln(os.Stderr, "studio: the input recorder did not "+
				"finish within three seconds of being stopped; leaving "+
				"it to the operating system")
		}
	}
	// AND A RECORDING ENDS WITH THE WINDOW TOO.
	//
	// Abandoned rather than finished: a window being closed is not somebody saying "keep
	// what you have", and a half-demonstration saved under the name they chose is worse
	// than none. Waited for, because a recorder still watching the screen after the
	// window that started it has gone is the same defect as a runner outliving it.
	if rec != nil && !rec.over {
		if rec.stop != nil {
			rec.stop()
		}
		if rec.done != nil {
			select {
			case <-rec.done:
			case <-time.After(3 * time.Second):
				fmt.Fprintln(os.Stderr, "studio: the recorder did not "+
					"finish within three seconds of being stopped; "+
					"leaving it to the operating system")
			}
		}
	}
	if !busy {
		// NOTHING IS CLAIMED, so there is nothing to stop and nothing that can
		// appear: a later `Run` is refused by the flag just set.
		return
	}

	// AND IF THE SLOT IS CLAIMED WITH NO CHILD YET, WAIT FOR ONE OR FOR THE ABORT.
	//
	// This is the interval the race lived in. A launch between its reservation and its
	// start is about to do one of two things — publish a child, or see `closing` and give
	// the slot back — and shutdown may not proceed until it has done one of them.
	// Returning here was what let a runner outlive the window.
	//
	// Bounded, because a shutdown that waits forever is a window somebody has to kill. A
	// launch cannot be slow: between the two points it creates a pipe.
	if cmd == nil {
		waited := time.Now().Add(2 * time.Second)
		for time.Now().Before(waited) {
			a.mu.Lock()
			busy, cmd, stop, done = a.busy, a.running, a.stopRun, a.done
			a.mu.Unlock()
			if !busy || cmd != nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if cmd == nil {
		// THE LAUNCH ABORTED, or never reached a child. Either way nothing is
		// running and nothing can start.
		return
	}
	// ASK FIRST, the way a person would.
	_ = stopsignal.Raise(stopsignal.Home())
	select {
	case <-done:
		return
	case <-time.After(6 * time.Second):
	}
	// THEN CANCEL THE CONTEXT, which kills the process group.
	if stop != nil {
		stop()
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		// AND SAY SO RATHER THAN HANGING. A run that will not end is worth a line on
		// stderr; a window that will not close is worse than either.
		fmt.Fprintln(os.Stderr, "studio: the supervised run did not finish within "+
			"nine seconds of being stopped; leaving it to the operating system")
	}
}

// read takes the child's lines, keeps them, and reads the canonical reference out of them.
func (a *Acts) read(pr *os.File) {
	defer pr.Close()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		a.mu.Lock()
		a.transcript = append(a.transcript, line)
		// THE RUN'S OWN REFERENCE, taken from the line the runner printed rather
		// than invented here. `runref.Parse` is the shared format both binaries
		// already read, and the outcome arrives on the closing citation.
		if r, ok := runref.Parse(line); ok {
			a.reference = r.ID
			if r.Outcome != "" {
				a.outcome = string(r.Outcome)
			}
		}
		a.mu.Unlock()
	}
}

// Watch is the run as it stands, for the page to poll.
//
// ONE VIEW, TWO KINDS OF RUN. A saved routine's performance is rendered in the same shape by the
// same page — a second poll and a second panel would be a second place that shows a transcript
// and an outcome, and the two would drift. Everything below this line is about a SUPERVISED
// performance: a canonical reference, the runner's own published state, a pause, a question. A
// routine has none of those and says so. See routines.go.
func (a *Acts) Watch() Running {
	a.mu.Lock()
	kind := a.kind
	a.mu.Unlock()
	if kind == kindRoutine {
		return a.watchRoutine()
	}
	a.mu.Lock()
	// THE RUN'S OWN ACT AND MODE, not the page's controls. A person who ticks the live
	// box while something is running must not be shown a dry run described as live.
	w := Running{Act: a.act, Live: a.live,
		Reference: a.reference, Outcome: a.outcome,
		Transcript: append([]string(nil), a.transcript...),
		Active:     a.busy}
	path, started := a.saying, a.saying != ""
	a.mu.Unlock()
	if !started {
		return w
	}

	// ── AND WHAT THE RUN SAYS ABOUT ITSELF, WHICH OUTRANKS EVERYTHING ABOVE ──
	//
	// The latest snapshot is the whole current state, so rendering it cannot leave a
	// stale prompt on screen: a question that has been answered is simply absent from
	// the event after it.
	//
	// Where the event and this surface's own bookkeeping both have an answer, the
	// event's wins — it was written by the thing that decided. The bookkeeping covers
	// the window before the first event arrives, which is real: the run has been
	// started and has not said anything yet.
	e, final, err := runstate.Latest(path)
	if err == nil && e.Version != 0 {
		w.State = &e
		w.Complete = final
		if e.Run != "" {
			w.Reference = e.Run
		}
		if e.Act != "" {
			w.Act = e.Act
		}
		w.Live = e.Live
		if e.Outcome != "" {
			w.Outcome, w.Reason = string(e.Outcome), e.Reason
		}
	}
	// ── AND WHAT THAT MEANS FOR WHAT THEY ASKED FOR ──
	//
	// The outcome word is kept and shown; this is the sentence beside it. See said.go for
	// why a person is not left to learn what `unconfirmed` means about their own run.
	w.Said = saidOf(w.Act, runref.Outcome(w.Outcome), w.Reason)
	// AND WHAT IT IS DOING, in the same words. The application comes from the Act rather
	// than from the event: the run knows which window it is grounding against and does
	// not publish it, and "Preparing…" with nothing named is a window withholding the
	// one thing that would tell somebody which program to go and look at.
	w.Doing = doingOf(a.applicationFor(w.Act), w.State, w.Active)
	w.Next = nextOf(runref.Outcome(w.Outcome), a.waitsFor(w.Act))
	w.StartsOn = a.startsOnFor(w.Act)
	// AN INCOMPLETE RUN: the child is gone and it never said how it ended.
	//
	// Not a failure, and reported apart from one. A run that refused says so in a final
	// event; a child that vanished — killed, crashed, or stopped by something outside
	// Marco — leaves a stream with no end to it, and the honest thing to show a person
	// is that nobody can speak for what happened.
	if !w.Active && !w.Complete {
		w.Incomplete = true
	}
	// ── AND WHETHER ANYBODY CAN STILL ANSWER WHAT IT WAS ASKING ──
	//
	// Last, because it depends on both answers above. A run that ENDED has no open
	// question by construction — a final event carries none — and a run whose child is
	// GONE may well have one, which is exactly the case this is here for. See `Asking`.
	if w.Active && !w.Complete && !w.Incomplete && w.State != nil {
		w.Asking = w.State.Question
	}
	return w
}

// ── CANCEL: the one cross-process stop ────────────────────────────────────────

// Cancel stops a run, wherever it is and whichever kind it is.
//
// `stopsignal` is the one stop broadcast in this product, and it is what F9 and `director stop`
// raise. Not a kill: the runner notices at the last moment it could matter and stops at a safe
// point, which is the difference between ending a run and abandoning a press mid-flight.
//
// A saved routine is stopped by the same raise: `marco do` watches the same generation file for
// every run it makes, so there is one stop in this product and not one per kind of thing running.
func (a *Acts) Cancel() string {
	if err := stopsignal.Raise(stopsignal.Home()); err != nil {
		return err.Error()
	}
	return ""
}

// ── INSPECT: the record the run wrote, by its own reference ───────────────────

// Inspect reads back the durable record of a run, by the reference the run printed.
//
// The record rather than the transcript: the transcript is what was said, and this is what was
// kept — the outcome, the counts, the Act and the demonstration both.
func (a *Acts) Inspect(reference string) map[string]any {
	if reference == "" {
		return map[string]any{"error": "no run reference"}
	}
	p := filepath.Join(a.home, marcopaths.RunsDir, reference+".json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return map[string]any{"error": err.Error(), "path": p}
	}
	var rec map[string]any
	if jerr := json.Unmarshal(raw, &rec); jerr != nil {
		return map[string]any{"error": jerr.Error(), "path": p}
	}
	rec["path"] = p
	return rec
}

// ForgetAct removes one Act: the play, its bindings and its provenance.
//
// Asks first, as Forget does for a run and `Done` does for a recording. An Act is cheap to
// compose again from the demonstration it came from — which is exactly why the question says
// so, rather than making somebody wonder what they are about to lose.
func (a *Acts) ForgetAct(name string, anyway bool) Forgotten {
	name = strings.TrimSpace(name)
	if name == "" {
		return Forgotten{Error: "name the Act to delete"}
	}
	a.mu.Lock()
	busy, running := a.busy, a.act
	a.mu.Unlock()
	if busy && strings.EqualFold(running, name) {
		return Forgotten{Error: "that Act is running; stop it first"}
	}
	var found *Act
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, name) {
			got := c
			found = &got
			break
		}
	}
	if found == nil {
		return Forgotten{Error: fmt.Sprintf("there is no Act called %q", name)}
	}
	if !anyway {
		from := found.From
		if from == "" {
			from = "the demonstration it came from"
		}
		return Forgotten{Asks: fmt.Sprintf("This removes the Act %q. What you showed "+
			"Marco as %q is NOT touched, so you can compose it again from that.",
			name, from)}
	}
	if err := demonstrated.Forget(a.reg(), routes.Route{Slug: routes.Slug(name)}); err != nil {
		return Forgotten{Error: err.Error()}
	}
	return Forgotten{Deleted: 1}
}

// ForgetScene removes a demonstration — the artifact, its pictures, its evidence — and every Act
// composed from it.
//
// # Why the Acts go with it
//
// Because an Act IS its demonstration's interactions, by name, and a play whose demonstration
// has gone cannot be performed: its preflight rebuilds the play from the bindings and the
// artifact, and there is no artifact. What is left is an entry in the Acts list that says
// `ready` and cannot run. That is the dangling reference deleting a Scene by hand produces, and
// this is the one place that knows both halves.
//
// So the question NAMES them and the second press takes them, which is informed consent rather
// than a surprise either way: refusing while an Act exists would make a Scene undeletable until
// somebody hunted down what depended on it, and taking them silently would delete work nobody
// mentioned.
func (a *Acts) ForgetScene(name string, anyway bool) Forgotten {
	name = strings.TrimSpace(name)
	if name == "" {
		return Forgotten{Error: "name the Scene to delete"}
	}
	a.mu.Lock()
	busy := a.busy
	recording := a.recordingLocked()
	a.mu.Unlock()
	if busy || recording {
		return Forgotten{Error: "something is in flight; stop it first"}
	}
	store, err := demo.Open(a.home)
	if err != nil {
		return Forgotten{Error: err.Error()}
	}
	if _, ok := store.Sequence(name); !ok {
		return Forgotten{Error: fmt.Sprintf("nothing called %q has been saved", name)}
	}
	// THE ACTS THAT WOULD BE LEFT POINTING AT NOTHING.
	var dependents []string
	for _, c := range a.Saved() {
		if strings.EqualFold(c.From, name) {
			dependents = append(dependents, c.Name)
		}
	}
	if !anyway {
		say := fmt.Sprintf("This removes %q for good — what Marco read, the "+
			"pictures of it, and the evidence of how you demonstrated it.", name)
		if len(dependents) > 0 {
			say += fmt.Sprintf(" It also removes the %d Act(s) built from it "+
				"(%s), because an Act performs this demonstration's "+
				"interactions and cannot run without it.",
				len(dependents), strings.Join(dependents, ", "))
		}
		return Forgotten{Asks: say}
	}
	// THE ACTS FIRST. A failure here leaves the demonstration, which is the direction
	// that keeps them runnable; the other order leaves Acts that cannot run.
	gone := 0
	for _, act := range dependents {
		if err := demonstrated.Forget(a.reg(),
			routes.Route{Slug: routes.Slug(act)}); err != nil {
			return Forgotten{Deleted: gone, Error: err.Error()}
		}
		gone++
	}
	if err := store.ForgetSequence(name); err != nil {
		return Forgotten{Deleted: gone, Error: err.Error()}
	}
	return Forgotten{Deleted: gone + 1}
}

// ForgetEverything puts this machine back to how it was before anything was demonstrated.
//
// # Why this exists as one thing
//
// Because "delete this Scene" and "delete this run" are for keeping a store tidy, and that is
// not what the owner asked for:
//
//	I want to be able to start fresh with the testing whenever I'd like
//
// Doing that one card at a time is a chore proportional to how much testing has been done,
// which is exactly backwards. So this is one press, and the question names what it will take in
// counts rather than making somebody find out afterwards.
//
// # What it keeps, and why
//
// The hotkey bindings, because a key bound to `enter freeplay` works again the moment that is
// demonstrated again under the same name, and throwing it away is a thing to redo rather than a
// thing to clean up. The capability declarations in `os.marco`, which are shipped and not
// anybody's data. And the semantic memory and action graph of the earlier Learn work, which are
// not Studio recordings and are not what "start fresh with the testing" means.
//
// Nothing here is a second implementation: the Acts go through `demonstrated.Forget`, the
// demonstrations through `demo.Store.ForgetSequence`, and the runs through `Forget` — one rule
// per kind, in the one place that already had it.
func (a *Acts) ForgetEverything(anyway bool) Forgotten {
	a.mu.Lock()
	busy := a.busy
	recording := a.recordingLocked()
	a.mu.Unlock()
	if busy || recording {
		return Forgotten{Error: "something is in flight; stop it first"}
	}
	store, err := demo.Open(a.home)
	if err != nil {
		return Forgotten{Error: err.Error()}
	}
	scenes := store.Sequences()
	plays := a.Saved()
	// THE RUNS, COUNTED BY THE ONE THING THAT KNOWS THE DIRECTORY. Asking rather than
	// counting here keeps a single answer to "how many runs are kept".
	runs := a.Forget("", false).Kept

	if !anyway {
		return Forgotten{
			Kept: len(scenes) + len(plays) + runs,
			Asks: fmt.Sprintf("This removes everything on this machine: %d "+
				"demonstration(s) with their pictures and evidence, %d Act(s), "+
				"and %d run record(s). Your hotkeys are kept, so a key bound to "+
				"a name works again as soon as you demonstrate it again.",
				len(scenes), len(plays), runs),
		}
	}

	// THE ACTS FIRST, then what they were built from, then the records of running them.
	// The other order leaves Acts that cannot run, which is the state deleting a store
	// by hand produces.
	gone, failed := 0, ""
	for _, c := range plays {
		if err := demonstrated.Forget(a.reg(),
			routes.Route{Slug: routes.Slug(c.Name)}); err != nil {
			failed = err.Error()
			continue
		}
		gone++
	}
	for _, art := range scenes {
		if err := store.ForgetSequence(art.Name); err != nil {
			failed = err.Error()
			continue
		}
		gone++
	}
	if runs > 0 {
		done := a.Forget("", true)
		gone += done.Deleted
		if done.Error != "" {
			failed = done.Error
		}
	}
	return Forgotten{Deleted: gone, Error: failed}
}

// actOnLayout says whether every interaction of this Act could be performed on its demonstrated
// layout, and gives the sentence a person reads either way.
//
// Two things are needed from the demonstration and neither can be added afterwards: the window
// it was made in, and the point of each press. A routine recorded before those were kept has
// neither, and the only honest answer is to name the fresh demonstration that would supply them
// rather than to offer a choice the first press would refuse.
//
// EVERY interaction, not the first: an Act that could replay two of three presses and then stop
// is an Act that leaves the application halfway through something.
func actOnLayout(store *demo.Store, binds demonstrated.Bindings) (bool, string) {
	if store == nil {
		return false, ""
	}
	for _, sc := range binds.Scenes {
		art, ok := store.Sequence(sc.Routine)
		if !ok {
			return false, "the demonstration behind this Act is not here"
		}
		if !art.Window.Known() {
			return false, "the demonstration behind this Act did not record " +
				"the window it was made in, so there is nothing to check " +
				"the layout against. Demonstrate it again and this becomes " +
				"available"
		}
		found := false
		for _, in := range art.Steps {
			if in.Position != sc.Position {
				continue
			}
			found = true
			if in.At == nil {
				return false, fmt.Sprintf("%q was demonstrated before Marco "+
					"kept the point of each click, so there is no click "+
					"to repeat. Demonstrate it again and this becomes "+
					"available", sc.Name)
			}
			break
		}
		if !found {
			return false, fmt.Sprintf("%q is not in the demonstration behind "+
				"this Act any more", sc.Name)
		}
	}
	w := windowOfFirst(store, binds)
	if w == nil {
		return false, ""
	}
	return true, fmt.Sprintf("Marco can press where you clicked, in the %dx%d window "+
		"you clicked in — without recognising the screen. Use it for a menu it "+
		"cannot read; it will refuse if the window is any other size",
		w.Width, w.Height)
}

// windowOfFirst is the window the Act's first interaction was demonstrated in.
func windowOfFirst(store *demo.Store, binds demonstrated.Bindings) *workflowstore.WindowGeometry {
	for _, sc := range binds.Scenes {
		if art, ok := store.Sequence(sc.Routine); ok && art.Window.Known() {
			return art.Window
		}
	}
	return nil
}
