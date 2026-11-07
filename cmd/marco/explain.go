package main

// WHERE WOULD THESE WORDS GO — asked without going there.
//
// # The failure this closes
//
// A routine demonstrated by hand, typed into the widget, and the surface said *failed:
// ark graphics live*. The reason was ten days old and three processes away: the overlay
// launches `marco.exe` by path, that binary predated the arm of the door that claims a
// demonstration, and so the words were handed to Director — which read them against the screen,
// found nothing matching, and failed. Every part of the product behaved exactly as built. The
// only way to find out WHICH part had the words was to read source, and the only way to watch
// the decision was to make it: `marco do` performs.
//
// So there is now a way to ask. It runs the one door — the same `invoke.Decide`, with the same
// two resolvers, over the same stores — prints the verdict and what would carry it out, and
// performs NOTHING.
//
// # Why it is not a second decision
//
// It would be worthless if it were. The whole value is that it is the production door answering
// about the production stores: a second copy would eventually disagree with the thing it exists
// to explain, and the explanation would be the thing you could not trust.
//
// So this reads the verdict and renders it. It does not consult a store directly, does not
// re-rank anything, and cannot reach an action.

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// explainInvocation says what `marco do` would do with these words, and does none of it.
func explainInvocation(d orchestrator.Deps, req invoke.Request) {
	if req.Skill != "" {
		fmt.Printf("skill        %s (built in; performed by its identity, no routine is decided)\n", req.Skill)
		return
	}
	if req.App == "" {
		req.App = appOf(d)
	}
	// NOT ASKED OF THE DIRECTOR. `runInvocation` asks the service whether a question is
	// pending, because an answer belongs to its question. Explaining is not answering, and
	// a diagnostic that started a service to describe a decision would change the state it
	// is describing.
	dec := invoke.Decide(d.Reg, demoSights{dir: directorDir()}, req)

	fmt.Printf("words        %q\n", req.Text)
	if req.App != "" {
		fmt.Printf("application  %s (what is in front now)\n", req.App)
	}
	fmt.Printf("decision     %s\n", dec.Kind)
	fmt.Printf("because      %s\n", dec.Why)
	switch dec.Kind {
	case invoke.KindPlay:
		fmt.Printf("would run    the play %s", dec.Play.Slug)
		if dec.Play.App != "" {
			fmt.Printf(" (scoped to %s)", dec.Play.App)
		}
		fmt.Println()
		// AND WHETHER A DEMONSTRATION OF THE SAME NAME IS BEING SHADOWED.
		//
		// The live failure this exists for: one name, two artifacts. A recorded
		// play of raw coordinates and a pixel demonstration with screens, control
		// readings and an outcome expectation — and the play arm answers first, so
		// typing the name ran the coordinates. From the outside the two are
		// indistinguishable: the same words, and nothing says which of the two
		// things you taught is the one that just happened.
		if d, ok := (demoSights{dir: directorDir()}).Demonstration(req.App, req.Text); ok {
			fmt.Printf("but note     %q is ALSO saved as something you "+
				"demonstrated, and the play above answers first. To "+
				"perform what you demonstrated instead:\n", d.Name)
			fmt.Printf("             %s %s\n", directorBin(),
				strings.Join(performWorkflowArgs(d), " "))
		}
	case invoke.KindDemonstration:
		fmt.Printf("would run    %s\n", directorBin())
		fmt.Printf("             %s\n", strings.Join(
			performWorkflowArgs(dec.Demonstration), " "))
		fmt.Printf("             one authorised action at a time; %s approves each one\n",
			approveKey)
	case invoke.KindControl:
		fmt.Println("would run    nothing new — it stops whatever is already running")
	case invoke.KindDirector:
		fmt.Println("would run    nothing durable — Director reads these words against " +
			"what is on screen")
		// AND WHETHER SOMETHING OF THAT NAME WAS SAVED AND COULD NOT BE USED.
		//
		// This is the sentence the live failure needed. A routine that exists and is
		// not performable by sight is indistinguishable, from the outside, from a
		// name nobody ever saved — and the two send a person to completely different
		// places: one to demonstrate it, the other to fix what is wrong with the one
		// they have.
		if why := whyNotBySight(req.Text); why != "" {
			fmt.Printf("but note     %s\n", why)
		}
	}
}

// approveKey is the key that authorises one action, from the map the runner prints it from, so
// an explanation cannot advertise one binding while the run listens for another.
var approveKey = navsource.ConsentKeys[navsource.ConsentApprove]

// whyNotBySight is why a saved routine of this name cannot be performed by sight, or "" when
// there is no such routine or it can.
//
// The three conditions of `Artifact.PerformableBySight`, each with its own sentence, because they
// send a person to three different places: a verb routine is run another way, an unscoped one
// needs demonstrating again, and one with no visual evidence is a record of something that
// happened rather than a thing to run.
func whyNotBySight(name string) string {
	store, err := demo.Open(directorDir())
	if err != nil {
		return ""
	}
	a, ok := store.Sequence(name)
	if !ok || a.PerformableBySight() {
		return ""
	}
	switch {
	case a.RunnableByVerbs():
		return fmt.Sprintf("%q IS saved, and it carries a verb on every step, so it "+
			"belongs to the ordinary execution path rather than to the pixel "+
			"runner", a.Name)
	case len(a.VisualTargets()) == 0:
		return fmt.Sprintf("%q IS saved and carries no visual control evidence, so "+
			"there is nothing to ground on a fresh look; demonstrate it again",
			a.Name)
	case !a.Scoped():
		return fmt.Sprintf("%q IS saved and is UNSCOPED — valid for no application "+
			"rather than for all of them, because the identity has to come from a "+
			"window that was observed. Demonstrate it again to record which "+
			"application it belongs to: director demonstrate-visual %q "+
			"--application <app>", a.Name, a.Name)
	}
	return fmt.Sprintf("%q IS saved and cannot be performed by sight", a.Name)
}
