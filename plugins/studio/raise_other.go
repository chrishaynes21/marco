//go:build !windows

package main

// THE OTHER SIDE OF A WINDOWS SURFACE, KEPT COMPILING.
//
// Cross-platform by construction: every OS surface in this product sits behind a function with a
// Windows implementation and a stub, and the stub side has to build. This one answers no, which
// is the honest answer — nothing has been brought forward — and the run then relies on the
// application already leading the desktop, exactly as it does when Windows refuses the raise.
//
// `checked` is false, which is the difference that matters: "it is not in front" would refuse
// every run on this platform, and what is true is that nobody here can say where it is. The
// run's own foreground guard then answers it at the instant it matters.

func raiseWindow(uintptr, uint32) (leading, checked bool) { return false, false }
