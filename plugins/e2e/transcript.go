//go:build windows

package main

// WHAT THE CHILD SAID, KEPT — THROUGH THE WINDOW'S OWN REPORT RATHER THAN OFF THE SCREEN.
//
// # Why the rendered page was not enough
//
// The harness already keeps `studio-screen.txt`, which is every word UI Automation can read off
// the window. That is not the child's transcript. The transcript lives in a `<pre>` behind
// `<details>Technical details</details>`, which is collapsed — correctly absent from the
// accessibility tree, because a thing nobody can see is a thing no assistive technology should
// claim is there. So the one artifact that says what `marco` actually said as it ran was the one
// the harness did not have.
//
// # And the product already gathers it
//
// Studio has a Report a problem button. It writes the run's record, what the run said as it ran,
// and the routine as it stands now into one file somebody can attach to an email — and nothing is
// sent anywhere, which is the whole design of it. That is exactly the artifact wanted here, so it
// is PRESSED rather than reimplemented: the harness exercises a product path that had no
// automated coverage at all, and keeps what the product produced.
//
// Two channels again, deliberately. The window says where the file went; the harness reads the
// file from the place the window named. A window that reported a path it had not written would be
// caught by the file not being there, and a report written somewhere the window could not say
// would be caught by the sentence not naming it.
//
// # An unavailable transcript is labelled, and the product is what labels it
//
// `Report` writes the section either way: the lines when they belong to the run being reported,
// and otherwise a sentence saying the window keeps only what the run in front of it said and that
// the record above is the durable account. So this asserts the section EXISTS and reports which of
// the two it got, rather than treating a missing transcript as a missing artifact. A report that
// silently omitted the section would be the finding.

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// The three sentences this reads, each of them the page's or the report's own words.
const (
	reportWroteTo   = "written to "
	reportHolds     = " — it holds "
	saidSection     = "WHAT IT SAID AS IT RAN"
	saidUnavailable = "not available. This window keeps only what the run in front of it said"
)

// keepReport presses Report a problem, keeps what it wrote, and says what was in it.
func keepReport(st *stage, reference string) (string, error) {
	if reference == "" {
		return "", fmt.Errorf("the run left no reference, so the window has nothing to " +
			"gather a report about")
	}
	// THE BUTTON HAS TO BE OFFERED, which is a product claim of its own: a finished run
	// carries a reference, and a reference is what makes a report possible. A disabled
	// button here would mean a person who hit this could not gather anything to send.
	if _, err := st.s.waitFor(20*time.Second, "reportBtn", enabled); err != nil {
		return "", fmt.Errorf("the run finished with reference %s and the window does "+
			"not offer to report on it: %w", reference, err)
	}
	if err := st.s.press("reportBtn"); err != nil {
		return "", fmt.Errorf("Report a problem could not be pressed: %w", err)
	}
	// AND THE WINDOW SAYS WHERE IT WENT. Waited for rather than assumed, because the
	// gathering is a binding call and the sentence is what tells a person it finished.
	var sentence string
	ok, saw := until(20*time.Second, func() (bool, string) {
		shot, err := st.s.look()
		if err != nil {
			return false, err.Error()
		}
		e, found := shot.labelled(reportWroteTo)
		if !found {
			return false, "the window has not said where the report went"
		}
		sentence = e.Label
		return true, e.Label
	})
	if !ok {
		st.s.keepRunView()
		return "", fmt.Errorf("Report a problem was pressed and the window never said "+
			"where the report went (%s), so a person pressing it is left with no "+
			"file and no error", saw)
	}
	where := reportPath(sentence)
	if where == "" {
		return "", fmt.Errorf("the window said %q, which does not name a file somebody "+
			"could open", sentence)
	}
	// INSIDE THIS RUN'S OWN STORE. Not a check on the product so much as on this harness:
	// a report gathered into the owner's home would be this run writing into the one place
	// it must never touch, and it would look like a passing scenario.
	if !within(st.store, where) {
		return "", fmt.Errorf("the window wrote its report to %q, which is outside this "+
			"run's own store %q", where, st.store)
	}
	if ok, saw := fileAppears(15*time.Second, where); !ok {
		return "", fmt.Errorf("the window says the report was written to %q and there "+
			"is no such file (%s)", where, saw)
	}
	st.w.art.copyIn("studio-report.txt", where)

	body, err := readFileString(where)
	if err != nil {
		return "", err
	}
	switch {
	case !strings.Contains(body, reference):
		return "", fmt.Errorf("the report at %q does not name the run %s it was "+
			"gathered for, so nobody receiving it could tell which run it is about",
			where, reference)
	case !strings.Contains(body, saidSection):
		return "", fmt.Errorf("the report at %q has no %q section. A report that leaves "+
			"the child's own output out silently is one somebody sends believing it "+
			"holds everything", where, saidSection)
	case strings.Contains(body, saidUnavailable):
		// LABELLED, NOT MISSING — and reported as such rather than as a pass with a
		// gap in it. This is the honest outcome when the window's live transcript
		// belongs to some other run, and the product says so in its own words.
		st.w.art.note("report", "the transcript was NOT AVAILABLE and the report says "+
			"so: this window keeps only what the run in front of it said")
		return "the window's report is kept at studio-report.txt; its transcript " +
			"section is present and labelled NOT AVAILABLE", nil
	}
	lines := strings.Count(said(body), "\n") + 1
	return fmt.Sprintf("the window's report is kept at studio-report.txt, holding the "+
		"run's record and %d line(s) of what the child said as it ran", lines), nil
}

// reportPath is the file the window named, out of the sentence it named it in.
func reportPath(sentence string) string {
	at := strings.Index(sentence, reportWroteTo)
	if at < 0 {
		return ""
	}
	rest := sentence[at+len(reportWroteTo):]
	if end := strings.Index(rest, reportHolds); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest)
}

// said is the body of the transcript section, for counting what is in it.
func said(body string) string {
	at := strings.Index(body, saidSection)
	if at < 0 {
		return ""
	}
	rest := body[at+len(saidSection):]
	// The next all-capitals heading ends the section; the ones after it are the routine or
	// the Act as it stands, which are not what the child said.
	for _, next := range []string{"\nTHE PLAY", "\nTHE ACT", "\nTHE RECORD"} {
		if end := strings.Index(rest, next); end >= 0 {
			rest = rest[:end]
		}
	}
	return strings.TrimSpace(rest)
}

// within reports whether a path is inside a directory, so a report cannot be read from outside
// the store this run owns.
func within(dir, path string) bool {
	d, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(d, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
