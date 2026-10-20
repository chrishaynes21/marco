//go:build !windows

package main

// THE OTHER SIDE OF A WINDOWS SURFACE, KEPT COMPILING.
//
// Cross-platform by construction: every OS surface in this product sits behind a function with a
// Windows implementation and a stub, and the stub side has to build. This one answers no, which
// is the honest answer — nothing has been brought forward — and the run then relies on the
// application already leading the desktop, exactly as it does when Windows refuses the raise.

func raiseWindow(uintptr) bool { return false }
