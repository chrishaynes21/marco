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
	"sort"
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
}

// readFixtureSide loads one capture and picks a settled look to measure from.
//
// The LAST readable look, not the first: a window that was just created is still painting, and
// the state this is meant to describe is the one that stood.
func readFixtureSide(dir string) (fixtureSide, error) {
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
	for i := len(frames) - 1; i >= 0; i-- {
		if i < len(readable) && !readable[i] {
			continue
		}
		if i >= len(regions) || len(regions[i]) == 0 {
			continue
		}
		if i >= len(words) || len(words[i]) == 0 {
			continue
		}
		out.look, out.regions, out.words = i, regions[i], words[i]
		return out, nil
	}
	return out, fmt.Errorf("perceptionreplay: no look of %s carries both proposals and "+
		"a reading, so nothing can be measured from it", dir)
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

// elsewhereOf is the words of one side read outside the excluded regions.
func elsewhereOf(side fixtureSide, excluded []observe.Region) map[string]bool {
	out := map[string]bool{}
	for _, w := range side.words {
		skip := false
		for _, r := range excluded {
			if inside2(r, w) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, t := range observe.TextWords(w.Text) {
			out[t] = true
		}
	}
	return out
}

// buildFixtureWorkflow derives a one-step artifact from a before and an after capture.
func buildFixtureWorkflow(beforeDir, afterDir, label, name string) (
	workflowstore.Artifact, string, error) {

	before, err := readFixtureSide(beforeDir)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
	after, err := readFixtureSide(afterDir)
	if err != nil {
		return workflowstore.Artifact{}, "", err
	}
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
	// ── THE OUTCOME: WHAT THE AFTER STATE SAYS THAT THE BEFORE STATE DID NOT ──
	//
	// Outside the control and everything containing it, so the expectation cannot rest
	// on the action site's own evidence — the same independence term the demonstration
	// path applies. See observe/outcome.go.
	excluded := append([]observe.Region{control}, containers...)
	was, is := elsewhereOf(before, excluded), elsewhereOf(after, excluded)
	var appeared []string
	for w := range is {
		if !was[w] {
			appeared = append(appeared, w)
		}
	}
	sort.Strings(appeared)
	if len(appeared) < observe.OutcomeWords {
		return workflowstore.Artifact{}, "", fmt.Errorf("perceptionreplay: only %d "+
			"word(s) appeared outside the control between the two captures (%s), "+
			"and an outcome needs %d; the two states are not distinguishable "+
			"enough for a verifiable action",
			len(appeared), strings.Join(appeared, " "), observe.OutcomeWords)
	}

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
			Unresolved: []string{"this workflow was CONSTRUCTED from two " +
				"captures of the application rather than demonstrated by " +
				"a person; it validates the execution path and claims " +
				"nothing about what a demonstration would produce"},
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
		"  outcome       %d word(s): %s\n"+
		"  screen        %d rectangle(s), %d stable word(s) read\n",
		before.application, filepath.Base(beforeDir),
		control.Width*100, control.Height*100, control.X, control.Y, label,
		len(containers), len(appeared), strings.Join(appeared, " "),
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
func reportFixtureWorkflow(beforeDir, afterDir, label, name, to string) error {
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("A ONE-STEP WORKFLOW DERIVED FROM CAPTURES OF THE TEST APPLICATION\n")
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  CONSTRUCTED, NOT DEMONSTRATED. Every number below is measured from the\n")
	fmt.Printf("  captured pixels; no person demonstrated this, and it claims\n")
	fmt.Printf("  nothing about what a demonstration would produce.\n\n")

	art, say, err := buildFixtureWorkflow(beforeDir, afterDir, label, name)
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
	if ok, _ := back.ScopeAllows("arkascended"); ok {
		return fmt.Errorf("perceptionreplay: the derived workflow is valid for an " +
			"application it was not captured in")
	}
	fmt.Printf("      and it is refused for any other application\n")
	return nil
}
