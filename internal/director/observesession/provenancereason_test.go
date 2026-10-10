package observesession

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A REFUSAL WITHOUT A REASON IS NOT A REPORT.
//
// Experiment-024 measured the cost. A four-minute live session printed:
//
//	Provenance: 27 of 94 samples refused evidence (2317 observations quarantined)
//	  ocr
//
// Two thousand three hundred readings refused, and the line that was supposed to say why said
// nothing at all. From the report there was no way to tell a missing engine from an engine
// whose evidence did not describe the window — and those send a reader to opposite places.
//
// ProviderOutcome.Reason is filled when the CALL goes wrong. A provenance refusal is the other
// case: the call succeeded, produced evidence, and could not show which window the evidence was
// about. Nothing upstream had a sentence for that, so nothing said one.

func TestARefusedProviderSaysWhyItWasRefused(t *testing.T) {
	// The measured case exactly: a provider outside the targeted interface, on a cycle
	// that WAS pinned. The collector wraps it with an unknown observed target and nothing
	// is assumed on its behalf, so the guard refuses everything it produced.
	var s Stats
	s.foldProvenance(observe.Sample{
		Providers: []observe.ProviderSummary{{
			Name: "ocr", State: "ok", Expected: 4, Observed: 0,
			Quarantined: 86, Proven: false,
		}},
	})

	if s.ProvenanceRefusals != 1 {
		t.Fatalf("refusals = %d, want 1", s.ProvenanceRefusals)
	}
	if s.ProvenanceQuarantined != 86 {
		t.Errorf("quarantined = %d, want 86", s.ProvenanceQuarantined)
	}
	why := s.RefusedProviders["ocr"]
	if strings.TrimSpace(why) == "" {
		t.Fatal("86 observations were refused and the report has nothing to say about " +
			"why. This is the live defect: a named provider, a quarantine count, and an " +
			"empty string")
	}
	// The two generations are the whole of the answer, and the one that was MISSING is the
	// finding — so the sentence has to distinguish "could not say" from "said the wrong
	// one" rather than printing a number and leaving the reader to guess which.
	if !strings.Contains(why, "cannot say which window") {
		t.Errorf("the reason does not say that the provider established no target:\n  %s",
			why)
	}
	if !strings.Contains(why, "generation 4") {
		t.Errorf("the reason does not name the generation the cycle was pinned to; a "+
			"verdict with neither number in view is one a reader has to take on faith:"+
			"\n  %s", why)
	}
}

// The other refusal, which is a completely different finding: the provider DID establish a
// target and it was a different window. Same panel, same count, and the remedy is not related.
func TestAProviderThatObservedAnotherWindowSaysSo(t *testing.T) {
	var s Stats
	s.foldProvenance(observe.Sample{
		Providers: []observe.ProviderSummary{{
			Name: "vision", State: "ok", Expected: 9, Observed: 7,
			Quarantined: 12, Proven: false,
		}},
	})
	why := s.RefusedProviders["vision"]
	if !strings.Contains(why, "generation 7") || !strings.Contains(why, "9") {
		t.Errorf("a cross-generation refusal must name BOTH windows:\n  %s", why)
	}
	if strings.Contains(why, "cannot say which window") {
		t.Errorf("a provider that proved generation 7 is reported as having proved "+
			"nothing:\n  %s", why)
	}
}

// And a reason the provider gave for itself is never overwritten. The guard's sentence is a
// fallback for the case nothing else had words for, not a replacement for what a failing
// provider said about its own failure.
func TestAProvidersOwnReasonSurvives(t *testing.T) {
	var s Stats
	s.foldProvenance(observe.Sample{
		Providers: []observe.ProviderSummary{{
			Name: "ocr", State: "failed", Reason: "no OCR engine is installed",
			Expected: 4, Quarantined: 3, Proven: false,
		}},
	})
	if got := s.RefusedProviders["ocr"]; got != "no OCR engine is installed" {
		t.Errorf("the provider's own reason was replaced by the guard's: %q", got)
	}
}

// A refusal still requires evidence to refuse.
//
// The rule that was already here, kept under test beside the new sentence: a provider that
// produced nothing has not had anything refused, and counting it reported "91 of 91 samples
// refused evidence (0 observations quarantined)" on the first live run — alarming, prominent
// and false.
func TestAProviderThatProducedNothingIsNotARefusal(t *testing.T) {
	var s Stats
	s.foldProvenance(observe.Sample{
		Providers: []observe.ProviderSummary{{
			Name: "ocr", State: "empty", Expected: 4, Quarantined: 0, Proven: false,
		}},
	})
	if s.ProvenanceRefusals != 0 {
		t.Errorf("refusals = %d from a provider that produced nothing", s.ProvenanceRefusals)
	}
	if len(s.RefusedProviders) != 0 {
		t.Errorf("refused providers = %v, want none", s.RefusedProviders)
	}
}
