package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
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
