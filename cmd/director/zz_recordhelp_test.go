package main

import "testing"

// mustSay is the action count a FINISHED record carries, or a failed test.
//
// For the readers that used to compare `RunRecord.Sent` with a number. The field is a pointer
// now, because an attempt that never finished says nothing about how many programs reached a
// computer and a plain zero would say "none" — so a test asking "how many" has to say what it
// expects of a record that does not answer, and what it expects is an answer.
func mustSay(t *testing.T, r RunRecord) int {
	t.Helper()
	n, said := r.SaidSent()
	if !said {
		t.Fatalf("the record for %q does not say how many actions were sent, and this "+
			"is a finished run: %+v", r.Run, r)
	}
	return n
}

// ptr is a pointer to a value, for a record literal in a test.
func ptr[T any](v T) *T { return &v }
