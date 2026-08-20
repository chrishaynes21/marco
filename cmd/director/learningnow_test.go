package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE DIRECTOR REPORTS WHETHER SOMEBODY IS DEMONSTRATING.
//
// # Why the service needs this and needs nothing else
//
// A Learn episode claims no command slot — the person is driving and Marco is watching — so
// `Registry.Cancel` cannot see one, and "stop" answered "nothing is running" through a whole
// demonstration. The service's third arm asks exactly one question before sending the ordinary
// Cancel request: is there anything to abandon. This is the answer to that question.
//
// It must track the COORDINATOR and not a flag of its own. A stop that cancelled a settled session
// would report success for doing nothing, and a stop that missed a running one is the defect.
//
// # The mutations this kills
//
//   - return false always, or drop the method: the service's assertion for service.Acquirer fails
//     and the stop arm is never reached, which is the state this whole change exists to leave.
//   - return true always: every idle "stop" sends an acquisition request to a Director with
//     nothing to learn, and reports `accepted` for having done nothing.
//   - read a field set at start rather than the coordinator: an episode that settled on its own
//     still reports as running.
func TestTheDirectorReportsWhetherSomebodyIsDemonstrating(t *testing.T) {
	// The service asks for this by name. A signature drift here is a stop that silently
	// stops reaching demonstrations, which is invisible until somebody needs it.
	var _ service.Acquirer = (*Runtime)(nil)

	plat := &focusDesktop{windows: []windowref.Candidate{
		{ID: "hwnd:11", Handle: 11, ProcessID: 3, Application: "code.exe", Title: "marco",
			Bounds: rect(0, 0, 1600, 900), Visible: true, OnScreen: true, Foreground: true},
	}}
	rt := &Runtime{
		observations: newObservationRegistry(), learn: &learnSession{},
		winPlatform: plat, winDirectory: windowref.NewDirectory(),
	}
	rt.observations.memory = &stubTeachMemory{}

	if rt.LearningNow() {
		t.Fatal("a Director with nobody demonstrating reports that somebody is")
	}

	if _, err := rt.Observation(service.ObserveQuery{Learn: &service.ObserveLearn{
		Name: "open downloads", Actor: "Downloads", Verb: "Open",
	}}); err != nil {
		t.Fatalf("starting a demonstration was refused: %v", err)
	}
	if !rt.LearningNow() {
		t.Fatal("a demonstration is under way and the Director says nothing is. `stop` then " +
			"answers \"nothing is running\" while Marco goes on watching.")
	}

	// AND THE ORDINARY CANCEL REQUEST ENDS IT — the same one the service's stop arm sends.
	// There is exactly one implementation of abandoning an episode and this is it.
	if _, err := rt.Observation(service.ObserveQuery{
		Learn: &service.ObserveLearn{Cancel: true}}); err != nil {
		t.Fatalf("cancelling the demonstration failed: %v", err)
	}
	if rt.LearningNow() {
		t.Error("the episode still reports as running after it was cancelled")
	}
}
