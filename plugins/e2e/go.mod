// The desktop harness is its own module, and that is the whole of how it stays out of the
// ordinary suite.
//
// `go test ./...` at the repo root never sees this directory, so nothing here can be run by
// accident by somebody checking a change — which matters more than usual, because running it
// opens windows, spawns Marco, and lets Marco perform real input into a window on the real
// screen. An environment-variable gate would have been a second thing to remember; a separate
// module is a fact about the tree.
//
// The module PATH carries the engine's prefix for the reason studio's does: Go's internal rule is
// about the importing module's path, not its location on disk, and a harness that could not
// import github.com/chaynes-simpleclouds/marco/internal/... could not seed a routine through the
// production generator — which is the one thing that makes a seeded routine worth playing.
//
// ZERO EXTERNAL DEPENDENCIES, like the engine. Everything this needs is the standard library, the
// engine's own packages, and three binaries it spawns.
module github.com/chaynes-simpleclouds/marco/plugins/e2e

go 1.26.2

require github.com/chaynes-simpleclouds/marco v0.0.0

replace github.com/chaynes-simpleclouds/marco => ../..
