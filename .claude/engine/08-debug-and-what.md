# Engine — Debug and `what`

The introspection surface. Shouldn't perturb execution; should feel conversational.

---

## `what` is a side-channel

`what` doesn't traverse edges. It's a query against the trace tree and the symbol table. The runtime has to:

- Maintain the trace tree (frame parent/child, edges executed, timing).
- Expose a query API used by the REPL / inspector.
- Render results as human text.

`what` cannot be invoked from inside running code. It's only valid in:

- REPL prompt
- Inspector panel
- Breakpoint context (when execution is paused)
- Post-mortem (after a crash, from a saved trace)

---

## The five canonical forms

Mapping each to what it touches:

| Form | Reads | Renders |
|---|---|---|
| `what is that?` | The bound `that` Frame | Frame metadata: origin node, status, slots populated, timing |
| `what's that?` | `that.Result` | The result set, pretty-printed |
| `what happened?` | Recent frame transitions on the trace tree | Timeline: spawned → resolved, with statuses |
| `what was previous that?` | The parent's `that` before the current frame | Previous-step Frame metadata |
| `what can that do?` | `that.Origin.Contract.Capabilities` | List of declared capabilities |

`what's that?` is the most-used; it answers "what came back from the last thing I did?" That's the bread-and-butter introspection in any debug session.

---

## Trace tree as the data source

The frame tree is the trace. No separate event log needed. Each Frame carries:

- ID, parent, children, origin
- input / status / result / error
- start / end times
- Spans: edges executed inside this frame

Example tree after a Save flow:

```
Frame(root, origin=script)
├── Frame(SaveRequested-listener, origin=hears-body)
│   ├── Frame(SaveFlow, origin=Game.Save, status=Saved)
│   │   └── Frame(WriteToFile, origin=Game.WriteToFile, status=ok)
│   └── Frame(SaveCompleted-emit, origin=Game)
└── ... etc
```

`what happened?` walks this tree and presents recent activity.

---

## Inspector API (sketch)

```go
type Inspector struct {
    runtime *Runtime
}

// Bound at the prompt — the "current that".
func (i *Inspector) Bind(frame FrameID) { ... }

func (i *Inspector) WhatIs(ref string) Description
func (i *Inspector) WhatsThat() Description
func (i *Inspector) WhatHappened(window time.Duration) Timeline
func (i *Inspector) WhatWasPreviousThat() Description
func (i *Inspector) WhatCanThatDo() []CapabilityInfo
```

The REPL parses the user's `what` query and calls the matching method. Output is text; structured equivalents available for IDE integration.

---

## Pretty-printing rules

- Frames render: `<id> <originName> [<status>] (<duration>)`
- Sets render: a multi-line key-value list, types inferred
- Errors render: status + message + stack-of-frames
- Lists render with indices; maps with keys; queues with depth and front
- Truncation: large sets get a head + count

Verbose mode (probably `what's that in detail?` later) shows everything; default is summary.

---

## Breakpoints — sketch only

The user mentioned `now break.` as a preliminary direction. Working assumption:

```marco
now break.
```

A sentence that:

- pauses the current cursor at this point
- binds `that` in the inspector to the most-recent frame
- transfers control to the REPL / IDE

Implementation:

- `now` is a marker word; `break` is the action.
- Compiler treats `now break.` as a special edge `BREAKPOINT` — only emitted when debug mode is on.
- Runtime: when we hit `BREAKPOINT`, park the cursor with a special "Debug" status, signal the inspector, wait for resume.
- Inspector signals: `step.`, `continue.`, `bind X.`, `what is X?` etc. → translated into runtime ops.

The form is provisional; we may end up with `pause.`, `break here.`, or something else. Locking the *behavior* (suspend + bind + transfer to inspector) is what matters; the surface word can shift.

---

## REPL surface

REPL mode is a special script context where:

- Top-level sentences run as if in a long-running root phrase.
- After each sentence, `that` is bound to the most recent frame.
- `what` queries are accepted between sentences.
- The inspector retains state across queries.

Implementation: the REPL is just a script that loops on stdin, each line parsed, compiled, and run. Frames accumulate as siblings under the root; `what happened?` shows the recent ones.

---

## Tracing levels

Per Open Question — should tracing be selectively disable-able? V1 default: full tracing always. We allocate Spans on every edge. For heavy production loads this could be expensive; a trace-level flag (`off` / `summary` / `full`) is reasonable.

Default `summary` — record frame transitions but not per-edge spans. `full` for active debugging.

This is a runtime config flag, not a language feature.

---

## Open

- **`what's <expr>?` for arbitrary expressions** — e.g., `what's User?` to inspect a named binding. Probably should work; not in the locked five forms but a natural extension.
- **`what changed?`** — diff between two frame states. Useful for understanding mutations under a `lock`. v2.
- **`why?`** — natural-language causality query. "Why did this fail?" → walks back through error propagation. Speculative; might be too magical.
- **Persisted traces** — save a trace tree to disk for post-mortem. Need a serialization format. Out of scope for v1, but design the runtime so the tree is serializable.

---

## Next file

`09-mvp-slice.md` — what's the smallest thing we can build that runs an actual Marco program.
