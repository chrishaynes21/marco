package observesession_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Part 10: a transition must not become a covert recording artifact.
//
// A relationship is the FIRST thing in this system that survives a restart and accumulates
// across days. Everything the passive record refuses is refused twice as hard here, because a
// session's evidence is discarded when the session ends and a durable edge is not.
//
// Two tests, and the structural one is the load-bearing half: a value test proves what one
// session happened to produce, and a type test proves what no session can produce.

// forbiddenInTransitionRecord is the vocabulary of things a durable observation must never name.
//
// The same list the session record's own privacy test uses, plus the ones that only become
// tempting once transitions exist: a pointer trail, a timestamp of a keystroke, a screenshot of
// "what it looked like before".
var forbiddenInTransitionRecord = []string{
	// physical input
	"keycode", "scancode", "vkcode", "rawkey", "keystroke", "keypress", "keyname",
	"character", "charcode", "rune", "typed", "clipboard", "deviceid", "device_id",
	// screen content
	"ocr", "rawtext", "raw_text", "screenshot", "image", "pixels", "bitmap", "caption",
	"tooltip", "placeholder", "innertext", "inner_text",
	// window and platform identity
	"title", "windowtitle", "window_title", "hwnd", "handle", "processid", "process_id",
	"runtimeid", "runtime_id", "elementid", "element_id",
	// absolute geometry: a trail of where somebody's cursor went
	"screenx", "screeny", "desktopx", "desktopy", "absolute",
}

// allowedInTransitionRecord are names containing a forbidden word that are demonstrably
// something else. Every entry is a hole in the rule, so each one earns its line.
var allowedInTransitionRecord = map[string]bool{
	// RelationshipEvidence.Preceded is a map of closed NavIntent to a COUNT. The key type
	// is checked separately by the closed-vocabulary test below.
	"RememberedRelationship.Preceded": true,
}

type walkedTypes map[reflect.Type]bool

func checkTransitionNames(t *testing.T, rt reflect.Type, path string, seen walkedTypes) {
	t.Helper()
	for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
		rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {

		if rt.Kind() == reflect.Map {
			checkTransitionNames(t, rt.Key(), path+"[key]", seen)
		}
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct || seen[rt] {
		return
	}
	seen[rt] = true
	for i := range rt.NumField() {
		f := rt.Field(i)
		at := path + "." + f.Name
		if !allowedInTransitionRecord[at] {
			low := strings.ToLower(f.Name)
			for _, bad := range forbiddenInTransitionRecord {
				if strings.Contains(low, bad) {
					t.Errorf("%s names %q. A relationship survives every session that "+
						"could have contradicted it; anything it can hold, it holds "+
						"forever", at, bad)
				}
			}
		}
		checkTransitionNames(t, f.Type, at, seen)
	}
}

// The durable transition record has nowhere to put private content.
func TestTheDurableTransitionRecordHasNowhereToPutContent(t *testing.T) {
	checkTransitionNames(t, reflect.TypeOf(observe.RememberedRelationship{}),
		"RememberedRelationship", walkedTypes{})
	checkTransitionNames(t, reflect.TypeOf(observe.RelationshipObservation{}),
		"RelationshipObservation", walkedTypes{})
	checkTransitionNames(t, reflect.TypeOf(observe.ScreenTransition{}),
		"ScreenTransition", walkedTypes{})
}

// What a transition CAN name is a closed vocabulary of meanings, never a key.
//
// The rule ADR-013 exists for, checked at the durable boundary rather than at the producer:
// the physical key dies in the platform adapter, and what crosses here is `confirm`.
func TestATransitionNamesMeaningsAndNeverKeys(t *testing.T) {
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 4)...)
	frames = append(frames, pressThen(screenfixture.Settings(), 6,
		observe.NavConfirm, observe.NavDown)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	if len(sh.Transitions) == 0 {
		t.Fatal("no transition; nothing here is being tested")
	}
	for _, tr := range sh.Transitions {
		for intent := range tr.Preceded {
			if !intent.Known() {
				t.Errorf("a transition named %q, which is not in the navigation vocabulary",
					intent)
			}
		}
		for _, s := range tr.Sequences {
			for _, intent := range s.Intents {
				if !intent.Known() {
					t.Errorf("a remembered order named %q", intent)
				}
			}
		}
	}
}

// Nothing observed can reach the durable record as text, however it arrived.
//
// The VALUE half: a session is driven with deliberately dangerous-looking content in every
// place a producer could put it, and the serialised result is searched for it. Between this and
// the type walk above, neither a value nor a field can carry it.
func TestNoObservedContentSurvivesIntoTheTransitionRecord(t *testing.T) {
	const secret = "AccountName-SuperSecret-42"

	// A session whose regions carry a role that looks like leaked text, and whose inputs
	// carry a pointer position. Both are things a careless producer might supply.
	poisoned := func() []observe.ShadowRegion {
		out := screenfixture.Settings()
		out = append(out, observe.ShadowRegion{
			Role:   secret, // a role is a closed vocabulary; this is not one
			Region: observe.Region{X: 0.5, Y: 0.5, Width: 0.1, Height: 0.1},
		})
		return out
	}
	withPointer := func(regions []observe.ShadowRegion, n int) []frame {
		out := pressThen(regions, n, observe.NavConfirm)
		out[0].inputs = append(out[0].inputs, observe.InputEvent{
			Intent: observe.NavPoint, AtMS: 10,
			Where: observe.PointerAt{X: 0.42, Y: 0.77},
		})
		return out
	}

	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 4)...)
	frames = append(frames, withPointer(poisoned(), 6)...)

	got := run(t, script{frames: frames})

	// The whole terminal Result, as it would reach the protocol, the CLI and any fixture.
	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("encoding the result: %v", err)
	}
	if strings.Contains(string(blob), secret) {
		t.Errorf("content supplied as a role survived into the session record.\n" +
			"It would reach the protocol, the CLI, and anything that stored a fixture")
	}

	// A pointer POSITION is admitted only on a pointer press, and it is never part of a
	// transition's evidence — a trail of where somebody clicked is exactly the durable
	// surveillance artifact this milestone must not create.
	for _, tr := range got.Stats.Shadow.Transitions {
		trBlob, err := json.Marshal(tr)
		if err != nil {
			t.Fatalf("encoding a transition: %v", err)
		}
		if strings.Contains(string(trBlob), "0.42") ||
			strings.Contains(string(trBlob), "0.77") {
			t.Errorf("a pointer position reached a transition record: %s", trBlob)
		}
	}
}

// A durable relationship is bounded, however long somebody uses an application.
//
// The other half of "not a recording artifact": a record that grew with observations rather
// than with subjects would become a transcript by accumulation.
func TestTheDurableRecordIsBoundedNotAccumulated(t *testing.T) {
	dir := t.TempDir()
	// Many sittings, each with a DIFFERENT order before the same change, so the record is
	// being pushed to grow in the one place it could.
	orders := [][]observe.NavIntent{
		{observe.NavDown, observe.NavConfirm},
		{observe.NavConfirm},
		{observe.NavDown, observe.NavDown, observe.NavConfirm},
		{observe.NavLeft, observe.NavConfirm},
		{observe.NavRight, observe.NavConfirm},
		{observe.NavBack, observe.NavConfirm},
		{observe.NavDown, observe.NavLeft, observe.NavConfirm},
		{observe.NavUp, observe.NavConfirm},
	}
	for _, order := range orders {
		sitting(t, memoryAt(t, dir), visit(order...))
	}

	for _, r := range relationshipsIn(t, dir) {
		ev := r.Evidence()
		if len(ev.Sequences) > observe.MaxDurableSequences {
			t.Errorf("%d remembered orders, bound is %d",
				len(ev.Sequences), observe.MaxDurableSequences)
		}
		if len(ev.Preceded) > observe.MaxDurableIntents {
			t.Errorf("%d remembered intents, bound is %d",
				len(ev.Preceded), observe.MaxDurableIntents)
		}
		for _, s := range ev.Sequences {
			if len(s.Intents) > observe.MaxSequenceLength {
				t.Errorf("a remembered order is %d long, bound is %d",
					len(s.Intents), observe.MaxSequenceLength)
			}
		}
		t.Logf("after %d sittings: observations=%d sequences=%d intents=%d",
			len(orders), r.Observations, len(ev.Sequences), len(ev.Preceded))
	}
}

var _ = observesession.Result{}
