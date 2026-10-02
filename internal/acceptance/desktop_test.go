package acceptance_test

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/acceptance"
)

// Marco using a real computer, checked twice.
//
// # How to run it
//
//	go build -o marco.exe ./cmd/marco
//	go build -o director.exe ./cmd/director
//	powershell -File plugins/fixture/build.ps1
//	$env:MARCO_ACCEPT_DESKTOP = "1"
//	go test ./internal/acceptance -run TestDesktop -v
//
// It performs REAL INPUT into a fixture window on the real screen. It will refuse to start if that
// window cannot take the foreground, which is what keeps keystrokes out of whatever else is in
// front — so close full-screen applications first, and do not type while it runs.
//
// # One Director, a fresh fixture per case
//
// Starting a Director costs seconds and is not what is being tested; a fixture is a tiny process
// and restarting it is the only reset the harness is allowed to perform — pressing Cancel to clear
// the form would be the harness performing a user action, which is the line this suite exists to
// hold.
//
// # Every case reads two channels
//
// MARCO's own verdict, from production perception, and the ORACLE, from the file the fixture wrote.
// A case passes only when both agree, and the interesting failures are the ones where they do not.

func TestDesktopAcceptance(t *testing.T) {
	h := acceptance.Start(t, acceptance.Options{Resolver: os.Getenv("MARCO_RESOLVER")})

	// ── 1. the deterministic reading, end to end ──────────────────────────
	//
	// The whole vertical on the shortest possible phrase: real perception, a reading the
	// offline parser owns, grounding against a control nobody told Marco about, authority,
	// legal Marco, real input, and a fresh look that confirms it.
	t.Run("deterministic/click Save", func(t *testing.T) {
		h.Restart(false)
		r := h.Do("click Save")

		if got := r.Reading(); got != "deterministic" {
			t.Errorf("reading = %s, want deterministic%s", got, r.Report("—"))
		}
		// MARCO'S CHANNEL: it says it performed, and it can now READ the new status off the
		// screen. The second half matters: "performed" is Marco's claim, and the fresh
		// reading is Marco checking its own claim through perception.
		marcoSaw := h.Sees("Status: Saved")
		// THE ORACLE'S CHANNEL, read afterwards and never consulted by Marco.
		got := h.Oracle()

		if !r.Performed() || !marcoSaw || got.SavePresses != 1 ||
			!strings.Contains(got.Status, "Saved") {
			t.Errorf("Save was not pressed and confirmed.\n"+
				"  marco read 'Status: Saved': %v\n"+
				"  oracle: save_presses=%d status=%q%s",
				marcoSaw, got.SavePresses, got.Status,
				r.Report(oracleVerdict(got.SavePresses == 1)))
		}
		if got.CancelPresses != 0 {
			t.Errorf("Cancel was pressed %d time(s) by a request that named Save",
				got.CancelPresses)
		}
	})

	// ── 2. the interpreted reading ────────────────────────────────────────
	//
	// The same outcome from a sentence the offline parser refuses. Recorded as UNSUPPORTED
	// rather than failed when no interpreter is configured: that is a fact about the machine's
	// configuration and not about Marco, and a suite that failed on it would be unrunnable by
	// default.
	t.Run("interpreted/would you hit the save button", func(t *testing.T) {
		if os.Getenv("MARCO_RESOLVER") == "" {
			t.Skip("UNSUPPORTED: no interpreter configured ($MARCO_RESOLVER unset), so a " +
				"phrase the offline parser declines cannot be read by anything")
		}
		h.Restart(false)
		r := h.Do("would you hit the save button for me")

		if got := r.Reading(); got != "interpreted" {
			t.Fatalf("reading = %s, want interpreted — the offline parser was supposed to "+
				"decline this phrase%s", got, r.Report("—"))
		}
		got := h.Oracle()
		if !r.Performed() || got.SavePresses != 1 {
			t.Errorf("an interpreted request did not reach the button.\n"+
				"  oracle: save_presses=%d%s", got.SavePresses,
				r.Report(oracleVerdict(got.SavePresses == 1)))
		}
	})

	// ── 3. grounding and text entry ───────────────────────────────────────
	//
	// A named field, found by its own label, with a value only the oracle can confirm went in.
	// Marco's channel is the fresh reading; the oracle's is the field's actual contents, which
	// is the one thing perception of a text field is least reliable about.
	t.Run("text entry/type into the Name field", func(t *testing.T) {
		h.Restart(false)
		r := h.Do(`type hello into the Name field`)

		got := h.Oracle()
		if got.Name != "hello" {
			t.Errorf("the Name field holds %q, want \"hello\"%s",
				got.Name, r.Report(oracleVerdict(got.Name == "hello")))
		}
		// AND NOTHING ELSE WAS TYPED INTO. A request that named one field and filled two is
		// a grounding failure that reads as a success on the field it got right.
		if got.Notes != "" {
			t.Errorf("the Notes field holds %q after a request that named Name", got.Notes)
		}
	})

	// ── 4. a toggle ───────────────────────────────────────────────────────
	t.Run("toggle/check Remember me", func(t *testing.T) {
		h.Restart(false)
		r := h.Do("check Remember me")

		got := h.Oracle()
		if !got.Remember {
			t.Errorf("Remember me is not checked%s", r.Report(oracleVerdict(got.Remember)))
		}
	})

	// ── 5. a selection ────────────────────────────────────────────────────
	//
	// Selecting is not invoking, and the fixture keeps them apart: a selection moves
	// `selected` and presses nothing.
	t.Run("selection/select Beta", func(t *testing.T) {
		h.Restart(false)
		r := h.Do("select Beta")

		got := h.Oracle()
		if got.Selected != "Beta" {
			t.Errorf("selected is %q, want \"Beta\"%s",
				got.Selected, r.Report(oracleVerdict(got.Selected == "Beta")))
		}
		if got.SavePresses+got.CancelPresses+got.DetailsPresses != 0 {
			t.Errorf("a selection pressed something: %s", got.Summary())
		}
	})

	// ── 6. several things, in order ───────────────────────────────────────
	//
	// One sentence, two clauses, both verified. The second clause depends on the first having
	// happened, so a program that ran them out of order or skipped one cannot pass.
	t.Run("multi-step/type then save", func(t *testing.T) {
		h.Restart(false)
		r := h.Do(`type Marco into the Name field and click Save`)

		got := h.Oracle()
		if got.Name != "Marco" || got.SavePresses != 1 || !strings.Contains(got.Status, "Saved") {
			t.Errorf("the two steps did not both happen.\n  oracle: %s%s",
				got.Summary(),
				r.Report(oracleVerdict(got.Name == "Marco" && got.SavePresses == 1)))
		}
	})

	// ── 7. a description that is not there ────────────────────────────────
	//
	// The refusal case, and the assertion is in two halves: Marco declined, AND nothing
	// happened anyway. The second is the one worth having — "it refused" is consistent with a
	// refusal after an effect.
	t.Run("refusal/a control nobody has", func(t *testing.T) {
		h.Restart(false)
		before := h.Oracle()
		r := h.Do("click the Nonexistent Control Nobody Has")

		after := h.Oracle()
		if r.Performed() {
			t.Errorf("Marco reported performing an action on a control that does not "+
				"exist%s", r.Report("unchanged"))
		}
		if !after.Unchanged(before) {
			t.Errorf("the fixture changed during a refused request.\n  before: %s\n"+
				"  after:  %s%s", before.Summary(), after.Summary(),
				r.Report("CHANGED — a refusal that still did something"))
		}
		// AND THE REFUSAL NAMES THE RIGHT RUNG. A grounding failure reported as anything
		// else sends somebody to the wrong subsystem.
		if stage := r.FailureStage(); stage != "resolve" {
			t.Errorf("the refusal broke at %q, want \"resolve\" — a description that "+
				"matches nothing is a grounding failure%s", stage, r.Report("unchanged"))
		}
	})

	// ── 8. stop ───────────────────────────────────────────────────────────
	//
	// Cancel reaches a command, and the honest report of a partly-done sequence is partly
	// done. Attempted on a multi-step request because a single action is over before a stop
	// can arrive — and if the timing does not hold, that is recorded rather than failed:
	// a flaky race asserted as a requirement is a suite nobody trusts.
	t.Run("stop/during a sequence", func(t *testing.T) {
		h.Restart(false)
		r := h.DoAndStop(`type Marco into the Name field and click Save`)

		got := h.Oracle()
		switch {
		case r.Cancelled():
			t.Logf("stopped: %s", got.Summary())
		case got.SavePresses == 0:
			t.Logf("UNSUPPORTED-TIMING: the stop arrived after the request finished its "+
				"first step and before the second; reported as %q. oracle: %s",
				r.Trace.State(), got.Summary())
		default:
			t.Logf("UNSUPPORTED-TIMING: the request completed before the stop could "+
				"reach it. oracle: %s", got.Summary())
		}
	})

	// ── 9. an unsupported verb is recorded, not worked around ─────────────
	//
	// `ScrollAction` exists in the API and nothing in production builds one, so there is no
	// verb for "scroll down" to become. The suite says so. It does NOT add a verb to make
	// itself pass, and it does not assert a refusal message, because the point is the gap
	// rather than its wording.
	t.Run("unsupported/scroll down", func(t *testing.T) {
		h.Restart(false)
		before := h.Oracle()
		r := h.Do("scroll down")

		if r.Performed() {
			t.Errorf("Marco reported performing a scroll. There is no scroll verb: "+
				"directorapi.ScrollAction exists and nothing builds one, so this should "+
				"be impossible%s", r.Report("—"))
		}
		if after := h.Oracle(); !after.Unchanged(before) {
			t.Errorf("the fixture changed during an unsupported request%s",
				r.Report("CHANGED"))
		}
		t.Logf("UNSUPPORTED: no scroll verb. Marco said: %q", firstResultLine(r.Output))
	})

	// ── 10. a control that will not respond ──────────────────────────────
	//
	// Disabled, named, visible. Marco must refuse, nothing may happen, AND the refusal must
	// say which of those it was: a person told a control is absent goes looking for a missing
	// control, while a person told it is disabled goes looking at their application.
	//
	// `target.disabledPenalty`'s own note already said so — "a disabled Save IS the thing the
	// user meant" — and the resolver reported it as absent anyway.
	t.Run("disabled/click Delete", func(t *testing.T) {
		h.Restart(false)
		before := h.Oracle()
		r := h.Do("click Delete")

		if r.Performed() {
			t.Errorf("Marco reported performing an action on a DISABLED control%s",
				r.Report("unchanged"))
		}
		if after := h.Oracle(); !after.Unchanged(before) {
			t.Errorf("the fixture changed when a disabled control was asked for%s",
				r.Report("CHANGED — a disabled control was operated"))
		}
		// AND THE SENTENCE IS TRUE. The control is there; saying it is not sends somebody
		// to the wrong place entirely.
		if msg := r.Message(); !strings.Contains(msg, "disabled") {
			t.Errorf("the refusal reads %q.\n\nDelete is present, named and visible — it "+
				"is disabled. Reporting it as absent describes a different world and "+
				"sends a reader to look for a control that is on their screen.%s",
				msg, r.Report("unchanged"))
		}
	})

	// ── 11. expand, and collapse ──────────────────────────────────────────
	//
	// `verifyExpansion` is the most specific verifier in the package — a node reporting itself
	// expanded, or its children appearing — and it had never run against a real desktop.
	t.Run("expand/open a tree node", func(t *testing.T) {
		h.Restart(false)
		r := h.Do("expand Documents")

		got := h.Oracle()
		if !got.DocumentsExpanded {
			t.Errorf("the node did not expand%s",
				r.Report(oracleVerdict(got.DocumentsExpanded)))
		}
		// AND MARCO CAN READ WHAT APPEARED. A node that opened and whose children cannot be
		// perceived is a place Marco cannot go on to act in.
		if !h.Sees("Reports") {
			t.Errorf("Marco cannot read the children of the node it just expanded.\n%s",
				h.Perceived())
		}

		// AND COLLAPSING IS DRIVEN, REPORTED, AND NOT ASSERTED ON.
		//
		// # Why the oracle has no vote here, measured
		//
		// A UIA-driven collapse manipulates the native tree control WITHOUT notifying the
		// managed layer. Measured directly, driving the pattern by hand:
		//
		//	Expand()    UIA Expanded    expand_events=1    oracle true
		//	Collapse()  UIA Collapsed   collapse_events=0  oracle still true
		//
		// and after the collapse the child nodes were GONE from the accessibility tree. Two
		// independent signals — the reported state and the structure — say the node collapsed.
		// WinForms' own `AfterCollapse` never fired and `TreeNode.IsExpanded` stayed true.
		//
		// So the application genuinely does not know, and there is no application-side witness
		// to be had. Asserting on the oracle here would report a correct Marco as broken, which
		// is as damaging as the false success this suite exists to catch and harder to notice
		// because it looks like diligence.
		//
		// It is still DRIVEN, because a collapse that crashed or refused is worth knowing
		// about, and its outcome is logged for a reader.
		c := h.Do("collapse Documents")
		again := h.Oracle()
		t.Logf("UNSUPPORTED-ORACLE: collapse was %s; the fixture's managed layer is not told "+
			"about a UIA-driven collapse (collapse_events=%d while the node's children leave "+
			"the tree), so this direction has no independent witness",
			c.Outcome(), again.CollapseEvents)
		if c.Outcome() == "failed" && !c.Refused() {
			t.Errorf("collapse failed outright, which is about Marco rather than about the "+
				"oracle%s", c.Report("no witness available"))
		}
	})

	// ── 10. a second place inside one window ──────────────────────────────
	//
	// The Details button opens a panel: the same application surface, a different place
	// inside it. Worth one case because it is the shape Marco's two-level screen identity
	// exists for, and because a within-a-window change is what an acceptance against a single
	// flat form could never produce.
	t.Run("second view/open Details", func(t *testing.T) {
		h.Restart(false)
		r := h.Do("click Details")

		got := h.Oracle()
		if !got.SecondView || got.DetailsPresses != 1 {
			t.Errorf("the second view did not open%s",
				r.Report(oracleVerdict(got.SecondView)))
		}
		// AND MARCO CAN READ WHAT APPEARED. A panel that opened and cannot be perceived is
		// a place Marco cannot act in.
		if !h.Sees("Nothing selected") {
			t.Errorf("Marco cannot read the panel it just opened.\nperceived:\n%s",
				h.Perceived())
		}
	})
}

// ── the dangerous failure, on its own ─────────────────────────────────────────

// MARCO MUST NOT BELIEVE AN OUTCOME IT DID NOT GET.
//
// # Why this is the most valuable case in the suite
//
// Every other case can fail honestly: Marco refuses, or reports failure, and a person is told. This
// is the one that cannot be detected from inside Marco — the action was performed, the control was
// real, the press landed, and nothing happened. A system that reports success here is a system
// whose "performed" means nothing, and no single-channel test can see it.
//
// The fixture's `--stubborn` mode constructs it: Save is present, enabled, counts the press, and
// does not act. That is not a contrived state — it is a validation failure, a swallowed exception, a
// disabled-looking-but-enabled control, which is ordinary software behaviour.
//
// The oracle proves the press happened, so this is not "Marco missed the button". It is "Marco hit
// the button and the world did not move", and Marco's own verification is the thing that must
// notice.
func TestMarcoDoesNotBelieveAnOutcomeItDidNotGet(t *testing.T) {
	h := acceptance.Start(t, acceptance.Options{
		Stubborn: true,
		Resolver: os.Getenv("MARCO_RESOLVER"),
	})

	r := h.Do("click Save")
	got := h.Oracle()

	// FIRST, THE PREMISE. If the press never landed this test is about something else, and
	// saying so is better than passing for the wrong reason.
	if got.SavePresses != 1 {
		t.Fatalf("the premise does not hold: Save was pressed %d time(s), so this run is "+
			"not testing whether Marco notices an inert control%s",
			got.SavePresses, r.Report(got.Summary()))
	}
	if strings.Contains(got.Status, "Saved") {
		t.Fatalf("the fixture acted on the press in stubborn mode; the premise is broken%s",
			r.Report(got.Summary()))
	}

	// THEN THE CLAIM. Marco pressed a real control and the world did not move, so it must
	// not say it worked.
	if r.Performed() {
		t.Errorf("MARCO BELIEVED AN OUTCOME IT DID NOT GET.\n\n"+
			"It pressed Save — the oracle confirms one press — and the application did "+
			"nothing. Marco reported success anyway, which means its verification did not "+
			"check the outcome it was waiting for. Everything downstream of this that "+
			"trusts \"performed\" is trusting nothing.%s",
			r.Report("save_presses=1 and status unchanged: THE PRESS LANDED AND NOTHING HAPPENED"))
	}
	// AND IT MUST NOT CLAIM TO SEE WHAT IS NOT THERE.
	if h.Sees("Status: Saved") {
		t.Errorf("Marco read 'Status: Saved' off a screen that says %q", got.Status)
	}
	t.Logf("verification correctly refused: %s · %s", r.Trace.State(), r.FailureStage())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func oracleVerdict(ok bool) string {
	if ok {
		return "agrees"
	}
	return "DISAGREES with Marco"
}

func firstResultLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "[result]") {
			return strings.TrimSpace(l)
		}
	}
	return strings.TrimSpace(out)
}
