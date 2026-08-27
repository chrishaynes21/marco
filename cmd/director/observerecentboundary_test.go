package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// What a retrospective Learn must NOT do.
//
// The permission boundary, from the other side. Every test above is about what gets written down;
// these are about the four things that stay exactly where they were — the keyboard, the desktop
// lease, the authority to act, and the account of what Marco has done. See ADR-094.

// LEARNING WHAT YOU JUST DID TOUCHES NOTHING THAT ACTS.
//
// No input, no desktop lease, no execution authority, no rehearsal, no performer.
//
// # How each of those is actually asserted
//
// Not by absence of a symptom. Every actuating entrance in this Director funnels through
// `beginPerformance` — that is why it exists, and why the ambient observer reads its counter to
// tell Marco's own work from the person's. So a retrospective Learn that emitted anything would
// have had to pass through it, and the counter says whether it did.
//
// The desktop lease is claimed inside a live rehearsal, which is downstream of the same slot; the
// rehearsal grant is the only authority object in the system and nothing here creates one.
//
// Deleting the assertions is not the risk. Adding a call to `Rehearse`, `PerformGoal` or the
// walker anywhere in the promotion path is, and this is what would catch it.
func TestLearningWhatYouJustDidTouchesNothingThatActs(t *testing.T) {
	rt, _, _ := recentRuntime(t)
	theWalkTheyJustTook(rt)

	out, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("learning: %v", err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("nothing was learned, so this proves nothing: %s", out.Said)
	}

	// NOTHING DROVE THE DESKTOP. The counter is incremented by the one slot every actuating
	// entrance passes through, before the registry check, so a Runtime with no registry —
	// which is this one — still counts.
	if rt.marcoIsActing() {
		t.Error("a performance is still open after a learn that performed nothing")
	}
	rt.actingMu.Lock()
	acting := rt.acting
	rt.actingMu.Unlock()
	if acting != 0 {
		t.Errorf("the performance slot was entered %d time(s) by a retrospective learn. "+
			"Nothing on this path may reach the desktop.", acting)
	}

	// AND NO AUTHORITY WAS MINTED. The ephemeral rehearsal grant is the only authority
	// object in this system; a learn that created one would be a learn that could act.
	if rt.observations.last != nil && rt.observations.last.Grant() != nil {
		t.Error("a rehearsal grant exists after a retrospective learn")
	}
}

// LEARNING IS NOT SOMETHING MARCO DID TO YOUR COMPUTER.
//
// Activity is the account of what MARCO has done, and every row is an action it took on somebody's
// behalf. A retrospective Learn takes none: the person navigated, and Marco read what it had
// already seen. Writing rows for it would put somebody's own afternoon into the one surface that
// says "here is what I did for you" — which is the same boundary ADR-093 drew for watching, from
// the other end.
//
// Deleting nothing catches this today, and that is the point: it is a claim about an absence, and
// the way it would stop being true is somebody adding a write.
func TestLearningFromWhatYouDidIsNotSomethingMarcoDid(t *testing.T) {
	rt, _, _ := recentRuntime(t)
	graph, err := actiongraph.OpenFile(filepath.Join(t.TempDir(), "action-graph.json"))
	if err != nil {
		t.Fatalf("opening the action graph: %v", err)
	}
	rt.graph = graph
	theWalkTheyJustTook(rt)

	if _, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"}); err != nil {
		t.Fatalf("learning: %v", err)
	}
	nodes, err := graph.Recent(0)
	if err != nil {
		t.Fatalf("reading the action graph: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("learning what somebody did wrote %d row(s) into the account of what MARCO "+
			"has done. Their own navigation is not Marco's activity.", len(nodes))
	}
}

// AND THE PLAY IS AN ORDINARY PLAY.
//
// There is no special execution path for a retrospectively learned play, and there must not be:
// it is written by the same lowering, saved by the same persistence path, found by the same
// resolver, and run by the same performer under the same authority. A second path would be a
// second set of rules about when Marco may act, and only one of them would be reviewed.
//
// What this asserts is the shape of the artifact rather than a run: a fresh registry — built from
// a directory and nothing else, with no session, no Director and no memory — resolves it exactly
// as it resolves a play learned by demonstration.
func TestARetrospectivelyLearnedPlayIsAnOrdinaryPlay(t *testing.T) {
	rt, _, dir := recentRuntime(t)
	theWalkTheyJustTook(rt)
	if _, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"}); err != nil {
		t.Fatalf("learning: %v", err)
	}

	fresh := routes.Registry{Dir: dir}
	route, ok := fresh.Resolve(recentApp, "open-mouse-settings")
	if !ok {
		t.Fatalf("a fresh registry cannot find the play. Tree: %v", tree(t, dir))
	}
	source, err := readRoute(fresh, route)
	if err != nil {
		t.Fatalf("reading the play back: %v", err)
	}
	// IT SAYS WHAT IT DOES AND NOT HOW MARCO CAME TO BELIEVE IT. The same boundary every
	// learned play holds — see TestAVerifiedRouteBecomesOrdinaryMarco — and the retrospective
	// path must not be where it breaks: this play was built from a trail, and every word in
	// the trail's vocabulary is backstage.
	//
	// The header comment is excluded, and only the header: `// Marco learned this by
	// watching` is a sentence for the person who opens the file, deliberately there, and part
	// of every learned play whichever way it was learned.
	body := withoutComments(source)
	for _, leak := range []string{
		"subj_", "seen_", "state_", "ambient", "trail", "watching", "observ", "promot",
		"shadow", "candidate", "evidence", "session",
	} {
		if containsFold(body, leak) {
			t.Errorf("the play mentions %q. Director may know WHY it learned this; the "+
				"play says WHAT it does:\n%s", leak, source)
		}
	}
	// AND IT SAYS THE RIGHT THING: the screens by the names the interface gave them, and the
	// control by the name the person pressed.
	for _, want := range []string{
		`do Screen's Showing with "Bluetooth & devices"`,
		`with Name "Mouse"`,
		`do Theater's Activate`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the play does not contain %q:\n%s", want, source)
		}
	}
}

// withoutComments strips whole-line comments, so a sweep over a play's vocabulary reads what the
// play DOES rather than what the header says about how it came to exist.
func withoutComments(source string) string {
	var kept []string
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// readRoute reads a resolved play's source.
func readRoute(reg routes.Registry, r routes.Route) (string, error) {
	b, err := os.ReadFile(reg.Path(r))
	return string(b), err
}

// containsFold is a case-insensitive substring test, for the backstage-vocabulary sweep.
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
