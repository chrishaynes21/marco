package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

type sessFile struct {
	Application string `json:"application"`
	Stats       struct {
		Shadow observe.ShadowTotals `json:"shadow"`
	} `json:"stats"`
}

// againstStore replays one session's states through the production identity path and asks the
// REAL matcher what each one recalls. Read-only: it opens a copy of the store and never writes.
func againstStore(sessionPath, storePath string) {
	b, err := os.ReadFile(sessionPath)
	if err != nil {
		fmt.Println("session:", err)
		return
	}
	var s sessFile
	if err := json.Unmarshal(b, &s); err != nil {
		fmt.Println("decode:", err)
		return
	}
	store, un := semanticmemory.Open(storePath)
	if un != "" {
		fmt.Println("store:", un)
		return
	}
	th := observe.DefaultHypothesisThresholds()
	states := s.Stats.Shadow.States
	sort.Slice(states, func(i, j int) bool { return states[i].Inferences > states[j].Inferences })

	for _, st := range states {
		sig, ok := observe.SignatureOfState(s.Stats.Shadow, st.ID, th)
		if !ok {
			fmt.Printf("%-10s NOT DESCRIBABLE (inf=%d settled=%v)\n",
				st.ID, st.Inferences, st.Settled)
			continue
		}
		rec := store.Recall(s.Application, sig)
		fmt.Printf("\n%-10s inf=%-4d settled=%-5v => RECALL %s %s\n",
			st.ID, st.Inferences, st.Settled, rec.Verdict, rec.Subject.ID)
		fmt.Printf("    roles=%v terms=%v\n", sig.Roles, sig.Terms)
		if rec.Verdict == observe.MatchSame {
			continue
		}
		// What the canonical matcher makes of it against each stored place, closest first.
		type scored struct {
			id, called string
			c          observe.StructureComparison
			off        int
		}
		var all []scored
		for _, sub := range store.Subjects() {
			if sub.Application != s.Application ||
				sub.Structure.Subject != observe.SubjectState {
				continue
			}
			c := observe.ExplainStructure(sig, sub.Structure)
			all = append(all, scored{sub.ID, string(sub.Called), c, len(c.Why)})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].off < all[j].off })
		for i, a := range all {
			if i >= 3 {
				break
			}
			fmt.Printf("    vs %-22s %-10q %s\n", a.id, a.called, a.c.Verdict)
			for _, w := range a.c.Why {
				fmt.Printf("        %-12s live=%-22s stored=%-22s decisive=%v\n",
					w.Field, w.Current, w.Remembered, w.Decisive)
			}
		}
	}
}
