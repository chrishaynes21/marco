package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// runServe starts the long-lived Director.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	maxNodes := fs.Int("max-nodes", 4000, "node ceiling per snapshot")
	dryRun := fs.Bool("dry-run", false, "never perform real input (for development)")
	quiet := fs.Bool("quiet", false, "suppress the startup banner")
	_ = fs.Parse(args)

	// A MISSING BRIDGE COSTS YOU AN ACTOR, NOT THE DIRECTOR.
	//
	// This used to be `os.Stat` and `return 1`. A machine with sight, with text, with OS input
	// and with everything else in working order could not observe AT ALL, because one Actor's
	// provider binary had not been built — and it found out by the service refusing to start,
	// which is the moment least useful to the person and tells them nothing about what Marco
	// could still have done for them.
	//
	// So it boots, and it says what it lost. The bridge is spawned lazily, every accessibility
	// call fails honestly, `Provider.Available` answers false, and `director status` carries
	// the reason — see AccessibilityUnavailable. Nothing here has to defend against the
	// absence; it was always tolerated below this line, and the gate was the only thing that
	// was not.
	//
	// Turning this back into a gate must fail TestServingDoesNotRefuseToStartOverAMissingBridge;
	// the sentence it prints is held by TestADirectorWithNoAccessibilityBridgeSaysWhy.
	if why := bridgeUnavailable(*bridgeFlag); why != "" && !*quiet {
		fmt.Fprintf(os.Stderr, "director: the Accessibility Actor is unavailable — %s\n", why)
		fmt.Fprintf(os.Stderr,
			"         starting anyway: sight, text and OS input are unaffected\n")
	}

	rt, err := NewRuntime(*bridgeFlag, *maxNodes, *dryRun, graph)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	defer rt.Close()

	srv := service.NewServer(service.Config{Dir: configDir(), Runtime: rt})
	ep, err := srv.Listen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	if !*quiet {
		fmt.Printf("Director service listening on %s (pid %d)\n", ep.Address, os.Getpid())
		fmt.Printf("  graph      %s (%d nodes)\n", graph.Path(), graph.Len())
		fmt.Printf("  endpoint   %s\n", service.EndpointPath(configDir()))
		if *dryRun {
			fmt.Printf("  DRY RUN    no real input will be performed\n")
		}
	}

	// Attach the accessibility client immediately rather than on first use.
	// Chromium exposes its interior only after sustained client presence, so the
	// service that waits until someone asks hands the first request a cold tree.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rt.Warm(ctx)
	}()

	// Ctrl-C and a SHUTDOWN request both end the service the same way.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		select {
		case <-sig:
			if !*quiet {
				fmt.Println("\ndirector: shutting down")
			}
			srv.Shutdown()
		case <-srv.Done():
		}
	}()

	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	return 0
}

// ── client-side helpers ───────────────────────────────────────────────────────

// connect finds the running service, starting one if needed.
func connect(autoStart bool) (*service.Client, error) {
	self, err := os.Executable()
	if err != nil {
		self = "director.exe"
	}
	return service.Connect(service.ConnectOptions{
		Dir:          configDir(),
		ServiceBin:   self,
		ServiceArgs:  []string{"serve", "--quiet"},
		AutoStart:    autoStart,
		StartTimeout: 20 * time.Second,
		DialTimeout:  2 * time.Second,
	})
}

// runStatus reports what the service is doing.
func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(args)

	c, err := connect(*start)
	if err != nil {
		if *jsonOut {
			fmt.Println(`{"running":false}`)
		} else {
			fmt.Println("Director: not running")
			fmt.Println("  start it with: director serve")
		}
		return 1
	}
	defer c.Close()

	st, err := c.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(st)
	}
	printStatus(st)
	return 0
}

// printStatus renders the service's self-report.
//
// Element COUNTS only, never labels or content: a status command must be safe to
// paste into a bug report, and the user's screen is their business.
func printStatus(st service.StatusPayload) {
	fmt.Printf("Director: running\n")
	fmt.Printf("PID: %d\n", st.PID)
	fmt.Printf("Uptime: %s\n", st.UptimeStr)

	if st.Active != nil {
		fmt.Printf("Active command: %s\n", st.Active.Phrase)
		if st.Active.Total > 0 {
			fmt.Printf("Iteration: %d/%d\n", st.Active.Iteration, st.Active.Total)
		} else if st.Active.Iteration > 0 {
			fmt.Printf("Iteration: %d\n", st.Active.Iteration)
		}
		fmt.Printf("Running for: %s\n", st.Active.Running.Round(time.Second))
	} else {
		fmt.Printf("Active command: none\n")
	}

	// WHY THERE ARE NONE, when there are none for a reason.
	//
	// "Accessibility clients: 0" has two causes that read identically — nothing has been
	// observed yet, or there is nothing to observe through — and the Director boots without a
	// bridge now rather than refusing to start, so the second is a state somebody can actually
	// be in. Printed before the count, because it explains it.
	if st.AccessibilityUnavailable != "" {
		fmt.Printf("Accessibility Actor: unavailable — %s\n", st.AccessibilityUnavailable)
	}
	fmt.Printf("Accessibility clients: %d\n", len(st.Providers))
	for _, p := range st.Providers {
		switch p.Status {
		case "unobservable":
			fmt.Printf("  %s provider: unobservable (%d elements, nothing operable)\n", p.App, p.Elements)
		case "shallow":
			fmt.Printf("  %s provider: shallow (%d elements, interior not exposed)\n", p.App, p.Elements)
		case "stale":
			fmt.Printf("  %s provider: stale (last seen %s ago)\n", p.App, time.Since(p.LastSeen).Round(time.Second))
		default:
			fmt.Printf("  %s provider: ready, %d elements (%d actionable, stable %s, %d observations)\n",
				p.App, p.Elements, p.Actionable, p.StableForStr, p.Observations)
		}
	}

	fmt.Printf("Action graph: %d nodes\n", st.GraphNodes)

	// The running or paused program's captured values, as METADATA. Names, types,
	// visibilities and lengths — and a preview only for the public ones, which the
	// snapshot decided, not this renderer.
	//
	// Absent entirely when nothing is running, which is the common case: an idle status
	// says "Active values: 0" and stops, rather than printing an empty section that
	// implies something is going on.
	if st.Values != nil {
		fmt.Println()
		fmt.Print(renderActiveValues(*st.Values))
	} else {
		fmt.Printf("Active values: 0\n")
	}

	// The same program's collections, as PROGRESS. How many of a bounded set have been
	// verified is the one thing a second process cannot otherwise discover about a long
	// iteration — without it a command working steadily through forty items is
	// indistinguishable from one that is stuck.
	//
	// Silent when nothing holds one. The server sends nil rather than an empty snapshot
	// for exactly this reason, so an idle status says nothing about collections instead
	// of printing a heading that implies an iteration is under way.
	if st.Collections != nil && len(st.Collections.Collections) > 0 {
		fmt.Println()
		fmt.Print(renderActiveCollections(*st.Collections))
	}

	// A paused clarification is the most actionable thing status can report: the
	// Director is waiting on the USER, and until this was shown there was no way to
	// discover that from a second process — the command simply looked idle.
	if q := st.Clarification; q != nil {
		fmt.Println()
		if q.Program() {
			fmt.Printf("Waiting for an answer — program %s, step %d of %d",
				firstNonBlank(q.ProgramID, "?"), q.StepIndex, q.StepCount)
			if q.CompletedSteps > 0 {
				fmt.Printf(" (%d verified)", q.CompletedSteps)
			}
			fmt.Println()
		} else {
			fmt.Println("Waiting for an answer")
		}
		fmt.Printf("  %s\n", q.Question)
		for _, c := range q.Candidates {
			fmt.Printf("    %d. %s", c.Index, firstNonBlank(c.Label, "(unlabelled)"))
			if c.Role != "" {
				fmt.Printf(" — %s", c.Role)
			}
			fmt.Println()
		}
		fmt.Printf("  %d choice(s), asked %s ago\n",
			len(q.Candidates), time.Since(q.AskedAt).Round(time.Second))
		fmt.Println()
	}

	if st.Conversation.LastPhrase != "" {
		fmt.Printf("Last phrase: %q\n", st.Conversation.LastPhrase)
	}
	if len(st.Recent) > 0 {
		fmt.Printf("Recent commands:\n")
		for _, r := range st.Recent {
			fmt.Printf("  %-10s %-28s %d action(s)\n", r.State, truncate(r.Phrase, 28), r.CompletedActions)
		}
	}
}

// runStop cancels the active command through the service.
func runStop(args []string) int {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(args)

	// Never auto-start: "stop" asking a service to exist so it can be told to stop
	// would be absurd, and would also start a Director the user did not ask for.
	c, err := connect(false)
	if err != nil {
		if *jsonOut {
			fmt.Println(`{"accepted":false,"message":"the Director is not running"}`)
		} else {
			fmt.Println("nothing to stop — the Director is not running")
		}
		return 0
	}
	defer c.Close()

	res, err := c.Cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(res)
	}
	fmt.Println(res.Message)
	return 0
}

// runShutdown stops the service itself, and does not say so until the process is gone.
//
// # What this used to claim, and why it was worse than a crash
//
// It sent the request, tolerated the connection error a service closing its socket produces, and
// printed "Director service stopped". Both of those are true of a service that exited AND of one
// whose socket went away while the process kept running — and the second case is invisible: the
// operator believes Marco is gone, starts another, and now two Directors hold global low-level
// input hooks. That is not a hypothetical. It happened twice in one session and doubled the
// desktop's input latency for half an hour before anybody looked.
//
// A silent-but-alive service is the worst outcome available here, so it is the one this reports.
//
// # The identity used
//
// The PID the running service reports for ITSELF, read before the request is sent, and checked
// afterwards with the same liveness call windowref uses to decide whether a window's process still
// exists. Not a name match: `director.exe` matches every Director, including the one the operator
// is deliberately running, and killing or reporting on those is not this command's business.
//
// Deleting the wait must fail TestShutdownDoesNotClaimSuccessWhileTheProcessLives.
func runShutdown(args []string) int {
	jsonOut := flag.NewFlagSet("shutdown", flag.ExitOnError)
	asJSON := jsonOut.Bool("json", false, "print as JSON")
	_ = jsonOut.Parse(args)

	c, err := connect(false)
	if err != nil {
		fmt.Println("the Director is not running")
		return 0
	}
	// The service's own account of which process it is, taken while it can still answer.
	pid := servicePID(c)
	if err := c.Shutdown(); err != nil {
		// A service that closes the connection as it exits is expected, not an error.
		if !strings.Contains(err.Error(), "reading a response") {
			c.Close()
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
	}
	c.Close()

	switch gone, why := waitForExit(pid, shutdownGrace); {
	case gone && *asJSON:
		fmt.Printf("{\"stopped\":true,\"pid\":%d}\n", pid)
	case gone:
		fmt.Println("Director service stopped")
	default:
		// NAMED, and a failure exit code. An operator who is told this can act on it; one
		// told "stopped" starts a second Director on top of the first.
		fmt.Fprintf(os.Stderr,
			"director: the shutdown request was accepted but %s\n"+
				"The service process may still be holding global input hooks. "+
				"Check with `director status`.\n", why)
		return 1
	}
	return 0
}

// shutdownGrace is how long a service is given to actually exit.
//
// Generous on purpose: it has an observation session to unwind, hooks to remove from the thread
// that installed them, and a memory file it may be writing. Reporting a failure early would send
// somebody looking for a problem that was about to resolve itself.
const shutdownGrace = 8 * time.Second

// servicePID is which process the running service says it is, or 0 if it will not say.
func servicePID(c *service.Client) int {
	st, err := c.Status()
	if err != nil {
		return 0
	}
	return st.PID
}

// waitForExit blocks until the named process is gone, or the grace runs out.
//
// Returns the reason when it does not, because "I could not tell" and "it is still running" are
// different things to be told and lead somewhere different.
func waitForExit(pid int, grace time.Duration) (bool, string) {
	if pid <= 0 {
		return false, "the service did not say which process it was, so its exit could not " +
			"be confirmed"
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !winctx.ProcessAlive(uint32(pid)) {
			return true, ""
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false, fmt.Sprintf("process %d is still running after %s", pid, grace)
}
