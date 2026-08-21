package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What a Place is called, and who said so.
//
// Three concepts that must not collapse into each other: the word the Audience offered, the word
// Marco worked out, and the description of what the screen is made of. The first is a person
// speaking, the second is an inference, and the third is diagnostics — and the live failure this
// file exists for was the third being asked to do the job of the first.

func selected(role directorapi.ElementRole, label string) observe.PlaceNameEvidence {
	return observe.PlaceNameEvidence{Role: role, Label: label, Confidence: 1, Selected: true}
}

// THE SELECTED NAVIGATION DESTINATION NAMES THE PLACE.
//
// Measured on the live Settings tree: `list_item "Home"`, Selected, in the navigation pane. It is
// the only signal that changes between Settings pages — the window title is "Settings" on all of
// them.
func TestTheSelectedDestinationNamesThePlace(t *testing.T) {
	got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
		selected(directorapi.RoleListItem, "Home"),
	})
	if got != "Home" {
		t.Errorf("the Place is called %q, want Home", got)
	}
}

// A VALUE IS NOT A DESTINATION.
//
// Settings Home reports TWO selected items: `Home` in the navigation pane, and `Dark` inside the
// Color-mode combo box. One says where you are. The other says what a setting is set to, and
// naming the screen "Dark" would be confidently wrong.
func TestASelectedValueDoesNotNameThePlace(t *testing.T) {
	dark := selected(directorapi.RoleListItem, "Dark")
	dark.InsideValueChooser = true

	got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
		selected(directorapi.RoleListItem, "Home"), dark,
	})
	if got != "Home" {
		t.Errorf("the Place is called %q, want Home — the combo box's value is not a place", got)
	}
}

// SEVERAL DIFFERENT ANSWERS NAME NOTHING.
//
// VS Code reports three selected items at once — an activity-bar tab, a tree item and an editor
// tab. Ranking them by depth would have named it "Explorer (Ctrl+Shift+E)", a keyboard hint. No
// name is better than a wrong one, because a wrong one is trusted.
//
// Deleting the disagreement check must fail this.
func TestSeveralSelectedItemsNameNothing(t *testing.T) {
	got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
		selected(directorapi.RoleTab, "Explorer"),
		selected(directorapi.RoleTreeItem, "graph.json"),
		selected(directorapi.RoleTab, "localhost:8765"),
	})
	if got != "" {
		t.Errorf("three selected items produced the confident name %q", got)
	}
}

// Two Actors agreeing on the same word is ONE hypothesis, not a disagreement.
func TestTwoActorsAgreeingStrengthenOneName(t *testing.T) {
	got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
		selected(directorapi.RoleListItem, "Mouse"),
		selected(directorapi.RoleTab, "Mouse"),
	})
	if got != "Mouse" {
		t.Errorf("two Actors saying Mouse produced %q", got)
	}
}

// A control is not a destination, whatever else is true of it.
//
// `Back` and `Close` are the reason the structural description begins with garbage. They are
// buttons: they never report themselves selected, and the role rule refuses them even if they did.
func TestAControlCannotNameThePlace(t *testing.T) {
	for _, word := range []string{"Back", "Close", "Minimize", "Search"} {
		if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
			selected(directorapi.RoleButton, word),
		}); got != "" {
			t.Errorf("a button called %q became the Place name (%q)", word, got)
		}
	}
}

// PASSIVE OBSERVATION MAY NOW INFER A NAME, AND STILL MAY NOT WRITE ONE DOWN.
//
// This test asserted the opposite until Roadmap 35A. `AdmittedPlaceName` opened with
// `if !demonstration { return "" }`, so outside an explicit Learn Marco could recognise the Place
// it was standing on and could not say what it appeared to be called: "where am I?" had no answer
// unless you happened to be teaching it something.
//
// The argument for that gate was that "a Place's name is read off somebody's screen, and passive
// observation has no business WRITING THAT DOWN". The second half is still policy. The first half
// is a different question that was being answered with the same word — and, measured, the durable
// write was already licensed one level up: the only non-transient consumer of this result is
// `PlaceNamesToRecord`, which is called from inside `Runner.establishPlace`, and that returns at
// its first line unless `Episode.EstablishPlaces` is set. This was a second gate, on the inference.
//
// What protects a person is the shape filter below, and it never depended on the licence.
//
// Putting a licence argument back in front of the inference must fail THIS test, and
// cmd/director's TestTheSamplerNamesThePlaceWhoeverIsWatching, which enters through the
// production method rather than this free function.
//
// It must NOT be assumed to fail cmd/director's TestObservationNamesThePlaceWithNoLearnEpisode.
// That was the first claim written here and it was wrong: measured by running the mutation, that
// test stayed green, because its fixture (`dryNamed`) hands the frame an `appearsCalled` directly
// and never reaches this function. It holds the PERSISTENCE half of the same inversion, which is
// a real and separate claim, and its own comment now says so.
func TestPassiveObservationMayInferAName(t *testing.T) {
	if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
		selected(directorapi.RoleListItem, "Home"),
	}); got != "Home" {
		t.Errorf("observation could not say what the Place is called (got %q). Recognising where "+
			"you are and writing it down are different operations, and only the second needs a "+
			"licence.", got)
	}
}

// The shape filter is unconditional, and it is now the ONLY thing between a person's screen and a
// name Marco will repeat back. It always was unconditional; what changed is that there is no
// longer a licence in front of it, so these cases carry the whole weight.
func TestTheShapeFilterDoesNotAdmitPrivateText(t *testing.T) {
	for _, word := range []string{"@BeeTeaSea", "chris.haynes2112@gmail.com", "https://x.com"} {
		if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{
			selected(directorapi.RoleTreeItem, word),
		}); got != "" {
			t.Errorf("%q was admitted as a Place name", word)
		}
	}
}

// ── presentation ──────────────────────────────────────────────────────────────

func namedPlaceSubject(called, semantic string) observe.RememberedSubject {
	return observe.RememberedSubject{
		Application: "settings", Called: called, Semantic: semantic,
		Structure: observe.StructureSignature{
			Subject: observe.SubjectState,
			Roles:   map[string]int{"button": 18, "group": 27, "text": 49},
		},
	}
}

// The Audience's word wins. Always.
//
// Deleting the Called branch, or ordering inference above it, must fail this.
func TestAnAudienceNameBeatsAnInferredName(t *testing.T) {
	if got := observe.PlaceWords(namedPlaceSubject("Advanced Mouse", "Mouse")); got != "Advanced Mouse" {
		t.Errorf("presented as %q; a person said Advanced Mouse and Marco guessed Mouse", got)
	}
}

// An inference beats a description of what the screen is made of.
func TestAnInferredNameBeatsTheStructuralDescription(t *testing.T) {
	got := observe.PlaceWords(namedPlaceSubject("", "Mouse"))
	if got != "Mouse" {
		t.Errorf("presented as %q, want Mouse", got)
	}
	if strings.Contains(got, "things on it") {
		t.Error("the diagnostic description is being read to a person as a name")
	}
}

// With neither, the structural description is the floor — and it still says something.
func TestWithNoNameTheDescriptionIsStillShown(t *testing.T) {
	got := observe.PlaceWords(namedPlaceSubject("", ""))
	if got == "" {
		t.Fatal("a Place nobody has named describes itself as nothing")
	}
	if strings.Contains(got, "subj_") {
		t.Errorf("an identifier reached a person: %q", got)
	}
}

// A name never carries counts, geometry or identifiers.
//
// Those are evidence. Putting them in the name is what recreated the twin problem in another form.
func TestAnInferredNameCarriesNoEvidence(t *testing.T) {
	got := observe.PlaceWords(namedPlaceSubject("", "Mouse"))
	for _, leak := range []string{"148", "things", "subj_", "hwnd", "uia:", "x=", "w="} {
		if strings.Contains(got, leak) {
			t.Errorf("the name %q contains %q", got, leak)
		}
	}
}

// Inference is not Audience authority, and the record keeps them apart.
//
// The distinction has to survive persistence: a later reader deciding whether a play may say a
// name out loud must be able to tell what somebody said from what Marco guessed.
//
// Writing an inferred name into Called must fail this.
func TestAnInferredNameIsNotRecordedAsSomethingSomebodySaid(t *testing.T) {
	p := namedPlaceSubject("", "Mouse")
	if p.Called != "" {
		t.Errorf("an inferred name landed in Called (%q), where it reads as the Audience's "+
			"own word forever after", p.Called)
	}
	if p.Semantic != "Mouse" {
		t.Errorf("the inference is recorded as %q", p.Semantic)
	}
}

// ── from evidence to a durable Place ──────────────────────────────────────────

// A name seen ONCE does not stick.
//
// The same rule ADR-073 applies to compositions, for the same reason: a transition frame can carry
// the name of the page being left, so the word that recurs is the screen's.
func TestANameSeenOnceDoesNotStick(t *testing.T) {
	once := observe.ScreenState{PlaceNames: map[string]int{"Bluetooth": 1}}
	if got := observe.SettledPlaceNameFor(once); got != "" {
		t.Errorf("one sighting became the Place's name (%q)", got)
	}
	twice := observe.ScreenState{PlaceNames: map[string]int{"Mouse": 2}}
	if got := observe.SettledPlaceNameFor(twice); got != "Mouse" {
		t.Errorf("a name seen twice settled as %q, want Mouse", got)
	}
}

// A tie is left unresolved rather than decided by map order.
//
// Deciding one would make a Place's name depend on how a hash table happened to walk, which is the
// non-determinism ADR-073 refused for compositions.
func TestATiedNameIsLeftUnresolved(t *testing.T) {
	tied := observe.ScreenState{PlaceNames: map[string]int{"Mouse": 3, "Touchpad": 3}}
	for i := 0; i < 40; i++ {
		if got := observe.SettledPlaceNameFor(tied); got != "" {
			t.Fatalf("a tie resolved to %q on run %d", got, i)
		}
	}
}

// The most-recurrent word wins.
func TestTheMostRecurrentNameWins(t *testing.T) {
	st := observe.ScreenState{PlaceNames: map[string]int{"Mouse": 9, "Bluetooth": 2}}
	if got := observe.SettledPlaceNameFor(st); got != "Mouse" {
		t.Errorf("settled on %q, want Mouse", got)
	}
}

// Two Actors disagreeing produce no name.
//
// Merging is where Actor evidence meets. Accessibility saying "Mouse" while OCR says
// "Bluetooth & devices" is Marco not knowing.
func TestActorsDisagreeingProduceNoName(t *testing.T) {
	a := observe.SemanticEvidence{PlaceName: "Mouse", Observed: true}
	b := observe.SemanticEvidence{PlaceName: "Bluetooth & devices", Observed: true}
	if got := a.Merge(b).PlaceName; got != "" {
		t.Errorf("disagreement became the confident name %q", got)
	}
	if got := b.Merge(a).PlaceName; got != "" {
		t.Errorf("disagreement became %q in the other order", got)
	}
	// Agreement is one name, and one Actor with nothing to say does not veto the other.
	if got := a.Merge(observe.SemanticEvidence{PlaceName: "Mouse"}).PlaceName; got != "Mouse" {
		t.Errorf("two Actors agreeing produced %q", got)
	}
	if got := a.Merge(observe.SemanticEvidence{Observed: true}).PlaceName; got != "Mouse" {
		t.Errorf("a silent Actor erased the name (%q)", got)
	}
}

// ── a play may say the name Marco worked out ──────────────────────────────────

// A PLAY MAY SAY THE NAME MARCO WORKED OUT.
//
// # The live failure
//
// Both screens of a demonstrated route carried correct inferred names — `Home` and
// `Bluetooth & devices`, from the Actor's own evidence, with no Audience name at all. Lowering
// read `Called`, found it empty, and refused `screen_unnamed`; the Audience was asked to name a
// screen Marco had already named, with no way to decline.
//
// `Called` still wins. The record still keeps the two apart. But a play refers to a screen by a
// word a reader understands, and it does not need that word to have come from a person.
func TestAPlayMaySayTheNameMarcoWorkedOut(t *testing.T) {
	audience := namedPlaceSubject("Advanced Mouse", "Mouse")
	if got := audience.Name(); got != "Advanced Mouse" {
		t.Errorf("a play would say %q; the Audience's word must win", got)
	}
	inferred := namedPlaceSubject("", "Bluetooth & devices")
	if got := inferred.Name(); got != "Bluetooth & devices" {
		t.Errorf("a play would say %q for a screen Marco named itself", got)
	}
	nameless := namedPlaceSubject("", "")
	if got := nameless.Name(); got != "" {
		t.Errorf("a screen with no name of any kind reports %q; a play cannot say where it "+
			"starts and the refusal is the honest answer", got)
	}
	// The record still distinguishes them. Name() is for SAYING; the fields are for KNOWING.
	if inferred.Called != "" {
		t.Error("the inference leaked into the Audience's field")
	}
}

// ── the section is not the Place ──────────────────────────────────────────────

// A SUB-PAGE IS NOT NAMED AFTER ITS SECTION.
//
// # The live failure
//
// Settings Mouse was durably named `Bluetooth & devices` — the same word as the actual Bluetooth
// page. Two distinct Places answering to one name, so `SubjectNamed` correctly resolved neither,
// and the generated Play could not find the screens it names. Measured:
//
//	"Bluetooth & devices"  resolves=false
//	subj_c3e77b6f5c01  "Bluetooth & devices"
//	subj_71727a02470f  "Bluetooth & devices"   ← Mouse
//
// The rule was right about the evidence and wrong about the question: a selected navigation item
// names the SECTION. The tree says which page you are on separately — on Mouse, two sibling
// buttons under one parent read `Bluetooth & devices` and `Mouse`; on Home, one reads `Home`.
func TestASubPageIsNotNamedAfterItsSection(t *testing.T) {
	// The rail says the section; the trail says the page.
	e := selected(directorapi.RoleListItem, "Bluetooth & devices")
	e.Trail = []string{"Bluetooth & devices", "Mouse"}
	if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{e}); got != "Mouse" {
		t.Errorf("the sub-page is called %q, want Mouse. Naming it after its section gives "+
			"two Places one name, and neither then resolves.", got)
	}
}

// A section root is legitimately named after itself.
//
// One trail entry means the section IS the page, and the two being equal there is correct rather
// than a collision to avoid.
func TestASectionRootKeepsItsOwnName(t *testing.T) {
	e := selected(directorapi.RoleListItem, "Home")
	e.Trail = []string{"Home"}
	if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{e}); got != "Home" {
		t.Errorf("the section root is called %q, want Home", got)
	}
}

// A deeper trail than anything measured names nothing.
//
// Three entries and no measured rule for which is the page. Guessing would be inventing one, and
// a wrong name is worse than none because it is trusted — and because a duplicate makes every
// Place sharing it unresolvable.
func TestADeeperTrailNamesNothing(t *testing.T) {
	e := selected(directorapi.RoleListItem, "System")
	e.Trail = []string{"System", "Display", "Advanced"}
	if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{e}); got != "" {
		t.Errorf("a three-entry trail produced the confident name %q", got)
	}
}

// A trail entry is admitted on the same terms as any other name.
func TestATrailEntryMeetsTheShapeFilter(t *testing.T) {
	e := selected(directorapi.RoleListItem, "Direct Messages")
	e.Trail = []string{"Direct Messages", "@BeeTeaSea"}
	if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{e}); got != "" {
		t.Errorf("a private-looking trail entry was admitted as %q", got)
	}
}

// With no trail the selected item still names the Place, as it did for Chrome and Discord.
func TestNoTrailLeavesTheSelectedItemAsTheName(t *testing.T) {
	e := selected(directorapi.RoleTreeItem, "Downloads")
	if got := observe.AdmittedPlaceName([]observe.PlaceNameEvidence{e}); got != "Downloads" {
		t.Errorf("with no trail the Place is called %q, want Downloads", got)
	}
}
