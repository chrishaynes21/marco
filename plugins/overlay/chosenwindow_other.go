//go:build !windows

package main

// windowInFront is unanswerable without a window backend, so the child is told nothing and
// refuses for itself. Never a fabricated handle: a recording confined to a window nobody chose
// is the thing this is here to prevent.
func windowInFront() string { return "" }
