package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// The demonstration commands.
//
//	director demonstrate start|stop|abandon|status
//	director demonstrations
//	director demonstration <id>
//	director extract <id> [--approve]
//	director explain procedure <name|id>
//
// All of them go through the daemon, because the daemon is what recorded the session:
// the recorder subscribes to the outcomes of requests, and those happen in the service.
// A client that read the store directly could list and show, but could not tell the
// recorder to start — so all of it goes one way, and there is one answer to "what is
// being recorded?".
//
// Every one of these is read-only with respect to the DESKTOP. None of them can move a
// mouse, and `director demonstrate stop` is answerable while the command it recorded is
// still running.

// runDemonstrate is `director demonstrate <action>`.
func runDemonstrate(args []string) int {
	fs := flag.NewFlagSet("demonstrate", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	reason := fs.String("reason", "", "why a session was abandoned")
	_ = fs.Parse(flagsFirst(args))

	action := service.DemoActive
	if fs.NArg() > 0 {
		switch strings.ToLower(fs.Arg(0)) {
		case "start", "begin", "record":
			action = service.DemoStart
		case "stop", "end", "finish":
			action = service.DemoStop
		case "abandon", "cancel", "discard":
			action = service.DemoAbandon
		case "status", "active":
			action = service.DemoActive
		default:
			fmt.Fprintf(os.Stderr,
				"director: %q is not a demonstration action (start, stop, abandon, status)\n",
				fs.Arg(0))
			return 2
		}
	}

	out, code := demoRequest(service.DemonstrationPayload{Action: action, Reason: *reason})
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(out)
	}
	switch {
	case out.Recording != nil && action == service.DemoStart:
		fmt.Printf("Recording %s.\n\n%s\n", out.Recording.ID, wrap(out.Message))
	case out.Demonstration != nil:
		fmt.Println(renderDemonstration(out.Demonstration))
		if out.Message != "" {
			fmt.Printf("\n%s\n", wrap(out.Message))
		}
	case out.Recording != nil:
		fmt.Println(renderDemonstration(out.Recording))
	default:
		fmt.Println(out.Message)
	}
	return 0
}

// runDemonstrations is `director demonstrations`.
func runDemonstrations(args []string) int {
	fs := flag.NewFlagSet("demonstrations", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	out, code := demoRequest(service.DemonstrationPayload{Action: service.DemoList})
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(out)
	}
	if out.Recording != nil {
		fmt.Printf("Recording now: %s (%d step(s))\n\n",
			out.Recording.ID, len(out.Recording.Steps))
	}
	if len(out.Demonstrations) == 0 {
		fmt.Println("Nothing has been demonstrated yet.")
		fmt.Println("  director demonstrate start   then perform the task once")
		return 0
	}
	fmt.Printf("%-24s  %-10s  %-5s  %-12s  %s\n",
		"ID", "STATUS", "STEPS", "APPLICATION", "STARTED")
	for _, d := range out.Demonstrations {
		app := d.Application
		if app == "" {
			app = "(several)"
		}
		fmt.Printf("%-24s  %-10s  %-5d  %-12s  %s\n",
			d.ID, d.Status, len(d.Steps), truncate(app, 12),
			d.Started.Local().Format("2006-01-02 15:04"))
	}
	return 0
}

// runDemonstration is `director demonstration <id>`.
func runDemonstration(args []string) int {
	fs := flag.NewFlagSet("demonstration", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: director demonstration <id>")
		return 2
	}
	out, code := demoRequest(service.DemonstrationPayload{
		Action: service.DemoShow, ID: fs.Arg(0),
	})
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(out)
	}
	fmt.Println(renderDemonstration(out.Demonstration))
	return 0
}

// runExtract is `director extract <id> [--approve]`.
//
//	The extractor proposes. The user approves.
//
// So --approve is a separate flag on a command that otherwise only shows. Running it
// without the flag is the review step, and it installs nothing.
func runExtract(args []string) int {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	approve := fs.Bool("approve", false, "install the proposed procedure")
	why := fs.Bool("why", false, "show the reason behind every extraction decision")
	_ = fs.Parse(flagsFirst(args))

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: director extract <demonstration-id> [--approve]")
		return 2
	}
	id := fs.Arg(0)

	if *approve {
		out, code := demoRequest(service.DemonstrationPayload{
			Action: service.DemoApprove, ID: id,
		})
		if code != 0 {
			return code
		}
		if *jsonOut {
			return printJSON(out)
		}
		fmt.Printf("%s\n", wrap(out.Message))
		return 0
	}

	out, code := demoRequest(service.DemonstrationPayload{Action: service.DemoExtract, ID: id})
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(out)
	}
	ex := out.Extraction
	if *why {
		// The whole account, in the order the extractor worked: what it saw, what it
		// recovered, what it generalised, what validation made of it, and what it did NOT
		// install. Shown for a refusal too — that is when it is most wanted.
		shown, code := demoRequest(service.DemonstrationPayload{
			Action: service.DemoShow, ID: id,
		})
		if code != 0 {
			return code
		}
		if ex != nil {
			fmt.Print(ex.Trace(shown.Demonstration))
		}
	}
	if ex == nil || ex.Candidate == nil {
		fmt.Printf("Nothing can be learned from %s.\n\n%s\n", id, wrap(refusalOf(ex)))
		return 1
	}
	if !*why {
		fmt.Print(ex.Candidate.Describe())
	}
	fmt.Printf("\nNothing has been installed. To accept it:\n  director extract %s --approve\n", id)
	return 0
}

// runExplainProcedure is `director explain procedure <name|id>`.
//
//	Why is this parameter? Why wasn't this parameterized? Why is Rename inferred?
//	Why wasn't this demonstration accepted? Why are these steps constants?
func runExplainProcedure(args []string) int {
	fs := flag.NewFlagSet("explain procedure", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr,
			"usage: director explain procedure <learned-procedure-name|demonstration-id>")
		return 2
	}
	subject := strings.Join(fs.Args(), " ")

	// A demonstration id or a procedure name — decided by SHAPE rather than by asking
	// the user which they meant. Demonstration ids are minted with a known prefix, and
	// nothing else is one.
	p := service.DemonstrationPayload{Action: service.DemoExplain}
	if strings.HasPrefix(subject, "demo-") {
		p.ID = subject
	} else {
		p.Name = subject
	}

	out, code := demoRequest(p)
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(out)
	}
	if out.Explanation == nil {
		fmt.Println("There is no account of that.")
		return 1
	}
	fmt.Print(out.Explanation.Describe())
	return 0
}

// ── rendering ─────────────────────────────────────────────────────────────────

// renderDemonstration draws one recorded session.
//
// The SEMANTIC content, because that is all there is: a step is a verb, a semantic
// target and what verification made of it. There is nothing mechanical to omit.
func renderDemonstration(d *demo.Demonstration) string {
	if d == nil {
		return "(no demonstration)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.ID)
	fmt.Fprintf(&b, "  status       %s\n", d.Status)
	if d.Application != "" {
		fmt.Fprintf(&b, "  application  %s\n", d.Application)
	}
	fmt.Fprintf(&b, "  started      %s\n", d.Started.Local().Format("2006-01-02 15:04:05"))
	if !d.Completed.IsZero() {
		fmt.Fprintf(&b, "  ended        %s\n", d.Completed.Local().Format("15:04:05"))
	}
	if len(d.Requests) > 0 {
		b.WriteString("\nAsked for\n")
		for _, r := range d.Requests {
			fmt.Fprintf(&b, "  %q\n", r)
		}
	}
	if len(d.Steps) > 0 {
		b.WriteString("\nSemantic steps\n")
		for _, s := range d.Steps {
			mark := "✓"
			if !s.Verified {
				mark = "✗"
			}
			fmt.Fprintf(&b, "  %s %d %-46s %s\n", mark, s.Index, s.Describe(), s.Node)
			for _, e := range s.Evidence {
				fmt.Fprintf(&b, "       %s\n", e)
			}
		}
	}
	if len(d.Notes) > 0 {
		b.WriteString("\nNotes\n")
		for _, n := range d.Notes {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	if d.Refusal != "" {
		fmt.Fprintf(&b, "\n%s\n", wrap(d.Refusal))
	}
	return b.String()
}

// refusalOf is why an extraction produced nothing.
func refusalOf(ex *demo.Extraction) string {
	if ex == nil || ex.Refusal == "" {
		return "no reason was recorded, which is itself a bug"
	}
	return ex.Refusal
}

// demoRequest sends one demonstration request to the daemon.
//
// It does NOT start the service. Recording is a conversation with a running Director, and
// a `demonstrate start` that silently launched one would open a session over a desktop
// nobody had been observing.
func demoRequest(p service.DemonstrationPayload) (service.DemonstrationResponse, int) {
	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return service.DemonstrationResponse{}, 1
	}
	defer client.Close()

	out, err := client.Demonstration(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return service.DemonstrationResponse{}, 1
	}
	return out, 0
}

// wrap breaks a sentence at 78 columns, so a refusal reads as prose rather than as one
// long line a terminal folds mid-word.
func wrap(s string) string {
	const width = 78
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > width {
			b.WriteString(line + "\n")
			line = w
			continue
		}
		line += " " + w
	}
	b.WriteString(line)
	return b.String()
}

// ── saved sequences ───────────────────────────────────────────────────────────

// runSave is `director save "<name>" [<demonstration id>]`.
//
//	The user names and saves the sequence.
//
// The name comes first because it is the point: everything else about this command has a
// sensible default, and the name has none. With no id it saves the most recent completed
// demonstration, which is what somebody who just finished one means.
func runSave(args []string) int {
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(rest))
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr,
			`director: say what to call it, e.g. director save "my settings tour"`)
		return 2
	}
	id := ""
	if fs.NArg() > 0 {
		id = fs.Arg(0)
	} else {
		// THE MOST RECENT COMPLETED ONE. Asked for here rather than defaulted in the
		// service, because "the last demonstration" is a thing a person means and not a
		// thing a protocol should decide.
		list, code := demoRequest(service.DemonstrationPayload{Action: service.DemoList})
		if code != 0 {
			return code
		}
		for _, d := range list.Demonstrations {
			if d.Status == demo.Completed {
				id = string(d.ID)
				break
			}
		}
		if id == "" {
			fmt.Fprintln(os.Stderr,
				"director: nothing has been demonstrated yet. Record one with:\n"+
					"  director demonstrate start   then perform the task once")
			return 1
		}
	}

	out, code := demoRequest(service.DemonstrationPayload{
		Action: service.DemoSave, ID: id, Name: name,
	})
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(out)
	}
	if out.Sequence != nil {
		fmt.Println(renderSequence(*out.Sequence))
		fmt.Println()
	}
	fmt.Println(wrap(out.Message))
	return 0
}

// runSequences is `director sequences [--forget <name>]`.
func runSequences(args []string) int {
	fs := flag.NewFlagSet("sequences", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	forget := fs.String("forget", "", "remove the sequence with this name")
	_ = fs.Parse(flagsFirst(args))

	if *forget != "" {
		out, code := demoRequest(service.DemonstrationPayload{
			Action: service.DemoDrop, Name: *forget,
		})
		if code != 0 {
			return code
		}
		fmt.Println(wrap(out.Message))
		return 0
	}
	// ONE NAMED, shown in full: a person asking about a routine that is about to drive
	// their desktop wants its steps, not a row in a table.
	if fs.NArg() > 0 {
		want := strings.Join(fs.Args(), " ")
		list, code := savedSequences()
		if code != 0 {
			return code
		}
		for _, a := range list {
			if strings.EqualFold(a.Name, want) {
				if *jsonOut {
					return printJSON(a)
				}
				fmt.Println(renderSequence(a))
				return 0
			}
		}
		fmt.Fprintf(os.Stderr, "director: nothing called %q has been saved\n", want)
		return 1
	}

	list, code := savedSequences()
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(list)
	}
	if len(list) == 0 {
		fmt.Println("Nothing has been saved as a named sequence yet.")
		fmt.Println("  from the accessibility tree:")
		fmt.Println("    director demonstrate start    then perform the task once")
		fmt.Println("    director demonstrate stop")
		fmt.Println(`    director save "my settings tour"`)
		fmt.Println("  from PIXELS, with your own hands:")
		fmt.Println(`    director demonstrate-visual "enter free play" --application X`)
		return 0
	}
	fmt.Printf("%-28s  %-5s  %s\n", "NAME", "STEPS", "SAVED")
	for _, a := range list {
		fmt.Printf("%-28s  %-5d  %s\n",
			truncate(a.Name, 28), len(a.Steps), a.SavedAt.Local().Format("2006-01-02 15:04"))
	}
	// ONLY A ROUTINE THAT COULD RUN IS OFFERED AS ONE. A pixel routine has no verbs, and
	// pointing somebody at `execute` for it is a confusing failure rather than a feature.
	// See runnable.
	for _, a := range list {
		if runnable(a) {
			fmt.Printf("\nRun one with: director execute \"run %s\"\n", a.Name)
			return 0
		}
	}
	// NONE RUNNABLE BY VERB IS NOT NONE RUNNABLE AT ALL. See howToRun: a pixel routine is
	// run by sight, and this used to say it could not be run.
	fmt.Printf("\nNone of these carry verbs for the ordinary execution path. What the\n")
	fmt.Printf("first one can do instead:\n")
	for _, line := range howToRun(list[0]) {
		fmt.Printf("  %s\n", line)
	}
	return 0
}

// renderSequence is a saved routine, step by step, as a person reads it.
//
// What it is aimed at, in the artifact's own order of preference: the role it was named by, the
// label it bore, or one of the three ways a step addresses a control without naming it. A reader
// deciding whether to run this needs to see which, because a role survives a change of language
// and a label does not.
func renderSequence(a workflowstore.Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", a.Name)
	fmt.Fprintf(&b, "  saved %s\n", a.SavedAt.Local().Format("2006-01-02 15:04"))
	for _, n := range a.Notes {
		fmt.Fprintf(&b, "  %s\n", n)
	}
	byLabel := map[string]workflowstore.Control{}
	for _, c := range a.Controls {
		byLabel[c.Label] = c
	}
	fmt.Fprintf(&b, "\nSteps\n")
	for _, st := range a.Steps {
		c := byLabel[st.Control]
		// WHAT IT IS AIMED AT, in the artifact's own order of preference — and a step
		// with only VISUAL evidence says so.
		//
		// "the focused control" was the fallback for all of them, which on a pixel
		// routine is a plain lie: a control with readings and no name was not aimed at
		// focus, it was aimed at a shape the proposer found. The two are the opposite
		// claims — one is resolved at run time by asking what has focus, the other by
		// comparing rectangles — and a person deciding whether to trust a routine has
		// to see which.
		what := "the focused control"
		switch {
		case c.Editor:
			what = "the editor the previous step opened"
		case c.Deictic:
			what = "the object the person pointed at"
		case c.Role != "":
			what = fmt.Sprintf("%s (by role)", goal.ControlRole(c.Role).Describe())
		case len(c.Text) > 0:
			what = fmt.Sprintf("%q (by name)", c.Text[0])
		case len(c.Readings) > 0:
			what = fmt.Sprintf("%s — a shape on screen, by sight only", c.Label)
		}
		// AND THE VERB, which a pixel step has none of: a frame recording observes a
		// press and a screen change and cannot know what the person meant by pressing.
		verb := st.Verb
		if verb == "" {
			verb = "go from " + st.From + " to " + st.To + ", pressing"
			if st.Control == "" {
				verb, what = "go from "+st.From+" to "+st.To, "(nothing was pressed)"
			}
		}
		line := fmt.Sprintf("  %d %s %s", st.Position+1, verb, what)
		if st.Value != "" {
			line += fmt.Sprintf(" = %q", st.Value)
		}
		if st.Seen > 1 {
			line += fmt.Sprintf("   [addressed %d times]", st.Seen)
		}
		fmt.Fprintln(&b, line)
	}
	if !runnable(a) {
		fmt.Fprintf(&b, "\n  Its steps carry visual evidence and no verb, so the ordinary\n")
		fmt.Fprintf(&b, "  execution path has nothing to act on. What it can do instead:\n")
		for _, line := range howToRun(a) {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	return b.String()
}

// runnable reports whether the ordinary execution path could do anything with this routine.
//
// A routine learned from the accessibility tree carries a verb and a role or a name per step,
// and `director execute "run <name>"` runs it. One learned from PIXELS carries neither: a frame
// recording observes a press and a screen change and cannot know what the press MEANT.
//
// Asked rather than assumed, because the alternative was printing *Run one with: director
// execute "run enter free play"* under a routine that cannot run — which sends somebody to a
// confusing failure at step three of a walkthrough.
//
// ONE DEFINITION, on the artifact: the listing, the one door that decides what a typed name
// means, and the runner all need this answer, and three copies would drift. See
// `workflowstore.Artifact.RunnableByVerbs` and its sibling `PerformableBySight`.
//
// Enforced by TestAPixelRoutineIsNotOfferedAsRunnable.
func runnable(a workflowstore.Artifact) bool {
	return a.RunnableByVerbs()
}

// savedSequences is the saved routines, from the daemon when it is running and from the store
// when it is not.
//
// # Why this one request does not need the service
//
// Every other demonstration request does, and for a reason that does not apply here: the daemon
// is what RECORDS, so "start recording" and "what is being recorded" have exactly one answer and
// it lives there. A saved sequence is a file, read fresh from disk every time it is asked for —
// `Store.Sequence` opens it on every call and the daemon holds no copy — so a direct read and a
// request through the service are the same bytes.
//
// It matters because of the shape of the teaching flow. `director demonstrate-visual` and
// `director reacquire` both work without a daemon, so a person teaching Marco a game menu had to
// start a service for the one command in the middle that only lists files. That is the kind of
// seam that makes a walkthrough fail at step three.
//
// The daemon is still PREFERRED, so a future daemon that did hold state stays authoritative, and
// its absence is not reported as an error — a store nobody has written to is empty, which is the
// ordinary answer.
//
// Enforced by TestListingSavedSequencesWorksWithoutTheService.
func savedSequences() ([]workflowstore.Artifact, int) {
	if client, err := connect(false); err == nil {
		defer client.Close()
		if out, derr := client.Demonstration(service.DemonstrationPayload{
			Action: service.DemoSequences,
		}); derr == nil {
			return out.Sequences, 0
		}
	}
	store, err := demo.Open(configDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return nil, 1
	}
	return store.Sequences(), 0
}

// howToRun is what a person can actually do with a saved routine, in the order of what it
// carries. One statement of it, because the list and the single-routine view both need it and
// two copies would eventually disagree about which commands exist.
//
// # The defect this closes
//
// Both views said a pixel routine CANNOT BE RUN and sent the person to `reacquire`, which
// measures whether a control can be found again and presses nothing. That was true when it was
// written: `runnable` asks whether the ORDINARY execution path can act, and a pixel routine
// carries no verb for it to act on.
//
// It stopped being true when `perform-workflow` learned to ground a saved control from pixels,
// authorise it, press it and verify the outcome. Marco was telling people the thing it had just
// learned to do was impossible — and the three Ark routines on this machine, demonstrated by
// hand, were each reported unrunnable.
//
// # And an unscoped routine really cannot be run
//
// Routines saved before workflows recorded their application are UNSCOPED, which is valid for
// no application rather than for all of them. That is not a nearly-runnable routine and must
// not read like one: the honest answer is to demonstrate it again, because the application
// identity has to come from an observed window and nothing here can invent it.
func howToRun(a workflowstore.Artifact) []string {
	if runnable(a) {
		return []string{fmt.Sprintf("director execute %q", "run "+a.Name)}
	}
	if len(a.VisualTargets()) == 0 {
		return []string{
			"This routine carries neither a verb nor visual control evidence, so",
			"there is nothing to run and nothing to look for.",
		}
	}
	if !a.Scoped() {
		// AND `reacquire` STAYS HERE, because it is still true. Measuring whether
		// this routine's controls can be found again needs no application scope —
		// it looks and reports and presses nothing. The first version of this
		// dropped it along with the false claim, and dropped something real: an
		// unscoped routine cannot be RUN and can still be MEASURED.
		return []string{
			"THIS ROUTINE CANNOT BE RUN AS SAVED: it is UNSCOPED. It was saved before",
			"routines recorded which application they belong to, so it is readable and",
			"valid for no application — an application identity has to come from a",
			"window that was observed, and nothing here can invent one.",
			"",
			"Demonstrate it again to run it:",
			fmt.Sprintf("  director demonstrate-visual %q --application <app>", a.Name),
			"Or measure whether its controls are still findable, which presses nothing:",
			fmt.Sprintf("  director reacquire %q --application <app>", a.Name),
		}
	}
	out := []string{
		"Run it by sight — its controls are grounded in the pixels, authorised, and",
		"the outcome is checked against what the demonstration saw:",
		fmt.Sprintf("  director perform-workflow --name %q --live --reading", a.Name),
	}
	// AND WHETHER ANYTHING WILL CHECK IT. A step saved before outcomes were recorded can
	// be performed and cannot be verified, and a person about to drive their desktop
	// should be told which of the two they are getting.
	for _, st := range a.Steps {
		if len(st.Outcome) == 0 {
			out = append(out,
				"",
				"Step "+fmt.Sprint(st.Position+1)+" carries no outcome expectation, so",
				"its action can be performed and cannot be verified. Demonstrating it",
				"again records one.")
			break
		}
	}
	return out
}
