// Package orchestrator is ACQUISITION and the AUTHORITY DOOR — how a play comes to exist, and
// whether a play that exists may be performed now. OS-agnostic; the OS-specific work is the
// injected recorder and host.
//
// # The invocation half of this package was retired in Phase 3
//
// This package used to own a second, complete way to invoke a play: `Deps.Resolve` answered which
// play, `Deps.Do` put it through the door and called `Deps.Run`, and `Deps.Run` executed it. By
// the time it was removed it had ZERO production callers — every entrance (the leader key, spoken
// words, typed words, `marco do`, a clicked Run) goes through `invoke.Decide` and then
// `cmd/marco/intake.go`'s `runInvocation` → `performOnePlay`, which calls `Classify` and
// `Authorize` from this package and then runs the play through `cmd/marco/panicstop.go`'s
// `runRoute`.
//
// It was not a harmless duplicate, and that is the part worth writing down. `Deps.Run` used
// `driver.RunFileWithHosts`: no context, so nothing could cancel a play once it started; no
// `routes.ApplyArgs`, so every argument was silently dropped; and no secret-arg plumbing, so a
// `{{password}}` never reached the credential store. A caller who found it would have got a
// quietly worse Marco than the one that ships. `docs/subsystems/Invocation.md` has said "one
// door, one local runner" for two milestones; this package is now consistent with that.
//
// The hazard it left behind is subtler and is why this note is long. The densest authority-seam
// coverage in the repository entered through `Deps.Do` — which meant the tests were proving that
// a function nothing calls honours the door. `cmd/marco/authoritybypass_test.go` records what that
// costs: entering through a non-production path is exactly what hid the last authority bypass. So
// the tests here now exercise the surviving PRODUCTION units directly (`Classify`, `Authorize`,
// `AskFirst`), and the claims that only mean something end-to-end live beside `runInvocation` in
// `cmd/marco`, where the product actually goes.
//
// If you are about to add an invocation entry point here: don't. Add it to `invoke.Decide`.
//
// # What is left
//
//   - Acquisition: `Learn` (demonstrate, interactive), `LearnAuto` (demonstrate, no prompts —
//     the overlay), `LearnVoice` (narrate), and `SimplifyRoute`.
//   - The door: `Classify`, `Authorize`, `AskFirst` and friends, in authority.go.
package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/simplify"
	"github.com/chaynes-simpleclouds/marco/internal/voicelearn"
)

// Deps are the orchestrator's collaborators (injectable for testing).
type Deps struct {
	Reg   routes.Registry
	Rec   recorder.Recorder
	Hosts map[string]runtime.Host // route execution hosts (e.g. {"*": oshost.New()})
	In    io.Reader               // prompts / confirmations
	Out   io.Writer               // user-facing output
	// Authority decides whether a resolved play may be performed now.
	//
	// Nil means Marco has no way to ask, and a learned play is then REFUSED rather than
	// assumed — a Marco that cannot put the question must not answer it for the user.
	// Authored and taught plays are unaffected: they run as they always have.
	Authority Gate
	App       func() string // current foreground app; nil disables context-awareness
	// HandedTo is the program owning the window this run was HANDED, as the host validated it
	// (`oshost.GiveWindow`), or "" when none was. Set only by the entrance, only after the
	// host accepted the window — a window the host refused has already refused the run. Read
	// by `AdmitInPlace`: a validated window of the routine's own application is what lets an
	// only-in routine run while the surface that chose it (Studio) is the foreground reading.
	HandedTo string
	// RecordingHome is the Marco home a recording is announced in, so a shortcut pressed in
	// another process is held off while it runs (internal/recordingnow). Set by the production
	// constructors; "" announces nothing, which is every test that does not ask.
	RecordingHome string
	// BeforeHooks, AfterShortcutChecked and BeforeForgetWrites are the instants a test can stand
	// in: after a recording is announced and before its hooks go on; after an assignment has
	// revalidated its routines under their locks and before it writes; after a forget has taken
	// its locks and before it changes anything. Nil in every production path.
	BeforeHooks          func()
	AfterShortcutChecked func()
	BeforeForgetWrites   func()
	// AfterShortcutsInvalidated is the instant in a replacing move after the replaced routine's
	// shortcuts are gone and before the destination is written. Nil in production.
	AfterShortcutsInvalidated func()
	// BetweenPressReads is the instant in ShortcutPressed after the shortcut was first read and
	// before its routines are locked and it is read again. Nil in production.
	BetweenPressReads func()
	// RenameFault makes a step of RenameInPlace ("create", "carry", "finish") fail when it
	// returns an error. Nil in production.
	RenameFault func(step string) error
	// StopKey is the gesture that ends a recording (e.g. "f12", "esc",
	// "ctrl+f12"). Empty uses recorder.DefaultStopKey. Keeping it off Esc lets a
	// macro record Esc as an ordinary key.
	StopKey string
	// SimplifySaves makes the learn "[s]implify" choice simplify AND save in one
	// step, with no re-confirmation. It's for a front-end that can't show the
	// simplified preview (the overlay HUD) — re-confirming a result you can't see
	// is pointless. The CLI leaves it false so the preview-then-confirm stays.
	SimplifySaves bool
	// Owner is the application that is DRIVING this recording — Marco's own window,
	// when a window is driving it. Empty when nothing is.
	//
	// It exists because a person who ends a recording by pressing a button has pressed a
	// real button in a real window, and the hooks record that like any other click. Left
	// in, it becomes a step: every replay would raise Marco and press Stop. The stop
	// GESTURE never had this problem because a key is not a place.
	Owner string
	// OwnerPID is the process of the window driving this recording, and it is the
	// boundary that actually holds. Zero when unknown.
	//
	// `Owner` above is a name the app POLLER reported, and a poller cannot answer about an
	// instant it has not sampled: somebody who brings the window forward and clicks Stop
	// inside the sample interval produces no switch event AND a click tagged with the app
	// they left. Both of the name-based rules then read the same stale sample and keep the
	// click. The process that owned the foreground window at the instant of the event does
	// not have that problem, because it is read at the instant of the event.
	OwnerPID uint32
	// Watch is which windows this recording admits input from. The ZERO VALUE is one
	// window, which is the stated default for Studio, Learn and LearnAuto.
	//
	// Stated by the caller rather than read from the environment, for the reason `Mode`
	// is: a window that stays open would otherwise inherit whatever launched it, and two
	// identical demonstrations would record different things with nothing in front of the
	// person to say why. See Watching, and recorder.Watching for the other half.
	Watch Watching
	// Window is WHICH window a confined recording is of, chosen before capture.
	//
	// Required when `Watch` is `OneWindow`, which is the default: `Begin` REFUSES without
	// it rather than recording the desktop, because a recording asked to be of one window
	// that does not know which one is the one outcome this is all here to prevent. See
	// Deps.Pickable for how a surface asks, and WhyNoWindow for the refusal.
	Window recorder.Target
	// AfterValidating runs inside the route's lock, after a replacement's version has
	// been checked and before anything is written.
	//
	// A seam for a timing window rather than a seam for behaviour, in the same shape as
	// Studio's `afterReserve`: the interval is real, it is where the race lived, and there
	// is no way to be inside it from outside. Nil in every production path.
	AfterValidating func()
	// CheckSource is the one compile gate, and nil means the real one.
	//
	// A SEAM AND NOT A CHOICE. `codegen` is trusted everywhere else here because its
	// input came from a recorder; an EDITED step list is input nobody recorded, so the
	// render is compiled before it can be offered. The default is production
	// (`driver.CheckSource`), so a caller that wires nothing still gets the gate — the
	// field exists so a test can stage a refusal without having to invent a step list
	// that happens to produce illegal Marco, which would be a test about `codegen`
	// rather than about the gate being asked.
	CheckSource func(src string) error
	// OwnerTailOnly takes the owner's window out ONLY off the tail of a recording the
	// BUTTON ended, instead of wherever it appears.
	//
	// # Two rules, both deliberate, and neither chosen by accident
	//
	// The default — false — is the window's: the owner comes out WHEREVER IT IS, because a
	// window is glanced at. Pressing a button in Studio means bringing Studio forward and
	// clicking in it, and a glance mid-demonstration costs the click before it as well as
	// the one at the end. That is what
	// [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] is about, and it is what
	// `Capture.Finish` has always done.
	//
	// THE TERMINAL AND THE WIDGET ASK FOR THE OTHER ONE, which is what they have always
	// done: the strip ran only for a recording the BUTTON ended, so a recording ended by
	// the stop GESTURE kept whatever it always kept — including a click somebody made in
	// Marco's own window. `TestAGestureEndedRecordingIsNotStripped` exists to record that
	// changing that would be a separate decision rather than a tidy-up, so moving those
	// two onto this pipeline must not change it in passing. In production the difference
	// is unreachable for them: `cmd/marco` sets no owner at all. It is preserved because
	// it was stated, not because it is load-bearing.
	//
	// One pipeline, two stated policies. See
	// [[ADR-266-one-set-of-rules-for-keeping-a-recording]].
	OwnerTailOnly bool
	// StopWhen ends a recording from outside the captured input — the button, rather
	// than the stop key. Nil leaves the gesture as the only way to finish.
	//
	// Both ways stay live together: a recording is ended by whichever arrives first, so
	// a window that has become unresponsive never traps a recording with no way out.
	StopWhen <-chan struct{}
	// ArgKey is the reserved demonstration key that drops a positional argument
	// placeholder ("an argument goes here" → {{N}}); empty uses simplify.DefaultArgKey
	// (F9). Set from $MARCO_ARG_KEY by the binary. "off" disables it.
	ArgKey string
	// Mode is what a recording started through this Deps is told to capture, or
	// Unstated — the zero value — to leave it to $MARCO_CV / $MARCO_ANCHORS.
	//
	// # Why a caller may state it, and why almost none do
	//
	// Because the CV switches are read once when a process starts, which is the right
	// shape for a command and the wrong one for a window that stays open. `marco learn`
	// and the overlay are started, do a thing, and end; whatever you set is what you
	// asked for. Studio is open all afternoon with whatever environment launched it, and
	// with anchors inherited on, a click whose crop happens to be distinctive carries a
	// picture — which makes the whole guarded recording unsaveable. The same
	// demonstration then saves or is refused depending on a variable nobody in the window
	// can see.
	//
	// So a caller with a mode in mind states it here and it wins. Unstated is every
	// caller there was, and they are unchanged: see RecordedAs and recorder.Mode.
	Mode RecordedAs
	// afterClassifying occupies the one instant the review flow cannot otherwise be asked
	// about: between deciding which of a routine's images are missing and creating them.
	//
	// UNEXPORTED, and nil in every `Deps` anything builds — a `Deps` is made from its
	// exported fields everywhere, so nothing in production can set this and nothing has a
	// reason to be notified of that instant. That is the point. The gap is closed by
	// creating the images exclusively rather than by being narrow, and the only way to
	// show a gap is closed is to put something in it.
	//
	// The same discipline as the stop checks in `internal/oshost`: a tested mechanism is
	// not a tested moment, and the moment is where this product's defects have been. See
	// `TestAnImageThatAppearsAfterClassificationIsRefused`.
	afterClassifying func()
	// afterDestination occupies the instant a MOVE cannot otherwise be asked about: the
	// destination is written and the original is still there, which is the one moment in
	// a rescope when both exist.
	//
	// Everything after it — reading the destination back, revalidating the original,
	// removing it — is about what can happen in exactly this interval. UNEXPORTED and nil
	// in every `Deps` anything builds, for the same reason as the seam above: the way to
	// show that an interval is handled is to stand in it.
	//
	// See `TestAMoveThatCannotRemoveTheOriginalSaysCopied`.
	afterDestination func()
}

// learnOptions are the simplify defaults for a demonstration, honoring the
// configured arg key.
func (d Deps) learnOptions() simplify.Options {
	return d.learnOptionsFor(d.recordedAs())
}

// learnOptionsFor is the same for ONE recording whose mode is known, which is not always this
// Deps'.
//
// The distinction is load-bearing and was missing. `Deps.Mode` is what a recording STARTED
// through this Deps is told to capture, and it is the right answer while a window is recording.
// It is the wrong answer for a recording that already happened: a repair rebuilds a saved
// routine's steps from the demonstration stored beside it, under a `Deps` built for the repair,
// which states nothing. Simplifying under that Deps' mode capped the recorded gaps to 50ms of a
// routine whose file says it was recorded as coordinates — taking the pauses out of a routine
// that has no anchor to wait on instead, which is a menu pressed before it has drawn.
//
// So the mode travels with the recording, from the capture that ran or from the source that
// states it, and the environment decides only where nothing says otherwise.
func (d Deps) learnOptionsFor(mode RecordedAs) simplify.Options {
	o := simplify.DefaultOptions()
	if k := strings.TrimSpace(d.ArgKey); k != "" {
		o.ArgKey = strings.ToLower(k)
	}
	// With CV/anchors OFF there is no wait-gate to absorb menu-transition time, so the ROUTE
	// must carry the real pauses: preserve the recorded timings faithfully instead of capping
	// every wait to 50ms (the cap only made sense when the CV poll did the real waiting).
	//
	// A STATED COORDINATE MODE IS ASKED FIRST, and for the same reason it exists at all.
	// `cvOff` reads the environment, so a Studio launched with `$MARCO_CV=on` capped every
	// recorded gap to 50ms while capturing no anchor to wait on — a routine with the
	// pauses taken out and nothing put in their place, which is a menu pressed before it
	// has drawn. The stated mode is about this recording and the variable is about the
	// process, so the statement wins.
	if mode == Coordinates || cvOff() {
		o.MaxWaitMs = 24 * 60 * 60 * 1000 // effectively no cap — keep the exact recorded gaps
	}
	return o
}

// cvOff reports whether computer vision / anchors are OFF — the DEFAULT now (the anchor stack
// isn't reliable yet, so CV is a feature flag). Off unless explicitly enabled with $MARCO_CV=
// on/max (or $MARCO_ANCHORS=1/on); when off, learn preserves the recorded timings faithfully.
// Kept consistent with recorder.anchorsEnabled and oshost.cvOff.
//
// # OPEN QUESTION: this default is an engineering flag, not a product decision
//
// Nobody has decided what Marco SHOULD do here. The default is off because the anchor stack was
// not reliable enough on the day somebody needed it out of the way, and it has stayed off since by
// inertia. Written down deliberately in Phase 3, because the difference is large and invisible:
// with CV off a learned play carries the exact recorded gaps and re-clicks the exact recorded
// pixels, so a moved menu misses; with CV on it waits on an image and finds a control that moved,
// at the cost of a screen capture and a match per anchored click.
//
// Answering it needs an EXPERIMENT — a real demonstration, replayed both ways, on a UI that
// actually moves — and that experiment is not this. Do not flip this default because the code
// looks tidier one way; flip it when there is evidence, and record the evidence in a decision.
func cvOff() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_CV"))) {
	case "max", "1", "on", "all", "true", "yes":
		return false // CV explicitly on
	case "0", "off", "false", "no", "none":
		return true // CV explicitly off
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_ANCHORS"))) {
	case "1", "on", "true", "yes":
		return false // anchors explicitly on
	}
	return true // default: CV off
}

// argKeyHint is the recording prompt's note about the arg key (or "" when off),
// listing the declared arg names when the learn phrase had a "with …" clause.
func (d Deps) argKeyHint(argNames []string) string {
	k := d.learnOptions().ArgKey
	if k == "" || k == "off" {
		return ""
	}
	if len(argNames) > 0 {
		return fmt.Sprintf(" (tap %s where each arg goes, in order: %s)", strings.ToUpper(k), strings.Join(argNames, ", "))
	}
	return fmt.Sprintf(" (tap %s where a value should be an argument)", strings.ToUpper(k))
}

// ArgKeyFor resolves the reserved demonstration key for "an argument goes here" from an
// override — $MARCO_ARG_KEY as a binary read it — and answers "" when there is none.
//
// ONE READING OF ONE OVERRIDE, because two readings is a defect this product has already
// shipped. The STOP key was parsed at one site and passed raw at another, so an unset
// $MARCO_STOP_KEY read as "there is no shortcut" on Studio's record card and as F12 in its
// recording band, three rows apart, about the same keyboard. `learnOptionsFor` resolves this
// one for the recorder that consumes it; a surface that has to tell somebody the key exists
// asks here rather than reading the variable a second time and getting the fallback wrong.
//
// The empty answer means the key is OFF, which is a thing to say nothing about rather than a
// thing to apologise for: a window with no arg key has no arg key to offer.
func ArgKeyFor(override string) string {
	k := Deps{ArgKey: override}.learnOptionsFor(Unstated).ArgKey
	if k == "off" {
		return ""
	}
	return k
}

// anchorKeyHint is the recording prompt's note about the anchor key (or "" when
// off): tap it, then click a moving target (a menu item) to match that click by
// image at run time, with the recorded coordinate as fallback.
func anchorKeyHint() string {
	if k := recorder.AnchorKey(); k != "" {
		return fmt.Sprintf(" (tap %s then click a moving target to match it by image)", strings.ToUpper(k))
	}
	return ""
}

// activeApp returns the foreground app name, or "" when unavailable.
func (d Deps) activeApp() string {
	if d.App == nil {
		return ""
	}
	return d.App()
}

// Learn records a demonstration of name, makes a DRAFT of it, prompts at every step — what was
// recorded, whether to simplify further, and where the play should work — and keeps it through the
// one set of save rules.
//
// LEARN is the person acting while Marco watches — see the LEARN/TEACH/DO invariant in CLAUDE.md.
// This doc comment said "Teach records…" until Phase 3, left behind by the acquisition rename;
// TEACH is reserved for the mirror image (Marco guiding a person) and must not name this.
//
// The user performs the actions and presses the stop gesture (F12 by default, `StopKey`) to
// finish. Esc is deliberately NOT the default, so a play can record Esc as an ordinary key.
//
// # It is an ADAPTER now, and it used to be a second implementation
//
// It started the recorder itself, waited for the stop itself, simplified itself, and wrote
// through `saveTaught` — the same pipeline Studio reaches through `Begin` → `Finish` → `Save`,
// written again. The two then disagreed about what saving means, and the disagreement cost
// somebody's routine: learning a name twice REPLACED the first one, with nothing asked and
// nothing said. See keep.go.
//
// So the recording, the simplification and the save rules are the shared ones, and what is left
// here is the INTERACTION: four prompts on a terminal, which is what this surface is.
func (d Deps) Learn(name string) error {
	base, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	fmt.Fprintf(d.Out, "Show me how to %q. Do it now, then press %s when finished.\n", base, stop.Label())
	fmt.Fprintf(d.Out, "(For a password, type {{name}} and set it with: marco secret set <name>.%s)%s\n", d.argKeyHint(argNames), anchorKeyHint())

	// THE LEGACY OWNER RULE, ASKED FOR RATHER THAN INHERITED. See Deps.OwnerTailOnly:
	// this surface strips the owner only off a button-ended recording, which is what it
	// has always done, and moving onto the shared pipeline must not change it in passing.
	c, err := d.asTerminal().Begin(name)
	if err != nil {
		return err
	}
	dr := c.Finish()
	if len(dr.Steps) == 0 {
		mlog.Warn("record: nothing recorded", "route", dr.Name)
		fmt.Fprintln(d.Out, "Nothing was recorded — not saving.")
		return nil
	}
	// AND A GESTURE THAT CANNOT BE WRITTEN DOWN IS NOT WRITTEN DOWN AS SOMETHING ELSE.
	//
	// Said and not saved, in the same shape as the empty recording above: the person
	// performed the gesture and is the only one who can perform it again differently, so
	// the sentence names what it was and what to do instead. The alternative is a routine
	// that quietly does something else and reads as a faithful recording of what was
	// shown. See simplify.modifierTrouble.
	if dr.Unrepresentable != "" {
		mlog.Warn("record: refused", "route", dr.Name, "why", dr.Unrepresentable)
		fmt.Fprintf(d.Out, "Not saving: %s.\n", dr.Unrepresentable)
		return nil
	}
	if dr.App != "" {
		fmt.Fprintf(d.Out, "(context: %s)\n", dr.App)
	}
	// AND WHY THE DRAFT IS SHORTER THAN THE DEMONSTRATION. Before the steps, because it
	// is about the steps that are not there. See Draft.AwaySentence.
	if said := dr.AwaySentence(); said != "" {
		fmt.Fprintln(d.Out, said)
	}
	// AND WHAT WAS LEFT OUT OF IT, for the same reason and in the same place: it is about
	// steps that are not there. See Draft.ExitSentence.
	if said := dr.ExitSentence(); said != "" {
		fmt.Fprintln(d.Out, said)
	}
	if err := d.preview("Recorded", dr); err != nil {
		return err
	}

	// Save / discard / simplify-further. Simplify is a one-shot (it folds as far as it
	// goes), so once it has run the prompt drops back to just save/discard — and it
	// rebuilds the DRAFT from the demonstration rather than re-folding the steps, which
	// is what lets typing coalesce before loop-folding ever sees repeated letters.
	simplified := false
	for {
		switch d.chooseSave(dr.Name, !simplified) {
		case actionDiscard:
			// NOTHING WAS WRITTEN, which is the whole of Cancel: the draft was
			// never on disk, so dropping it is the entire operation.
			fmt.Fprintln(d.Out, "Discarded.")
			return nil
		case actionSave:
			return d.keepWhatWasShown(dr)
		}
		next := d.SimplifiedDraft(dr)
		if next.Unrepresentable != "" {
			// A REFUSAL CANNOT ARRIVE HERE THAT WAS NOT ALREADY TRUE ABOVE.
			// "Simplify further" re-runs the same pipeline over the same events
			// with typing folded harder, and what gets refused is a property of
			// the EVENTS. Answered anyway rather than dropped: a reason that
			// could only appear by surprise is the one worth not swallowing.
			fmt.Fprintf(d.Out, "Not saving: %s.\n", next.Unrepresentable)
			return nil
		}
		dr, simplified = next, true
		if err := d.preview("Simplified to", dr); err != nil {
			return err
		}
		// On a front-end that can't show the preview (the overlay), "simplify" means
		// "simplify and save" — don't re-prompt for a result the user can't see.
		if d.SimplifySaves {
			return d.keepWhatWasShown(dr)
		}
	}
}

// preview prints the route a draft would become, without writing anything.
//
// With relative template names: the saved copy embeds the real per-scope directory once the scope
// is chosen, and a preview that embedded one would be showing a path the save may not use.
func (d Deps) preview(said string, dr *Draft) error {
	src, _, err := codegen.Route(dr.Name, dr.App, dr.Steps, "", dr.Args...)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "%s %d steps:\n--- route ---\n%s\n", said, len(dr.Steps), src)
	return nil
}

// nameWithArgs puts a draft's argument names back on its name, which is what `draftFor` splits.
//
// Because a draft has already had them taken off. Rebuilding from `dr.Name` alone would lose the
// declared arguments, and a routine that quietly stopped taking the arguments somebody declared
// is a routine that looks fine and refuses when it is asked for.
func nameWithArgs(dr *Draft) string {
	if len(dr.Args) == 0 {
		return dr.Name
	}
	return dr.Name + " with " + strings.Join(dr.Args, ", ")
}

// keepWhatWasShown asks where the routine should work and keeps it there.
//
// The two refusals a terminal can answer are answered HERE rather than guessed around: a scope
// this recording cannot be performed under is re-asked with the one that works named, and a name
// already in the library is a question with Enter meaning NO.
func (d Deps) keepWhatWasShown(dr *Draft) error {
	scope, ok := d.askScope(dr)
	if !ok {
		fmt.Fprintln(d.Out, "Left alone. Nothing was saved.")
		return nil
	}
	// REFUSED AND NOT RE-ASKED, which is the bounded shape of this.
	//
	// `askScope` already declines to offer a scope this recording cannot be performed
	// under, so reaching this is a disagreement between the two and not a person's
	// mistake. The first version looped back to the question instead — and `readLine` on a
	// closed stdin answers "" for ever, so a terminal whose input had gone would have sat
	// there asking itself the same question until somebody killed it.
	if why := d.WhyNotThisScope(dr, scope); why != "" {
		fmt.Fprintf(d.Out, "Not saving: %s\n", why)
		return nil
	}
	{
		rt, err := d.Keep(dr, scope, "", Consent{})
		if err == nil {
			d.saidKept(dr, scope, rt, "Learned")
			return nil
		}
		// ── IT IS SAVED, AND WHAT FAILED IS THE ACCOUNT BESIDE IT ──
		//
		// Before the collision question below, which would otherwise be asked
		// about the routine this save just wrote: `Collision` would find it,
		// report it as taken, and offer to replace it. Saying "Learned" and then
		// the caveat is the whole answer — the routine runs, and the sentence
		// says what will refuse later and why. See ErrStepsNotWritten.
		if errors.Is(err, ErrStepsNotWritten) {
			d.saidKept(dr, scope, rt, "Learned")
			fmt.Fprintf(d.Out, "%s\n", err)
			return nil
		}
		// ── ALREADY THERE, AND THE QUESTION NAMES EXACTLY WHAT WOULD GO ──
		//
		// Not a retry of the same save: `Keep` refused, and what follows is a
		// DIFFERENT request — replace that one identity and no other — which it
		// re-checks at the instant of writing. Enter means no, because the cost of
		// the two answers is not the same: no costs a second attempt, and yes
		// costs a routine and the demonstration beside it, neither of which comes
		// back.
		at := d.Collision(dr, scope, "")
		if !at.Exists {
			return err
		}
		fmt.Fprintf(d.Out, "%s\n", at.Says)
		if !d.confirmDefaultNo(fmt.Sprintf("Replace %q (%s) in the library — "+
			"its steps and its demonstration? [y]es / [N]o: ",
			at.Name, at.ID)) {
			fmt.Fprintln(d.Out, "Left alone. Nothing was saved.")
			return nil
		}
		// THE VERSION THEY WERE SHOWN, not just the route they named: the
		// question quoted this routine and this is the one they agreed to lose.
		// See orchestrator.Consent.
		rt, err = d.Keep(dr, scope, "", Consent{ID: at.ID, Source: at.Source,
			Recording: at.Recording})
		if err != nil {
			return err
		}
		d.saidKept(dr, scope, rt, fmt.Sprintf("Replaced %q with", at.Name))
		return nil
	}
}

// saidKept is the one line a terminal gets when something was written.
func (d Deps) saidKept(dr *Draft, scope Scope, rt routes.Route, kept string) {
	mlog.Info("record: saved", "route", dr.Name, "scope", string(scope),
		"steps", len(dr.Steps), "path", d.Reg.Path(rt))
	fmt.Fprintf(d.Out, "%s %q (%s) → %s\n", kept, dr.Name, scopeLabel(scope.mode(), dr.App),
		d.Reg.Path(rt))
}

// askScope asks where the routine should work, offering only the scopes it can be saved under,
// and reports false when nothing should be saved.
//
// Where the recording's presses check their own window there is only one answer, and it IS said —
// because a person who expected "only here" needs to know why they are not being offered it, and a
// surface that silently narrowed the choice would be the one deciding.
//
// ENTER IS THE RULED DEFAULT, FROM ANYWHERE → THIS APPLICATION, and the prompt says that Enter
// changed: it used to mean "only in", and somebody with that in their fingers deserves to read the
// difference before it costs them a routine that switches windows.
//
// WITH NO APPLICATION THERE IS NO DEFAULT. It used to return the current-foreground scope without
// asking, because that was the only one left; the ruling is ask or refuse. So it is asked, with
// Enter meaning NO, and with nobody to ask it refuses.
func (d Deps) askScope(dr *Draft) (Scope, bool) {
	def, why := DefaultScope(dr)
	if def == "" {
		fmt.Fprintf(d.Out, "Not saved yet: %s.\n", why)
		if !d.confirmDefaultNo(fmt.Sprintf("Save it as %q? [y]es / [N]o: ",
			ScopeLabel(ScopeEverywhere, ""))) {
			return "", false
		}
		return ScopeEverywhere, true
	}
	if ScopeFor(dr) != "" {
		fmt.Fprintf(d.Out, "Every press in this recording checks the window it was made "+
			"in, so it has to be %q: %q or %q would refuse every press.\n",
			ScopeLabel(def, dr.App), ScopeLabel(ScopeApp, dr.App),
			ScopeLabel(ScopeEverywhere, dr.App))
		return def, true
	}
	return d.askAmongScopes(dr.App), true
}

// askAmongScopes is the three-way question, Enter meaning From anywhere → app.
//
// Shared by the demonstration path and the narration path so the two cannot ask it differently.
func (d Deps) askAmongScopes(app string) Scope {
	fmt.Fprintf(d.Out, "Enter now means %q (it used to mean %q).\n",
		ScopeLabel(ScopeAppAnywhere, app), ScopeLabel(ScopeApp, app))
	for {
		fmt.Fprintf(d.Out, "Where? [f] %s (Enter) / [c] %s / [g] %s: ",
			ScopeLabel(ScopeAppAnywhere, app), ScopeLabel(ScopeApp, app),
			ScopeLabel(ScopeEverywhere, app))
		if d.In == nil {
			return ScopeAppAnywhere
		}
		switch strings.TrimSpace(strings.ToLower(readLine(d.In))) {
		case "", "f", "focus", "switch":
			return ScopeAppAnywhere
		case "c", "context", "here", "only here":
			return ScopeApp
		case "g", "global", "anywhere", "everywhere":
			return ScopeEverywhere
		}
		fmt.Fprintf(d.Out, "Please answer f (%s), c (%s) or g (%s).\n",
			ScopeLabel(ScopeAppAnywhere, app), ScopeLabel(ScopeApp, app),
			ScopeLabel(ScopeEverywhere, app))
	}
}

// scopeMode is where a recorded route is available and whether it switches windows.
type scopeMode int

const (
	scopeContext scopeMode = iota // only when <app> is in front; no window switch
	scopeFocus                    // anywhere; brings <app> to the front first
	scopeGlobal                   // anywhere; no window switch (app-agnostic actions)
)

// chooseScope asks where the route should work, through the same question `askScope` asks, and
// reports false when nothing should be saved.
//
// FROM ANYWHERE → THIS APPLICATION is the default (the ruling; see scopeterms.go). It used to be
// "only here", and with no application it used to return the current-foreground scope without
// asking. Now no application is a yes/no question with Enter meaning no, and with nobody to ask
// it refuses.
func (d Deps) chooseScope(app string) (scopeMode, bool) {
	if strings.TrimSpace(app) == "" {
		_, why := DefaultScope(&Draft{})
		fmt.Fprintf(d.Out, "Not saved yet: %s.\n", why)
		if !d.confirmDefaultNo(fmt.Sprintf("Save it as %q? [y]es / [N]o: ",
			ScopeLabel(ScopeEverywhere, ""))) {
			return scopeGlobal, false
		}
		return scopeGlobal, true
	}
	return d.askAmongScopes(app).mode(), true
}

// saveTaught writes a freshly recorded route under the chosen scope. Only the focus
// mode keeps the leading Activate (window switch); context and global strip it.
// events (the raw demonstration) is persisted beside the route when non-nil so it
// can be re-simplified later. Shared by demonstration and narration learn.
func (d Deps) saveTaught(name string, argNames []string, steps []macroir.Step, app string, events []recorder.RecordedEvent) error {
	mode, ok := d.chooseScope(app)
	if !ok {
		fmt.Fprintln(d.Out, "Left alone. Nothing was saved.")
		return nil
	}
	rt := routes.Route{Slug: routes.Slug(name)}
	codegenApp := "" // only focus prepends Activate(app) so the run switches first
	switch mode {
	case scopeContext:
		rt.App = app // routes/<app>/context — foreground-only, no switch
		steps = stripLeadingActivate(steps)
	case scopeFocus:
		rt.App, rt.Focus = app, true // routes/<app>/focus — switch to the app first
		codegenApp = app
	default: // scopeGlobal: routes/global — app-less, no switch
		steps = stripLeadingActivate(steps)
	}
	finalSrc, assets, err := codegen.Route(name, codegenApp, steps, d.Reg.ScopeDir(rt), argNames...)
	if err != nil {
		return err
	}
	mlog.Info("record: saving", "route", name, "mode", mode, "path", d.Reg.Path(rt))
	if err := d.Reg.Save(rt, finalSrc); err != nil {
		mlog.Error("record: save failed", "route", name, "err", err)
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		mlog.Error("record: asset write failed", "route", name, "err", err)
		return err
	}
	// Keep the raw demonstration beside the route so it can be re-simplified later
	// (marco simplify). Best-effort: a failure here doesn't fail the learn.
	if events != nil {
		if data, mErr := json.Marshal(events); mErr == nil {
			_ = d.Reg.SaveRecording(rt, data)
		}
	}
	mlog.Info("record: saved", "route", name, "steps", len(steps), "path", d.Reg.Path(rt))
	fmt.Fprintf(d.Out, "Learned %q (%s) → %s\n", name, scopeLabel(mode, app), d.Reg.Path(rt))
	return nil
}

// scopeLabel describes a saved route's reach for the "Learned …" line.
//
// The shared vocabulary's label, so the line a terminal prints after a save names the scope the
// way every other surface does.
func scopeLabel(mode scopeMode, app string) string {
	return ScopeLabel(scopeOfMode(mode), app)
}

// stripLeadingActivate drops a route's opening Activate (and the window-settle wait
// after it), so a context/global route doesn't switch windows when it runs.
func stripLeadingActivate(steps []macroir.Step) []macroir.Step {
	if len(steps) > 0 && steps[0].Kind == macroir.StepActivate {
		steps = steps[1:]
		if len(steps) > 0 && steps[0].Kind == macroir.StepWait {
			steps = steps[1:]
		}
	}
	return steps
}

// LearnAuto records a demonstration and saves it with NO interactive prompts —
// it simplifies with defaults and scopes the route to the app it was recorded in
// (global only if no app context). This is what the overlay uses so learning is
// driven entirely from the HUD (record → F12 → saved) without a console window.
func (d Deps) LearnAuto(name string) error {
	base, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	fmt.Fprintf(d.Out, "Recording %q — do it now, press %s when done.%s%s\n", base,
		stop.Label(), d.argKeyHint(argNames), anchorKeyHint())

	c, err := d.asTerminal().Begin(name)
	if err != nil {
		return err
	}
	dr := c.Finish()
	if len(dr.Steps) == 0 {
		mlog.Warn("record-auto: nothing recorded", "route", dr.Name)
		fmt.Fprintln(d.Out, "Nothing was recorded — not saving.")
		return nil
	}
	// AND THE SAME REFUSAL, which matters MORE here than in the reviewed path: auto-learn
	// saves without showing anybody the route first, so a gesture that came out as
	// something else would reach the library with nobody having looked at it.
	if dr.Unrepresentable != "" {
		mlog.Warn("record-auto: refused", "route", dr.Name, "why", dr.Unrepresentable)
		fmt.Fprintf(d.Out, "Not saving: %s.\n", dr.Unrepresentable)
		return nil
	}
	// ── A SCOPE THIS RECORDING CAN ACTUALLY BE PERFORMED UNDER ──
	//
	// Auto-learn defaulted to a CONTEXT route for the app it was recorded in, global if
	// there was no app. For a recording whose presses check their own window that is a
	// routine that refuses every press, saved with nobody having looked at it — measured, on
	// this path. So the draft is asked what it can be saved as first, and the default is
	// what is left.
	//
	// THE SHARED DEFAULT, and with no application NO save at all. This used to fall back to
	// "only in" with an application and to the current-foreground scope without one; the
	// ruling is From anywhere → this application, and with no application ask or refuse.
	// Nobody is here to ask.
	scope, noDefault := DefaultScope(dr)
	if scope == "" {
		mlog.Warn("record-auto: no scope to save under", "route", dr.Name, "why", noDefault)
		fmt.Fprintf(d.Out, "Not saving: %s, and an unattended recording cannot ask.\n",
			noDefault)
		return nil
	}
	if why := d.WhyNotThisScope(dr, scope); why != "" {
		// UNREACHABLE AND REFUSED ANYWAY. `ScopeFor` just named this scope, so it
		// cannot be one the draft rejects; if that ever stops being true the answer
		// is to refuse rather than to write the routine it would refuse to perform.
		mlog.Warn("record-auto: no scope fits", "route", dr.Name, "why", why)
		fmt.Fprintf(d.Out, "Not saving: %s\n", why)
		return nil
	}
	// ── AND A COLLISION IS REFUSED, BECAUSE THERE IS NOBODY TO ASK ──
	//
	// This is the widget's unattended path: record, press F12, saved, no prompt and no
	// review. It REPLACED a routine of the same name — the library kept one, the file held
	// the second recording, and the line printed said `Learned` as though it were a fresh
	// save. Of the three surfaces this is the one where a silent overwrite is worst, so it
	// is the one that will not do it at all: the existing routine is named, it is left
	// exactly as it is, and nothing is written.
	rt, err := d.Keep(dr, scope, "", Consent{})
	if err != nil {
		mlog.Warn("record-auto: not saved", "route", dr.Name, "err", err)
		fmt.Fprintf(d.Out, "Not saving: %v\n", err)
		return nil
	}
	mlog.Info("record-auto: saved", "route", dr.Name, "scope", string(scope),
		"steps", len(dr.Steps), "path", d.Reg.Path(rt))
	fmt.Fprintf(d.Out, "Learned %q (%s), %d steps\n", dr.Name,
		scopeLabel(scope.mode(), dr.App), len(dr.Steps))
	return nil
}

// LearnVoice builds a route by narration instead of demonstration: each phrase from
// the phrases channel is parsed and applied to a voicelearn.Session reading the live
// cursor/screen through env (status, if non-nil, gets a short line per step for a
// HUD). It saves on "done" (scoped to the app, like LearnAuto), discards on "cancel".
func (d Deps) LearnVoice(name string, env voicelearn.Env, phrases <-chan string, status func(string)) error {
	name, argNames := routes.SplitArgs(name)
	mlog.Info("narrate: session started", "route", name, "args", len(argNames))
	sess := voicelearn.New(env, argNames...)
	fmt.Fprintf(d.Out, "Narrate %q — say what to do (\"click this\", \"type …\", \"wait for this screen\"), then \"done\".\n", name)
	for phrase := range phrases {
		cmd := voicelearn.Parse(phrase)
		mlog.Debug("narrate: phrase", "route", name, "phrase", phrase, "kind", cmd.Kind)
		st, done, canceled := sess.Apply(cmd)
		fmt.Fprintf(d.Out, "  %s\n", st)
		if status != nil {
			status(st)
		}
		if canceled {
			mlog.Info("narrate: canceled", "route", name)
			fmt.Fprintln(d.Out, "Canceled — nothing saved.")
			return nil
		}
		if done {
			mlog.Debug("narrate: done received", "route", name)
			break
		}
	}
	steps := sess.Steps()
	if len(steps) == 0 {
		mlog.Warn("narrate: nothing recorded", "route", name)
		fmt.Fprintln(d.Out, "Nothing narrated — not saving.")
		return nil
	}
	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	// Same save / scope prompts as demonstration learn (no simplify — narration has
	// no raw event stream to re-fold, and no recording is persisted).
	preview, _, err := codegen.Route(name, app, steps, "", argNames...)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Narrated %d steps:\n--- route ---\n%s\n", len(steps), preview)
	if d.chooseSave(name, false) == actionDiscard {
		fmt.Fprintln(d.Out, "Discarded.")
		return nil
	}
	return d.saveTaught(name, argNames, steps, app, nil)
}

// SimplifyRoute re-renders an already-saved routine from its stored demonstration with the typed
// runs folded further, shows what that would be, and replaces the routine if the person says so.
//
// # It goes through the review flow, and that closed four things
//
// This used to resolve a name, re-simplify, print, prompt and write. Each of those was wrong in a
// way that had already been ruled on somewhere else:
//
//   - IT SIMPLIFIED UNDER THE ENVIRONMENT'S MODE. `d.learnOptions()` reads this process's idea of
//     what a recording captures, and the simplification depends on it: with no anchor to wait on,
//     the recorded gaps are the only waiting a routine has. A routine whose file states
//     `coordinates`, simplified by a process launched with `$MARCO_CV=on`, had every gap capped to
//     50ms and nothing put in its place — a menu pressed before it had drawn. The defect
//     [[ADR-254-what-a-recording-captures-is-stated]] fixed in `Resave`, still here until now.
//   - IT DELETED THE `// Recorded as:` LINE, because it wrote through `codegen.Route` rather than
//     `RouteAs`. The other half of the same defect: a claim about somebody's afternoon edited out
//     of their file by a tidy-up.
//   - IT WROTE WITH `Reg.Save` AND `Reg.WriteAssets` — create-or-truncate on the source, and
//     overwriting on the images. Both refused on the review path; see `Deps.Replace`.
//   - AND NOTHING CHECKED THE FILE WAS STILL THE ONE IT HAD PRINTED, across a gap containing a
//     human answering a prompt — the longest such window in this product.
//
// So the rebuild, the mode, the scope, the token, both digests and the writing are the review
// flow's, and what is left here is a command: find exactly one routine, show it, ask, replace.
func (d Deps) SimplifyRoute(name string) error {
	rt, err := d.oneRoutineCalled(name)
	if err != nil {
		return err
	}
	p, err := d.Prepare(rt, RebuildSimplifyTyping)
	if err != nil {
		return err
	}
	// WHICH FILE THIS IS ABOUT, BEFORE ANYTHING ELSE. A name is what the person typed and
	// a routine is a place; printing the place is what lets them stop if it is not the one
	// they meant. See `oneRoutineCalled` for why this cannot be the wrong one.
	fmt.Fprintf(d.Out, "%q in %s\n", p.Name, whereItLives(rt))
	switch {
	case p.Refused != "":
		// A REFUSAL LEAVES THE SAVED ROUTINE ALONE, which is the right answer here
		// and a different one from the record paths: there is already a routine on
		// disk, and replacing it with a rebuild that could not represent one of its
		// gestures would lose something the person already has.
		mlog.Warn("simplify: refused", "route", rt.Slug, "why", p.Refused)
		fmt.Fprintf(d.Out, "Leaving it as it is: %s\n", p.Refused)
		return nil
	case !p.Changes():
		fmt.Fprintln(d.Out, "Simplifying it would change nothing — leaving it as is.")
		return nil
	}
	fmt.Fprintf(d.Out, "%s\n--- route ---\n%s\n", p.Rebuild.Caution(), p.Now)
	// AND A CALLER MAY SAY IN ADVANCE THAT IT CANNOT SHOW A PREVIEW.
	//
	// `SimplifySaves` is that statement, made by whoever built this `Deps`. It is not
	// evidence of WHO the caller is — an environment variable cannot establish that, and
	// a surface that can show a comparison must not be excused from doing so by one. The
	// overlay sets it because it has no way to render two sources; Studio never does, and
	// never reads it.
	//
	// What it skips is the QUESTION. It does not skip the preparation, the digests or the
	// checked replacement below, so an unattended save still cannot truncate an image or
	// write over a file that moved while this was deciding.
	if !d.SimplifySaves && !d.confirmDefaultNo("Save the simplified version? [y]es / [N]o: ") {
		fmt.Fprintln(d.Out, "Left unchanged.")
		return nil
	}
	if err := d.Replace(p); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Saved %q (%d steps) → %s\n", p.Name, len(p.assets)+countSteps(p.Now),
		d.Reg.Path(rt))
	return nil
}

// whereItLives is a routine's place, in the words the product uses for it.
func whereItLives(rt routes.Route) string {
	switch {
	case rt.App == "":
		return "no application (it works anywhere and switches nothing)"
	case rt.Focus:
		return rt.App + ", brought forward first"
	}
	return rt.App + ", only while it is in front"
}

// oneRoutineCalled is the lookup a command that REWRITES a file may make.
//
// # Why it is not Registry.Resolve
//
// Because `Resolve` is built for PERFORMING: it tries the foreground application's routes, then
// any other application's focus route with that slug, then a global one. Reaching into another
// application is exactly right for "do the thing I named" — a focus route brings its application
// forward, so it works from wherever you are. It is exactly wrong for "rewrite the thing I
// named": with `settings` saved for two applications, simplifying `settings` from a third would
// find whichever came first in a sorted list and overwrite it.
//
// `marco simplify` already guards one half of this — it resolves exactly rather than by fuzzy
// match, because a guess that rewrites a file destroys somebody's work — and the guard was
// correct about the MATCHING and not about the SELECTION. An exact slug shared by two routines is
// still two routines.
//
// So every routine with that slug is found, and more than one is a refusal naming them. A command
// that takes a name cannot say which it meant, so the refusal says where the choice can be made.
// Local to Simplify on purpose: `Resolve` is right for every caller that performs.
func (d Deps) oneRoutineCalled(name string) (routes.Route, error) {
	slug := routes.Slug(name)
	if slug == "" {
		return routes.Route{}, fmt.Errorf("%q is not a name I can look up", name)
	}
	var found []routes.Route
	for _, rt := range d.Reg.List() {
		if rt.Slug == slug {
			found = append(found, rt)
		}
	}
	switch len(found) {
	case 0:
		return routes.Route{}, fmt.Errorf("I don't know %q", name)
	case 1:
		return found[0], nil
	}
	var where []string
	for _, rt := range found {
		where = append(where, "  "+whereItLives(rt))
	}
	return routes.Route{}, fmt.Errorf("there are %d routines called %q and simplifying "+
		"rewrites one of them, so I will not choose:\n%s\nOpen Studio and review a "+
		"rebuilt version of the one you mean — its card says which application and "+
		"scope it is in", len(found), name, strings.Join(where, "\n"))
}

// countSteps is how many `do` sentences a generated routine has, for the line a person reads.
//
// Counted off the SOURCE rather than carried from the draft, because the source is what was
// written and a chain emits one `do` per action either way. It is a number in a message and
// nothing depends on it.
func countSteps(src string) int { return strings.Count(src, "    do ") }

// enrichAnchors labels each DEMONSTRATED anchor — a click the recorder captured a button
// template for — with what the available resolver hosts can recognise: the on-screen TEXT
// (the Text host's Read) and/or the element CLASS (the Vision host's Identify) under the
// click. Each recognised signal becomes a run-time `Find`/`Locate` fallback, so a moved
// control (text), or one with no text and no clean edge — an icon on a gradient (vision) —
// still resolves. This is the SINGLE learn path: demonstrate once, the anchor gains
// whatever resolvers are wired. Best-effort and graceful — a host that's absent (no
// $MARCO_OCR / $MARCO_VISION) or can't read leaves that signal off and the anchor keeps
// whatever it has (image/colour/edge). Mutates steps in place, recursing loop bodies.
func (d Deps) enrichAnchors(steps []macroir.Step) {
	text, vision := d.Hosts["Text"], d.Hosts["Vision"]
	if text == nil && vision == nil {
		return
	}
	d.labelAnchors(text, vision, steps)
}

// labelAnchors walks steps (and nested loop bodies), labelling each anchored click via the
// wired hosts — OCR text into AnchorText, the detected element class into AnchorVision —
// skipping a signal already present (e.g. AnchorText from an earlier pass).
func (d Deps) labelAnchors(text, vision runtime.Host, steps []macroir.Step) {
	for i := range steps {
		s := &steps[i]
		if len(s.Steps) > 0 {
			d.labelAnchors(text, vision, s.Steps)
		}
		if s.Kind != macroir.StepClick || len(s.Template) == 0 {
			continue
		}
		if text != nil && s.AnchorText == "" {
			if t := readAnchorText(text, s.Template, s.AnchorClickX, s.AnchorClickY); t != "" {
				s.AnchorText = t
				mlog.Info("record: labelled anchor by OCR", "text", t)
			}
		}
		if vision != nil && s.AnchorVision == "" {
			if label, box, ok := readAnchorVision(vision, s.Template, s.AnchorClickX, s.AnchorClickY); ok {
				s.AnchorVision = label
				mlog.Info("record: labelled anchor by vision", "label", label)
				// Re-crop the template to the DETECTED button box — a pixel-tight anchor the
				// geometric crop can't produce (it fuses stacked menu buttons). Keeps the
				// recorded screen click; only the template image and the click-within-template
				// move. Best-effort: a bad/oversized box leaves the template untouched.
				if cropped, ncx, ncy, ok := cropTemplate(s.Template, box, s.AnchorClickX, s.AnchorClickY); ok {
					s.Template = cropped
					s.AnchorClickX, s.AnchorClickY = ncx, ncy
					mlog.Info("record: re-cropped anchor to detected button", "box", fmt.Sprintf("%dx%d", box[2], box[3]))
				}
			}
		}
	}
}

// readAnchorText OCRs one template via the Text host's Read action, returning the
// recognised label or "" on any miss (no text on the button, host failed/declined, or
// the OCR engine is absent). clickX,clickY locate the click WITHIN the template so Read
// labels the button under the click, not the whole crop; (0,0) lets Read fall back to
// the template centre. The bridge never errors the Go call — a failure arrives as a
// non-"ok" status — so a missing tesseract degrades to "no text anchor", never a failed
// learn.
func readAnchorText(host runtime.Host, template []byte, clickX, clickY int) string {
	in := runtime.NewSet()
	in.Put("Image", runtime.Text(base64.StdEncoding.EncodeToString(template)))
	in.Put("ClickX", runtime.Number(float64(clickX)))
	in.Put("ClickY", runtime.Number(float64(clickY)))
	status, data, err := host.Invoke(runtime.HostCall{
		Act:    "Text",
		Action: "Read",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    context.Background(),
	})
	if err != nil || status != "ok" || !data.IsSet() {
		return ""
	}
	if v, ok := data.AsSet().Get("Text"); ok {
		return strings.TrimSpace(v.AsText())
	}
	return ""
}

// readAnchorVision identifies the clicked control via the Vision host's Identify action,
// returning its CLASS label ("button", "icon", …) and its BOX (X,Y,W,H, image-local) under
// the click. ok=false on any miss (no element there, host failed/declined, or no model).
// Same graceful contract as readAnchorText: the bridge never errors the Go call, so a
// missing model degrades to "no vision anchor".
func readAnchorVision(host runtime.Host, template []byte, clickX, clickY int) (label string, box [4]int, ok bool) {
	in := runtime.NewSet()
	in.Put("Image", runtime.Text(base64.StdEncoding.EncodeToString(template)))
	in.Put("ClickX", runtime.Number(float64(clickX)))
	in.Put("ClickY", runtime.Number(float64(clickY)))
	status, data, err := host.Invoke(runtime.HostCall{
		Act:    "Vision",
		Action: "Identify",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    context.Background(),
	})
	if err != nil || status != "ok" || !data.IsSet() {
		return "", box, false
	}
	set := data.AsSet()
	if v, k := set.Get("Label"); k {
		label = strings.TrimSpace(v.AsText())
	}
	if label == "" {
		return "", box, false
	}
	box = [4]int{setNum(set, "X"), setNum(set, "Y"), setNum(set, "W"), setNum(set, "H")}
	return label, box, true
}

// setNum reads an integer field from a set (numbers arrive as float64), 0 if absent.
func setNum(set *runtime.Set, name string) int {
	if v, ok := set.Get(name); ok {
		if n, ok := v.AsNumber(); ok {
			return int(n)
		}
	}
	return 0
}

// cropTemplate re-crops a template PNG to box (X,Y,W,H, in the template's own pixels) — the
// vision-detected control — returning the new PNG and the click position WITHIN it. ok=false
// when the template isn't decodable, the box is degenerate, or it barely shrinks the
// template (no isolation gained), so the original is kept.
func cropTemplate(template []byte, box [4]int, clickX, clickY int) (out []byte, ncx, ncy int, ok bool) {
	img, err := png.Decode(bytes.NewReader(template))
	if err != nil {
		return nil, 0, 0, false
	}
	b := img.Bounds()
	r := image.Rect(box[0], box[1], box[0]+box[2], box[1]+box[3]).Add(b.Min).Intersect(b)
	if r.Dx() < minCropPx || r.Dy() < minCropPx {
		return nil, 0, 0, false
	}
	// Only worth it if the box actually isolates something smaller than the template.
	if r.Dx() >= b.Dx()-2 && r.Dy() >= b.Dy()-2 {
		return nil, 0, 0, false
	}
	sub := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(sub, sub.Bounds(), img, r.Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, sub); err != nil {
		return nil, 0, 0, false
	}
	return buf.Bytes(), clickX - (r.Min.X - b.Min.X), clickY - (r.Min.Y - b.Min.Y), true
}

// minCropPx is the smallest side a vision-cropped template may have — below it the detection
// is too tight to be a real control.
const minCropPx = 10

// firstActivateApp returns the app of the route's leading Activate step (the app
// the recorder saw in front when the demonstration began), or "" if the steps
// don't start by activating an app.
func firstActivateApp(steps []macroir.Step) string {
	if len(steps) > 0 && steps[0].Kind == macroir.StepActivate {
		return steps[0].Text
	}
	return ""
}

// waitForStop blocks until the stop gesture is seen on the recorder's event
// stream, or the stream closes.
// waitForStop blocks until the recording should end, by the stop gesture or from outside.
//
// `outside` is the button. Nil leaves the gesture as the only way to finish, which is what
// every caller but Studio passes. The two are a race on purpose: whichever arrives first ends
// the recording, so neither a key that cannot be pressed nor a window that has stopped
// answering can trap a recording with no way out.
//
// It returns which one ended it, because the two leave different tails behind: a gesture
// leaves keypresses, and a button leaves a trip to another window and a click in it.
// A LIVE INDICATOR, AND IT IS NOT OPTIONAL.
//
// A confined recording goes silent the moment somebody leaves the window, and silence looks
// exactly like a recording that is still going. Somebody who alt-tabs to check something and
// carries on demonstrating in the wrong place gets a routine missing half its steps and nothing
// told them. The review sentence afterwards is not a substitute: by then the demonstration they
// have to give again is already over.
//
// Said HERE because this function already reads every event, markers included, so there is one
// consumer of the channel and no second reader to steal from it. `watching` is who hears it: the
// terminal writes to its own output, and the overlay's HUD streams the same child's stdout, so
// both say it from one place. Studio does not come through here -- see Capture.Paused.
const (
	saidLeft = "-- paused: you have left the window being recorded. Nothing you do " +
		"elsewhere is recorded. --"
	saidBack  = "-- recording again --"
	saidGone  = "-- the window being recorded has closed, so the recording has ended --"
	saidCross = "-- that gesture crossed out of the window, so this cannot be saved: "
)

func waitForStop(rec recorder.Recorder, stop *recorder.StopKey,
	outside <-chan struct{}, watching func(string)) (byButton bool) {

	ch := rec.Events()
	if ch == nil {
		// NO EVENT STREAM, which is not a reason to ignore the button: a recorder
		// that cannot publish still records, and a caller holding a button is
		// entitled to have it work.
		if outside != nil {
			<-outside
			return true
		}
		return false
	}
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return false
			}
			if stop.Triggered(ev) {
				return false
			}
			// AND THE PERSON IS TOLD WHAT THE CAPTURE IS DOING, as it
			// happens. See saidLeft.
			if watching != nil {
				switch ev.Kind {
				case recorder.EvLeft:
					watching(saidLeft)
				case recorder.EvEntered:
					watching(saidBack)
				case recorder.EvTargetGone:
					watching(saidGone)
				case recorder.EvCrossed:
					watching(saidCross + ev.KeyName)
				}
			}
			// THE CHOSEN WINDOW CLOSED, which ends the recording: there is
			// nothing left to record into, and the handle may already belong
			// to somebody else's window. It is not a stop and not a button —
			// `Finish` reads which it was from the marker in the buffer.
			if ev.Kind == recorder.EvTargetGone {
				return false
			}
		case <-outside:
			return true
		}
	}
}

// firstReason is the first of the given reasons that is not empty.
//
// It exists so the ORDER is stated in one place and read where the refusal is assembled, rather
// than being whichever `if` somebody wrote last.
func firstReason(reasons ...string) string {
	for _, r := range reasons {
		if r != "" {
			return r
		}
	}
	return ""
}

// stripTrailingStop removes the stop-gesture keypresses (and any trailing moves)
// the stop leaves at the end of the recording.
func stripTrailingStop(events []recorder.RecordedEvent, stop recorder.StopKey) []recorder.RecordedEvent {
	end := len(events)
	for end > 0 {
		e := events[end-1]
		isStop := e.Kind == recorder.EvKey && stop.Has(e.KeyName)
		isMove := e.Kind == recorder.EvMove
		if isStop || isMove {
			end--
			continue
		}
		break
	}
	return events[:end]
}

// stripTrailingOwner removes the tail that belongs to ENDING a recording rather than to the
// recording itself.
//
// A person who stops by pressing a button has to reach Marco's window to press it. The poller
// records the switch, the hooks record the moves, the hooks record the click. Every bit of it is
// real input and none of it is what they were demonstrating — so left in, the saved routine
// raises Marco and presses Stop on every replay. The stop GESTURE never had this problem,
// because a key is not a place.
//
// Cut at the LAST switch into the owner, and only there: a demonstration that visited Marco in
// the middle and went back somewhere else is not this case, and its switch is not the last one.
// Everything after the person set off to press Stop was them pressing Stop, whatever it is.
//
// And when no switch was recorded at all — the poller samples on an interval, and a button can
// be pressed between two samples — the tail is dropped by the app each event was tagged with
// instead. Both rules are needed: the first catches the trip, the second catches a trip too
// quick to be sampled.
func stripTrailingOwner(events []recorder.RecordedEvent, owner string,
	ownerPID uint32) []recorder.RecordedEvent {

	if owner == "" && ownerPID == 0 {
		return events
	}
	// ── THE EVENT-TIME BOUNDARY, FIRST AND ON ITS OWN TERMS ──
	//
	// A click carries the process that owned the foreground window at the instant it
	// was made, so this rule holds whatever the poller had sampled — including the
	// case the name rules cannot see, where the poller still says the game and the
	// click that stopped the recording is tagged with the game.
	//
	// It runs before the name rules and does not depend on them. Moves come with it
	// because a move carries no attribution of its own and a trailing move was never
	// a step; a move is only ever consumed next to something that IS the owner's.
	end := len(events)
	if ownerPID != 0 {
		for end > 0 {
			e := events[end-1]
			if e.PID == ownerPID {
				end--
				continue
			}
			// A MOVE HAS NO OWNER BY CONSTRUCTION, so it is looked past —
			// and only ever to something the owner owns, never on its own
			// account.
			if e.Kind == recorder.EvMove && end > 1 &&
				events[end-2].PID == ownerPID {
				end--
				continue
			}
			// AND A RELEASE WITH NO OWNER, BUT ONLY ITS OWN PRESS'S.
			//
			// `winctx.ForegroundProcess` can fail, and a builder that
			// stamped one edge and not the other produced exactly this: a
			// release carrying nothing, which stopped the walk and hid the
			// correctly stamped press one event behind it. That hole is
			// closed in the builder and held by a test there; this is the
			// same hole closed here as well, because it has now been
			// opened twice in two different ways.
			//
			// Matched to its OWN press and nothing else — same button,
			// same point, owned by the owner. An unattributed event is not
			// shown to be the owner's by merely following one of theirs,
			// and a looser rule here would delete whatever happened to sit
			// after a click of Marco's.
			if e.Kind == recorder.EvClick && !e.Down && e.PID == 0 && end > 1 {
				p := events[end-2]
				if p.Kind == recorder.EvClick && p.Down &&
					p.PID == ownerPID &&
					p.Button == e.Button && p.X == e.X && p.Y == e.Y {
					end--
					continue
				}
			}
			break
		}
	}
	// ── AND THE NAME RULES, for what the event-time one cannot reach ──
	//
	// An app switch carries no process, so the trip into the owner's window is still
	// identified by name; and a key carries no process either. Cut at the LAST switch
	// into the owner and only there, so a demonstration that visited the owner in the
	// middle and went back keeps everything after it came back.
	for owner != "" && end > 0 {
		e := events[end-1]
		if e.Kind == recorder.EvAppSwitch {
			if e.KeyName == owner {
				end-- // the trip to press Stop
			}
			break // any other switch is where the demonstration was
		}
		// ── KNOWN OWNERSHIP OUTRANKS A SAMPLED NAME, IN BOTH DIRECTIONS ──
		//
		// The app tag is the poller's last sample and can name the owner about an
		// event that demonstrably is not the owner's. The sequence: visit Marco,
		// so the poller samples it; go back to the game and click before the next
		// sample, so the click carries the game's process and Marco's stale name;
		// then return to Marco and stop. The process rule above correctly keeps
		// that click — and the name rule would then delete it anyway, for being
		// tagged with a window it was not made in.
		//
		// So where the event says whose it is and the sample disagrees, the event
		// wins and the walk stops. Only where the owner's own process is unknown
		// is a name all there is to go on.
		if ownerPID != 0 && e.PID != 0 && e.PID != ownerPID {
			break
		}
		if e.App == owner || e.Kind == recorder.EvMove {
			end--
			continue
		}
		break
	}
	return events[:end]
}

// learnAction is the user's choice at the save prompt.
type learnAction int

const (
	actionSave learnAction = iota
	actionDiscard
	actionSimplify
)

// chooseSave presents the save menu and returns the chosen action. The
// "simplify further" option is offered only when canSimplify is true (it's a
// one-shot). It re-prompts on unrecognized input; an empty line (or no input)
// means save.
func (d Deps) chooseSave(name string, canSimplify bool) learnAction {
	for {
		if canSimplify {
			fmt.Fprintf(d.Out, "Save this as %q? [y]es / [N]o / [s]implify: ", name)
		} else {
			fmt.Fprintf(d.Out, "Save this as %q? [y]es / [N]o: ", name)
		}
		if d.In == nil {
			return actionSave
		}
		// ENTER MEANS NO (F6): a bare Enter discards, and only an explicit yes saves.
		switch strings.TrimSpace(strings.ToLower(readLine(d.In))) {
		case "y", "yes":
			return actionSave
		case "", "n", "no":
			return actionDiscard
		case "s", "simplify", "simplify further":
			if canSimplify {
				return actionSimplify
			}
		}
		if canSimplify {
			fmt.Fprintln(d.Out, "Please answer y (save), n (discard), or s (simplify).")
		} else {
			fmt.Fprintln(d.Out, "Please answer y (save) or n (discard).")
		}
	}
}

// asTerminal is this Deps with the owner rule the terminal and the widget have always used.
//
// A COPY, so nothing a caller handed in is mutated: a `Deps` is passed by value everywhere and a
// surface that found its own field changed by a call it made would be the kind of surprise this
// project spends its comments on. One line, in one place, named after the thing it is for.
func (d Deps) asTerminal() Deps {
	d.OwnerTailOnly = true
	return d
}

// confirmDefaultNo prints a prompt and reports whether the reply is an explicit yes.
//
// ENTER MEANS NO, which is the opposite of `confirm` one function down and is the whole reason
// this exists. `confirm` is for questions whose two answers cost about the same; this is for the
// one that does not — no costs a second attempt, and yes costs a routine and the demonstration
// beside it, neither of which comes back. A person pressing Enter through a prompt they did not
// read must not lose work by it.
func (d Deps) confirmDefaultNo(prompt string) bool {
	fmt.Fprint(d.Out, prompt)
	if d.In == nil {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(readLine(d.In))) {
	case "y", "yes":
		return true
	}
	return false
}

// readLine reads up to and including a newline from r, one byte at a time (no
// read-ahead), and returns the line without the trailing CR/LF.
func readLine(r io.Reader) string {
	var b []byte
	var one [1]byte
	for {
		n, err := r.Read(one[:])
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			if one[0] != '\r' {
				b = append(b, one[0])
			}
		}
		if err != nil {
			break
		}
	}
	return string(b)
}
