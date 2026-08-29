package observation

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE RULE NAMES NO SENSOR, AND MUST NOT LEARN TO.
//
// The tempting fix for a detector polluting screen identity is `if source == vision: ignore`.
// It would have worked, today, for this detector — and it would be a rule about a product
// rather than about evidence, so the next optional sensor would arrive and reintroduce the
// defect while the guard sat there looking like protection.
//
// What the rule is allowed to know is what KIND of thing a source is: whether it can describe an
// object, or only how the screen looks. That is `ActuatingSource`, it is one list, and adding a
// source to it is a visible decision about what Marco believes it may operate.
func TestTheCompositionRuleNamesNoSensor(t *testing.T) {
	src, err := os.ReadFile("kind.go")
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	for _, name := range []string{
		"SourceVision", "SourceOCR", "SourceModel", "SourceAccessibility", "SourceDOM",
		"SourceNative", "ScreenParser", "screenparser", "icon_detect",
	} {
		if strings.Contains(code, name) {
			t.Errorf("kind.go names %s.\nWhich sensor produced a detection is not the "+
				"question — whether anything that can describe an object accounted for what "+
				"this thing IS is. A rule keyed on a product's name protects against that "+
				"product and nothing else.", name)
		}
	}
}

// An element nobody recorded a source for keeps its place in the composition.
//
// The absence rule, and it is load-bearing rather than defensive: elements are CONSTRUCTED as
// well as observed — fixtures, a capability pack's enrichment, a hand-built query — and none of
// those record provenance. Reading "nobody wrote it down" as "only a camera saw it" would empty
// every composition built that way, silently, and the emptiness would look like a screen with
// nothing on it.
func TestAnElementWithNoProvenanceIsNotTreatedAsPixels(t *testing.T) {
	got := KindOf(directorapi.RoleButton, directorapi.Provenance{}, nil)
	if got != directorapi.KindDescribed {
		t.Errorf("an element with no provenance classified as %q, want %q — the same rule, "+
			"and the same reason, as Provenance.OnlyDescribesPixels denying on evidence "+
			"rather than on absence", got, directorapi.KindDescribed)
	}
}

// The three cases, one each, against the live population they were measured from.
func TestWhoNamedItDecidesWhatItCountsAs(t *testing.T) {
	claims := StructuralKindClaims([]Observation{
		NewElement(directorapi.Observation{
			ID: "a-button", Source: directorapi.SourceAccessibility, Role: directorapi.RoleButton,
		}),
		// Accessibility SAW this one and could not say what it was. Not a claim.
		NewElement(directorapi.Observation{
			ID: "a-blob", Source: directorapi.SourceAccessibility, Role: directorapi.RoleUnknown,
		}),
		// A detector naming something is never a claim about kind, whatever it says.
		NewElement(directorapi.Observation{
			ID: "v-blob", Source: directorapi.SourceVision, Role: directorapi.RoleIcon,
		}),
		NewElement(directorapi.Observation{
			ID: "v-alone", Source: directorapi.SourceVision, Role: directorapi.RoleIcon,
		}),
	})
	if claims["a-blob"] {
		t.Error("`unknown` from accessibility was counted as a claim about what the thing " +
			"is. It is not one — which is exactly why fusion lets a detector name it.")
	}
	if claims["v-blob"] || claims["v-alone"] {
		t.Error("a detector's role was counted as a structural claim")
	}
	if !claims["a-button"] {
		t.Fatal("accessibility naming a button is not a claim; nothing would ever be described")
	}

	ref := func(id string, s directorapi.ObservationSource) directorapi.Provenance {
		return directorapi.Provenance{Sources: []directorapi.ObservationReference{
			{Observation: directorapi.ObservationID(id), Source: s},
		}}
	}
	both := directorapi.Provenance{Sources: []directorapi.ObservationReference{
		{Observation: "a-blob", Source: directorapi.SourceAccessibility},
		{Observation: "v-blob", Source: directorapi.SourceVision},
	}}

	for _, c := range []struct {
		what string
		role directorapi.ElementRole
		p    directorapi.Provenance
		want directorapi.KindEvidence
	}{
		{"a button accessibility named", directorapi.RoleButton,
			ref("a-button", directorapi.SourceAccessibility), directorapi.KindDescribed},
		{"an element accessibility saw and a detector named", directorapi.RoleIcon,
			both, directorapi.KindPixelNamed},
		{"a detection with nothing beside it", directorapi.RoleIcon,
			ref("v-alone", directorapi.SourceVision), directorapi.KindPixelOnly},
		{"text accessibility reported", directorapi.RoleText,
			ref("a-note", directorapi.SourceAccessibility), directorapi.KindDescribed},
	} {
		if got := KindOf(c.role, c.p, claims); got != c.want {
			t.Errorf("%s classified as %q, want %q", c.what, got, c.want)
		}
	}
}
