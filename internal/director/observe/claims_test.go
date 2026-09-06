package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A SCREEN IS HEARD FROM EVERY SOURCE THAT SPOKE, AND NONE OF THEM IS PRIVILEGED.
//
// # The measurement this exists for
//
// Two applications put the same kind of fact in different places:
//
//	Discord   selected navigation "Sometimes Silly"   the server — true, and the wrong level
//	          container label     "Messages in irl"   the channel — the local state
//	Explorer  selected navigation "Documents"         the folder — the local state
//	          container label     "Items View"        generic, names nothing
//
// A rule reading the container label as the state is right about Discord and blind to Explorer. A
// rule reading selected navigation is right about Explorer and names the server in Discord. Both
// are claims; which one discriminates is interpretation's question, and it needs the provenance to
// answer it.
//
// So the collector keeps every claim with where it came from, and decides nothing.
func TestEveryClaimKeepsWhereItCameFrom(t *testing.T) {
	in := []observe.SemanticClaim{
		{Text: "Sometimes Silly", Source: observe.FromSelectedNavigation},
		{Text: "Messages in irl", Source: observe.FromContainerLabel, Container: "list"},
		{Text: "Documents - File Explorer", Source: observe.FromWindowTitle},
	}
	got := observe.AdmitClaims(in)
	if len(got) != 3 {
		t.Fatalf("%d claim(s) survived admission, want 3: %+v", len(got), got)
	}
	sources := map[observe.ClaimSource]string{}
	for _, c := range got {
		sources[c.Source] = c.Text
	}
	if sources[observe.FromSelectedNavigation] != "Sometimes Silly" {
		t.Errorf("the navigation claim is %q", sources[observe.FromSelectedNavigation])
	}
	if sources[observe.FromContainerLabel] != "Messages in irl" {
		t.Errorf("the container claim is %q", sources[observe.FromContainerLabel])
	}
	if sources[observe.FromWindowTitle] != "Documents - File Explorer" {
		t.Errorf("the title claim is %q", sources[observe.FromWindowTitle])
	}
}

// AND THE COLLECTOR DOES NOT DECIDE WHICH ONE IS USEFUL.
//
// `Items View` names nothing and `Messages in irl` names the channel, and no rule available here
// can tell them apart — telling them apart requires knowing what Explorer's file pane is called,
// which is exactly the application-specific knowledge this design refuses.
//
// So both are admitted, faithfully, and the discovery that one is generic is left to
// interpretation, which can see that it never varies while another claim does.
//
// Filtering `Items View` here must fail this.
func TestAGenericClaimIsAdmittedRatherThanJudged(t *testing.T) {
	got := observe.AdmitClaims([]observe.SemanticClaim{
		{Text: "Items View", Source: observe.FromContainerLabel, Container: "list"},
		{Text: "Messages in irl", Source: observe.FromContainerLabel, Container: "list"},
	})
	if len(got) != 2 {
		t.Fatalf("%d claim(s), want both — the collector may not judge which names "+
			"anything: %+v", len(got), got)
	}
}

// AND A CLAIM PASSES THE SAME SHAPE FILTER EVERY OTHER WORD DOES.
//
// A claim is text off somebody's screen. There is one policy for whether such text may be kept and
// it is `safeLabelText`; a second one here would be a privacy decision nobody reviewed.
func TestAClaimPassesTheOneShapeFilter(t *testing.T) {
	got := observe.AdmitClaims([]observe.SemanticClaim{
		{Text: "@someone", Source: observe.FromContainerLabel},
		{Text: "quarterly-report-final-v3.xlsx", Source: observe.FromWindowTitle},
		{Text: "Documents", Source: observe.FromSelectedNavigation},
	})
	if len(got) != 1 || got[0].Text != "Documents" {
		t.Errorf("admission kept %+v; a handle and a filename are not names of places", got)
	}
}

// AND CLAIMS SURVIVE THE CONSTRUCTORS THAT REBUILD EVIDENCE.
//
// The trap this file's neighbour records twice: `PlaceName` and then `Affordances` were each added
// upstream, passed their own tests, and were dropped by a constructor that rebuilds the struct
// field by field.
func TestClaimsSurviveTheProductionEvidencePath(t *testing.T) {
	in := observe.SemanticEvidence{
		Observed: true,
		Claims: []observe.SemanticClaim{
			{Text: "Documents", Source: observe.FromSelectedNavigation},
		},
	}
	merged := in.Merge(observe.SemanticEvidence{
		Observed: true,
		Claims: []observe.SemanticClaim{
			{Text: "Items View", Source: observe.FromContainerLabel, Container: "list"},
		},
	})
	if len(merged.Claims) != 2 {
		t.Fatalf("merging two Actors' claims produced %d, want both: %+v",
			len(merged.Claims), merged.Claims)
	}
}

// AND A WINDOW TITLE IS A CLAIM AND NOT IDENTITY.
//
// "A window is not a place" is unchanged. The title may help decide what is on screen NOW — it is
// where Explorer puts the folder name — and it may not become part of what a Place is remembered
// by, because a title changes with a document while the screen stays the screen.
//
// Held structurally: `NewScreenSignature` takes regions, and a claim is not a region.
func TestAWindowTitleIsAClaimAndNotIdentity(t *testing.T) {
	sig := observe.NewScreenSignature(pageLike(9, 2, 14))
	if len(sig.Roles) == 0 {
		t.Fatal("the fixture produced an empty signature")
	}
	for role := range sig.Roles {
		if role == string(observe.FromWindowTitle) || role == "title" {
			t.Errorf("a signature carries %q; a window title may inform the current "+
				"reading and may never be part of durable identity", role)
		}
	}
}

// AND THE SOURCES TURN OUT TO COVER FOR EACH OTHER, WHICH IS WHY THERE ARE THREE.
//
// # Measured, and not designed
//
// The shape filter refuses a Discord window title outright: `#irl | Sometimes Silly - Discord`
// carries `#` and `discord`, both of which are in `privateMarkers` — the list that exists to catch
// social identifiers. So the title source, which is where Explorer puts its folder, is
// unavailable on exactly the application whose content is most sensitive.
//
// And it does not matter, because that is the application whose container label names the state.
// Where the title is refused the container spoke; where the container was generic the title spoke.
//
// Nobody arranged that. It is the argument for collecting from several sources rather than
// choosing one: any single privileged source is refused or generic somewhere.
func TestWhereOneSourceIsRefusedAnotherStillSpeaks(t *testing.T) {
	discord := observe.AdmitClaims([]observe.SemanticClaim{
		{Text: "#irl | Sometimes Silly - Discord", Source: observe.FromWindowTitle},
		{Text: "Messages in irl", Source: observe.FromContainerLabel, Container: "list"},
	})
	if len(discord) != 1 || discord[0].Source != observe.FromContainerLabel {
		t.Fatalf("Discord kept %+v; its title is refused by the shape filter and its "+
			"container label is what carries the channel", discord)
	}
	explorer := observe.AdmitClaims([]observe.SemanticClaim{
		{Text: "Documents - File Explorer", Source: observe.FromWindowTitle},
		{Text: "Items View", Source: observe.FromContainerLabel, Container: "list"},
	})
	if len(explorer) != 2 {
		t.Fatalf("Explorer kept %+v; both its title and its container label are "+
			"admissible, and only one of them names anything", explorer)
	}
}

// AND A CLAIM REACHES THE SCREEN STATE THROUGH THE PRODUCTION SEGMENTER.
//
// # The trap this closes, for the third time in this file's neighbourhood
//
// `admissibleTerms` rebuilds SemanticEvidence field by field, and anything it does not know about
// is dropped silently. `PlaceName` was added upstream, passed its own tests, and vanished there.
// `Affordances` was added later and a test had to be written for the same reason. A mutation
// removing claims from that constructor survived every direct test of `AdmitClaims` and `Merge`,
// because both of those are reachable without it.
//
// So this drives `ScreenSegmenter.Observe` — the production path — and reads the tally back.
func TestAClaimReachesTheScreenStateThroughTheSegmenter(t *testing.T) {
	var g observe.ScreenSegmenter
	sem := observe.SemanticEvidence{
		Observed: true,
		Claims: []observe.SemanticClaim{
			{Text: "Documents", Source: observe.FromSelectedNavigation},
			{Text: "Items View", Source: observe.FromContainerLabel, Container: "list"},
		},
	}
	var last observe.ScreenStateID
	for i := range 3 {
		last = g.Observe(i+1, pageLike(9, 2, 14), nil, sem)
	}
	if last == observe.ScreenStateUnknown {
		t.Fatal("the readings were not placed, so this proves nothing")
	}
	for _, st := range g.States() {
		if st.ID != last {
			continue
		}
		if len(st.Claims) != 2 {
			t.Fatalf("the state tallied %v; both claims must survive the constructor "+
				"that rebuilds evidence field by field", st.Claims)
		}
		// KEYED BY SOURCE. `Documents` from navigation and from a title are two
		// independent witnesses, and collapsing them loses the corroboration that is the
		// whole reason for collecting from several places.
		for key := range st.Claims {
			if key == "Documents" || key == "Items View" {
				t.Errorf("the tally key is %q, without its source", key)
			}
		}
		return
	}
	t.Fatalf("the placed state %q is not among the segmenter's states", last)
}
