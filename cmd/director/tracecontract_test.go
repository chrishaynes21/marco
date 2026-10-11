package main

import (
	"reflect"
	"strings"
	"testing"
)

// THE TRACE FORMAT IS A CONTRACT WITH A SEPARATE PROGRAM.
//
// `shadowtrace.go` says what this file enforces:
//
//	a replay that reconstructed a slot without its shapes would feed the tracker a
//	different screen from the one production fed it — and the whole purpose of this trace
//	is that production and replay are the same measurement.
//
// `cmd/perceptionreplay` is that replay. It cannot import this package — both are mains — so
// the two agree through JSON tag names and nothing else. Rename one here and the replay decodes
// that field to its zero value: a session that saw nothing, reported without an error, and
// indistinguishable from a genuine finding about a blind run.
//
// Experiment-025's symptom was a session that reported no selection. A recording that silently
// lost its shading reproduces that symptom exactly, which is why this is a frozen list rather
// than a comment asking for care.

// tracedSlotContract is every tag cmd/perceptionreplay reads, and what it reads it for.
//
// Adding a field to tracedSlot is free. Renaming or removing one of these is a format change
// that breaks an existing corpus, and it has to be done deliberately — update the replay, bump
// the recording format, and say so in the subsystem note.
var tracedSlotContract = map[string]string{
	"n":          "which look, and the order they are replayed in",
	"at_ms":      "the producer-relative clock the belief's timestamps are built from",
	"outcome":    "whether this look carried evidence at all",
	"generation": "the window epoch; geometry from two of them is two different worlds",
	"latency_ms": "what the inference cost, for the cadence a replay reports",
	"detections": "how much named structure survived admission",
	"reason":     "Marco's own words for a look that produced nothing",
	"regions":    "the named structure, with its shading",
	"shapes":     "the geometry nobody could name, which is most of a game screen",
	"inputs":     "what the person did, so a transition can be attributed",
}

// Every field the replay depends on is still called what it was called.
func TestTheTraceFormatIsTheReplayContract(t *testing.T) {
	tags := map[string]bool{}
	rt := reflect.TypeOf(tracedSlot{})
	for i := range rt.NumField() {
		tag := rt.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		tags[strings.Split(tag, ",")[0]] = true
	}

	for name, why := range tracedSlotContract {
		if !tags[name] {
			t.Errorf("the trace no longer writes %q (%s).\n"+
				"cmd/perceptionreplay reads that tag and cannot import this package, "+
				"so it will decode the field as a zero value and report a session "+
				"that observed nothing — with no error anywhere. Every recording "+
				"already on disk becomes unreplayable at the same moment.", name, why)
		}
	}

	// AND THE SHADING INSIDE THE NESTED TYPES, which is the field whose loss is least
	// visible: unshaded geometry is dropped before grouping, so a replay without it
	// produces candidates that are all permanently quiet — Experiment-025's exact
	// symptom, manufactured by the recorder.
	for _, c := range []struct {
		what string
		rt   reflect.Type
	}{
		{"observe.ShadowRegion", reflect.TypeOf(tracedSlot{}.Regions).Elem()},
		{"observe.ShadowShape", reflect.TypeOf(tracedSlot{}.Shapes).Elem()},
	} {
		found := false
		for i := range c.rt.NumField() {
			if strings.Split(c.rt.Field(i).Tag.Get("json"), ",")[0] == "shade" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s no longer carries a `shade` tag. It is the only evidence that "+
				"can say which row holds the mark, and a replay that loses it still "+
				"groups rows and still reports candidates — every one of them quiet",
				c.what)
		}
	}
}
