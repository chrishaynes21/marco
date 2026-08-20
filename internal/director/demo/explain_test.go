package demo_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// TestTheTraceShowsEveryStageInOrder.
//
//	Trace should show: observed semantic actions → recovered goal → recovered
//	parameters → generalized procedure → validation → registry
func TestTheTraceShowsEveryStageInOrder(t *testing.T) {
	d := renameDemo()
	trace := demo.Extract(d).Trace(d)

	stages := []string{
		"Observed semantic actions", "Recovered goal", "Recovered parameters",
		"Generalized procedure", "Validation", "Registry",
	}
	at := -1
	for _, s := range stages {
		i := strings.Index(trace, s)
		if i < 0 {
			t.Fatalf("the trace omits %q:\n%s", s, trace)
		}
		if i < at {
			t.Errorf("%q appears out of order:\n%s", s, trace)
		}
		at = i
	}
	// And it says plainly that nothing was installed.
	if !strings.Contains(trace, "not installed") {
		t.Errorf("the trace does not say the proposal is uninstalled:\n%s", trace)
	}
}

// TestTheTraceOfARefusalStillShowsTheStages — the case a reader most needs it for.
func TestTheTraceOfARefusalStillShowsTheStages(t *testing.T) {
	d := renameDemo()
	d.Steps[1].Verified = false
	d.Steps[1].Status = directorapi.ActionFailed

	trace := demo.Extract(d).Trace(d)
	if !strings.Contains(trace, "REFUSED") {
		t.Errorf("the trace does not report the refusal:\n%s", trace)
	}
	if !strings.Contains(trace, "nothing can be installed") {
		t.Errorf("the trace does not say the registry was untouched:\n%s", trace)
	}
}
