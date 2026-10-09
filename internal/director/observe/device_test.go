package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The device vocabulary is closed, and admission is what closes it.
//
// NavDevice's underlying type is a string, so a platform layer can construct one holding
// anything. The field was exempted from the keylogger name rule on the grounds that it holds
// three kinds of plastic; this is the half that makes that true at runtime rather than by
// convention.
func TestADeviceWordOutsideTheVocabularyIsCleared(t *testing.T) {
	var totals observe.ShadowTotals
	s := observe.ShadowSample{
		Detector: "test", Ran: true, TargetProven: true,
		Inputs: []observe.InputEvent{
			{Intent: observe.NavDown, Device: observe.NavDevice("Xbox Elite Series 2")},
			{Intent: observe.NavUp, Device: observe.DeviceController},
		},
	}
	totals.Add(s)

	// Admission runs on the way in; read it back off the sample the totals kept.
	for _, tr := range totals.Transitions {
		_ = tr
	}
	got := observe.AdmissibleInputsForTest(s.Inputs)
	if len(got) != 2 {
		t.Fatalf("admission dropped an event: %d of 2 survive", len(got))
	}
	if got[0].Device != "" {
		t.Errorf("an invented device word survived admission as %q.\nThe field is "+
			"exempted from the physical-identity name rule because it holds three words; "+
			"a product name passing through is that exemption being false.", got[0].Device)
	}
	if got[0].Intent != observe.NavDown {
		t.Error("the EVENT was dropped along with its device word.\nProvenance is an " +
			"attribution: losing it is not losing the observation, the same rule Target " +
			"already follows.")
	}
	if got[1].Device != observe.DeviceController {
		t.Errorf("a real device word was cleared: %q", got[1].Device)
	}
}

// Two producers merge into one time-ordered stream, and neither is lost.
func TestMergingTwoDevicesKeepsBoth(t *testing.T) {
	keys := []observe.InputEvent{
		{Intent: observe.NavDown, AtMS: 100, Device: observe.DeviceKeyboard},
		{Intent: observe.NavDown, AtMS: 300, Device: observe.DeviceKeyboard},
	}
	pad := []observe.InputEvent{
		{Intent: observe.NavUp, AtMS: 200, Device: observe.DeviceController},
		{Intent: observe.NavConfirm, AtMS: 400, Device: observe.DeviceController},
	}

	got := observe.MergeInputs(keys, pad)
	if len(got) != 4 {
		t.Fatalf("the merge produced %d of 4 events: %+v", len(got), got)
	}
	want := []int64{100, 200, 300, 400}
	for i, at := range want {
		if got[i].AtMS != at {
			t.Fatalf("merged out of order: %+v", got)
		}
	}
	devices := map[observe.NavDevice]int{}
	for _, e := range got {
		devices[e.Device]++
	}
	if devices[observe.DeviceKeyboard] != 2 || devices[observe.DeviceController] != 2 {
		t.Errorf("a producer was lost in the merge: %v", devices)
	}

	// And the degenerate cases, which is where a merge usually goes wrong.
	if n := len(observe.MergeInputs(nil, pad)); n != 2 {
		t.Errorf("merging into nothing produced %d events", n)
	}
	if n := len(observe.MergeInputs(keys, nil)); n != 2 {
		t.Errorf("merging nothing in produced %d events", n)
	}
	if n := len(observe.MergeInputs(nil, nil)); n != 0 {
		t.Errorf("merging two empties produced %d events", n)
	}
}
