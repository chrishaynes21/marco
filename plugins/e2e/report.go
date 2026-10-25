//go:build windows

package main

// WHAT THIS RUN ESTABLISHED, SAID IN THREE PARTS THAT ARE NOT INTERCHANGEABLE.
//
// # Why the layers are named and never totalled
//
// Because they are evidence about different things, and a single number over them would be worse
// than no number. A desktop scenario establishes that Studio's own window performed a routine and
// that the application received the presses. A recording-integration scenario establishes that a
// draft becomes a reviewable, savable, collision-protected routine. Adding them together produces
// "9 of 9 passed", which somebody will reasonably read as "the recording path works on Windows" —
// and that is the one thing no scenario here establishes.
//
// So the summary has three lines and the third is a limit rather than a score:
//
//	passed automated desktop scenarios
//	passed recording-integration scenarios
//	physical recording capture: NOT EXERCISED
//
// The third is printed on a green run, in the same words, every time. A limit that is only
// mentioned when something fails is a limit nobody knows about.
//
// # NOT RUN is never green and never red
//
// It is its own ending with its own exit code, because the thing that reads this most often is a
// script, and a harness that exited 0 when it could not find a screen would be telling that
// script the product works.

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	layerDesktop   = "desktop"
	layerRecording = "recording"

	// notExercised is said the same way every time, green or not.
	notExercised = "physical recording capture: NOT EXERCISED — Marco's hooks discard " +
		"injected input by design, so no automated run can establish that Windows " +
		"captured a physical gesture. A person demonstrating in Studio is still the " +
		"only evidence of that; see E2E.md."
)

// verdictOf is what a set of results adds up to, and the ending it adds up to.
//
// # A SKIPPED SCENARIO IS NOT A SUCCESS
//
// This used to return 0 whenever something passed and nothing failed, which made a partial run
// indistinguishable from a complete one. The resized-window refusal could decline to run — the
// window would not resize, the fixture would not start, the foreground was not ours — and the
// harness exited 0 with the one scenario that establishes the guard refuses never having run. A
// script reading that exit code was told the product works.
//
// So the endings are, and a reader of the code is the point of the list being here:
//
//	0  every scenario selected ran, and passed
//	1  a scenario failed — which outranks everything, because a failure is a finding
//	2  something selected did not run: nothing did, some did not, or the selection
//	   matched nothing at all. None of those is a result about Marco
//	3  the harness itself could not get started
//
// `selected` is how many scenarios the selection matched, which is the only way to tell "every
// scenario passed" from "`-only` matched nothing, so an empty run passed vacuously".
func verdictOf(results []result, selected int) (verdict string, code int) {
	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		switch {
		case r.skipped:
			skipped++
		case r.passed:
			passed++
		default:
			failed++
		}
	}
	switch {
	case failed > 0:
		return "failed", 1
	case selected == 0:
		return "nothing selected", 2
	case passed == 0:
		return "not run", 2
	case skipped > 0:
		return "incomplete", 2
	default:
		return "passed", 0
	}
}

// report prints the summary, writes it beside the artifacts, and returns the exit code.
//
// `checks` are the harness's own preconditions — the things it established about itself before any
// scenario leaned on them — printed on every run because a reader of a green summary is entitled
// to know they were checked rather than assumed.
func report(art *artifacts, m machine, checks []string, results []result, fatal error,
	started time.Time) int {
	fmt.Printf("\n%s\n", strings.Repeat("─", 78))

	if fatal != nil {
		var nr notRun
		if asNotRun(fatal, &nr) {
			fmt.Printf("NOT RUN: %s\n", nr.why)
			describe(m)
			fmt.Printf("\n%s\n", notExercised)
			keepSummary(art, m, checks, results, "not run", nr.why, started)
			return 2
		}
		fmt.Printf("the harness could not start: %v\n", fatal)
		keepSummary(art, m, checks, results, "broken", fatal.Error(), started)
		return 3
	}

	for _, layer := range []string{layerDesktop, layerRecording} {
		var lines []string
		for _, r := range results {
			if r.layer != layer {
				continue
			}
			mark := "FAILED "
			switch {
			case r.skipped:
				mark = "not run"
			case r.passed:
				mark = "passed "
			}
			lines = append(lines, fmt.Sprintf("  %s  %-34s %s", mark, r.name,
				r.firstLine()))
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Printf("\n%s scenarios\n%s\n", layer, strings.Join(lines, "\n"))
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 78))
	for _, layer := range []string{layerDesktop, layerRecording} {
		p, f, s := 0, 0, 0
		for _, r := range results {
			if r.layer != layer {
				continue
			}
			switch {
			case r.skipped:
				s++
			case r.passed:
				p++
			default:
				f++
			}
		}
		if p+f+s == 0 {
			continue
		}
		fmt.Printf("passed %s scenarios: %d of %d", layer, p, p+f)
		if s > 0 {
			fmt.Printf(" (%d not run)", s)
		}
		fmt.Println()
	}
	fmt.Printf("%s\n", notExercised)

	verdict, code := verdictOf(results, len(results))
	// AND A PARTIAL RUN SAYS SO IN WORDS, not only in an exit code.
	//
	// The counts above are per layer and a reader totals them in their head; the one sentence
	// that has to be unmissable is that this run does not cover everything it selected.
	switch verdict {
	case "incomplete":
		fmt.Printf("\nINCOMPLETE: %d of the %d selected scenarios did not run, so this "+
			"result says nothing about what they establish.\n",
			countSkipped(results), len(results))
	case "not run":
		fmt.Printf("\nNOT RUN: none of the %d selected scenarios ran.\n", len(results))
	case "nothing selected":
		fmt.Printf("\nNOTHING SELECTED: no scenario matched, so nothing was " +
			"established. A selection that matches nothing is not a pass.\n")
	}
	for _, c := range checks {
		fmt.Printf("%s\n", c)
	}
	describe(m)
	fmt.Printf("\nartifacts: %s\n", art.dir)

	keepSummary(art, m, checks, results, verdict, "", started)
	return code
}

func countSkipped(results []result) int {
	n := 0
	for _, r := range results {
		if r.skipped {
			n++
		}
	}
	return n
}

// firstLine is what a reader needs beside a scenario's name: its finding, or what it established.
func (r result) firstLine() string {
	if r.why != "" {
		return r.why
	}
	if len(r.evidence) > 0 {
		return r.evidence[0]
	}
	return r.about
}

// describe says what machine this was, which is half of what any result means.
func describe(m machine) {
	if m.ScreenW == 0 {
		return
	}
	fmt.Printf("machine: %dx%d at %d dpi", m.ScreenW, m.ScreenH, m.DPI)
	if m.Head != "" {
		fmt.Printf(", %s", m.Head)
		if m.Dirty {
			fmt.Printf(" (uncommitted changes)")
		}
	}
	fmt.Println()
	if m.OtherStudio != "" {
		fmt.Printf("note: another Studio was running — %s. Every window this run read "+
			"or pressed was addressed by the process it started.\n", m.OtherStudio)
	}
}

func keepSummary(art *artifacts, m machine, checks []string, results []result,
	verdict, why string, started time.Time) {

	type line struct {
		Name     string   `json:"name"`
		Layer    string   `json:"layer"`
		About    string   `json:"about"`
		Verdict  string   `json:"verdict"`
		Why      string   `json:"why,omitempty"`
		Evidence []string `json:"evidence,omitempty"`
		TookMs   int64    `json:"took_ms"`
	}
	var lines []line
	for _, r := range results {
		v := "failed"
		switch {
		case r.skipped:
			v = "not run"
		case r.passed:
			v = "passed"
		}
		lines = append(lines, line{Name: r.name, Layer: r.layer, About: r.about,
			Verdict: v, Why: r.why, Evidence: r.evidence,
			TookMs: r.took.Milliseconds()})
	}
	out, err := json.MarshalIndent(map[string]any{
		"verdict":                    verdict,
		"why":                        why,
		"physical_recording_capture": "not exercised",
		"checks":                     checks,
		"machine":                    m,
		"scenarios":                  lines,
		"took_ms":                    time.Since(started).Milliseconds(),
	}, "", "  ")
	if err == nil {
		art.keep("summary.json", out)
	}
}
