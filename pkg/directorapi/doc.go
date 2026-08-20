// Package directorapi is the Director's public contract: the types and interfaces
// that describe a desktop, an intent, a plan, and the machinery that observes and
// acts on them. It is the ONLY package the Director's internals and its host are
// both allowed to see.
//
// # Why this package exists
//
// The Director decides WHAT should happen to the computer; Marco makes it happen.
// Keeping those two systems apart is the whole architectural bet, and a shared
// vocabulary with no implementations is what enforces it:
//
//   - internal/director/**  imports directorapi and nothing platform-specific. It
//     must never import oshost, winctx, screen, or recorder. There is a test that
//     checks this (see internal/director/boundary_test.go), because the rule is
//     easy to break by accident and expensive to restore afterwards.
//   - internal/platform/**  implements these interfaces over real Marco/OS code.
//   - cmd/director          is the only place both halves are visible, and is where
//     the wiring happens.
//
// That direction — Director → directorapi ← platform — is what lets the Director
// run in-process today and as a separate service, a plugin, or an embedded library
// later without touching its own logic.
//
// # Invariants
//
// This package imports ONLY the standard library. Marco's engine has zero external
// dependencies and that is load-bearing; a public contract that dragged in a
// dependency would leak it into everything that speaks the contract. Anything
// needing a real dependency (an OCR engine, an ONNX runtime, a model client) lives
// behind one of these interfaces, in its own module under plugins/.
//
// It also contains NO behaviour beyond small pure helpers on value types (rectangle
// geometry, mostly). Scoring, fusion, ranking and policy are decisions, and
// decisions live in the Director, not in the vocabulary it speaks.
package directorapi
