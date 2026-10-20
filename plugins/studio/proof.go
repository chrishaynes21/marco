package main

// THE HEADLESS HALF OF THE PROOF, WHICH IS WHERE THE EVIDENCE IS.
//
// Gate 2 of the feasibility assignment: *use the production structured event producer with a
// dry/instrumented effect host. Demonstrate Start, the canonical run reference, backend-driven
// countdown and phase, and acknowledged Cancel with zero automated input. Canned state and
// transcript parsing do not qualify.*
//
// So this starts a real Director — its own, `--dry-run`, in a temporary store — and drives it
// through the same `service.Client` the overlay uses. What it prints is what the window would
// render, taken from the same values by the same code: `Backend.sink` is the window's event
// emitter with a different destination.
//
// It opens no window, sends no input, and leaves nothing running.
//
// # What it cannot demonstrate, and says so
//
// A backend-driven COUNTDOWN. There is no countdown, deadline or remaining-time field anywhere in
// `pkg/playbill` or in `ProgressPayload`; `Doing` carries elapsed time (`RunningMS`) and nothing
// else temporal. The only countdown in the repository is a `time.Timer` inside
// `cmd/director/performworkflow.go`, printed once to stdout. Adding one is a field on the single
// read-only account of what Marco is doing — an ADR-level change with a subsystem note and an
// "Enforced by" test, per CLAUDE.md — and it is missing behind a browser exactly as much as
// behind a webview. This prints `countdown: NOT AVAILABLE` rather than dressing elapsed time up
// as one.
//
// SUPERVISED CONSENT KEYS, either. The consent desk belongs to `director perform-workflow`,
// which is the supervised runner a person authorises press by press with F7/F8/F9. The service's
// phrase path has no such prompt, so a passed gate here says nothing at all about whether a
// person can answer one from a Wails window — and the evidence says so on its own line rather
// than leaving a reader to notice the absence.

import (
	"fmt"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// runProof drives the real backend and prints what the gate asks for. Returns an exit code.
//
// The store is the isolated one `proofStore` makes, and the reason it is the SAME function the
// window uses is the containment defect: the headless proof had a temporary store from the start
// and the window did not, so the evidence was produced in isolation while the thing a person
// launches was not. One way to choose a store, used by both.
func runProof(phrase, explicitHome string) int {
	home, release, err := proofStore(explicitHome)
	if err != nil {
		fmt.Printf("FAIL  %v\n", err)
		return 1
	}
	defer release()

	var mu sync.Mutex
	var envelopes []Started
	var phases []Phase
	b := &Backend{dry: true, home: home}
	b.sink = func(kind string, v any) {
		mu.Lock()
		defer mu.Unlock()
		switch e := v.(type) {
		case Started:
			envelopes = append(envelopes, e)
			fmt.Printf("  envelope  %-24s %s %s\n", e.Type, short(e.CommandID), e.Detail)
		case Phase:
			phases = append(phases, e)
		}
	}
	defer b.Close()

	kind := "isolated and removed on exit; the owner's store and Director are untouched"
	if explicitHome != "" {
		// AND IT SAYS SO. This line read "temporary" unconditionally, which would have
		// described a store named on the command line as one nobody owns.
		kind = "NAMED ON THE COMMAND LINE — not an isolated one"
	}
	fmt.Printf("STUDIO BACKEND PROOF\n")
	fmt.Printf("  store        %s (%s)\n", home, kind)
	fmt.Printf("  director     %s serve --dry-run --quiet\n", directorBin())
	fmt.Printf("  phrase       %q, dry=true\n\n", phrase)

	// ── 1. START, and the run reference it is acknowledged with ──
	b.Start(phrase)

	// ── 2. WATCH THE PHASE WHILE IT RUNS, and cancel it in flight if it lasts ──
	//
	// The cancel is attempted as soon as the backend reports a phase that is doing
	// something, because an acknowledged cancel of an in-flight command is the strong
	// form of the evidence. If the command has already finished, the acknowledgement
	// comes back `accepted=false` and this says so rather than claiming otherwise.
	deadline := time.Now().Add(40 * time.Second)
	var cancelled, inFlight bool
	var ack service.CancelPayload
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		mu.Lock()
		p := lastPhase(phases)
		done := terminal(envelopes)
		mu.Unlock()
		if !cancelled && p.Phase != "" && p.Phase != "idle" && !done {
			ack, inFlight = b.Cancel(), true
			fmt.Printf("\n  CANCEL    accepted=%v id=%s %q\n\n",
				ack.Accepted, short(string(ack.CommandID)), ack.Message)
			cancelled = true
			continue
		}
		if done {
			break
		}
	}
	if !cancelled {
		// The command finished before a cancel could land on it. That is a real outcome
		// and it is NOT the one the gate asks for, so it is recorded as the attempt it
		// was rather than counted as an acknowledged cancellation.
		ack = b.Cancel()
		fmt.Printf("\n  CANCEL    accepted=%v id=%s %q  (the command had already ended)\n\n",
			ack.Accepted, short(string(ack.CommandID)), ack.Message)
	}

	// ── AND THE VERDICT, PER ELEMENT, WITH WHAT IT RESTED ON ──
	mu.Lock()
	defer mu.Unlock()
	ref := referenceOf(envelopes)
	stages := stagesOf(envelopes)
	p := richestPhase(phases)

	fmt.Printf("EVIDENCE\n")
	// AN ENVELOPE IS NOT AN ACKNOWLEDGEMENT, and this line used to count any of them. A run
	// that was refused a backend outright printed `start acknowledged YES` about the
	// refusal, which is the same failure as the hardcoded cancel below it.
	line("start acknowledged", acknowledged(envelopes),
		fmt.Sprintf("%d envelope(s) from the service, first %s", len(envelopes), firstType(envelopes)))
	line("canonical run reference", ref != "",
		fmt.Sprintf("AcceptedPayload.CommandID = %s", orDash(ref)))
	line("backend-driven phase", p.Phase != "",
		fmt.Sprintf("playbill Doing.Phase = %s, step %d/%d, elapsed %dms, over %d poll(s)",
			orDash(p.Phase), p.Step, p.Steps, p.RunningMS, len(phases)))
	line("structured progress stages", len(stages) > 0,
		fmt.Sprintf("ProgressPayload.Stage = %v", stages))
	// THE CANCEL LINE ASKS THE PAYLOAD, AND IT USED TO ASSERT ITSELF.
	//
	// It read `line("acknowledged cancel", true, …)` — a hardcoded YES about a request whose
	// answer was sitting in a variable three lines away. The Architect found it, and it is
	// the exact failure mode this product is built to refuse: evidence that reports what was
	// attempted as though it were what happened. A cancel of a command that had already
	// ended comes back `accepted=false`, which is the service being truthful, and the gate
	// is only met by an acknowledged cancellation of a command that was IN FLIGHT.
	line("acknowledged cancel", inFlight && ack.Accepted,
		fmt.Sprintf("in flight=%v, CancelPayload.Accepted=%v, for %s — %q",
			inFlight, ack.Accepted, orDash(short(string(ack.CommandID))), ack.Message))
	cancelMatched := string(ack.CommandID) != "" && string(ack.CommandID) == ref
	line("cancel applied to this run", cancelMatched,
		fmt.Sprintf("CancelPayload.CommandID %s vs the run reference %s",
			orDash(short(string(ack.CommandID))), orDash(short(ref))))
	fmt.Printf("  %-26s %-4s %s\n", "backend-driven countdown", "NO",
		"no countdown/deadline/remaining field exists in pkg/playbill or ProgressPayload; "+
			"elapsed time is not a countdown and is not reported as one")
	// AND WHAT THIS CANNOT SPEAK TO, SAID WHERE THE EVIDENCE IS READ.
	//
	// The service's phrase path is not the supervised runner. `director perform-workflow`
	// is what puts F7/F8/F9 to a person through the consent desk; an `EXECUTE_PHRASE`
	// command carries no such prompt. So nothing above is evidence about consent keys, and
	// a reader who assumed otherwise would take a passed gate for a supervised one.
	fmt.Printf("  %-26s %-4s %s\n", "consent-key compatibility", "N/A",
		"NOT ESTABLISHED by this proof: the service phrase path has no consent desk — that "+
			"is director perform-workflow's, and it needs a person's hands")
	line("zero automated input", true,
		"--dry-run swaps in runtime.DryRunHost; the server refuses a dry request against a "+
			"live Director (service/server.go:743-755)")
	line("no transcript parsing", true,
		"every value above came from a service envelope or the playbill; nothing ran a "+
			"command and read its stdout")

	if !acknowledged(envelopes) || ref == "" || p.Phase == "" || !inFlight || !ack.Accepted || !cancelMatched {
		fmt.Printf("\nVERDICT  INCOMPLETE — see the NO lines above\n")
		return 1
	}
	fmt.Printf("\nVERDICT  PASSED-MINUS-COUNTDOWN (and silent about consent keys)\n")
	return 0
}

func line(name string, ok bool, why string) {
	mark := "NO"
	if ok {
		mark = "YES"
	}
	fmt.Printf("  %-26s %-4s %s\n", name, mark, why)
}

func short(id string) string {
	if len(id) > 18 {
		return id[:18]
	}
	return id
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func firstType(es []Started) string {
	if len(es) == 0 {
		return "—"
	}
	return es[0].Type
}

func referenceOf(es []Started) string {
	for _, e := range es {
		if e.CommandID != "" {
			return e.CommandID
		}
	}
	return ""
}

func stagesOf(es []Started) []string {
	var out []string
	seen := map[string]bool{}
	for _, e := range es {
		if e.Type != "PROGRESS" || e.Detail == "" || seen[e.Detail] {
			continue
		}
		seen[e.Detail] = true
		out = append(out, e.Detail)
	}
	return out
}

func lastPhase(ps []Phase) Phase {
	if len(ps) == 0 {
		return Phase{}
	}
	return ps[len(ps)-1]
}

// richestPhase is the poll that saw the most, so a one-frame phase is not lost to a later idle.
func richestPhase(ps []Phase) Phase {
	var best Phase
	for _, p := range ps {
		if p.Phase == "" {
			continue
		}
		if best.Phase == "" || p.Steps > best.Steps || p.RunningMS > best.RunningMS {
			best = p
		}
	}
	return best
}

func terminal(es []Started) bool {
	for _, e := range es {
		switch e.Type {
		case "COMPLETED", "FAILED", "UNVERIFIED", "CANCELLED", "ERROR", "NO_BACKEND",
			"CLARIFICATION_REQUIRED":
			return true
		}
	}
	return false
}

// acknowledged is whether the service ACCEPTED a command, which is the only thing that counts
// as a start. NO_BACKEND and ERROR are envelopes this shell wrote about its own failure.
func acknowledged(es []Started) bool {
	for _, e := range es {
		if e.Type == "ACKNOWLEDGED" || e.Type == "ACCEPTED" {
			return true
		}
	}
	return false
}
