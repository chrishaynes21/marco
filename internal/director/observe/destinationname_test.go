package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A SCREEN CARRIES SEVERAL TRUE NAMES AT ONCE.
//
// Measured live on 2026-08-29, `director name-probe`, Windows Settings' Printers page at 1500px:
//
//	Settings                the application shell
//	Bluetooth & devices     the section, and the selected item in the navigation rail
//	Printers & scanners     where the person actually is
//
// None of those is wrong. Only the last may be a Place's name. The rule that separates them is
// the TRAIL: the group of sibling buttons that contains the selected word is the path you took,
// and the entry that is not the selected word is the leaf you are standing on.
//
// These fixtures are transcribed from that probe run, so they are the shapes production actually
// meets rather than shapes chosen to make a rule look good.

func atLevel(role directorapi.ElementRole, label string, trail ...string) observe.PlaceNameEvidence {
	return observe.PlaceNameEvidence{
		Role: role, Label: label, Confidence: 1, Selected: true, Trail: trail,
	}
}

// THE SECTION IS NOT THE DESTINATION, AND THE TRAIL SAYS WHICH IS WHICH.
//
// The live shape: the rail reports `Bluetooth & devices` selected, and a group of sibling buttons
// reads `Bluetooth & devices` and `Printers & scanners`. The person is on Printers.
func TestTheTrailLeafNamesTheDestinationAndTheAncestorNamesTheSection(t *testing.T) {
	naming := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		atLevel(directorapi.RoleListItem, "Bluetooth & devices",
			"Bluetooth & devices", "Printers & scanners"),
	})
	if naming.Name != "Printers & scanners" {
		t.Fatalf("the destination is %q, want Printers & scanners — the selected rail item "+
			"names the section, and the trail holds the page", naming.Name)
	}
	// And both levels are REPORTED, because a diagnostic that showed only the winner cannot
	// explain a wrong name.
	levels := map[string]observe.NameLevel{}
	for _, c := range naming.Claims {
		levels[c.Value] = c.Level
	}
	if levels["Bluetooth & devices"] != observe.LevelSection {
		t.Errorf("the section was reported as %q, want section", levels["Bluetooth & devices"])
	}
	if levels["Printers & scanners"] != observe.LevelDestination {
		t.Errorf("the destination was reported as %q, want destination",
			levels["Printers & scanners"])
	}
}

// TWO DESTINATIONS UNDER ONE SECTION ARE NAMED APART.
//
// The same section, two leaves. If the section could win, every page under `Bluetooth & devices`
// would carry one name — which is exactly what the Printers trap below produces.
func TestTwoDestinationsUnderOneSectionAreNamedApart(t *testing.T) {
	mouse := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		atLevel(directorapi.RoleListItem, "Bluetooth & devices",
			"Bluetooth & devices", "Mouse"),
	}).Name
	printers := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		atLevel(directorapi.RoleListItem, "Bluetooth & devices",
			"Bluetooth & devices", "Printers & scanners"),
	}).Name
	if mouse != "Mouse" || printers != "Printers & scanners" {
		t.Fatalf("two pages under one section named %q and %q", mouse, printers)
	}
	if mouse == printers {
		t.Fatal("both pages took the section's name")
	}
}

// A SELECTED ITEM WITH NO TRAIL IS ADMITTED, AND THE TWO MEASURED CONSEQUENCES ARE OPPOSITE.
//
// This is the finding 37K exists to record, and it is a NEGATIVE result stated as a test so that
// nobody has to rediscover it.
//
// Both cases below reach the rule as the identical shape — one selected navigable item, no trail
// containing it — and their correct answers differ:
//
//	Settings Home     selected `Home`, no trail group holds it      -> "Home" is RIGHT
//	Settings Printers selected `Bluetooth & devices` at 850px,      -> "Bluetooth & devices"
//	  at 850px        the trail collapsed into an overflow so no       is WRONG; the page is
//	                  group holds the selected word                    Printers & scanners
//
// Measured, both, with `director name-probe`. At 850 the sibling group had become
// `[More, Printers & scanners]` — the ancestor replaced by an overflow control — so the lookup
// that finds the path found nothing, and the section was admitted by default.
//
// Requiring corroboration was implemented and measured: it fixes Printers and VS Code (which
// names itself `Terminal (Ctrl+`)`, a keyboard hint, from an uncorroborated selected tab) and
// it leaves Settings HOME unnamed, which is a worse trade on the most ordinary page in the
// application. ADR-076 recorded a one-entry trail on Home; this Windows build no longer produces
// one.
//
// So the rule is unchanged, and this test states what that costs. A future phase that separates
// these two must change this test deliberately and say what new evidence it found.
func TestATrailLessSelectionIsAdmittedAndThatIsNotAlwaysRight(t *testing.T) {
	home := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		atLevel(directorapi.RoleListItem, "Home"),
	})
	if home.Name != "Home" {
		t.Errorf("Settings Home is called %q, want Home. This is the case that makes the "+
			"trail-less selection admissible, and losing it is why corroboration was "+
			"rejected.", home.Name)
	}

	// The identical shape, and the wrong answer. `Bluetooth & devices` is the SECTION; the
	// page is Printers & scanners, whose name is on screen as a button the rule cannot reach
	// because the group no longer holds the selected word.
	trapped := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		atLevel(directorapi.RoleListItem, "Bluetooth & devices"),
	})
	if trapped.Name != "Bluetooth & devices" {
		t.Errorf("the Printers-at-850 trap now yields %q rather than the section name.\n"+
			"That is the outcome 37K wanted and could not buy safely, so it did not arrive "+
			"by accident: check that Settings Home is still named, and update this test with "+
			"the evidence that earned the distinction.", trapped.Name)
	}
}

// A VALUE IS STILL NOT A DESTINATION, AND THE PROBE SAYS SO IN WORDS.
//
// Live at 1500px the Mouse page reports four selected list items: three are combo-box values
// (`Left`, `Multiple lines at a time`, `Down motion scrolls down`) and one is the rail. Naming
// the screen after a scroll setting would be confidently wrong, and the reason has to survive
// into the diagnostic or the next person reads the rejection as a failure to see the item.
func TestAValueIsRejectedWithAReasonAPersonCanRead(t *testing.T) {
	value := observe.PlaceNameEvidence{
		Role: directorapi.RoleListItem, Label: "Multiple lines at a time", Confidence: 1,
		Selected: true, InsideValueChooser: true,
	}
	naming := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		value, atLevel(directorapi.RoleListItem, "Bluetooth & devices",
			"Bluetooth & devices", "Mouse"),
	})
	if naming.Name != "Mouse" {
		t.Fatalf("the destination is %q, want Mouse", naming.Name)
	}
	for _, c := range naming.Claims {
		if c.Value != "Multiple lines at a time" {
			continue
		}
		if c.Admitted {
			t.Error("a combo box's value was admitted as a destination")
		}
		if c.Why == "" {
			t.Error("the value was rejected without saying why; a probe that shows a " +
				"missing candidate and no reason sends somebody looking for a perception bug")
		}
		return
	}
	t.Error("the rejected value was not reported at all, so the probe cannot explain itself")
}

// THE APPLICATION IS NOT THE DESTINATION.
//
// Every Settings page carries the window title `Settings`. It is a true name for the application
// and says nothing about where in it you are — which is why the naming rule never reads a title,
// and why `AdmittedPlaceName` takes element evidence rather than a window.
//
// Buttons cannot name a place at all, and the shell's controls are buttons.
func TestTheApplicationShellCannotNameTheDestination(t *testing.T) {
	for _, word := range []string{"Settings", "Close Settings", "Minimize Settings", "More"} {
		naming := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
			atLevel(directorapi.RoleButton, word),
		})
		if naming.Name != "" {
			t.Errorf("the shell control %q named the destination %q", word, naming.Name)
		}
	}
}

// A TRAIL DEEPER THAN ANYTHING MEASURED NAMES NOTHING.
//
// Two entries is a section and its page, and the rule reads that. Three is a hierarchy nobody has
// measured, and picking one of them would be inventing a rule rather than reading evidence — the
// trail is a SET, the fused world is a map with no order, so there is not even a "last" entry to
// prefer. Failing closed is the only honest answer, and it needs its own fixture because every
// live surface measured so far has one entry or two.
func TestATrailDeeperThanAnythingMeasuredNamesNothing(t *testing.T) {
	naming := observe.ExplainPlaceName([]observe.PlaceNameEvidence{
		atLevel(directorapi.RoleListItem, "System",
			"System", "Display", "Advanced display"),
	})
	if naming.Name != "" {
		t.Errorf("a three-entry trail produced the confident name %q.\n"+
			"Nothing orders a trail, so choosing among three entries is a guess with a "+
			"one-in-three chance of naming a Place after somewhere the person is not.",
			naming.Name)
	}
	if naming.Why == "" {
		t.Error("it refused without saying why")
	}
}
