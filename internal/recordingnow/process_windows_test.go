//go:build windows

package recordingnow

import "testing"

// This process, asked for real, is alive and has a start time — the Windows half of the contract.
func TestThisProcessCanBeIdentified(t *testing.T) {
	st, since := processState(me)
	if st != alive || since.IsZero() {
		t.Fatalf("this process reads as %v at %v", st, since)
	}
	if st, _ := processState(0); st != gone {
		t.Errorf("pid 0 reads as %v, want gone", st)
	}
}
