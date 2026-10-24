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
		return "", nil, fmt.Errorf("this recording contains %s, and a routine "+
			"whose presses are checked against the window they were made in "+
			"has to stop when one of them does not happen — which this "+
			"version cannot make %s do. Record it again using ordinary "+
			"left-clicks and the keyboard", why, why)
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
		links, tail, err = chainOf(prepared, g)
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
		"the %s is a Point with X %d, Y %d%s%s.", name, x, y, rel, measured(in)))
	return name
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

// clickPoint declares the Point for a click step — window-relative when the
// recorder captured an offset, else a plain absolute point.
// A point goes down the window-relative form when it is ABOUT a window, which is the basis and
// not the offset. Branching on the offset was the gap: a press that declared the window and could
// not be measured in it came out as a bare absolute point — indistinguishable from a route saved
// before any of this existed, and pressed with the same unchecked trust.
func (p *points) clickPoint(s macroir.Step) string {
	if s.WinRel || s.RelTo != "" {
		return p.addRel(s.X, s.Y, s)
	}
	return p.add(s.X, s.Y)
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
			case s.Button != "" && s.Button != "left":
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
				if seg.secret {
					// A declared secret arg (password/pin/…) is route-qualified so each
					// route remembers its own — "<route>:password" — while a plain typed
					// {{name}} secret keeps its global name.
					sname := seg.text
					if seg.qualified {
						sname = g.base + ":" + seg.text
					}
					fmt.Fprintf(&b, "%sdo OS's Secret with \"%s\".\n", pad, escape(sname))
				} else if seg.text != "" {
					fmt.Fprintf(&b, "%sdo OS's Type with \"%s\".\n", pad, escape(seg.text))
				}
			}
		case macroir.StepDrag:
			// A real press-hold-release drag (OS's Drag primitive): press at the start, glide to
			// the end, release. The Drag set type comes from os.marco.
			btn := s.Button
			if btn == "" {
				btn = "left"
			}
			g.dragN++
			fmt.Fprintf(&b, "%sthe drag%d is a Drag with FromX %d, FromY %d, ToX %d, ToY %d, Button %q.\n",
				pad, g.dragN, s.X, s.Y, s.X2, s.Y2, btn)
			fmt.Fprintf(&b, "%sdo OS's Drag with drag%d.\n", pad, g.dragN)
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
// whose only press was a right-click had no step this could wrap, so it was not a guarded route,
// so the refusal below never fired and it was generated as an ordinary block — a press that
// declared the window it was made in, pressed without any of it being looked at. Whether a
// demonstration asked for the contract and whether this version can honour it are two questions,
// and conflating them answers "no" to the first whenever the second is "no".
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
	return s.Kind == macroir.StepClick || s.Kind == macroir.StepMove
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
//   - a NON-LEFT CLICK is an aim and then a press. Wrapping the aim leaves the press
//     unwrapped, and the press is the thing that acts. One guarded operation would need a
//     button beside the point — a permanent word in the language for a case the owner's
//     recordings have not needed yet, so it waits for a recording that needs it.
//   - a DRAG resolves no offset, so it carries no window and is checked against nothing.
//   - an ANCHORED press is a ladder of locators with a click in each arm.
//   - a REPEATED block runs its body again, which is the one thing a stop cannot survive.
//
// Unreachable from a recorder-first recording today except the first two: anchors are off unless
// computer vision is turned on, and loop folding is off for a recording with a guarded press.
func cannotPropagate(s macroir.Step) string {
	switch {
	case s.Kind == macroir.StepClick && len(s.Template) > 0:
		return "a press found by matching a picture"
	case s.Kind == macroir.StepClick && s.AnchorText != "":
		return "a press found by reading words on the screen"
	case s.Kind == macroir.StepClick && s.Button != "" && s.Button != "left":
		return "a " + s.Button + "-click"
	case s.Kind == macroir.StepDrag:
		return "a drag"
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
	case !guardedStep(s):
		return "", "", false
	case s.Kind == macroir.StepMove:
		return "OS's Move with " + g.pts.clickPoint(s), "", true
	case s.Button != "" && s.Button != "left":
		// A press that is not left-handed is an AIM and then a press, and the aim is
		// what carries the window — the click that follows takes the cursor's
		// position. So the Move is what gets checked and what stops the routine, and
		// the button press belongs with it once it has gone through.
		//
		// It also used to be emitted with no point at all, which sent a recorded
		// right-click to wherever the cursor happened to be.
		name := g.pts.clickPoint(s)
		return "OS's Move with " + name,
			fmt.Sprintf("do OS's Click with %q.\n", s.Button), true
	default:
		return "OS's Click with " + g.pts.clickPoint(s), "", true
	}
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
