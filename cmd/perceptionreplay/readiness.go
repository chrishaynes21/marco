package main

// One truthful end-to-end offline readiness report.
//
// # What this is for
//
// Every piece of the teach-then-reuse chain now exists and has been measured on its own. The
// question this answers is different and is the only one that matters at this point:
//
//	how far does a RELOADED workflow get through the production contracts, without borrowing
//	evidence or authority from the recording that taught it?
//
// Seven stages, each reported as ready, refused, unobservable or not-reached, with the exact
// prerequisite that is missing. Nothing is executed and nothing is manufactured: where a person
// would decide, a labelled fixture decides, and the label travels with the result.
//
// # The rules this obeys, which are the point of running it at all
//
//   - Fold A learns and saves. Fold B is a fresh observer that shares no counter, no registry and
//     no screen identity with it. The artifact crosses between them as bytes.
//   - A simulated decision is named `SIMULATED` wherever it appears. A reader must never have to
//     work out whether a person was involved.
//   - A stage that cannot run because an earlier one refused is `not reached`, never `ready`.
//   - Lowering produces Marco source and compiles it. It does not run it, and nothing here can:
//     `workflow` cannot reach the lowering path at all, which is the boundary ADR-181 holds.

import (
	"fmt"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"path/filepath"
	"sort"
)

// stageState is how far one stage got.
type stageState string

const (
	stageReady        stageState = "READY"
	stageRefused      stageState = "REFUSED"
	stageUnobservable stageState = "UNOBSERVABLE"
	stageNotReached   stageState = "not reached"
)

// stage is one link in the chain.
type stage struct {
	n     int
	name  string
	state stageState
	// detail is what happened, and missing the exact prerequisite that is absent.
	detail  string
	missing string
	// simulated says a decision in this stage was made by a fixture rather than a person.
	simulated bool
}

// readiness accumulates the stages and stops honestly.
type readiness struct {
	stages  []stage
	blocked bool
	// progress is how far the walk actually got, kept apart from the stages
	// because a stage says what WORKED and this says what HAPPENED.
	progress *workflowstore.Progress
	// arrivals is what the observation after each simulated action boundary said.
	arrivals map[int]workflowstore.Arrival
}

func (r *readiness) add(s stage) {
	if r.blocked && s.state == stageReady {
		s.state = stageNotReached
		s.detail = "an earlier stage did not complete"
	}
	if s.state == stageRefused || s.state == stageUnobservable {
		r.blocked = true
	}
	r.stages = append(r.stages, s)
}

// notReached records a stage that could not even be attempted.
func (r *readiness) notReached(n int, name, why string) {
	r.stages = append(r.stages, stage{n: n, name: name, state: stageNotReached, missing: why})
}

func reportReadiness(dir string, limit int, onnx string, authorised bool) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	if len(frames) < 20 {
		return fmt.Errorf("%s carries %d frames, too few to split", dir, len(frames))
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })
	regions, readable := proposeAll(dir, frames)

	newFeed := func() *slowFeed {
		if onnx == "" {
			return nil
		}
		fd, ferr := loadSlowFeed(onnx, 8)
		if ferr != nil {
			err = ferr
			return nil
		}
		return fd
	}
	// WHAT THE RECORDING WAS READ TO SAY, read once for every fold below it.
	read := wordsBeside(dir, frames)
	half := len(frames) / 2
	a, aerr := foldAndLearn(frames, inputs, regions, readable, read, newFeed(), 0, half)
	if aerr != nil {
		return aerr
	}
	defer a.obs.Close()
	oracle, oerr := foldRange(frames, regions, readable, nil, nil, 0, len(frames))
	if oerr != nil {
		return oerr
	}
	if err != nil {
		return err
	}

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("OFFLINE READINESS — a reloaded workflow through the production contracts\n")
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  fold A  looks 1-%d     learned %d step(s) and saved them\n",
		half, len(a.draft.Steps))
	fmt.Printf("  fold B  looks %d-%d   a fresh observer sharing nothing with fold A\n",
		half+1, len(frames))
	fmt.Printf("  decisions a person would make are made by a labelled fixture and marked\n")
	fmt.Printf("  SIMULATED. Nothing is executed.\n")

	var r readiness
	_, _, loaded := stageReload(&r, a, half, frames)
	if len(loaded.Steps) == 0 {
		r.notReached(2, "the step-scoped walk", "no artifact to walk")
	} else {
		// AUTHORISED is a fixture and says so. Nothing in this experiment may
		// manufacture permission, so it is passed in and labelled, and the
		// unauthorised run is the one that shows what it is worth.
		stepWalk(&r, loaded, a, oracle, regions, readable, frames, half,
			newFeed(), authorised)
	}
	printReadiness(r)
	return nil
}
