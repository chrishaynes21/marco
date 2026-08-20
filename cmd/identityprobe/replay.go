package main

// Replaying a finished session through the ESTABLISHMENT and RELATIONSHIP path.
//
// # Why this is here rather than in a test
//
// A live learn refusal reads "4 transition(s) were seen and none had two recognisable
// endpoints". That is a statement about durable subjects, and by the time it is printed the
// session is over and the store is whatever it is. The measurement that answers it — WHICH
// places would have been established, and WHICH edges would then have resolved — needs the
// session's own evidence and a store nobody has touched.
//
// This runs both against a cold store in a temporary directory: `observe.PlacesToEstablish`,
// then `EstablishPlace` for each candidate, then `observe.RelationshipsFrom`. Every one of
// those is the production function. Nothing here reimplements a gate, a signature or a
// matcher, so a verdict printed here is the verdict a Director would have reached from the
// same evidence.
//
// It reads a recorded session and writes only to its own temporary directory. It never drives
// input and never touches the user's memory.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// replayFile is the part of a session's JSON the establishment path reads.
//
// Wider than sessionFile because relationships resolve their endpoints from the session's own
// hypotheses rather than by re-deriving a signature — see RelationshipsFrom, which is emphatic
// about there being one answer to "what screen is this".
type replayFile struct {
	ID          string               `json:"id"`
	Application string               `json:"application"`
	Samples     int                  `json:"samples"`
	Generations []int                `json:"generations"`
	Hypotheses  []observe.Hypothesis `json:"hypotheses"`
	Stats       struct {
		Shadow observe.ShadowTotals `json:"shadow"`
	} `json:"stats"`
}

// replay reports what a licensed pass over this recorded session would have made durable.
func replay(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var s replayFile
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "identityprobe-replay")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	// A COLD store. The point of the measurement is what a first encounter does.
	store, note := semanticmemory.Open(dir + "/semantic-memory.json")
	if note != "" {
		fmt.Printf("  store: %s\n", note)
	}
	th := observe.DefaultHypothesisThresholds()
	shadow := s.Stats.Shadow

	fmt.Printf("\n=== %s (%s) — %d sample(s) ===\n", shortName(path), s.Application, s.Samples)
	fmt.Printf("  current state: %s\n", shadow.CurrentState)

	// Every state the session saw, and whether the gates would admit it. Printed before the
	// establishment so a refused place is visible as a refusal rather than as an absence.
	states := append([]observe.ScreenState{}, shadow.States...)
	sort.Slice(states, func(i, j int) bool {
		return states[i].FirstInference < states[j].FirstInference
	})
	for _, st := range states {
		sig, ok := observe.SignatureOfState(shadow, st.ID, th)
		fmt.Printf("  %-9s inf=%-3d settled=%-5v describable=%-5v discriminating=%-5v terms=%v\n",
			st.ID, st.Inferences, st.Settled, ok, ok && sig.Discriminating(), sig.Terms)
	}

	candidates, refusal := observe.PlacesToEstablish(shadow, s.Application, store, th)
	fmt.Printf("  --- PlacesToEstablish: %d candidate(s), current refusal=%q ---\n",
		len(candidates), refusal)
	for _, c := range candidates {
		id, err := store.EstablishPlace(s.Application, c.Signature)
		switch {
		case err != nil:
			fmt.Printf("  %-9s current=%-5v NOT WRITTEN: %v\n", c.State, c.Current, err)
		default:
			fmt.Printf("  %-9s current=%-5v -> %s\n", c.State, c.Current, id)
		}
	}

	// The endpoint resolver reads the session's HYPOTHESES, not SignatureOfState. Whether
	// those two agree is the whole question when every place established and no edge did.
	fmt.Printf("  --- endpoint signatures, as RelationshipsFrom derives them ---\n")
	fromHypothesis := map[observe.ScreenStateID]observe.StructureSignature{}
	for _, h := range s.Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		id := observe.ScreenStateID(h.Subject.Ref)
		if _, seen := fromHypothesis[id]; seen {
			continue
		}
		fromHypothesis[id] = observe.SignatureOf(h)
	}
	for _, st := range states {
		hSig, ok := fromHypothesis[st.ID]
		if !ok {
			fmt.Printf("  %-9s NO state hypothesis -> endpoint can never resolve\n", st.ID)
			continue
		}
		eSig, _ := observe.SignatureOfState(shadow, st.ID, th)
		verdict := observe.CompareStructure(hSig, eSig)
		rec := store.Recall(s.Application, hSig)
		fmt.Printf("  %-9s hypothesis-vs-established: %-12s recall=%-12s terms=%v\n",
			st.ID, verdict, rec.Verdict, hSig.Terms)
		if verdict != observe.MatchSame {
			for _, line := range whyDiffer(hSig, eSig) {
				fmt.Printf("        %s\n", line)
			}
		}
	}

	// THE question the live refusal was about.
	cont := observe.Continuity{Generations: len(s.Generations)}
	edges, report := observe.RelationshipsFrom(shadow, s.Hypotheses, s.Application, store, cont)
	fmt.Printf("  --- RelationshipsFrom: %d transition(s) seen ---\n", len(shadow.Transitions))
	if len(shadow.Crossings) == 0 {
		// A session recorded before the segmenter kept the walk. Bridging reads the walk,
		// so an interval here cannot be paired — said out loud, because "no edges" would
		// otherwise read as a finding about the evidence rather than about the recording.
		fmt.Printf("  NOTE: this recording predates the walk log, so no unplaceable " +
			"interval can be paired. See ADR-064.\n")
	}
	for _, c := range shadow.Crossings {
		fmt.Printf("  walk %-9s -> %-9s run=%d\n", c.From, c.To, c.Run)
	}
	for _, tr := range shadow.Transitions {
		fmt.Printf("  %-9s -> %-9s count=%d unattributed=%d\n",
			tr.From, tr.To, tr.Count, tr.Unattributed)
	}
	fmt.Printf("  durable edges: %d   session-local: %d   why: %s\n",
		len(edges), report.SessionLocal, report.Why())
	for _, e := range edges {
		fmt.Printf("  EDGE %s -> %s\n", e.From, e.To)
	}
	if len(edges) == 0 {
		fmt.Printf("  NO EDGE. A demonstration over this session could not be learned.\n")
	}
	return nil
}
