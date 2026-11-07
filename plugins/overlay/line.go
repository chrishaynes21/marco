package main

// THE COMMAND LINE READS BACK WHAT ENTER WOULD RUN (S6, proposal §5.4).
//
// While somebody types after `m, the engine is asked what the line means (`marco line --json`),
// off the hook thread and off the action loop, once the line has settled. The answer is drawn
// under the line: the value a routine still needs as a muted `‹resource›`, and the read-back
// `Cloud pull · resource: stone` once it can run. This file words and draws nothing of its own
// about routines; the reading is the engine's.
//
// # What Enter runs
//
// A line the engine reads as a routine that takes values runs as that routine's IDENTITY and the
// VERSION the read-back was read from: `marco do --source=typed --play=… --app=… [--focus]
// --expect=<version> --arg=name=value…`. The words are not read again, and a routine saved since
// the read-back refuses at the door. And Enter runs it only if THAT read-back was on screen for
// exactly this line: if it had not arrived yet, Enter shows it and the next Enter runs it, so
// nothing with a value in it runs unread.
//
// A line with a value missing, or several values without labels, runs nothing: the line stays
// open with the text and says what is needed, and the value is typed in place.
//
// Every other line (a routine with no values, anything that names no routine, an engine too old
// to answer) goes to the intake as words, exactly as before.
//
// # Values Director proposed
//
// When nothing took a phrase and Director named a routine that takes values, the command line
// opens on that routine with what Director proposed written in as `name:value`, editable. The
// reading is asked with the identity and the proposals, so the read-back labels a value left alone
// as read out of what was said and one the person changed as theirs (the engine compares; this
// surface only says what it offered), and a proposal for an earlier revision refuses. Nothing
// runs until Enter on that read-back.

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// lineReading is `marco line --json`, the fields this surface draws and sends.
type lineReading struct {
	Line     string            `json:"line"`
	Kind     string            `json:"kind"`
	ID       string            `json:"id"`
	App      string            `json:"app"`
	Slug     string            `json:"slug"`
	Focus    bool              `json:"focus"`
	Name     string            `json:"name"`
	Needs    bool              `json:"needs"`
	Values   map[string]string `json:"values"`
	Parts    []linePart        `json:"parts"`
	Pending  string            `json:"pending"`
	Readback string            `json:"readback"`
	Says     string            `json:"says"`
	Runnable bool              `json:"runnable"`
	SourceAt string            `json:"source_at"`
	Times    int               `json:"times"`
	// Resolving is a skill whose target is still being worked out (Focus app asking Marco's model
	// which open program was meant): not runnable, and read again with --resolve (resolveReading).
	Resolving bool `json:"resolving"`
}

// linePart is one argument in the read-back.
type linePart struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
	Said   string `json:"said"`
}

// lineProposal is what Director proposed, and the identity and version it was proposed for.
type lineProposal struct {
	app, slug string
	focus     bool
	values    map[string]string
	at        string
}

func (p *lineProposal) flags() []string {
	if p == nil || p.slug == "" {
		return nil
	}
	f := []string{"--play=" + p.slug}
	if p.app != "" {
		f = append(f, "--app="+p.app)
	}
	if p.focus {
		f = append(f, "--focus")
	}
	keys := make([]string, 0, len(p.values))
	for k := range p.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f = append(f, "--proposed="+k+"="+p.values[k])
	}
	if p.at != "" {
		f = append(f, "--proposed-at="+p.at)
	}
	return f
}

// lineWithin bounds one reading. A reading is one `marco` start and a library read.
var lineWithin = 2 * time.Second

// askLine is a package variable so a test can answer for the engine. Production never reassigns it.
var askLine = readLineChild

// readLineChild runs `marco line --json`, bounded and reaped. ok is false when there is no
// reading at all (an engine too old to have `line`, a timeout, an unreadable answer), and the line
// is then sent as words, as it always was.
func readLineChild(line string, p *lineProposal) (lineReading, bool) {
	return readLineChildWith(line, p, lineWithin)
}

// readLineChildWith is readLineChild bounded by within, with any extra flags first.
func readLineChildWith(line string, p *lineProposal, within time.Duration, extra ...string) (lineReading, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	args := append(append([]string{"line", "--json"}, extra...), p.flags()...)
	args = append(args, "--", line)
	out, err := exec.CommandContext(ctx, marcoBin(), args...).Output()
	if ctx.Err() != nil || len(strings.TrimSpace(string(out))) == 0 {
		mlogD("overlay: no reading of the line", "err", errText(err))
		return lineReading{}, false
	}
	var r lineReading
	if json.Unmarshal(out, &r) != nil || r.Line != strings.TrimSpace(line) {
		return lineReading{}, false
	}
	return r, true
}

// takesValues is whether a reading is one this file runs itself: a routine that takes values, or a
// built-in skill, which always takes one (internal/skills), so nothing with a value runs unread.
func (r lineReading) takesValues() bool {
	return (r.Kind == "routine" && (r.Needs || r.Times > 0)) || r.Kind == "skill"
}

// lineRunArgs is the invocation a runnable reading becomes: its identity, its version and its
// values, and nothing read from words.
//
// Reading the words again here must fail TestAReadBackRunsAsItsIdentityAndVersion.
func lineRunArgs(r lineReading) []string {
	if r.Kind == "skill" {
		// A BUILT-IN SKILL RUNS BY ITS IDENTITY too, with its one value; it has no version.
		args := []string{"do", "--source=typed", "--skill=" + r.Slug}
		for k, v := range r.Values {
			args = append(args, "--arg="+k+"="+v)
		}
		return args
	}
	args := []string{"do", "--source=typed", "--play=" + r.Slug}
	if r.App != "" {
		args = append(args, "--app="+r.App)
	}
	if r.Focus {
		args = append(args, "--focus")
	}
	args = append(args, "--expect="+r.SourceAt)
	if r.Times > 0 {
		args = append(args, "--times="+strconv.Itoa(r.Times))
	}
	keys := make([]string, 0, len(r.Values))
	for k := range r.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--arg="+k+"="+r.Values[k])
	}
	return args
}

// pollLineReading keeps the reading of the line being typed current, off the hook thread and the
// action loop, once the line has stopped changing.
func pollLineReading(m *model) {
	last, settled := "", false
	for {
		time.Sleep(150 * time.Millisecond)
		in, editing := m.inputForHint()
		in = strings.TrimSpace(in)
		if !editing || in == "" {
			last, settled = "", false
			continue
		}
		if in != last {
			last, settled = in, false
			continue
		}
		if settled {
			continue
		}
		settled = true
		refreshReading(m, in)
	}
}

// refreshReading asks for the reading of line and shows it, if the line is still that line.
func refreshReading(m *model, line string) {
	r, ok := askLine(line, m.lineProposal())
	if !ok {
		r = lineReading{Line: line, Kind: "none"}
	}
	m.setReading(line, r)
	if r.Resolving {
		go resolveReading(m, line)
	}
}

// typedLine is what Enter does with a typed line. It reports whether this file took the line;
// false means it goes to the intake as words, as before.
//
// Running a line whose read-back was not on screen must fail TestNothingWithAValueRunsUnread.
func typedLine(h *model, line string) bool { return typedLineFrom(h, line, false) }

// typedLineFrom is typedLine for a line typed or spoken, which it also keeps for ↑ (recall.go): a
// routine by its name and nothing it was given, anything else typed as typed, and a spoken line
// only when it named a routine.
func typedLineFrom(h *model, line string, spoken bool) bool {
	r, shown := h.readingFor(line)
	if !shown {
		got, ok := askLine(line, h.lineProposal())
		if !ok {
			if !spoken {
				h.remember(line) // replaced by the routine it ran as, if it ran as one (intake.go)
			}
			return false
		}
		r = got
	}
	if r.Kind == "routine" || r.Kind == "skill" || !spoken {
		h.remember(recallOf(line, r))
	}
	// WHILE DIRECTOR'S VALUES ARE ON THE LINE, ONLY THEIR ROUTINE RUNS FROM IT. A line edited to
	// name another routine (a namesake in another scope above all) would have its reading drop
	// the proposals and run those values as if they had been typed. Refused until the line is
	// dismissed, which drops the proposal.
	//
	// Running such a line must fail TestALineEditedAwayFromItsProposalRunsNothing.
	if p := h.lineProposal(); p != nil && !sameRoute(r, p) {
		why := "this line is not " + routeWords(p.app, p.slug, p.focus) + ", the routine Director " +
			"proposed these values for, so nothing was run — Esc clears it"
		h.setReading(line, lineReading{Line: line, Kind: "routine", Needs: true, Says: why})
		reopenLine(h, line)
		h.setStatus(why)
		return true
	}
	if !r.takesValues() {
		h.clearProposal()
		return false
	}
	// FOCUS APP RUNS ON THE FIRST ENTER, typed or spoken: it changes nothing but which window is in
	// front, and a second Enter to confirm that was the owner's complaint. With its window found
	// it raises that window; while it is still being found, the run works it out the same way
	// (cmd/marco skill.go). The history names the program it brought forward. A line that cannot
	// run (no app, two programs, nothing open) still stays open and says why.
	//
	// Asking for the read-back first must fail TestFocusRunsOnTheFirstEnter.
	if r.Kind == "skill" && r.Slug == focusSkill && (r.Runnable || r.Resolving) {
		h.clearProposal()
		args := lineRunArgs(r)
		mlogI("overlay: running the read-back", "id", r.ID, "args", len(args))
		h.setStatus("running: " + capitalisedName(r.Name))
		kept := recallOf(line, r)
		go func() {
			res := spawnIntakeChild(h, capitalisedName(r.Name), true, args...)
			if prog, ok := focusRan(res); ok {
				h.rememberAsRoutine(kept, "focus "+prog)
			}
		}()
		return true
	}
	if !r.Runnable || !shown {
		// NOTHING RUNS: the line stays open on what was typed, with what it needs or with
		// the read-back it now has, and the next Enter is the one that runs it.
		h.setReading(line, r)
		reopenLine(h, line)
		if r.Runnable {
			h.setStatus("Enter runs it")
		} else {
			h.setStatus(r.Says)
		}
		return true
	}
	h.clearProposal()
	args := lineRunArgs(r)
	// THE RUN IS TRACKED UNDER THE ROUTINE'S NAME, NEVER ITS VALUES. The tracked name is what
	// the history, the status line and the console log say when the child names no route,
	// which is exactly when it could not start; the read-back with the text in it stays where
	// it was shown, under the line.
	//
	// Tracking the read-back must fail TestASuppliedValueNeverReachesTheLog.
	label := capitalisedName(r.Name) + timesSuffix(r.Times)
	mlogI("overlay: running the read-back", "id", r.ID, "args", len(args))
	h.setStatus("running: " + label)
	go func() { _ = spawnIntakeChild(h, label, true, args...) }()
	return true
}

// reopenEditHook puts the controller back into command-line mode. Set by the Windows controller;
// a no-op elsewhere, and in a test.
var reopenEditHook = func() {}

// reopenLine opens the command line on text again, for the value to be typed in place.
func reopenLine(h *model, text string) {
	// A SPACE AFTER THE BADGE, the one a person types after the leader: a line the widget writes
	// for somebody reads "`m cloud pull", not "`mcloud pull". Every reader trims it.
	h.setInput(" " + strings.TrimSpace(text))
	h.setEditing(true)
	h.show(true)
	reopenEditHook()
}

// review is what reviewProposal did.
type review int

const (
	// plainOffer: no review; the routine is offered to run as before (it takes no values, or
	// nothing could read the line).
	plainOffer review = iota
	// reviewed: the command line is open on the proposal.
	reviewed
	// refusedReview: the line reads as ANOTHER routine of that name, so the proposal is not
	// put anywhere, nothing is offered, and the HUD says why.
	refusedReview
)

// sameRoute is whether a reading is of the routine p was proposed for: the WHOLE identity, which
// is the application, the scope and the name. A context routine and a focus routine can share
// the other two.
//
// Dropping the scope from this must fail TestAProposalForTheFocusRoutineIsNotPutOnItsNamesake.
func sameRoute(r lineReading, p *lineProposal) bool {
	return p != nil && r.App == p.app && r.Slug == p.slug && r.Focus == p.focus
}

// reviewProposal opens the command line on the routine Director named, with what it proposed
// written in, when that routine takes values.
//
// Forwarding the proposed values without this review must fail
// TestAProposedValueIsReviewedBeforeItRuns.
func reviewProposal(h *model, name, app, slug string, focus bool, values map[string]string, at string) review {
	return reviewProposalTimes(h, name, app, slug, focus, values, at, 0)
}

// reviewProposalTimes is reviewProposal for a routine asked for a number of times: the line it
// opens on is the explicit repeat, "repeat open social 5 times", read back as "Open social ×5".
//
// Opening the line without the count must fail TestAProposedRepeatIsReviewedWithItsCount.
func reviewProposalTimes(h *model, name, app, slug string, focus bool, values map[string]string,
	at string, times int) review {
	p := &lineProposal{app: app, slug: slug, focus: focus, values: values, at: at}
	line := name
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		line += " " + k + ": " + values[k] // a space after the colon, for reading; the parser takes either
	}
	if times > 0 {
		line = "repeat " + line + " " + strconv.Itoa(times) + " times"
	}
	r, ok := askLine(line, p)
	switch {
	case !ok:
		return plainOffer
	case r.Kind == "routine" && !sameRoute(r, p) && len(values) > 0:
		// THE LINE NAMES A NAMESAKE. Its reading would have dropped the proposals and shown
		// the values as if typed, so they are not put on the line at all.
		h.log(namesakeSays(name, r, p))
		return refusedReview
	case !sameRoute(r, p) || !r.takesValues():
		return plainOffer
	}
	h.setProposal(p)
	h.setReading(line, r)
	reopenLine(h, line)
	if r.Runnable {
		h.setStatus("Enter runs it · edit to change")
	} else {
		h.setStatus(r.Says)
	}
	return reviewed
}

// namesakeSays is why a proposal was not put on the command line.
func namesakeSays(name string, r lineReading, p *lineProposal) string {
	return "Director proposed values for " + routeWords(p.app, p.slug, p.focus) + ", and \"" + name +
		"\" here is " + routeWords(r.App, r.Slug, r.Focus) + ", a different routine, so nothing was " +
		"put on the command line and nothing was run"
}

// routeWords names a routine's whole identity for a sentence.
func routeWords(app, slug string, focus bool) string {
	switch {
	case app == "":
		return slug + " (everywhere)"
	case focus:
		return slug + " (from anywhere → " + app + ")"
	}
	return slug + " (only in " + app + ")"
}

// readDwell is how long a read-back must have been on screen before Enter runs it: one Enter
// pressed as the reading lands is not somebody having read it.
const readDwell = 300 * time.Millisecond

func (h *model) setReading(line string, r lineReading) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.TrimSpace(h.input) != line && h.editing {
		return // the line has moved on; this reading is for something no longer there
	}
	h.reading, h.readingLine, h.readingAt = r, line, time.Now()
}

// readingFor is the reading on screen for line, and whether it has been there long enough to
// have been read.
func (h *model) readingFor(line string) (lineReading, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.readingLine != line {
		return lineReading{}, false
	}
	return h.reading, time.Since(h.readingAt) >= readDwell
}

func (h *model) lineProposal() *lineProposal { h.mu.Lock(); defer h.mu.Unlock(); return h.proposal }
func (h *model) setProposal(p *lineProposal) { h.mu.Lock(); h.proposal = p; h.mu.Unlock() }
func (h *model) clearProposal()              { h.mu.Lock(); h.proposal = nil; h.mu.Unlock() }

// shownReading is the reading to draw under the line: the one for what is typed now, or none.
func (h *model) shownReadingLocked() (lineReading, bool) {
	if !h.editing || h.readingLine == "" || strings.TrimSpace(h.input) != h.readingLine {
		return lineReading{}, false
	}
	return h.reading, true
}

// readingRuns is the read-back under the line, coloured, and never by colour alone: each value is
// behind its label, and a line that cannot run says why in words.
func readingRuns(s snapshot) []editRun {
	if !s.editing || !s.hasReading {
		return nil
	}
	r := s.reading
	if !r.takesValues() {
		return nil
	}
	if !r.Runnable {
		return []editRun{{r.Says, th.idle}}
	}
	rs := []editRun{{capitalisedName(r.Name), th.run}}
	if r.Times > 0 {
		rs = append(rs, editRun{timesSuffix(r.Times), th.run})
	}
	for _, p := range r.Parts {
		rs = append(rs, editRun{" · " + p.Label + ": ", th.desc})
		switch {
		case p.Secret:
			rs = append(rs, editRun{p.Said, th.lastCmd})
			continue
		case p.Value == "":
			rs = append(rs, editRun{"nothing", th.name})
		default:
			rs = append(rs, editRun{p.Value, th.name})
		}
		if p.Said != "" {
			rs = append(rs, editRun{" " + p.Said, th.lastCmd})
		}
	}
	return append(rs, editRun{"   Enter runs it", th.lastCmd})
}

func capitalisedName(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// timesSuffix is " ×5" for a routine asked for five times, and nothing for once.
func timesSuffix(n int) string {
	if n <= 0 {
		return ""
	}
	return " ×" + strconv.Itoa(n)
}

// askResolved reads a line again with its skill's target worked out (`marco line --resolve`),
// bounded like the ask about which routine a phrase named, because it may wait on a model. A
// package variable so a test can answer for it; production never reassigns it.
var askResolved = func(line string) (lineReading, bool) {
	return readLineChildWith(line, nil, resolveWithin, "--resolve")
}

// resolveReading replaces a reading that is still finding its target with the one that found it
// (or says it could not), if the line is still that line. One at a time per line.
//
// Not asking again must fail TestAReadBackShowsTheProgramOnceItIsFound.
func resolveReading(m *model, line string) {
	m.mu.Lock()
	if m.resolvingFor == line {
		m.mu.Unlock()
		return
	}
	m.resolvingFor = line
	m.mu.Unlock()
	defer func() { m.mu.Lock(); m.resolvingFor = ""; m.mu.Unlock() }()
	if r, ok := askResolved(line); ok {
		m.setReading(line, r)
	}
}
