---
type: decision
status: accepted
date: 2026-08-06
supersedes: []
affects:
  - marco-boundary
  - execution
  - action-graph
  - safety
---

# Every desktop effect lowers through legal Marco

## Context

The Director knows what it wants to do and has a host that can do it. Calling
`Host.Invoke` directly is one function call; generating Marco source, compiling it, and
running it through lexer → parser → graph → compile → runtime is considerably more work for
the same click.

The reason to do the expensive thing is that the cheap thing creates a **second path to the
desktop** — one the Marco compiler never sees, never validates, and cannot refuse.

## Decision

**The Director does not call `Host.Invoke`. It lowers each planned step to legal Marco
source**, which then goes through the ordinary pipeline like any taught route.

There is no second path. The former `internal/platform/marcohost` adapter was deleted rather
than kept as a fallback.

## Consequences

- Marco's compiler validates every action before anything moves. A step the language cannot
  express is refused at compile time rather than half-performed.
- The language stays honest. Lowering revealed three capabilities that worked from `oshost`
  but that no line of Marco could call — `ClipboardGet`/`ClipboardSet`, `MoveWindow`, and the
  `Accessibility` act. They are now declared in `internal/osmod/os.marco` and
  `internal/uiamod/uia.marco`.
- Anything the Director can do, a person can write by hand, read, and edit. The generated
  program is inspectable with `director lower`.
- Compile happens *before* mutation, so a malformed program cannot leave a half-applied
  change.
- It costs real latency and a whole subsystem (`marcoexec`) that a direct call would not
  need.

## "Is this legal Marco" has exactly one answer

The check must resolve `use` the way the runtime resolves it, through `driver.CheckSource` —
`buildGraph` plus `compile.Compile`, the same path `RunSource` takes.

It did not, for a while, and the failure is worth keeping. The Director assembled its own world:
`osmod.Source + screenmod.Source`, with `use os.` and `use screen.` deleted by string replacement.
The spec suite assembled a different one: four module sources, four `use` lines deleted. Two
hand-maintained copies of one fact, and the copy on the path a person walks was the shorter one.

They drifted the moment a play could press a control by name, because such a play imports the
Theater act. A route the Audience demonstrated, named at both ends, rehearsed and verified 1/1 then
ended at:

```
not_lowerable: core_cannot_express
the generated play does not compile: 101:13: unknown type "Target"
```

— while the spec test asserting that exact play compiles was green. The refusal named the language
as the problem, and the language was fine; the Director was asking a question the runtime never
asks. A wrong answer that blames Core is worse than a crash, because the next move it invites is
widening Core.

A checker that assembles the world differently from the product is not checking the product.

## Enforced by

- **implementation** — `internal/director/marcoexec` (`lower.go`, `encode.go`, `exec.go`),
  `internal/platform/marcorunner`
- **one resolver** — `driver.CheckSource`, called by both `cmd/director`'s pre-flight and
  `internal/spectest`'s `compileAgainstTheRealOS`
- **production-entry test** — `TestThePreflightAcceptsAPlayThatPressesAControlByName` and
  `TestThePreflightResolvesEveryActALearnedPlayImports`
  (`cmd/director/learnedplaywiring_test.go`) enter through `compileGenerated` itself; restoring
  the hand-maintained module list reproduces the live `unknown type "Target"` verbatim
- **boundary test** — `TestDirectorHasNoDuplicatePlatformImplementation`
  (`internal/director/boundary_test.go`) fails if a second platform implementation appears
- **boundary test** — `TestDirectorImportsNoPlatformCode` (same file)
- **milestone record** — [[director-marco-boundary]], including *Compile before mutation* and
  *The tests that hold this up*

## Related

- [[Marco-Boundary]], [[Programs]], [[Action-Graph]]
- [[Architecture]]
