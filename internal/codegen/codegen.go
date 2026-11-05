// Package codegen turns a macro step list into a runnable Marco route on the OS
// act. The output is the same shape as the hand-written routes under routes/
// (an actor with a Run capability that does OS clicks/sleeps/keys/types, with
// loops emitted as `repeat N times...`). It is OS-agnostic.
package codegen

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// Generation is which generation of GENERATOR RULES this build writes a routine under.
//
// A routine keeps the shape it was generated with, for ever. Nothing rewrites saved Marco —
// deliberately, because the file is the only executable truth about a routine and rewriting it by
// hand is how two things that have to agree stop agreeing. So every rule about what a generated
// routine does reaches a routine when that routine is WRITTEN, and a library accumulates files
// written under rules that no longer match each other. A routine saved before typed text became a
// sequence of submissions still carries on past text that never arrived.
//
// Which was invisible, because nothing on disk said which rules wrote a file. It cannot be
// inferred either: the shapes in a routine are a function of the demonstration as much as of the
// generator, which is exactly how a guarded drag came to read as `legacy` before `draggedTerms`
// existed — a reader that does not know a shape reports a false answer confidently. So it is
// STATED, by the only thing that knows it, at the moment it is true.
//
// # Why it is not the build
//
// Because an older build does not establish that a routine needs anything. Most builds generated
// what today's would; a build id answers "when" and the question is "under which rules". See
// [[ADR-262-a-routine-states-which-generation-wrote-it]].
//
// # Why 1, with no history behind it
//
// Because a generation is a claim about a file, and nobody stamped the files written before this.
// Numbering past rule changes 1..5 would place routines in a history they never stated — the
// inference again, with a version number on it. Generation 1 is what this generator does; the
// generations before it are unstated and read as unstated.
//
// The reader, the words for each generation, and the rule that this is the one authority for both
// live in `internal/orchestrator`, which may not be imported here —
// `TestTheGenerationASaveWritesIsTheGenerationThisReads` holds the two together.
const Generation = 1

// DefaultTolerance is the per-channel color slack used when a click is matched
// by image and the step didn't specify its own. Paired with screen.MatchThreshold
// (a fraction of pixels, not all), this tolerates anti-aliasing and overlays.
const DefaultTolerance = 24

// DefaultFindTimeoutMs is how long an anchored click's GATE waits to confirm before
// it clicks the recorded coordinate anyway. Short on purpose: with every click auto-
// anchored, the gate is a quick confirmation, not a load-wait — and it must never hang
// the route when a signal can't match (a toggled button, a tooltip in the crop). The
// click is correct regardless (it's the recorded/window-relative point), so failing
// fast and clicking beats waiting. Use an explicit "wait for this screen" step for a
// genuine load (that's DefaultWaitTimeoutMs).
const DefaultFindTimeoutMs = 1500

// DefaultWaitTimeoutMs is how long a "wait for this screen" step polls for the
// captured screen before giving up — longer than a click find, since it's
// explicitly there to sit through a load.
const DefaultWaitTimeoutMs = 60000

// DefaultEndWaitMs is a settle wait appended after a route's last action, so the final
// input (especially a click) is delivered and the screen settles before the run ends and
// focus returns. Without it the last step can fire and the process exit before the app has
// processed it — the "final click doesn't take" flake. Editable/removable in `marco edit`.
const DefaultEndWaitMs = 250

// TextSearchMargin (px) is the half-size of the box a DEMONSTRATED text anchor searches
// around its recorded click — generous enough that a label which reflowed within its
// panel is still inside it, tight enough to stay fast and avoid a stray match elsewhere.
const TextSearchMargin = 400

// DefaultTextTimeoutMs is how long a text (OCR) locator polls. It's short: a text
// anchor is for a target that MOVED but is already on screen (a reflowing UI), and
// each poll re-captures + re-OCRs (expensive), so we don't sit on it for long. When
// a text anchor backs an image gate, the gate already absorbed the loading wait.
const DefaultTextTimeoutMs = 3000

// Route renders a named macro as Marco source. name is the human command name
// (e.g. "open chest"); it becomes the route's actor (PascalCase) and the log
// label. app, when non-empty, is the application the route was recorded in — the
// route brings it to the foreground first, so the route is context-aware (e.g.
// "login to facebook" focuses Chrome before clicking). The generated route runs
// on the OS act (`use os.`).
//
// A click that carries a captured Template becomes a robust image-find ("locate
// this image and click it; fall back to the recorded coordinate if not found").
// Each such template is returned in assets, keyed by the absolute path codegen
// embedded in the route (under assetDir) — the caller writes those files beside
// the route. assets is nil when no click was image-matched.
func Route(name, app string, steps []macroir.Step, assetDir string, declaredArgs ...string) (src string, assets map[string][]byte, err error) {
	return RouteAs(Recorded{}, name, app, steps, assetDir, declaredArgs...)
}

// Recorded is what a generation knows about the RECORDING behind it, beyond the steps.
//
// A struct rather than another parameter because there is going to be more than one such fact
// eventually, and because the thing it carries is of a different kind from the rest: `name`, `app`
// and `steps` are what the route will DO, and this is how what it will do came to be known.
type Recorded struct {
	// Mode is what the recording was told to capture, as the saved route will state it.
	//
	// Empty states nothing — which is what every caller before Studio's coordinate mode
	// did, and what every route already in a library says for ever. Absent is not a mode;
	// see orchestrator.Unstated.
	Mode string
}

// RouteAs is Route, told what is known about the recording behind it.
//
// Two entrances rather than one changed signature, because `Route` has a dozen callers that know
// nothing about a mode and should go on saying nothing — a caller forced to pass a value it does
// not have would pass a guess, and a guess written into a saved routine is a claim about how
// somebody's recording was made.
func RouteAs(rec Recorded, name, app string, steps []macroir.Step, assetDir string, declaredArgs ...string) (src string, assets map[string][]byte, err error) {
	actor := pascal(name)
	if actor == "" {
		return "", nil, fmt.Errorf("route name %q has no usable identifier characters", name)
	}

	declared := map[string]bool{}
	for _, a := range declaredArgs {
		declared[strings.ToLower(a)] = true
	}
	g := &gen{pts: &points{}, assetDir: assetDir, base: fileBase(name), assets: map[string][]byte{}, declared: declared}
	// ── TWO SHAPES, AND WHICH ONE IS DECIDED HERE ──
	//
	// A route with a press that asked to be checked against its window is a CHAIN, so a
	// refusal stops it. Every other route is the one block it always was. Deciding it
	// here, before anything is written, is what keeps the second promise: a route saved
	// before any of this existed is generated exactly as it was.
	chained := guardsAnything(steps)
	if why := Unpropagatable(steps); why != "" {
		// ── REFUSED RATHER THAN GENERATED HALF-GUARDED ──
		//
		// A routine whose presses are checked has to stop when one of them does not
		// happen, and this step cannot be stopped at: whatever follows it would run
		// regardless, which is the guard's promise broken by the one step nobody
		// could wrap. Writing the file anyway would be writing a routine that reads
		// as checked and is not.
		// AND IT NAMES WHAT CANNOT BE WRAPPED AND NOT WHAT TO DO INSTEAD. It used
		// to end "record it again using ordinary left-clicks and the keyboard",
		// which named the wrong thing even when a right-click was on the list —
		// and what remains unwrappable is a press found by matching a picture or
		// by reading words, and a repeated block. None of those is a hand somebody
		// chose to use.
		return "", nil, fmt.Errorf("this recording contains %s, and a routine "+
			"whose presses are checked against the window they were made in "+
			"has to stop when one of them does not happen — which this "+
			"version cannot make %s do", why, why)
	}
	var body, tail string
	var links []link
	if chained {
		// THE PREPARATION IS PART OF THE CHAIN. `Activate` is what binds the window
		// each press is then checked against, so a routine whose application never
		// came forward has nothing to press in and must not try. It becomes the
		// first link rather than a line above the body.
		prepared := steps
		if app != "" && !(len(steps) > 0 && steps[0].Kind == macroir.StepActivate) {
			prepared = append([]macroir.Step{{Kind: macroir.StepActivate, Text: app}},
				steps...)
		}
		links, tail, err = chainOf(textBySegment(keyByKey(prepared), declared), g)
	} else {
		body, err = emit(steps, g, 1)
	}
	if err != nil {
		return "", nil, err
	}
	// The trailing settle wait lives at the end of whichever shape this is; see below.
	if n := len(steps); n > 0 && steps[n-1].Kind != macroir.StepWait {
		tail += fmt.Sprintf("do OS's Sleep with %d.\n", DefaultEndWaitMs)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Auto-generated by `marco learn %q`. Edit freely.\n", name)
	if app != "" {
		fmt.Fprintf(&b, "// Context: %s (brought to the foreground before running).\n", app)
	}
	// ── AND WHAT THE RECORDING WAS TOLD TO CAPTURE, WHEN SOMEBODY SAID ──
	//
	// A comment, not a word of the language. The mode is not something the route DOES —
	// nothing here reads it to decide anything, and a Marco program that named a recorder
	// setting would be teaching its reader to expect backstage in the play. It is written
	// down because a person looking at a routine a month later is owed the answer to "how
	// was this made", and because `marco do` puts it in the run record, which is where
	// somebody reporting a problem with it will look.
	//
	// It states how the recording was MADE, which is history and which no later edit can
	// change. So a line that survives a hand-edited route may describe a file that has
	// moved on — the same way the `marco learn` line above still names whatever it was
	// called the day it was generated. Nothing infers it: a route with no line says
	// nothing, and that is read back as nothing rather than as a default.
	if rec.Mode != "" {
		fmt.Fprintf(&b, "// Recorded as: %s.\n", rec.Mode)
	}
	// ── AND WHICH GENERATION OF THESE RULES WROTE IT ──
	//
	// UNCONDITIONALLY, which is the difference between this and every line above it.
	// `Context` depends on the route and `Recorded as` depends on what the caller knew;
	// this depends on nothing, because whatever called it, THIS generator wrote the file.
	// A condition here would be a file written by this generator that does not say so.
	//
	// AND IT IS PROVENANCE AND NOT A VERDICT. It says which rules wrote the source. It does
	// not say that source is guarded, or unmodified, or that anything else about it is still
	// true: a routine of the current generation can carry legacy behaviour on purpose, since
	// a demonstration with no measured press generates exactly the unchecked route it always
	// did. What the presses promise is `orchestrator.ContractOf`'s question and stays a
	// separate one.
	fmt.Fprintf(&b, "// Generation: %d.\n", Generation)
	b.WriteString("use os.\n")
	// A text anchor resolves through the OCR resolver plugin (the Text act).
	if g.textN > 0 {
		b.WriteString("use text.\n")
	}
	// A vision anchor resolves through the semantic detector plugin (the Vision act).
	if g.visionN > 0 {
		b.WriteString("use vision.\n")
	}
	b.WriteString("\n")
	// An anchored click needs an Anchor set type — one shape carries every resolver:
	// OS's Find reads Image/Color/X/Y, Text's Find reads Text; each ignores the rest.
	if g.anchorN > 0 {
		// The comment a person reads describes WHAT an Anchor is, not how the host scores
		// one. How a target gets found is backstage; a route that explained the resolver
		// would be teaching its reader to expect cognition in Marco.
		// See spec/Core.md#governance.
		b.WriteString("// Where something is on screen, described several ways so it can still be found\n")
		b.WriteString("// after a window moves: a picture of it, the colour under the click, where it was\n")
		b.WriteString("// last time (X,Y, and RelX,RelY relative to the window), and the words on it.\n")
		b.WriteString("// Marco asks the host to find it; how the host decides is the host's business.\n")
		b.WriteString("the Anchor is a set.\n")
		b.WriteString("this's Image is a text.\n")
		b.WriteString("this's Color is a text.    // signal: the pixel colour at X,Y\n")
		b.WriteString("this's Window is a text.   // context: the window title to act within\n")
		b.WriteString("this's Text is a text.     // locator: on-screen text to find by OCR\n")
		b.WriteString("this's Label is a text.    // locator: the control CLASS for the Vision detector\n")
		b.WriteString("this's X is a number.\n")
		b.WriteString("this's Y is a number.\n")
		b.WriteString("this's RelX is a number.   // window-relative click (resolved after Activate)\n")
		b.WriteString("this's RelY is a number.\n")
		b.WriteString("this's RelTo is a text.    // which rectangle RelX,RelY are measured from\n")
		b.WriteString("this's WinW is a number.   // how big that window was, on the outside\n")
		b.WriteString("this's WinH is a number.\n")
		b.WriteString("this's Dpi is a number.    // the scale it was drawn at\n")
		b.WriteString("this's Process is a text.  // the program that owned it\n")
		b.WriteString("this's X1 is a number.     // search region for the Text / Vision locators\n")
		b.WriteString("this's Y1 is a number.\n")
		b.WriteString("this's X2 is a number.\n")
		b.WriteString("this's Y2 is a number.\n")
		b.WriteString("this's Tolerance is a number.\n")
		b.WriteString("this's Timeout is a number.  // ms to wait for it to appear\n\n")
	}
	fmt.Fprintf(&b, "the %s is an actor.\n\n", actor)
	if chained {
		b.WriteString(chainNote)
		writeChain(&b, actor, links, tail)
	} else {
		b.WriteString("this can Run.\n")
		b.WriteString("this's Run does...\n")

		// Point locals are declared at the top of the action body, then the steps.
		for _, decl := range g.pts.decls {
			fmt.Fprintf(&b, "%s%s\n", indent(1), decl)
		}
		// Context-aware: focus the app the route was recorded in (no-op if already
		// in front). A failure floats to the route's contract and is handled at the
		// call site. Skipped when the steps already lead with their own Activate
		// (the recorder detected the app switches), to avoid focusing twice.
		if app != "" && !(len(steps) > 0 && steps[0].Kind == macroir.StepActivate) {
			fmt.Fprintf(&b, "%sdo OS's Activate with \"%s\".\n", indent(1),
				escape(app))
		}
		b.WriteString(body)
		// Trailing settle wait so the final action registers before the run ends
		// (see DefaultEndWaitMs). Skipped for an empty body or one that already
		// ends on a wait.
		b.WriteString(shift(tail, 1))
		fmt.Fprintf(&b, "%sthis is ok!\n\n", indent(1))
	}

	fmt.Fprintf(&b, "the App is a script.\n\n")
	fmt.Fprintf(&b, "do %s's Run...\n", actor)
	fmt.Fprintf(&b, "%swhen ok?\n", indent(1))
	fmt.Fprintf(&b, "%slog \"%s: done\".\n", indent(2), escape(name))
	fmt.Fprintf(&b, "%sor?\n", indent(1))
	fmt.Fprintf(&b, "%slog that's error.\n", indent(2))
	if chained {
		// AND WHAT THAT MEANT, SAID PLAINLY. The sentence above is the host's, about
		// one press; this one is about the routine, which stopped where that press
		// was refused and did nothing after it. The run's own answer is not a log
		// line — see `oshost.Host.Refused` and `runRoute` — but a person reading the
		// output should not have to work it out from a refusal and a silence.
		fmt.Fprintf(&b, "%slog \"%s: stopped there — nothing after it was done\".\n",
			indent(2), escape(name))
	}
	if len(g.assets) == 0 {
		g.assets = nil
	}
	return b.String(), g.assets, nil
}

// gen carries the mutable state threaded through emit: point locals, the asset
// directory + name base for template files, the accumulated assets, and the
// running anchor counter.
type gen struct {
	pts      *points
	assetDir string
	base     string
	assets   map[string][]byte
	anchorN  int
	textN    int             // anchors carrying a Text locator → emit `use text.`
	visionN  int             // anchors carrying a Vision locator → emit `use vision.`
	dragN    int             // Drag steps → numbered `the dragN is a Drag …` locals
	declared map[string]bool // declared arg names: {{name}} kept (filled at run), not a secret
}

// points allocates and declares numbered Point locals (p1, p2, …).
type points struct {
	n     int
	decls []string
}

func (p *points) add(x, y int) string {
	p.n++
	name := fmt.Sprintf("p%d", p.n)
	p.decls = append(p.decls, fmt.Sprintf("the %s is a Point with X %d, Y %d.", name, x, y))
	return name
}

// addRel declares a window-relative Point: X,Y are the absolute fallback and
// RelX,RelY the offset inside the active window. The OS host resolves RelX,RelY
// against the foreground (just-activated) window so the click lands at the same
// spot in the window on any monitor or position, falling back to X,Y if it's gone.
// `in` is what that window was like when the press was made — the basis the offset is measured
// from, the window's outer size, its DPI and the program that owned it. Written out when the
// recording measured it, because those terms ARE the request for the guarded contract: a point
// carrying them asks the host to bind that window and check it again immediately before
// pressing, and a point without them keeps the behaviour it was saved with. A route saved before
// any of this existed, or one recorded where the window would not describe itself, therefore
// reads exactly as it did and runs exactly as it did.
// The offset itself is one of the measurements, and is written only when it was taken: a press
// that declared the window and could not be measured inside it saves its basis with no `RelX`,
// which is what tells a replay the measurement is MISSING rather than zero.
func (p *points) addRel(x, y int, in macroir.Step) string {
	p.n++
	name := fmt.Sprintf("p%d", p.n)
	rel := ""
	if in.WinRel {
		rel = fmt.Sprintf(", RelX %d, RelY %d", in.RelX, in.RelY)
	}
	p.decls = append(p.decls, fmt.Sprintf(
		"the %s is a Point with X %d, Y %d%s%s%s.", name, x, y, buttonTerm(in),
		rel, measured(in)))
	return name
}

// buttonTerm is the button a press uses, as a Point field, and "" for a left-handed one.
//
// WRITTEN ONLY WHEN IT IS NOT LEFT, which is not tidiness. Every Point ever generated is a
// left-click and says nothing about a button; writing `Button "left"` onto them would change the
// text of every guarded routine in existence for no change in meaning, and each of those files is
// something somebody can read and diff. Silence already means left, in the host and in os.marco.
//
// It goes beside the COORDINATES rather than among the window terms because it is not a
// measurement: nothing about it is compared with the desktop before pressing, and the contract
// reader's list of terms a guarded press must state is unchanged by it. Same place `Drag` has
// carried its button since it existed.
func buttonTerm(s macroir.Step) string {
	if s.Button == "" || s.Button == "left" {
		return ""
	}
	return fmt.Sprintf(", Button %q", s.Button)
}

// measured is the window a press was made in, as Point fields, or "" for a point that is about no
// window at all.
//
// The BASIS is the contract and the rest are measurements. The size, the DPI and the program are
// each written only when known — a DPI nobody could read must not be written as 96, and absent is
// a different claim from a default — and none of them is written without the basis, because
// checking a window's size while having no idea which rectangle the offset belongs to is checking
// the easy half of the question.
//
// What the basis does NOT depend on is whether the measurements arrived. A press that declared
// the window and could not be measured in it writes `RelTo` alone, and that is the third saved
// contract: not legacy, which is a point about no window and is pressed as it always was, and not
// guarded-and-runnable either, but a point that asked to be checked and cannot be — refused at
// replay, naming each term it is missing.
func measured(s macroir.Step) string {
	if s.RelTo == "" {
		return ""
	}
	out := fmt.Sprintf(", RelTo \"%s\"", escape(s.RelTo))
	if s.WinW > 0 && s.WinH > 0 {
		out += fmt.Sprintf(", WinW %d, WinH %d", s.WinW, s.WinH)
	}
	if s.Dpi > 0 {
		out += fmt.Sprintf(", Dpi %d", s.Dpi)
	}
	if s.Process != "" {
		out += fmt.Sprintf(", Process \"%s\"", escape(s.Process))
	}
	return out
}

// dragSet declares the Drag for a drag step and returns its name.
//
// The window terms go on when the step carries the BASIS, which is the same rule `clickPoint`
// follows and the same reason: a drag that declared the window it was made in and could not be
// measured in it must come out as something replay REFUSES, not as a bare pair of coordinates
// indistinguishable from a drag saved before any of this existed.
//
// One `measured(s)` for both ends, because there is one window. Two sets of window terms on one
// gesture would be inviting it to be checked against two windows, which is not a gesture; what
// makes one set honest is that `simplify` has already shown both edges agreed about it.
// It returns the declaration rather than placing it, because the two callers put it in different
// places and both are right where they are. An ordinary block writes it immediately before the
// `do`, which is where every drag ever generated has had it; a CHAIN puts it in the link's own
// locals, so the set is declared inside the capability that performs the drag and a gesture whose
// guard refused leaves nothing declared and nothing after it run.
func (g *gen) dragSet(s macroir.Step) (name, decl string) {
	btn := s.Button
	if btn == "" {
		btn = "left"
	}
	g.dragN++
	name = fmt.Sprintf("drag%d", g.dragN)
	rel := ""
	if s.WinRel {
		rel = fmt.Sprintf(", FromRelX %d, FromRelY %d, ToRelX %d, ToRelY %d",
			s.RelX, s.RelY, s.ToRelX, s.ToRelY)
	}
	return name, fmt.Sprintf("the %s is a Drag with FromX %d, FromY %d, ToX %d, "+
		"ToY %d, Button %q%s%s.", name, s.X, s.Y, s.X2, s.Y2, btn, rel,
		measured(s))
}

// clickPoint declares the Point for a click step — window-relative when the
// recorder captured an offset, else a plain absolute point.
// A point goes down the window-relative form when it is ABOUT a window, which is the basis and
// not the offset. Branching on the offset was the gap: a press that declared the window and could
// not be measured in it came out as a bare absolute point — indistinguishable from a route saved
// before any of this existed, and pressed with the same unchecked trust.
func (p *points) clickPoint(s macroir.Step) string {
	if aboutAPlaceInAWindow(s) {
		return p.addRel(s.X, s.Y, s)
	}
	return p.add(s.X, s.Y)
}

// aboutAPlaceInAWindow reports whether a press knows where in a window it goes — either because
// it states the window it was measured in, or because it carries an offset into one.
//
// ONE PREDICATE AND TWO CALLERS, which is the point of it. `clickPoint` asks it to choose between
// the window-relative Point and a bare absolute one; `emit` asks it to choose between a Point and
// the button alone, which presses wherever the cursor is. Those are the same question — does this
// press know a place — and they were the same question written twice, in opposite directions, one
// of them by hand for a button. Written twice they can disagree: a non-left press that knew its
// offset would have gone to the cursor while the left press beside it went to the offset, for no
// reason either file states.
func aboutAPlaceInAWindow(s macroir.Step) bool {
	return s.WinRel || s.RelTo != ""
}

// emit renders steps at the given indent depth.
func emit(steps []macroir.Step, g *gen, depth int) (string, error) {
	var b strings.Builder
	pad := indent(depth)
	for _, s := range steps {
		switch s.Kind {
		case macroir.StepActivate:
			fmt.Fprintf(&b, "%sdo OS's Activate with \"%s\".\n", pad, escape(s.Text))
		case macroir.StepClick:
			switch {
			case len(s.Template) > 0 || s.AnchorText != "":
				g.emitAnchoredClick(&b, s, depth) // image/colour gate and/or text locator
			case s.Button != "" && s.Button != "left" &&
				!aboutAPlaceInAWindow(s):
				// A PRESS THAT KNOWS NO PLACE KEEPS THE FORM IT HAS ALWAYS
				// HAD: the button alone, pressed where the cursor is. Every
				// legacy route says this and means it, and the contract it
				// was saved under is that a click is attempted and the run
				// carries on. A press that knows a place falls through to
				// the Point below, which carries the button WITH it —
				// including one that knows an offset and no window, which
				// is what the left button beside it has always done.
				fmt.Fprintf(&b, "%sdo OS's Click with %q.\n", pad, s.Button)
			default:
				name := g.pts.clickPoint(s)
				fmt.Fprintf(&b, "%sdo OS's Click with %s.\n", pad, name)
			}
		case macroir.StepMove:
			name := g.pts.add(s.X, s.Y)
			fmt.Fprintf(&b, "%sdo OS's Move with %s.\n", pad, name)
		case macroir.StepWait:
			fmt.Fprintf(&b, "%sdo OS's Sleep with %d.\n", pad, s.Ms)
		case macroir.StepKey:
			count := s.Count
			if count <= 1 {
				fmt.Fprintf(&b, "%sdo OS's Key with %q.\n", pad, s.Key)
			} else {
				fmt.Fprintf(&b, "%srepeat %d times...\n", pad, count)
				fmt.Fprintf(&b, "%sdo OS's Key with %q.\n", indent(depth+1), s.Key)
			}
		case macroir.StepKeyDown:
			fmt.Fprintf(&b, "%sdo OS's KeyDown with %q.\n", pad, s.Key)
		case macroir.StepKeyUp:
			fmt.Fprintf(&b, "%sdo OS's KeyUp with %q.\n", pad, s.Key)
		case macroir.StepType:
			// {{name}} placeholders: a DECLARED arg (or numeric {{N}}) stays in the
			// text to be filled at run time; any other {{name}} becomes a Secret lookup
			// so passwords are never written into the route.
			for _, seg := range splitSecrets(s.Text, g.declared) {
				if seg.secret || seg.text != "" {
					fmt.Fprintf(&b, "%sdo %s.\n", pad, g.typeSentence(seg))
				}
			}
		case macroir.StepDrag:
			// A real press-hold-release drag (OS's Drag primitive): press at the start, glide to
			// the end, release. The Drag set type comes from os.marco.
			name, decl := g.dragSet(s)
			fmt.Fprintf(&b, "%s%s\n", pad, decl)
			fmt.Fprintf(&b, "%sdo OS's Drag with %s.\n", pad, name)
		case macroir.StepLoop:
			fmt.Fprintf(&b, "%srepeat %d times...\n", pad, s.Count)
			inner, err := emit(s.Steps, g, depth+1)
			if err != nil {
				return "", err
			}
			b.WriteString(inner)
		case macroir.StepFind:
			g.emitWaitImage(&b, s, depth)
		default:
			return "", fmt.Errorf("codegen: unknown step kind %q", s.Kind)
		}
	}
	return b.String(), nil
}

// emitAnchoredClick renders a click resolved by one or more anchors. There are two
// kinds, and they compose:
//
//   - GATE (Image and/or Colour): a wait gate. Find blocks until the captured
//     screen/menu appears (its pixels don't move on a static page), then we click the
//     recorded COORDINATE — exactly where you clicked, regardless of how the template
//     was cropped (capture the whole menu for context without dragging the click to
//     its centre). If it never appears we click the coordinate anyway (best effort).
//
//   - LOCATOR (Text, then Vision): used when the gate fails (or alone), because the
//     target MOVED — a reflowing UI. Text's Find returns where the WORD is now; Vision's
//     Locate returns where the nearest control of the same CLASS ("button"/"icon") is now
//     — the resolver for a control with no text and no clean edge. Each clicks THERE
//     (`that`). The locators run in order of precision — exact text, then class — and the
//     chain falls through to the recorded coordinate if none resolves (or no host wired).
//
// Anchors are declared up front (siblings, so locals stay top-level, not nested in an
// arm). A left click clicks the Point directly; a right/middle click moves there first.
func (g *gen) emitAnchoredClick(b *strings.Builder, s macroir.Step, depth int) {
	g.anchorN++
	n := g.anchorN
	pad := indent(depth)
	point := g.pts.clickPoint(s) // the recorded click coordinate (window-relative)
	hasGate := len(s.Template) > 0
	hasText := s.AnchorText != ""
	hasVision := s.AnchorVision != ""

	// clickRecorded clicks the recorded Point (the gate model / final fallback).
	clickRecorded := func(d int) {
		if s.Button == "" || s.Button == "left" {
			fmt.Fprintf(b, "%sdo OS's Click with %s.\n", indent(d), point)
			return
		}
		fmt.Fprintf(b, "%sdo OS's Move with %s.\n", indent(d), point)
		fmt.Fprintf(b, "%sdo OS's Click with %q.\n", indent(d), s.Button)
	}
	// clickFound clicks where the located text is (`that`, the Point Text's Find returns).
	clickFound := func(d int) {
		if s.Button == "" || s.Button == "left" {
			fmt.Fprintf(b, "%sdo OS's Click with that.\n", indent(d))
			return
		}
		fmt.Fprintf(b, "%sdo OS's Move with that.\n", indent(d))
		fmt.Fprintf(b, "%sdo OS's Click with %q.\n", indent(d), s.Button)
	}

	// Declare the gate anchor (Image + optional Colour) up front.
	if hasGate {
		fname := fmt.Sprintf("%s-anchor-%d.png", g.base, n)
		abs := filepath.Join(g.assetDir, fname)
		g.assets[abs] = s.Template
		tol := s.Tolerance
		if tol <= 0 {
			tol = DefaultTolerance
		}
		// The scorer needs the recorded click point on the anchor: X,Y (absolute) plus
		// RelX,RelY (window-relative) so colour/position probe the RIGHT pixel after an
		// Activate. Colour, when captured, is an extra signal at that point.
		extra := fmt.Sprintf(", X %d, Y %d", s.X, s.Y)
		if s.WinRel {
			// AND WHAT THE WINDOW WAS LIKE, by the same names a Point uses, so the
			// one resolver can read either. See oshost.resolve.
			extra += fmt.Sprintf(", RelX %d, RelY %d%s", s.RelX, s.RelY, measured(s))
		}
		if s.Color != "" {
			extra = fmt.Sprintf(", Color \"%s\"", escape(s.Color)) + extra
		}
		if s.Window != "" {
			extra = fmt.Sprintf(", Window \"%s\"", escape(s.Window)) + extra
		}
		fmt.Fprintf(b, "%sthe a%d is an Anchor with Image \"%s\"%s, Tolerance %d, Timeout %d.\n", pad, n, escape(abs), extra, tol, DefaultFindTimeoutMs)
	}
	// A DEMONSTRATED anchor (hasGate → it carries the recorded click coordinate) restricts
	// the locator search to a box around that point: the control moved within its panel,
	// not across the screen, so a region is faster than a whole-screen scan and avoids a
	// stray match elsewhere on a busy desktop. A coordinate-less narrated anchor searches
	// the whole screen.
	region := ""
	if hasGate {
		x1, y1 := s.X-TextSearchMargin, s.Y-TextSearchMargin
		if x1 < 0 {
			x1 = 0
		}
		if y1 < 0 {
			y1 = 0
		}
		region = fmt.Sprintf(", X1 %d, Y1 %d, X2 %d, Y2 %d", x1, y1, s.X+TextSearchMargin, s.Y+TextSearchMargin)
	}

	// Declare the text locator (Text's Find: exact on-screen text).
	if hasText {
		g.textN++
		fmt.Fprintf(b, "%sthe t%d is an Anchor with Text \"%s\"%s, Timeout %d.\n", pad, n, escape(s.AnchorText), region, DefaultTextTimeoutMs)
	}
	// Declare the vision locator (Vision's Locate: the control CLASS, nearest the recorded
	// point) — the resolver for a control with no text and no clean edge.
	if hasVision {
		g.visionN++
		hint := ""
		if hasGate {
			hint = fmt.Sprintf(", X %d, Y %d", s.X, s.Y)
		}
		fmt.Fprintf(b, "%sthe v%d is an Anchor with Label \"%s\"%s%s, Timeout %d.\n", pad, n, escape(s.AnchorVision), hint, region, DefaultTextTimeoutMs)
	}

	// The locators, in order of preference: exact TEXT, then semantic CLASS, then (in
	// emitChain's base case) the recorded coordinate. Each resolves to a point and clicks
	// it; on failure it falls through to the next.
	var calls []string
	if hasText {
		calls = append(calls, fmt.Sprintf("Text's Find with t%d", n))
	}
	if hasVision {
		calls = append(calls, fmt.Sprintf("Vision's Locate with v%d", n))
	}
	var emitChain func(i, d int)
	emitChain = func(i, d int) {
		if i >= len(calls) {
			clickRecorded(d) // exhausted the locators — best-effort recorded coordinate
			return
		}
		fmt.Fprintf(b, "%sdo %s...\n", indent(d), calls[i])
		fmt.Fprintf(b, "%swhen ok?\n", indent(d+1))
		clickFound(d + 2)
		fmt.Fprintf(b, "%sor?\n", indent(d+1))
		emitChain(i+1, d+2)
	}

	if hasGate {
		fmt.Fprintf(b, "%sdo OS's Find with a%d...\n", pad, n) // scored resolve
		fmt.Fprintf(b, "%swhen ok?\n", indent(depth+1))
		clickFound(depth + 2) // confident → click the scored location (follows a moved target)
		fmt.Fprintf(b, "%sor?\n", indent(depth+1))
		emitChain(0, depth+2) // unsure — try text, then vision, then the recorded coordinate
		return
	}
	// No gate (a narrated text/vision-only anchor): run the locator chain directly.
	emitChain(0, depth)
}

// emitWaitImage renders a "wait for this screen" step: poll for the captured image
// for up to DefaultWaitTimeoutMs, then continue. It does not click — it's purely a
// barrier that sits through a load. A timeout logs a note but lets the route go on,
// so a slightly-different screen doesn't hard-fail the whole macro.
func (g *gen) emitWaitImage(b *strings.Builder, s macroir.Step, depth int) {
	g.anchorN++
	fname := fmt.Sprintf("%s-wait-%d.png", g.base, g.anchorN)
	abs := filepath.Join(g.assetDir, fname)
	g.assets[abs] = s.Template
	tol := s.Tolerance
	if tol <= 0 {
		tol = DefaultTolerance
	}
	anchor := fmt.Sprintf("w%d", g.anchorN)
	pad := indent(depth)
	fmt.Fprintf(b, "%sthe %s is an Anchor with Image \"%s\", Tolerance %d, Timeout %d.\n", pad, anchor, escape(abs), tol, DefaultWaitTimeoutMs)
	fmt.Fprintf(b, "%sdo OS's Find with %s...\n", pad, anchor)
	fmt.Fprintf(b, "%swhen ok?\n", indent(depth+1))
	fmt.Fprintf(b, "%slog \"screen ready\".\n", indent(depth+2))
	fmt.Fprintf(b, "%sor?\n", indent(depth+1))
	fmt.Fprintf(b, "%slog \"timed out waiting for screen\".\n", indent(depth+2))
}

// fileBase turns a route name into a filesystem-safe base for template files
// (lowercase, runs of non-alphanumerics collapsed to one '-').
func fileBase(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	if b.Len() == 0 {
		return "anchor"
	}
	return b.String()
}

func indent(depth int) string { return strings.Repeat("    ", depth) }

// placeholderRe matches a {{name}} placeholder. A NUMERIC name ({{1}}, {{2}}) is a
// positional run-time argument and is left in the typed text verbatim (the engine
// fills it when the route is run with `… with a, b`); a named one ({{password}}) is
// a secret looked up from the credential store.
var placeholderRe = regexp.MustCompile(`\{\{([A-Za-z0-9_-]+)\}\}`)

type segment struct {
	text      string
	secret    bool
	qualified bool // a declared secret arg → route-qualified secret name
}

// splitSecrets breaks a typed string into literal and secret segments. A numeric
// {{N}} or a declared PLAIN {{name}} stays in the literal (the run fills it); a
// declared SECRET-typed arg ({{password}}) becomes a route-qualified secret; any
// other {{name}} is a global secret.
func splitSecrets(s string, declared map[string]bool) []segment {
	var out []segment
	last := 0
	for _, m := range placeholderRe.FindAllStringSubmatchIndex(s, -1) {
		name := s[m[2]:m[3]]
		isDeclared := declared[strings.ToLower(name)]
		if isNumeric(name) || (isDeclared && !isSecretArg(name)) {
			continue // a plain arg placeholder — keep {{…}} for run-time fill
		}
		if m[0] > last {
			out = append(out, segment{text: s[last:m[0]]})
		}
		out = append(out, segment{text: name, secret: true, qualified: isDeclared})
		last = m[1]
	}
	if last < len(s) {
		out = append(out, segment{text: s[last:]})
	}
	if len(out) == 0 {
		out = append(out, segment{text: s})
	}
	return out
}

// MeansASecret reports whether this argument name is one the generator resolves from the
// CREDENTIAL STORE rather than typing.
//
// EXPORTED BECAUSE A SURFACE THAT LETS SOMEBODY NAME AN ARGUMENT HAS TO KNOW. The step editor
// can turn the text a step types into a supplied value under a name of the person's choosing,
// and a name on this list does not produce supplied text at all — it produces
// `do OS's Secret …`, which reads their credential store and types whatever is in it. That is a
// surprise nobody asked for, so the editor refuses the name and says why.
//
// One list, here, because two would disagree the first time either changed and the disagreement
// would be a routine that reads a password where somebody expected their own words.
func MeansASecret(name string) bool { return isSecretArg(name) }

// isSecretArg reports whether a declared arg name should be handled as a secret
// (resolved from the credential store, never written into the route, masked).
func isSecretArg(name string) bool {
	switch strings.ToLower(name) {
	case "password", "pass", "passwd", "pwd", "pin", "secret", "token", "otp", "apikey", "api-key", "key":
		return true
	}
	return false
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// escape escapes a string for a Marco double-quoted literal.
func escape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// pascal turns a command name into a PascalCase identifier (letters/digits only,
// first char forced to a letter).
func pascal(name string) string {
	var b strings.Builder
	upNext := true
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			if upNext {
				b.WriteRune(upper(r))
			} else {
				b.WriteRune(r)
			}
			upNext = false
		case r >= '0' && r <= '9':
			if b.Len() > 0 {
				b.WriteRune(r)
			}
			upNext = false
		default:
			upNext = true
		}
	}
	return b.String()
}

func upper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 32
	}
	return r
}

// ── A GUARDED ROUTE IS A CHAIN, NOT A BLOCK ──
//
// A press that checks the window it was recorded in can REFUSE, and a refusal has to stop the
// routine: the next press of a sequence means nothing once the one before it did not happen, and
// a routine that pressed on regardless would be doing the thing the check exists to prevent one
// step later.
//
// Marco's way of saying that is the one `testdata/macro_ark_transfer` uses: a `do` whose two arms
// both END, with the rest of the work inside the one that succeeded. Written as a single block
// that is one indent level per press, which at thirty presses is a route nobody can read — so
// each propagating action gets its own capability and calls the next only when its own action
// went through. `Run` is the first link, so `do <Actor>'s Run` still performs the routine.
//
// It is NOT the shape of a route with no guarded press. Those are generated exactly as they
// always were, because they were saved under a contract that says a click is attempted and the
// run carries on.

// chainNote is what a person reading the generated file is told about its shape.
const chainNote = "" +
	"// Each press checks the window it was recorded in before it happens, and a press that\n" +
	"// cannot be checked stops the routine — so the steps are written one at a time, each\n" +
	"// calling the next only when its own press went through. Nothing after a refusal runs.\n"

// link is one capability of a guarded route's chain: what runs before its propagating action,
// that action, and what belongs with it once it has gone through.
type link struct {
	decls  []string
	before string // relative-indented, runs before the action
	guard  string // the sentence after `do `, e.g. `OS's Click with p1`
	follow string // relative-indented, belongs with the action once it succeeded
}

// guardsAnything reports whether any step asked to be checked against the window it was recorded
// in. It is the only thing that turns a route into a chain.
// It asks whether a press ASKED to be checked, and not whether this can wrap it. Those came out
// as one question first, and that was a hole with the shape of every other hole here: a recording
// whose only press was one this version could not wrap had no step to find, so it was not a
// guarded route, so the refusal below never fired and it was generated as an ordinary block — a
// press that declared the window it was made in, pressed without any of it being looked at.
// Whether a demonstration asked for the contract and whether this version can honour it are two
// questions, and conflating them answers "no" to the first whenever the second is "no".
//
// The press that found it was a RIGHT-CLICK, which is now wrapped like any other; the hole it
// found is not about any particular step, and the list of steps this cannot wrap has only ever
// got shorter.
// GuardsAnything says whether any step names a window it is checked against, which is what makes
// the generated routine a CHECKED one. Exported so the editor can ask the generator's own question
// — whether a click at a screen point may be added — instead of keeping a second copy of it.
func GuardsAnything(steps []macroir.Step) bool { return guardsAnything(steps) }

func guardsAnything(steps []macroir.Step) bool {
	for _, s := range steps {
		if s.RelTo != "" {
			return true
		}
		if len(s.Steps) > 0 && guardsAnything(s.Steps) {
			return true
		}
	}
	return false
}

// guardedStep is a press carrying the window it was recorded in, performed by ONE `do` sentence
// that a chain can propagate through.
//
// An ANCHORED press is excluded: its emitted form is a ladder of locators with a click in each
// arm rather than one sentence, so there is nothing here to wrap. Anchors are off unless computer
// vision is explicitly turned on, so no recorder-first recording produces one — recorded as a gap
// in [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] rather than silently covered.
func guardedStep(s macroir.Step) bool {
	if s.RelTo == "" || cannotPropagate(s) != "" {
		return false
	}
	return s.Kind == macroir.StepClick || s.Kind == macroir.StepMove ||
		s.Kind == macroir.StepDrag
}

// cannotPropagate names what a step is, when its failure cannot be made to stop a guarded
// routine, and is "" for every step that can.
//
// The chain works by wrapping ONE `do` sentence in arms that both end. A step emitted as
// something else cannot be stopped at, and in a guarded routine that is not a smaller promise —
// it is the promise broken, because the action after it runs regardless.
//
// Each of these could be supported and none is supported now:
//
//   - an ANCHORED press is a ladder of locators with a click in each arm.
//   - a REPEATED block runs its body again, which is the one thing a stop cannot survive.
//
// A DRAG USED TO BE ON THAT LIST, and the entry read "a drag resolves no offset, so it carries no
// window and is checked against nothing". That was a true statement about the recorder rather than
// about the gesture: a drag is ALREADY one `do` sentence, which is the property this needs, and
// what it lacked was a measurement. It has one now — both edges, shown to agree about one window
// — so it propagates like a click.
//
// What stays refused is a drag that ASKED to be checked and cannot be, and it is two sentences
// rather than one because they are two different recordings and the person reading the refusal is
// the one who has to tell them apart. A missing measurement is something the machine would not
// report; `simplify` has already refused the recording outright for anything the PERSON did (a
// gesture across two windows, a window that moved, a drag out of the window), with a sentence
// naming it.
//
// Unreachable from a recorder-first recording today except the first one: anchors are off unless
// computer vision is turned on, and loop folding is off for a recording with a guarded press.
func cannotPropagate(s macroir.Step) string {
	switch {
	case s.Kind == macroir.StepClick && len(s.Template) > 0:
		return "a press found by matching a picture"
	case s.Kind == macroir.StepClick && s.AnchorText != "":
		return "a press found by reading words on the screen"
	case s.Kind == macroir.StepDrag && s.RelTo != "" && !s.WinRel:
		// THE BASIS WITHOUT THE OFFSETS. `simplify` writes exactly this when a
		// drag's two edges agreed about the window and a measurement would not
		// come, and it is the one case that must not be mistaken for an ordinary
		// drag: an ordinary drag runs at its recorded coordinates, and this one
		// asked not to.
		return "a drag whose window could not be measured"
	case s.Kind == macroir.StepDrag && s.RelTo != "" &&
		(s.WinW == 0 || s.WinH == 0 || s.Dpi == 0 || s.Process == ""):
		return "a drag missing part of the window it was made in"
	case s.Kind == macroir.StepLoop:
		return "a repeated block"
	}
	return ""
}

// Unpropagatable names the first thing in a demonstration that cannot be saved as part of a
// routine whose presses are checked, or "" when all of it can.
//
// Exported because the window that saves a recording has to be able to say so BEFORE somebody
// presses Save, and a generator error arriving at a button press is a worse way to learn it.
func Unpropagatable(steps []macroir.Step) string {
	if !guardsAnything(steps) {
		return ""
	}
	for _, s := range steps {
		if why := cannotPropagate(s); why != "" {
			return why
		}
	}
	return ""
}

// propagating is the `do` sentence a step must be stopped by, or ok=false when it is an ordinary
// step that simply runs.
//
// `Activate` is in because it is the PREPARATION: it is what binds the window every press is then
// checked against, so a routine whose application never came forward has nothing to press in and
// must not try.
func (g *gen) propagating(s macroir.Step) (guard, follow string, ok bool) {
	switch {
	case s.Kind == macroir.StepActivate:
		return fmt.Sprintf("OS's Activate with %q", s.Text), "", true
	case s.Kind == macroir.StepKey && s.Count <= 1:
		// A KEYSTROKE IS A LINK TOO, and not because anything checks it.
		//
		// Nothing measures a window for a key and nothing here compares one. It is a
		// link because it can fail to be DELIVERED, and what follows it then acts on a
		// screen the key never changed: an Escape that did not arrive leaves the menu
		// open, the press after it lands on whatever is now under that coordinate, and
		// the window guard cannot see the difference — the window is the same window.
		// A routine that went on from there reported that it performed.
		//
		// The count is at most one because `keyByKey` has already split a repeated key
		// into its presses; a single guard standing for three presses would be a claim
		// about three things from one answer.
		return fmt.Sprintf("OS's Key with %q", s.Key), "", true
	case s.Kind == macroir.StepKeyDown:
		// AND SO IS THE START OF A HOLD, for a reason of its own.
		//
		// A keystroke that did not arrive leaves the screen unchanged. A HOLD that did
		// not go down leaves the screen unchanged AND leaves everything after it acting
		// as though a key were held when it is not — `hold Q, click, release Q` is one
		// gesture, and the click is a different gesture entirely without the Q. The
		// window guard cannot see the difference: the window is the same window.
		return fmt.Sprintf("OS's KeyDown with %q", s.Key), "", true
	case s.Kind == macroir.StepType:
		// TYPED TEXT IS A LINK, for the keystroke's reason and one of its own.
		//
		// It can fail to be DELIVERED — `SendInput` is blocked by an elevated
		// foreground window, by UIPI, by a game with raw input — and what follows it
		// then acts on a screen the text never reached. The window guard cannot see the
		// difference, because the window is the same window.
		//
		// And its own reason: typing is the one input that can go in PART WAY. A field
		// holding `he` of `hello` is not the field the next step was written for, and
		// the step after a password that stopped half way is a Save pressed on a form
		// that will be rejected — or worse, accepted with the wrong value. See
		// `internal/oshost`'s typing.go for what the account says about it.
		//
		// One segment per step by the time this is reached: `textBySegment` has already
		// split a mixed text into its Types and Secrets, so there is one sentence here
		// to wrap. A step carrying several would be a link around the first of them.
		segs := splitSecrets(s.Text, g.declared)
		if len(segs) != 1 {
			return "", "", false
		}
		return g.typeSentence(segs[0]), "", true
	case s.Kind == macroir.StepKeyUp:
		// AND THE END OF ONE, which is the half that is easy to think does not matter.
		//
		// It does: a release that the machine would not take leaves everything after it
		// running with the key STILL DOWN, so the clicks that follow are modified
		// clicks and the typing that follows is different typing. The route-end sweep
		// may recover the key afterwards, and that is not the same as the routine
		// having been right to carry on.
		return fmt.Sprintf("OS's KeyUp with %q", s.Key), "", true
	case !guardedStep(s):
		return "", "", false
	case s.Kind == macroir.StepMove:
		return "OS's Move with " + g.pts.clickPoint(s), "", true
	case s.Kind == macroir.StepDrag:
		// A DRAG IS ONE SENTENCE, which is the whole reason it can be here and the
		// reason an anchored press cannot.
		//
		// Its declaration goes in `before` rather than beside the action, because that
		// is where a link's own locals belong: the set is declared inside the capability
		// that performs the drag, so a drag whose guard refused leaves nothing declared
		// and nothing after it runs. `chainOf` collects Points the same way.
		//
		// And a drag is a link for the same reason a keystroke is, over and above its
		// own window check: it can fail to be DELIVERED, and what follows it then acts
		// on a screen the gesture never changed — the thing that was dragged is where it
		// was, the press after it lands on whatever is still under that coordinate, and
		// the window guard cannot see the difference because the window is the same
		// window.
		name, decl := g.dragSet(s)
		g.pts.decls = append(g.pts.decls, decl)
		return "OS's Drag with " + name, "", true
	default:
		// EVERY PRESS, WHICHEVER BUTTON, IS THIS ONE SENTENCE.
		//
		// A right or middle press used to be refused here, and the refusal's reason
		// was true of how it was GENERATED rather than of the press: an aim and then a
		// click at the cursor is two sentences, and wrapping the first leaves the
		// second — the one that acts — unwrapped. The Point carries the button now, so
		// there is one operation to wrap, one thing to check before it, and one
		// failure for the arm to stop at. See `buttonTerm`.
		return "OS's Click with " + g.pts.clickPoint(s), "", true
	}
}

// typeSentence is the `do` sentence one segment of typed text is emitted as, and the one place
// that decision is made.
//
// A declared secret arg (password/pin/…) is route-qualified, so each route remembers its own —
// `<route>:password` — while a plain typed `{{name}}` secret keeps its global name.
//
// ONE FUNCTION BECAUSE THERE ARE TWO CALLERS NOW. An ordinary block writes these with `emit` and
// a chain writes them as link guards, and a second place deciding which sentence a segment gets
// is a second place for the two to disagree — about the qualification above, most of all, where
// disagreeing means looking up a secret nobody stored.
func (g *gen) typeSentence(seg segment) string {
	if seg.secret {
		name := seg.text
		if seg.qualified {
			// THE ONE COMPOSER. See secretname.go: the orchestrator has to ask
			// the same question when it preserves a lookup across a re-render,
			// and two places composing this is two places to disagree.
			name = secretName(g.base, seg.text)
		}
		return fmt.Sprintf("OS's Secret with \"%s\"", escape(name))
	}
	return fmt.Sprintf("OS's Type with \"%s\"", escape(seg.text))
}

// textBySegment expands one typed step into the steps it is emitted as, for a chain only.
//
// `splitSecrets` turns `"user{{password}}"` into THREE sentences — a Type, a Secret, a Type — out
// of ONE `StepType`. A link is built around one `do` sentence, so a step that emits three would
// put two of them somewhere nobody can stop at: whichever one the chain wrapped, the other two
// ran whether it went in or not. Wrapping only the first or only the last leaves exactly the
// defect this is about, and leaves it in the place where it matters most — a password typed into
// a field after the username it was supposed to follow did not arrive.
//
// So the split happens HERE, before the chain is built, and every segment becomes its own step
// and its own link. The segment's own text is what the step carries, so the sentence is reached
// through `typeSentence` either way.
//
// A legacy route keeps its single step, because this is only reached when something in the
// recording asked to be checked. `keyByKey` does the same thing for a repeated key and for the
// same reason.
func textBySegment(steps []macroir.Step, declared map[string]bool) []macroir.Step {
	out := make([]macroir.Step, 0, len(steps))
	for _, s := range steps {
		if s.Kind != macroir.StepType {
			out = append(out, s)
			continue
		}
		for _, seg := range splitSecrets(s.Text, declared) {
			if !seg.secret && seg.text == "" {
				continue
			}
			one := s
			one.Text = seg.text
			if seg.secret {
				// WRITTEN BACK AS THE PLACEHOLDER IT CAME FROM, so the
				// step says what it is: re-reading it through
				// `splitSecrets` is how the sentence is chosen, and a
				// bare name would be read as a literal to type.
				one.Text = "{{" + seg.text + "}}"
			}
			out = append(out, one)
		}
	}
	return out
}

// keyByKey expands a repeated key into the presses it stands for, for a chain only.
//
// `simplify` folds a run of the same key into one step with a count, and the generator writes
// that as a `repeat` block. A block cannot stop a routine: whatever is inside it runs again after
// the press that did not happen, which is the same reason a recorded loop is refused outright. A
// key pressed three times is three keystrokes, so it becomes three links, each able to stop what
// comes after it.
//
// A legacy route keeps its `repeat`, because this is only reached when something in the recording
// asked to be checked.
func keyByKey(steps []macroir.Step) []macroir.Step {
	out := make([]macroir.Step, 0, len(steps))
	for _, s := range steps {
		if s.Kind != macroir.StepKey || s.Count <= 1 {
			out = append(out, s)
			continue
		}
		one := s
		one.Count = 1
		for i := 0; i < s.Count; i++ {
			out = append(out, one)
		}
	}
	return out
}

// chainOf splits the steps at every propagating action, one link each, and returns what follows
// the last one.
//
// Locals are collected per link from the generator's own list, by the span of indices a link's
// steps appended, so a Point is declared in the capability that presses it.
func chainOf(steps []macroir.Step, g *gen) (links []link, tail string, err error) {
	var waiting strings.Builder
	from := len(g.pts.decls)
	for _, s := range steps {
		guard, follow, ok := g.propagating(s)
		if !ok {
			text, err := emit([]macroir.Step{s}, g, 0)
			if err != nil {
				return nil, "", err
			}
			waiting.WriteString(text)
			continue
		}
		links = append(links, link{
			decls:  append([]string(nil), g.pts.decls[from:]...),
			before: waiting.String(),
			guard:  guard,
			follow: follow,
		})
		waiting.Reset()
		from = len(g.pts.decls)
	}
	// WHATEVER FOLLOWS THE LAST PROPAGATING ACTION runs inside its success, so it does
	// not happen when that action refused — and its locals are declared in that same
	// capability.
	if len(links) > 0 {
		last := &links[len(links)-1]
		last.decls = append(last.decls, g.pts.decls[from:]...)
	}
	return links, waiting.String(), nil
}

// writeChain renders the links as capabilities.
func writeChain(b *strings.Builder, actor string, links []link, tail string) {
	for i, l := range links {
		name := "Run"
		if i > 0 {
			name = fmt.Sprintf("Step%d", i+1)
		}
		fmt.Fprintf(b, "this can %s.\n", name)
		fmt.Fprintf(b, "this's %s does...\n", name)
		for _, d := range l.decls {
			fmt.Fprintf(b, "%s%s\n", indent(1), d)
		}
		b.WriteString(shift(l.before, 1))
		fmt.Fprintf(b, "%sdo %s...\n", indent(1), l.guard)
		fmt.Fprintf(b, "%swhen ok?\n", indent(2))
		b.WriteString(shift(l.follow, 3))
		if i+1 < len(links) {
			fmt.Fprintf(b, "%sdo %s's Step%d.\n", indent(3), actor, i+2)
			// AND ITS ANSWER IS THIS ONE'S ANSWER. `this is that!` adopts the
			// last outcome verbatim, so a refusal three presses along comes back
			// out of `Run` as the refusal it was.
			fmt.Fprintf(b, "%sthis is that!\n", indent(3))
		} else {
			b.WriteString(shift(tail, 3))
			// THE ONE `this is ok!`, AND IT IS NOT UNCONDITIONAL. It sits inside
			// the success of the last press, so there is no path on which a
			// refused routine reports that it worked.
			fmt.Fprintf(b, "%sthis is ok!\n", indent(3))
		}
		fmt.Fprintf(b, "%sor?\n", indent(2))
		fmt.Fprintf(b, "%sthis is that!\n", indent(3))
		b.WriteString("\n")
	}
}

// shift re-indents a block emitted at depth 0 to the depth it is going to sit at, leaving its own
// relative shape alone.
func shift(text string, depth int) string {
	if text == "" {
		return ""
	}
	pad := indent(depth)
	var b strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString(line)
			continue
		}
		b.WriteString(pad)
		b.WriteString(line)
	}
	return b.String()
}
