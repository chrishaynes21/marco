package main

import "testing"

// The plugin boundary is where a model's own words stop.
//
// This has now failed twice in production, identically both times: a detector ran perfectly and
// every one of its detections was discarded as an unknown class. Grounding DINO lost twelve of
// thirteen; ScreenParser produced fourteen and lost all fourteen. Neither was a broken model —
// both were a producer and a consumer that were connected and disagreed about vocabulary.
//
// See [[Wiring-Tests]]: integration correctness requires invocation, consumption AND contract
// compatibility, and this file is the contract-compatibility gate for vision classes.

// marcoVocabulary is the closed set every consumer above this plugin understands. Duplicated
// from internal/director/perception/providers/vision deliberately — the plugin is a separate
// module, and a test that imported the consumer's list could not detect the two drifting apart.
var marcoVocabulary = map[string]bool{
	"button": true, "icon": true, "text": true, "field": true, "checkbox": true,
	"radio": true, "slot": true, "bar": true, "panel": true, "menu": true, "image": true,
}

// The mutation this catches: labelFor returning the model's native class.
func TestVocabularyIsNormalisedAtThePluginBoundary(t *testing.T) {
	// ScreenParser's own words, exactly as they appear in the model's embedded metadata.
	native := []string{"Button", "Heading", "Text", "Icon", "Menu Item", "List Item"}
	labels := native
	for i := range labels {
		got := labelFor(i, labels)
		if !marcoVocabulary[got] {
			t.Errorf("labelFor(%q) = %q, which is not in Marco's vocabulary — every "+
				"detection carrying it will be refused downstream as an unknown class",
				native[i], got)
		}
	}
}

// Case and separators must not decide whether a detection survives. `Button` vs `button` is the
// difference that cost ScreenParser all fourteen of its detections.
func TestClassMatchingIgnoresCaseAndSeparators(t *testing.T) {
	spellings := []string{"List Item", "list_item", "List-Item", "listitem", "LIST ITEM"}
	want, ok := normaliseClass(spellings[0])
	if !ok {
		t.Fatalf("%q did not map at all", spellings[0])
	}
	for _, s := range spellings[1:] {
		got, ok := normaliseClass(s)
		if !ok || got != want {
			t.Errorf("normaliseClass(%q) = %q (mapped=%v), want %q", s, got, ok, want)
		}
	}
}

// Unknown must stay unknown. The temptation this blocks is mapping an ambiguous class onto
// `button` to lift a nameable score — precisely the inflation NameablePrecision exists to catch.
func TestAnUnmappedClassKeepsItsNativeWordAndIsRefused(t *testing.T) {
	for _, native := range []string{"Kitchen Sink", "boost_meter", ""} {
		got, mapped := normaliseClass(native)
		if mapped {
			t.Errorf("normaliseClass(%q) claimed a mapping to %q; an unrecognised class "+
				"must be refused, not guessed at", native, got)
		}
		if got != native {
			t.Errorf("normaliseClass(%q) = %q; an unmapped class keeps its own word so the "+
				"refusal downstream names the real cause", native, got)
		}
	}
}

// A model whose vocabulary is mostly unmapped produces mostly refused detections. That is worth
// knowing at startup rather than inferring from a benchmark result of zero.
func TestVocabularyCoverageCountsBothSides(t *testing.T) {
	mapped, unknown := VocabularyCoverage([]string{"Button", "Panel", "Kitchen Sink"})
	if mapped != 2 || unknown != 1 {
		t.Errorf("coverage = %d mapped / %d unknown, want 2/1", mapped, unknown)
	}
}

// Every value in the table must itself be a Marco class. A typo here would map a detection onto
// a word no consumer knows, reproducing the original defect through the very file that fixes it.
func TestEveryMappingTargetsTheClosedVocabulary(t *testing.T) {
	for native, marco := range marcoClass {
		if !marcoVocabulary[marco] {
			t.Errorf("%q maps to %q, which is not a Marco vision class", native, marco)
		}
	}
}
