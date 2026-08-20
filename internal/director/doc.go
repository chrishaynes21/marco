// Package director is the Director: Marco's intelligent desktop-control layer.
//
// It translates a natural-language request into safe, observable, verifiable
// computer actions. Where Marco's macro engine replays what it was shown, the
// Director maintains a structured understanding of the desktop — the World Model —
// plans against that representation, executes through Marco, and checks that each
// action had the intended effect.
//
// The loop it runs:
//
//	observe → build world state → interpret intent → resolve references →
//	plan → validate policy → execute one step → observe again → verify →
//	continue, recover, or stop
//
// Sub-packages own the stages: perception, world, fusion, identity, intent,
// references, planner, policy, execute, verify, recovery, memory, skills.
//
// # The one rule
//
// Nothing under internal/director may import a platform or Marco implementation
// package — not oshost, winctx, screen, recorder, runtime, or anything under
// internal/platform. The Director sees the world ONLY through the interfaces in
// pkg/directorapi, and cmd/director is where real implementations are wired in
// behind them.
//
// This is what keeps the Director a system that could run in-process, as a local
// service, as a separate binary, or as a library — and it is what keeps the
// architectural boundary a real boundary rather than an intention. It is also
// trivially easy to break with one convenient import, so boundary_test.go enforces
// it mechanically.
//
// (Not to be confused with internal/dispatch, which decides which SAVED ROUTE a
// phrase means. That is a small closed decision over the user's existing macros;
// this is desktop understanding and planning. Neither imports the other.)
package director
