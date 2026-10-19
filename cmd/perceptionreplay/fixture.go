package main

// A SCOPED, ONE-STEP WORKFLOW DERIVED FROM CAPTURES OF THE APPLICATION IT WILL RUN IN.
//
// # What this is for, and what it honestly is not
//
// The pixel execution path has never been run. Proving it needs an artifact, and an artifact
// needs a demonstration — but a demonstration needs a person's hand on the mouse, because
// `navsource` discards any press it did not see a hand make. That rule is deliberate and is not
// being relaxed here.
//
// So for a CONTROLLED test environment the owner allowed the other thing:
//
//	Use a real human demonstration if needed for the [learning] claim; programmatically
//	constructed fixtures can validate execution wiring but must be reported as such.
//
// This builds one, and every number in it is MEASURED rather than written down: the control's
// rectangle comes from the production proposer over real captured pixels, its label from real
// OCR of those pixels, the outcome expectation from the difference between two real captures,
// and the application scope from the recording's own `session.json`. Nothing is asserted about
// geometry that was not observed.
//
// It is a fixture and the report says so. What it can establish is that the EXECUTION path
// works — ground, authorise, press, verify — which is the thing that has never been tested. It
// establishes nothing about how a person would show it, because no person did.
//
// # And it may not be pointed at anything else
//
//	The controlled menu must have its own correctly scoped workflow. We shouldn't weaken
//	application binding or repurpose an Ark artifact as though it were demonstrated in the test
//	application.
//
// The scope is the `application` field of the capture it was derived from, which is what
// `capture-game-session` recorded about the window it actually watched. A fixture built from
// captures of one application is valid for that application and refused everywhere else, by
// the same gate every other workflow goes through.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// fixtureSide is one captured state of the test interface.
type fixtureSide struct {
	dir string
	// application is what the recorder said it was watching.
	application string
	// look is the settled look the measurements come from, regions its proposals and
	// words its reading.
	look    int
	regions []observe.Region
	words   []observe.ReadWord
	// window is the readings of up to OutcomeSettleLooks usable looks ending at `look`,
	// oldest first — the unit the live verifier measures an outcome over.
	//
	// One look per side is what the first version used, and it cannot ask the persistence
	// question at all: a word read on the single after-look "appeared", whether it stood
	// for a second or was one frame of OCR flicker. See observe.Independent.
	window [][]observe.ReadWord
	// at is when the chosen look was captured, and inputs the recording's whole observed
	// input timeline. Together they answer whether a press was seen inside the control
	// between the two looks, which is the only evidence there is that the control caused
	// the difference between them. See pressBetween.
	at     int64
	inputs []inputRecord
}

// readFixtureSide loads one capture and picks a settled look to measure from.
//
// `at` names the look when a caller knows which one it wants, and -1 takes the LAST readable
// one. Last rather than first because a window that was just created is still painting, and the
// state this is meant to describe is the one that stood.
//
// An explicit look exists because a recording of a MENU WALK is one capture of several states.
// Two separate captures are the simple case and a single walk is the common one — the owner's
// Trackmania recording visits a sequence of menus in 864 frames — and a fixture derived from
// two looks of one walk is the same measurement over the same pixels.
func readFixtureSide(dir string, at int) (fixtureSide, error) {
	out := fixtureSide{dir: dir}
	raw, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return out, fmt.Errorf("perceptionreplay: %s has no session.json, so there is "+
			"no record of which application it watched: %w", dir, err)
	}
	var s struct {
		Application string `json:"application"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return out, fmt.Errorf("perceptionreplay: reading %s/session.json: %w", dir, err)
	}
	if strings.TrimSpace(s.Application) == "" {
		return out, fmt.Errorf("perceptionreplay: %s does not say which application "+
			"it watched, so a workflow from it could not be scoped", dir)
	}
	out.application = s.Application

	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return out, err
	}
	if len(frames) == 0 {
		return out, fmt.Errorf("perceptionreplay: %s has no frames", dir)
	}
	regions, readable := proposeAll(dir, frames)
	words := readPositions(dir, frames)
	usable := func(i int) bool {
		if i < 0 || i >= len(frames) {
			return false
		}
		if i < len(readable) && !readable[i] {
			return false
		}
		if i >= len(regions) || len(regions[i]) == 0 {
			return false
		}
		return i < len(words) && len(words[i]) > 0
	}
	// AN EXPLICIT LOOK IS TAKEN OR REFUSED, never silently replaced by a nearby one: a
	// caller naming look 300 and being given look 412 would be describing a state it
	// did not choose.
	pick := -1
	if at >= 0 {
		if !usable(at) {
			return out, fmt.Errorf("perceptionreplay: look %d of %s carries no "+
				"proposals or no reading, so nothing can be measured from it",
				at, filepath.Base(dir))
		}
		pick = at
	} else {
		for i := len(frames) - 1; i >= 0; i-- {
			if usable(i) {
				pick = i
				break
			}
		}
	}
	if pick < 0 {
		return out, fmt.Errorf("perceptionreplay: no look of %s carries both proposals "+
			"and a reading, so nothing can be measured from it", dir)
	}
	out.look, out.regions, out.words = pick, regions[pick], words[pick]
	out.at = frames[pick].AtMS
	// THE OBSERVED INPUT TIMELINE, read with its error ignored as every other mode in
	// this tool ignores it: a recording with no input.jsonl is a recording in which
	// nothing was seen, and `pressBetween` refuses on that rather than here.
	out.inputs, _ = readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	out.window = windowEndingAt(words, usable, pick, observe.OutcomeSettleLooks)
	return out, nil
}

// windowEndingAt is the readings of up to `n` usable looks ending at `at`, oldest first.
//
// Walking BACKWARDS from the chosen look and skipping the ones that carry no reading, then
// reversed, so the window is this state's own readings in the order they happened. Capped at
// `n` because the live verifier caps its own window at `observe.OutcomeSettleLooks` and the two
// have to measure over the same amount of evidence.
//
// Its own function so that a test can reach it: `readFixtureSide` needs a capture directory on
// disk, and an off-by-one in the cap or a dropped `usable` check both survived a mutation run
// while this was inline there.
func windowEndingAt(words [][]observe.ReadWord, usable func(int) bool, at, n int) [][]observe.ReadWord {
	var back []int
	for i := at; i >= 0 && len(back) < n; i-- {
		if usable(i) {
			back = append(back, i)
		}
	}
	var out [][]observe.ReadWord
	for i := len(back) - 1; i >= 0; i-- {
		out = append(out, words[back[i]])
	}
	return out
}

// fixtureHalves splits a run of readings into two consecutive windows for the self-test.
//
// The same shape as `workflowrun.windowsOf`, and deliberately not an import of it: that one is
// unexported and belongs to the run. Halves, so each side is as long as the evidence a real
// verification rests on, and fewer than two readings yields nothing — which `Stationary` then
// reports as 0 quiet windows, meaning the question was not asked rather than that it passed.
func fixtureHalves(reads [][]observe.ReadWord) [][][]observe.ReadWord {
	if len(reads) < 2 {
		return nil
	}
	half := len(reads) / 2
	return [][][]observe.ReadWord{reads[:half], reads[half:]}
}

// theControlSaying is the smallest proposal holding exactly the named word.
//
// SMALLEST and EXACTLY ONE, which is the acceptance criterion
// [[Experiment-085-the-graphics-tab-is-a-button]] met on Ark: a rectangle holding one label is
// the control that label names, and a rectangle holding six is the bar they sit in.
func theControlSaying(side fixtureSide, label string) (observe.Region, []observe.Region, error) {
	want := strings.ToLower(label)
	var best observe.Region
	found := false
	for _, r := range side.regions {
		inside := 0
		hit := false
		for _, w := range side.words {
			if !inside2(r, w) {
				continue
			}
			inside++
			for _, t := range observe.TextWords(w.Text) {
				if t == want {
					hit = true
				}
			}
		}
		if !hit || inside != 1 {
			continue
		}
		if !found || r.Width*r.Height < best.Width*best.Height {
			best, found = r, true
		}
	}
	if !found {
		return observe.Region{}, nil, fmt.Errorf("perceptionreplay: no proposal on "+
			"look %d of %s holds exactly the one word %q; the control cannot be "+
			"identified and no fixture may guess at it",
			side.look, filepath.Base(side.dir), label)
	}
	// AND EVERY PROPOSAL THAT CONTAINS IT, which is the region an outcome may not be
	// read from. The same rule the demonstration path applies.
	var containers []observe.Region
	for _, r := range side.regions {
		if sameRegionExactly(r, best) {
			continue
		}
		if observe.Encloses(r, best) {
			containers = append(containers, r)
		}
	}
	return best, containers, nil
}

// inside2 reports whether a word's box centre falls in a region.
func inside2(r observe.Region, w observe.ReadWord) bool {
	c := observe.PointerAt{X: w.Where.X + w.Where.Width/2, Y: w.Where.Y + w.Where.Height/2}
	return holds(r, c)
}

// pressBetween is the recording's own evidence that the control caused the difference.
//
// # The defect this closes, and it was mine
//
// Measured with the four perception terms all satisfied, this tool derived a Trackmania
// workflow saying that pressing SETTINGS produces twelve words — `resume`, `author`, `gold`,
// `select`, `back` and eight others. Every term passed: the words were positive, they stood on
// all five readings of the after window, none came from inside the control, and the before
// window held still. The conclusion was still false, because nobody pressed SETTINGS. The two
// looks were the same menu with the highlight bar on a different row, and a row whose text
// stops being inverted becomes readable — persistently, independently and on a still screen.
//
// No perception term can fix that, because the missing fact is not about pixels. It is that an
// outcome is only an outcome OF something, and the difference between two looks of one
// recording may be attributed to a control only if the recording saw somebody press it.
//
// # Which the recording knows
//
// `capture-game-session` writes an observed input timeline beside the frames, and the owner's
// rule for what a demonstrated coordinate is good for applies exactly:
//
//	A demonstrated coordinate is authoritative evidence of where the person acted. It must not
//	become an unconditional coordinate replay instruction.
//
// So it is used as evidence and not as an instruction. Ark's capture carries
// `{"intent":"point","at_ms":24892,"where":{"x":0.414,"y":0.261}}` — inside the GRAPHICS
// rectangle at 0.410,0.241 and between look 245 at 24501ms and look 262 at 26200ms. The
// Trackmania capture carries 31 inputs and not one `point`, so there is nothing to attribute
// anything to, and it is refused.
//
// Inside the rectangle with no tolerance, because the rectangle is already the proposer's
// padded button box rather than the glyphs: a press on the label's edge is inside it.
func pressBetween(side fixtureSide, control observe.Region, from, to int64) (inputRecord, bool) {
	for _, in := range side.inputs {
		if in.Intent != string(observe.NavPoint) {
			continue
		}
		if in.AtMS < from || in.AtMS > to {
			continue
		}
		if !holds(control, in.Where) {
			continue
		}
		return in, true
	}
	return inputRecord{}, false
}

// fixtureAttribution says what links the two looks to the control, or refuses to guess.
//
// ONE RECORDING, which is a menu walk, has a single input timeline spanning both looks, so the
// question has a measured answer: was a press seen inside this control between them. If not,
// whatever changed was not this control's doing and the derivation is refused.
//
// TWO RECORDINGS have no shared timeline and the question cannot be asked at all. The
// controlled test menu is deliberately this shape — it serves its before and after states from
// a URL so that nobody has to click to capture either one — and there the link is an operator's
// statement rather than an observation. That statement is REQUIRED, written onto the step, and
// never implied by silence: a derivation whose causal claim rests on somebody's word must say
// whose, in the artifact, where whoever reads the verdict will see it.
func fixtureAttribution(before, after fixtureSide, control observe.Region,
	beforeDir, afterDir, asserted string) (string, error) {

	if filepath.Clean(beforeDir) != filepath.Clean(afterDir) {
		if strings.TrimSpace(asserted) == "" {
			return "", fmt.Errorf("perceptionreplay: the two looks come from two " +
				"different recordings, which share no input timeline, so " +
				"nothing here can show that pressing this control is what " +
				"produced the second state. Pass -fixture-asserted with who " +
				"or what asserts that link; it is written onto the step")
		}
		return "NO observed press links these two states: they come from two separate " +
			"recordings with no shared input timeline, and the link is asserted " +
			"rather than measured — " + strings.TrimSpace(asserted), nil
	}
	// AND THE AFTER STATE MUST ACTUALLY COME AFTER.
	//
	// An earlier version silently swapped the two timestamps so that a window running
	// backwards still had a press in it. That makes `-fixture-look 262 -fixture-after-look
	// 245` describe the GRAPHICS panel closing as though it opened, with the same press
	// attributed to both readings of the same event. Refused instead, because the caller
	// named the two looks and the order they named them in is a claim.
	if after.at <= before.at {
		return "", fmt.Errorf("perceptionreplay: look %d was captured at %dms and "+
			"look %d at %dms, so the after state is not after the before state; "+
			"nothing can be attributed across a window that runs backwards",
			before.look, before.at, after.look, after.at)
	}
	in, ok := pressBetween(before, control, before.at, after.at)
	if !ok {
		points := 0
		for _, i := range before.inputs {
			if i.Intent == string(observe.NavPoint) {
				points++
			}
		}
		return "", fmt.Errorf("perceptionreplay: %s saw no mouse press inside this "+
			"control between look %d (%dms) and look %d (%dms) — %d press(es) "+
			"and %d input(s) in the whole recording. Whatever differs between "+
			"those two looks was not this control's doing, and an outcome is "+
			"only an outcome OF something",
			filepath.Base(beforeDir), before.look, before.at, after.look, after.at,
			points, len(before.inputs))
	}
	return fmt.Sprintf("an observed mouse press at %.3f,%.3f %dms into the recording, "+
		"inside this control's own rectangle and between the two looks measured; "+
		"that press is evidence of what caused the difference and is NOT a replay "+
		"coordinate", in.Where.X, in.Where.Y, in.AtMS), nil
}

// fixtureOutcome measures what the action produced, THROUGH the production rule.
//
// `excluded` is the control and everything containing it, so the expectation cannot rest on the
// action site's own evidence. `OutcomeBetween` then applies the positive and persistent terms
// over the two windows and `Independent` the fourth — exactly what the live correlator and the
// live verifier do. Nothing is measured here that is not measured there, and that is the whole
// point of the function: the hand-rolled version that stood here was wrong twice, by one word
// on Ark and by twelve on a Trackmania recording whose fold is a single screen.
//
// `quiet` and `moved` come back for the report whether or not they refuse, because a reader
// deciding whether to trust a derived expectation needs to see that the question was asked.
func fixtureOutcome(before, after fixtureSide, control observe.Region,
	containers []observe.Region) (observe.LocalOutcome, int, int, error) {

	excluded := append([]observe.Region{control}, containers...)
	o := observe.Independent(excluded, before.window,
		observe.OutcomeBetween(excluded, before.window, after.window))
	// ── AND THE SCREEN MUST HOLD STILL, or none of it means anything ──
	//
	// The self-test, over the before side's own readings with no action between them.
	// A screen whose words change by themselves produces an outcome from nothing, and
	// this tool minted one: on a Trackmania recording whose fold is ONE screen over 862
	// looks, it reported twelve words — a clock, a ping counter, and the rows whose text
	// stopped being inverted when the highlight bar moved off them.
	quiet, moved := observe.Stationary(excluded, fixtureHalves(before.window))
	if quiet > 0 && moved > 0 {
		return o, quiet, moved, fmt.Errorf("perceptionreplay: %d of %d window(s) of "+
			"%s before the action changed by themselves, so this screen cannot "+
			"support a local outcome at all and no expectation derived from it "+
			"would mean anything",
			moved, quiet, filepath.Base(before.dir))
	}
	if len(o.Appeared) < observe.OutcomeWords {
		return o, quiet, moved, fmt.Errorf("perceptionreplay: only %d word(s) "+
			"appeared outside the control and STOOD on every reading between the "+
			"two captures (%s); %d transient and %d reused were discarded, and an "+
			"outcome needs %d. The two states are not distinguishable enough for "+
			"a verifiable action",
			len(o.Appeared), strings.Join(o.Appeared, " "), len(o.Transient),
			len(o.Reused), observe.OutcomeWords)
	}
	return o, quiet, moved, nil
}

// buildFixtureWorkflow derives a one-step artifact from a before and an after capture.
func buildFixtureWorkflow(beforeDir, afterDir, label, name, asserted string,
	beforeLook, afterLook int) (workflowstore.Artifact, string, error) {

	before, err := readFixtureSide(beforeDir, beforeLook)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
	after, err := readFixtureSide(afterDir, afterLook)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
	return assembleFixture(before, after, label, name, asserted, beforeDir, afterDir)
}

// assembleFixture is everything buildFixtureWorkflow does once the two states are in hand.
//
// Split off the disk reading so that every gate below can be entered with states made by hand.
// `buildFixtureWorkflow` is two `readFixtureSide` calls and this, so a test over this one
// function covers the control search, the attribution, the four outcome terms and what reaches
// the artifact — all of which were, between them, wrong three times.
func assembleFixture(before, after fixtureSide, label, name, asserted,
	beforeDir, afterDir string) (workflowstore.Artifact, string, error) {

	// ── THE SAME APPLICATION ON BOTH SIDES, or the two captures describe two things ──
	if !strings.EqualFold(before.application, after.application) {
		return workflowstore.Artifact{}, "", fmt.Errorf("perceptionreplay: the before "+
			"capture watched %q and the after capture %q; a workflow cannot be "+
			"derived from captures of two applications",
			before.application, after.application)
	}
	control, containers, err := theControlSaying(before, label)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
	// ── AND SOMETHING MUST HAVE BEEN SEEN TO CAUSE THE DIFFERENCE ──
	//
	// Two modes, because the two shapes carry different evidence and must not be allowed
	// to look alike. See pressBetween for the measurement that forced this.
	attributed, err := fixtureAttribution(before, after, control, beforeDir, afterDir,
		asserted)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
	o, quiet, moved, err := fixtureOutcome(before, after, control, containers)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
	appeared := o.Appeared

	// ── THE ARTIFACT: ONE SCREEN, ONE CONTROL, ONE STEP THAT STAYS WHERE IT IS ──
	//
	// From == To, because that is what the interface does: the panel changes and the
	// screen does not. Such a step can never arrive anywhere and is finished by its own
	// outcome, which is the case this whole exercise exists to execute.
	arrangement := observe.ArrangeLook(before.regions)
	text := observe.TextOf("ocr", phrasesOf(before.words))
	art := workflowstore.Artifact{
		Version: workflowstore.Version, Name: name,
		SavedAt:     time.Now().UTC(),
		Application: workflowstore.NormaliseScope(before.application),
		Screens: []workflowstore.Screen{{
			Label: "screen_a", Arrangement: arrangement, Text: &text,
		}},
		Controls: []workflowstore.Control{{
			Label: "control_1", Readings: []observe.Region{control},
			Containers: containers,
			Text:       []string{strings.ToUpper(label)},
		}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1, Outcome: appeared,
			Unresolved: []string{"this workflow was ASSEMBLED from " +
				"captures of the application rather than acquired by " +
				"the live correlator while a person demonstrated it; " +
				"it validates the execution path and claims nothing " +
				"about what a live demonstration would produce",
				attributed},
		}},
		Notes: []string{fmt.Sprintf("derived from %s (look %d) and %s (look %d) by "+
			"perceptionreplay -fixture; every rectangle, label and outcome word "+
			"here was measured from those captured pixels",
			filepath.Base(beforeDir), before.look, filepath.Base(afterDir),
			after.look)},
	}
	say := fmt.Sprintf("  application   %q, from %s/session.json\n"+
		"  control       %.1f%%x%.1f%% at %.3f,%.3f, holding exactly %q\n"+
		"  containers    %d proposal(s) enclose it, and the outcome may not be read there\n"+
		"  caused by     %s\n"+
		"  outcome       %d word(s) on every reading of %d look(s): %s\n"+
		"  discarded     %d transient, %d reused by the control, %d vanished\n"+
		"  held still    %d of %d window(s) before the action changed by themselves\n"+
		"  screen        %d rectangle(s), %d stable word(s) read\n",
		before.application, filepath.Base(beforeDir),
		control.Width*100, control.Height*100, control.X, control.Y, label,
		len(containers), attributed, len(appeared), len(after.window),
		strings.Join(appeared, " "),
		len(o.Transient), len(o.Reused), len(o.Vanished), moved, quiet,
		arrangement.Total, len(text.Words))
	return art, say, nil
}

// phrasesOf is the text of a reading, for the vocabulary.
func phrasesOf(words []observe.ReadWord) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, w.Text)
	}
	return out
}

// reportFixtureWorkflow derives the artifact and writes it.
func reportFixtureWorkflow(beforeDir, afterDir, label, name, to, asserted string,
	beforeLook, afterLook int) error {
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("A ONE-STEP WORKFLOW DERIVED FROM CAPTURES OF THE TEST APPLICATION\n")
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  ASSEMBLED, NOT ACQUIRED. Every rectangle, label and outcome word below\n")
	fmt.Printf("  is measured from captured pixels, and the line marked `caused by` says\n")
	fmt.Printf("  what links the two states to the control — an observed press, or\n")
	fmt.Printf("  somebody's word. The live correlator acquired none of this.\n\n")

	art, say, err := buildFixtureWorkflow(beforeDir, afterDir, label, name, asserted,
		beforeLook, afterLook)
	if err != nil {
		return err
	}
	fmt.Print(say)
	body, err := art.Marshal()
	if err != nil {
		return fmt.Errorf("perceptionreplay: the derived workflow is not valid: %w", err)
	}
	if to == "" {
		fmt.Printf("\n  --fixture-save was not given, so nothing was written.\n")
		return nil
	}
	if err := os.WriteFile(to, body, 0o600); err != nil {
		return err
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		return fmt.Errorf("perceptionreplay: the derived workflow does not load: %w", err)
	}
	fmt.Printf("\n  SAVED AND READ BACK\n      %s — %d byte(s), version %d\n",
		to, len(body), back.Version)
	ok, why := back.ScopeAllows(string(art.Application))
	fmt.Printf("      scope: %v — %s\n", ok, why)
	// AND IT IS REFUSED SOMEWHERE ELSE. Against a sentinel that cannot be a real
	// executable name, because the first version of this check compared against
	// `arkascended` — which is the application the Ark capture WAS taken in, so the
	// check failed on correct output and said the opposite of what had happened.
	const elsewhere = "not.an.application"
	if ok, _ := back.ScopeAllows(elsewhere); ok {
		return fmt.Errorf("perceptionreplay: the derived workflow is valid for %q, "+
			"so its scope is not binding anything", elsewhere)
	}
	fmt.Printf("      and it is refused for any other application\n")
	return nil
}
