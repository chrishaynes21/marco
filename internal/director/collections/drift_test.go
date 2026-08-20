package collections_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Membership drift.
//
//	An ordinal belongs to one offered list in one observed world.
//	A fresh world may invalidate an old answer.

func fp(t *testing.T, keys ...string) collections.MembershipFingerprint {
	t.Helper()
	return collections.FingerprintCandidates(query(directorapi.RoleTab, "new"), keys)
}

func TestAnUnchangedListKeepsItsOrdinalMeaning(t *testing.T) {
	offered := fp(t, "new-tab", "new-window")
	current := fp(t, "new-tab", "new-window")
	got := collections.CompareMembership(offered, current, 1, true)
	if got != collections.DriftUnchanged {
		t.Fatalf("drift = %s, want unchanged", got)
	}
	if !got.Resumable() {
		t.Fatal("an unchanged list refused to resume")
	}
}

func TestANewContenderAtTheTopInvalidatesTheOrdinal(t *testing.T) {
	// THE case. Offered: 1 New tab, 2 New window. The user says "the first one" while a
	// "New folder" appears at the top. Applying the ordinal would click a control they
	// were never shown.
	offered := fp(t, "new-tab", "new-window")
	current := fp(t, "new-folder", "new-tab", "new-window")

	got := collections.CompareMembership(offered, current, 1, true)
	if got.Resumable() {
		t.Fatalf("drift = %s — the old ordinal would select the new contender", got)
	}
	if got != collections.DriftNewContender {
		t.Fatalf("drift = %s, want new_contender_appeared", got)
	}
	// The list is long enough, which is exactly the reasoning that must NOT be used.
	if len(current.OrderedKeyDigests) < 1 {
		t.Fatal("fixture is wrong")
	}
}

func TestAChosenContenderThatDisappearedIsReportedHonestly(t *testing.T) {
	offered := fp(t, "new-tab", "new-window")
	current := fp(t, "new-window")

	got := collections.CompareMembership(offered, current, 1, true)
	if got != collections.DriftChosenDisappeared {
		t.Fatalf("drift = %s, want chosen_member_disappeared", got)
	}
	if got.Resumable() {
		t.Fatal("a disappeared choice was resumable")
	}
	if !strings.Contains(got.Describe(), "no longer present") {
		t.Fatalf("message = %q", got.Describe())
	}
}

func TestAChosenContenderThatMovedIsNotSelectedByPosition(t *testing.T) {
	// Same members, different order. "The first one" now points at something else.
	offered := fp(t, "new-tab", "new-window")
	current := fp(t, "new-window", "new-tab")

	got := collections.CompareMembership(offered, current, 1, true)
	if got.Resumable() {
		t.Fatalf("drift = %s — a reordered list kept its ordinal", got)
	}
	if got != collections.DriftOrderChanged {
		t.Fatalf("drift = %s, want order_changed", got)
	}
}

func TestAChoiceThatKeptItsPositionStillResumes(t *testing.T) {
	// The list changed BELOW the chosen position. The answer still means what it meant,
	// so re-asking would be noise.
	offered := fp(t, "new-tab", "new-window")
	current := fp(t, "new-tab", "new-folder", "new-window")

	got := collections.CompareMembership(offered, current, 1, true)
	if got != collections.DriftChosenPresent {
		t.Fatalf("drift = %s, want chosen_member_present", got)
	}
	if !got.Resumable() {
		t.Fatal("an unmoved choice refused to resume")
	}
}

func TestEmptyAndUnobservableStayDistinctAcrossAPause(t *testing.T) {
	offered := fp(t, "new-tab", "new-window")

	empty := collections.CompareMembership(offered, fp(t), 1, true)
	if empty != collections.DriftEmpty {
		t.Fatalf("drift = %s, want collection_empty", empty)
	}
	if !strings.Contains(empty.Describe(), "now empty") {
		t.Fatalf("message = %q", empty.Describe())
	}

	blind := collections.CompareMembership(offered, fp(t), 1, false)
	if blind != collections.DriftUnobservable {
		t.Fatalf("drift = %s, want collection_unobservable", blind)
	}
	if strings.Contains(blind.Describe(), "empty") {
		t.Fatalf("an unobservable collection was described as empty: %q", blind.Describe())
	}
}

func TestADifferentQueryMakesTheAnswerUntrustworthy(t *testing.T) {
	offered := collections.FingerprintCandidates(query(directorapi.RoleTab, "new"),
		[]string{"a", "b"})
	current := collections.FingerprintCandidates(query(directorapi.RoleButton, "other"),
		[]string{"a", "b"})
	if got := collections.CompareMembership(offered, current, 1, true); got !=
		collections.DriftIdentityUncertain {
		t.Fatalf("drift = %s, want identity_uncertain", got)
	}
}

func TestAFingerprintCarriesNoMemberIdentity(t *testing.T) {
	// Digests only. It exists for one comparison and must carry nothing that could be
	// used to act on a member directly.
	members := []collections.Member{
		{Application: "code", Role: directorapi.RoleTab, Label: "Private Document.docx",
			NativeID: "uia:41"},
	}
	f := collections.Fingerprint(query(directorapi.RoleTab, ""), members)
	raw, _ := json.Marshal(f)
	for _, forbidden := range []string{"Private", "docx", "uia:41", "element_id", "bounds"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("the fingerprint carries %q:\n%s", forbidden, raw)
		}
	}
	if len(f.OrderedKeyDigests) != 1 || f.OrderedKeyDigests[0] == "" {
		t.Fatalf("no digest recorded: %+v", f)
	}
}

func TestAnEventIDChangesWhenTheOfferChanges(t *testing.T) {
	// An answer belongs to ONE event. Without a distinct id, a response arriving after a
	// fresh question replaced the old one would be applied to the new contender list.
	a := collections.NewEventID("tabs", 3, fp(t, "new-tab", "new-window"))
	same := collections.NewEventID("tabs", 3, fp(t, "new-tab", "new-window"))
	if a != same {
		t.Fatalf("the same offer produced two ids: %s vs %s", a, same)
	}
	changed := collections.NewEventID("tabs", 3, fp(t, "new-folder", "new-tab"))
	if a == changed {
		t.Fatal("a changed offer kept its event id")
	}
	later := collections.NewEventID("tabs", 4, fp(t, "new-tab", "new-window"))
	if a == later {
		t.Fatal("a different iteration kept the same event id")
	}
	if !strings.HasPrefix(string(a), "clarification_") {
		t.Fatalf("id = %q", a)
	}
}
