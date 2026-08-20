---
type: milestone
status: historical
---

# The Marco boundary

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> Director decides what should happen. Legal Marco describes the effect.
> Marco's compiler and runtime are the only path by which the effect reaches the
> computer.

## What the boundary actually is

Marco's pipeline is:

```
source → lexer.Lex → parser.Parse → ast.File → graph.Build → compile.Compile
       → runtime.RunWithHostsContext → Host.Invoke
```

`runtime.HostCall` is **not** a Marco program. It is the struct `runtime.invokeForeign`
builds when the runtime walks a foreign-action node in an already-compiled graph. A
component that calls `Host.Invoke` directly is entering at the last step, skipping the
parser, the graph builder and the compiler.

That is what the Director used to do, and it hid three real gaps:

| what | where it lived | why nothing noticed |
|---|---|---|
| `clipboardget` / `clipboardset` | `oshost` only | `os.marco` never exported them |
| `movewindow` / `windowstate` | `oshost` only | same |
| act `Accessibility` | nowhere in Marco | declared in no `.marco` file at all |

All three "worked". None could be called by a line of Marco. Verified with the real
compiler:

```
$ marco check probe.marco
OS has no capability "ClipboardGet"
OS has no capability "MoveWindow"
unknown subject "Accessibility"
```

## The flow now

```
Director resolves and plans one step
  → marcoexec.Operation      (typed; JSON round-trips)
  → marcoexec.Lower          (legal Marco source, declared capabilities only)
  → lexer → parser → graph → compile.Compile     ← Marco validates
  → runtime.RunWithHostsContext                  ← Marco executes
  → Director observes and verifies
  → next step
```

`internal/director/marcoexec` is Director-side and imports only the standard library
plus `pkg/directorapi`. `internal/platform/marcorunner` is the adapter that knows both
systems; the Director sees only `directorapi.MarcoRunner` and imports no engine code.

**There is no second path.** `internal/platform/marcohost` — the adapter that turned
Actuator calls into `runtime.HostCall` — has been deleted. `marcoexec.Executor` is the
`directorapi.Actuator`, the focuser, the value provider and the clipboard.

### Why source, not a hand-built AST

`graph.Build` does take a typed `*ast.File`, so the parser *could* be skipped. It is
not, for two reasons. `ast.Part` is effectively a token stream — `PartWord`,
`PartString`, `PartPossessive` — so building one by hand is harder to review than the
Marco it represents. And skipping the parser would make *"every generated program
parses"* untestable. The repository sets the same precedent: `codegen.Route` lowers
`[]macroir.Step` to source, and `TestGeneratedRoutesCompileAndRun` parses the result.

The typed `Operation` is the representation. The source is a rendering of it, not a
string assembled ad hoc — callers never build Marco themselves.

## The capability table

| Director operation | Marco capability | declared in |
|---|---|---|
| `KindClick` (left) | `OS's Click` with a `Point`, ×N for a count | `os.marco` |
| `KindClick` (right/middle) | `OS's Move` then `OS's Click` with a button name | `os.marco` |
| `KindMove` | `OS's Move` | `os.marco` |
| `KindDrag` | `OS's Drag` with a `Drag` set | `os.marco` |
| `KindKey` | `OS's Key` (a `Press` set when held) | `os.marco` |
| `KindType` | `OS's Type` | `os.marco` |
| `KindTypeSecret` | `OS's Secret` | `os.marco` |
| `KindActivate` / `KindLaunch` | `OS's Activate` / `OS's Launch` | `os.marco` |
| `KindMoveWindow` | `OS's MoveWindow` with a `WindowMove` | `os.marco` |
| `KindWindowState` | `OS's WindowState` with a `WindowStateChange` | `os.marco` |
| `KindClipboardGet` / `KindClipboardSet` | `OS's ClipboardGet` → `Clipboard` set / `OS's ClipboardSet` | `os.marco` |
| `KindFocus` / `KindSetValue` / `KindGetValue` | `Accessibility's Focus` / `SetValue` / `GetValue` with a `Control` | `uia.marco` |
| scroll | — | **refused**, not approximated |

`Operation.Capabilities()` is the single table. A `Kind` with no entry cannot be
lowered — it fails by name, in Go, before a program exists.

**Why a right-click is two capabilities.** `OS's Click` takes *either* a `Point` (and
always clicks left) *or* a button name (and always clicks at the cursor). That is the
language's actual shape, so a right-click at a point is a `Move` followed by a `Click`.
Not a workaround — the alternative would be inventing a Click that does not exist.

## String escaping: `strconv.Quote` is wrong here

Marco's lexer (`internal/lexer/lexer.go`, `readString`) accepts **exactly five**
escapes:

```
\"   \'   \\   \n   \t
```

Anything else is `invalid escape \c`. There is no `\x` and no `\u`. Bytes that are not
a quote, a backslash or a newline are written through verbatim — which is why UTF-8
needs no escaping, and must not be escaped.

`strconv.Quote` emits `é` for é, `\x00` for a NUL and `\r` for a carriage return.
Marco rejects all three. **Any non-ASCII text the Director typed would have generated a
program the lexer refused** — an accented name, a Japanese message, an emoji.

`marcoexec.Quote` escapes the five, passes everything else through raw, and **refuses**
a NUL rather than mangling it. Round-tripped against the real lexer for quotes,
backslashes, tabs, newlines, `café`, `日本語`, `🎉`, empty strings and Windows paths.

## What a generated program looks like

```marco
// Generated by the Director. Every capability below is declared in
// internal/osmod/os.marco or internal/uiamod/uia.marco.
use os.

the DirectorMoveWindow is an actor.

this can Run.
this's Run does...
    the place is a WindowMove with Window "hwnd:12345", X -1920, Y 0, W 960, H 1032.
    do OS's MoveWindow with place.
    this is ok!

the App is a script.

do DirectorMoveWindow's Run...
    when ok?
        log "DirectorMoveWindow: done".
    or?
        log that's error.
```

The shape follows `marco teach`'s output (see `routes/global/copy.marco`), so the
result is something a person can read, save as a route and edit.

**Actors are prefixed `Director…`.** Found by the compiler: lowering a drag produced
`the Drag is an actor.` while `os.marco` already declares `the Drag is a set.`, and the
build failed with `duplicate symbol "Drag"`. `Point`, `Press`, `Control`, `Clipboard`,
`WindowMove` and `WindowStateChange` were all the same mine. The prefix removes the
class rather than blacklisting names one at a time.

**Unset fields are omitted, never written as `""`.** An empty `Window` means "the
foreground window" to the host, which is a different instruction from a blank handle.

**Negative coordinates survive unchanged.** A monitor left of or above the primary one
has them, and `%d` carries the sign through.

## Getting values back out

`ClipboardGet` and `GetValue` produce values the Director needs in Go, and
`driver.RunSourceWithHostsCtx` returns only an `error`. Rather than have the program
`log` the value and the Director parse text back, `marcorunner` wraps each host in a
**tee**: the call goes to the real host untouched, its results come back untouched, and
what it returned is recorded under `"Act's Capability"`.

A pure observer. If it ever needed to *modify* a call it would be a second execution
path, which is the thing this arrangement exists to prevent.

## Compile before mutation

```
validate → confirm the target context → lower → lex → parse → graph → compile → run
```

Everything before `run` can refuse, and refusing at any of them means the desktop was
never touched. There is no partial execution: a program compiles whole or does not run.

## Status is not collapsed into FAILED

| status | means | is retrying safe? |
|---|---|---|
| `completed` | compiled, ran, resolved ok | — |
| `unsupported` | the Director cannot express this in Marco; refused before generation | nothing happened |
| `compile_failed` | Marco rejected the program; the runtime was never entered | nothing happened |
| `runtime_failed` | Marco ran it and a capability resolved failed | depends on the act |
| `cancelled` | the context ended; nothing was wrong | nothing partial |
| `target_context_changed` | the intended target was not in front, so it was refused | nothing was sent |

Collapsing these destroys exactly the information that decides the next move.

## Foreground protection

A previous focus operation proves nothing at the instant of execution. The user
alt-tabs, a notification steals focus, an installer pops up.

Before **Type, TypeSecret, Key and Drag** — the operations that deliver input to
whatever holds focus — the executor:

1. re-observes the world through the same pipeline the planner used;
2. compares the foreground against the intended window;
3. attempts one structural repair (`OS's Activate`, itself a compiled program);
4. **re-observes** — activating is a request, not a result;
5. refuses if it still does not match.

An operation with **no** window means "whatever has focus" — the deliberate meaning of
"type hello" with no named control — and is allowed. Window and clipboard operations
name their target explicitly and are not guarded; guarding them would break "tidy my
windows".

This exists because of a real incident: an edit resolved against Notepad executed while
Discord held the foreground and the text went into a message box — successfully,
verifiably, and into the wrong application. Reproduced deliberately and refused:

```
status       target_context_changed
diagnostic   the intended window hwnd:4591094 is not in front — "Rocket League
             (64-bit, DX11, Cooked)" (rocketleague) is, so nothing was sent
```

`Executor.Guarded()` plus a panic at construction exists because `WithGuard` returns a
copy — and the unguarded original was, once, the one that reached the pipeline. It
looked exactly like working code.

## Secrets

The value never reaches the Director. `OS's Secret` takes a credential **name** and
fetches the value inside the host.

The name is redacted from stored source, from `Describe`, from diagnostics, from the
action graph and from `director lower` — including from the credential store's own
error message. The *executed* source still names it, because the host has to look it
up; redaction governs what is retained.

```
type_secret — type a saved credential (<redacted>)
  capability   OS's Secret
  status       runtime_failed
  diagnostic   secret "<redacted>" is not set
        do OS's Secret with <redacted>.
```

## What is NOT compiled into Marco

Marco executes only what it can evaluate from its own runtime vocabulary. These stay in
the Director:

- **perception-dependent waits** — "the Save button is enabled" is answered from the
  accessibility tree, fusion and element identity, none of which Marco has;
- **policy evaluation**, **target resolution**, **clarification**;
- **verification** — comparing world states;
- **sequence orchestration** — the Director observes and verifies *between* Marco
  executions, which is what lets a failed edit stop the next one.

Pretending these compile would mean giving the Marco runtime access to the Director's
perception layer, inverting the dependency the architecture rests on.

## Diagnostics

```
director lower key ctrl+s        the Marco an operation compiles to — never executes
director lower --recent          what was actually lowered and run, with source
director op launch notepad       execute one operation through the ordinary path
```

`director op` exists because launch, activate and window state have no spoken phrase.
It is not a bypass: a client submits a typed `Operation`, never source, and the service
runs it through the same executor, guard and compiler a planned action uses.

The execution trace carries the lowering:

```
execute   window move sent to (-1920,0 960x1032)
marco     marco OS's MoveWindow · compile ok · runtime ok
verify    requested (-1920,0 960x1032), got (-1920,0 960x1032)
```

The action graph stores the semantic operation, the exact non-secret source, the
compile result, the runtime result and the verification result. Generated source is
diagnostic and export material — **never** the semantic key, which stays the Action.

## Adding a capability

1. do **not** approximate it with invented syntax — the lexer will reject it;
2. do **not** hide a low-level workaround in the Director;
3. name the missing runtime capability;
4. add the smallest **general-purpose** primitive to the relevant act surface *and* its
   host — it must be useful to a route, not just to one Director phrase;
5. add parse, validation, execution and round-trip tests;
6. only then use it from the Director.

## The tests that hold this up

| test | proves |
|---|---|
| `TestEveryOperationLowersToLegalMarco` | every operation lexes, parses and passes `marco check` |
| `TestTheHostReceivesWhatTheOperationMeant` | the right capability with the right payload, in order |
| `TestUnsupportedOperationsFailBeforeAnythingRuns` | eight refusal cases, none reaching a host |
| `TestACapabilityMarcoDoesNotExportFailsAtCompileTimeNotAtTheHost` | compile-before-mutation |
| `TestScrollIsRefusedRatherThanApproximated` | no silent substitution |
| `TestTextSurvivesLoweringExactly` | quotes, backslashes, tabs, newlines, Unicode, empty |
| `TestStrconvQuoteWouldProduceInvalidMarco` | guards why `encode.go` exists |
| `TestNegativeCoordinatesSurviveLowering` | the negative-monitor case |
| `TestOperationsSerialiseWithoutLoss` | JSON round-trip, identical lowering |
| `TestASecretsNameNeverAppearsInAnythingRetained` | source, diagnostic, summary, recorder |
| `TestNothingRenderedForASecretCarriesItsName` | every rendered string at once |
| `TestForegroundMismatchRefusesTypingRatherThanTypingElsewhere` | the Discord incident, as a test |
| `TestTheGuardIsConsultedAtExecutionNotOnceAtResolution` | no stale confirmation |
| `TestWindowOperationsAreNotBlockedByTheForeground` | the guard is not over-broad |
| `TestStatusesAreNotCollapsedIntoFailed` | four distinct statuses |
| `TestBundledModulesMatchTheOnesCompilationUses` | no shadowing act surface |
| `TestDirectorHasNoDuplicatePlatformImplementation` | no `HostCall`, no `.Invoke(`, no hand-rolled input |
| `TestEachEditIsObservedAndVerifiedSeparately` | observe and verify between Marco steps |

### Drift

`buildGraph` prefers a sibling `<module>.marco` over the built-in, so a stale copy
**shadows** the real act surface for everything beside it. It happened: `routes/os.marco`
was missing `KeyDown`, `KeyUp`, `Drag`, `Roll`, `EightBall` and `Restore`, so every
route in that tree compiled against a stale surface.

`testdata/` fixtures are excluded on purpose: a golden test's `os.marco` declares only
what its program uses, and making it track the canonical surface would churn goldens
for nothing.
