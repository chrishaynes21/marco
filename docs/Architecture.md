---
type: map
status: active
updated: 2026-08-06
source_paths:
  - cmd/marco
  - internal
  - pkg/directorapi
  - plugins
---

# Architecture

Marco is two systems that share a language. The **engine** runs Marco programs. The
**Director** ([[Director]]) generates them from semantic requests. The engine does not know
the Director exists.

## The dependency rule that shapes everything

**The engine has zero external dependencies.** `cmd/marco` and `internal/*` import only the
standard library. Anything needing a dependency is a separate Go module under `plugins/`,
reached over the host-FFI bridge.

That rule is why the accessibility bridge is C# in `plugins/uia`, why OCR is a separate
binary in `plugins/ocr`, why the ONNX detector lives in `plugins/vision` behind a build
tag, and why the Director's heavy adapters sit in `internal/platform` rather than inside
`internal/director`.

## Layers

```
pkg/directorapi        dependency-free contract: world, observation, intent,
                       plan, action, confidence, actionability
       ↑
internal/director/*    the Director proper — sees only directorapi interfaces,
                       imports no engine code
       ↑
internal/platform/*    adapters: uiaclient, ocrclient, wincapture, winprovider,
                       marcorunner
       ↑
cmd/director           composition, CLI, service wiring
```

The arrows are enforced, not conventional:

- `TestDirectorImportsNoPlatformCode` — the Director may not reach an adapter directly
- `TestDirectorAPIIsStdlibOnly` — the contract stays dependency-free
- `TestDirectorHasNoDuplicatePlatformImplementation` — there is exactly one way to reach the desktop
- `TestOnlyPerceptionKnowsWhatAnObservationIs` — evidence types do not leak upward
- `TestNothingOutsidePerceptionKnowsThatOCRExists` — a source is an implementation detail

All in `internal/director/boundary_test.go` and `internal/director/perception_boundary_test.go`.

## The engine

`lexer → parser → graph → compile → runtime`. `driver` is the run/serve/check entry;
`routes` is the registry and argument parsing; `orchestrator` is the record-and-simplify Learn loop
(its exported methods are still spelled `Teach`);
`oshost` fulfils the `OS` act; `winctx`/`screen`/`recorder` are the OS surfaces, each
behind an interface with a Windows backend and a cross-platform stub.

## Where the two systems meet

Exactly one place: the Director lowers each planned step to legal Marco source, which then
goes through the ordinary compiler. See [[Marco-Boundary]] and
[[ADR-005-legal-marco-only]]. There is no second path, and a test asserts there is none.

This is a deliberately expensive choice. It caught three capabilities that worked from
`oshost` but that no line of Marco could call.

## Related

- [[Director]] — the system map
- [[Decisions]] — the constraint index
- [[Glossary]] — terms used throughout
- [[Wiring-Tests]] — why a green mechanism may still not be a feature
