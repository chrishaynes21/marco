// twinprobe reads the REAL semantic memory and asks the canonical matcher what it makes of
// every pair of durable places in one application.
//
// It computes nothing of its own: `observe.ExplainStructure` is the one matcher, and a verdict
// here is the verdict the Director reaches. It never writes.
package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

func main() {
	if len(os.Args) > 3 && os.Args[1] == "-session" {
		againstStore(os.Args[2], os.Args[3])
		return
	}
	store, unavailable := semanticmemory.Open(os.Args[1])
	if unavailable != "" {
		fmt.Println("open:", unavailable)
		return
	}
	app := os.Args[2]
	var subs []observe.RememberedSubject
	for _, s := range store.Subjects() {
		if s.Application == app && s.Structure.Subject == observe.SubjectState {
			subs = append(subs, s)
		}
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].ID < subs[j].ID })
	fmt.Printf("%d place subject(s) in %s\n\n", len(subs), app)
	for _, s := range subs {
		fmt.Printf("%-22s called=%-10q members=%-4d roles=%v terms=%v\n",
			s.ID, string(s.Called), s.Structure.Members, s.Structure.Roles, s.Structure.Terms)
	}
	fmt.Println("\n--- every pair, through the canonical matcher ---")
	for i := 0; i < len(subs); i++ {
		for j := i + 1; j < len(subs); j++ {
			c := observe.ExplainStructure(subs[i].Structure, subs[j].Structure)
			if c.Verdict == observe.MatchDifferent {
				continue // only the interesting ones
			}
			fmt.Printf("\n%s  vs  %s   => %s\n", subs[i].ID, subs[j].ID, c.Verdict)
			for _, w := range c.Why {
				fmt.Printf("      %-12s current=%-24s remembered=%-24s decisive=%v\n",
					w.Field, w.Current, w.Remembered, w.Decisive)
			}
		}
	}
	fmt.Println("\n--- the live twin pair, whatever the verdict ---")
	for _, pair := range [][2]string{
		{"subj_61ffd6bc8602", "subj_a3b069b996ac"}, // unnamed twin vs Mouse
		{"subj_543793ccc326", "subj_bef5e3d29af8"}, // unnamed twin vs Home
		{"subj_543793ccc326", "subj_892a4cc30f41"}, // unnamed twin vs Bluetooth
	} {
		a, aok := store.Subject(pair[0])
		b, bok := store.Subject(pair[1])
		if !aok || !bok {
			fmt.Printf("%s vs %s: missing (%v/%v)\n", pair[0], pair[1], aok, bok)
			continue
		}
		c := observe.ExplainStructure(a.Structure, b.Structure)
		fmt.Printf("\n%s vs %s => %s\n", pair[0], pair[1], c.Verdict)
		for _, w := range c.Why {
			fmt.Printf("      %-12s current=%-24s remembered=%-24s decisive=%v\n",
				w.Field, w.Current, w.Remembered, w.Decisive)
		}
	}
}
