---
# Hosts {#hosts}
---

Marco describes *intent*: actors, sentences, sequences, branches, messaging. It does
not, by itself, touch the outside world — it cannot press a key, sleep, read a pixel, or
hear a global hotkey. Those effects belong to a **host**: a provider, written in any
language, that fulfills the primitives Marco calls for.

The boundary between the two is the **act**. An act declares *what* foreign behavior
exists; a host supplies *how*. This is Marco's foreign-function interface. The design
goal is that Marco "plays nicely with other languages" — the host may be in-process Go,
or an out-of-process program in any language speaking a small JSON protocol.

---
## Foreign acts {#foreign-acts}
---

An act capability is **foreign** when it is `exports`-ed and has **no `does...` body**.
Such a capability has no Marco implementation; the runtime dispatches it to the
registered host instead of walking a block.

Canonical:
```marco
the OS is an act.
this exports Key.
this exports Click with Point.
this exports Sleep with Duration.
```

The same act may mix foreign and native capabilities — an exported capability *with* a
`does...` body remains ordinary Marco and is not foreign.

### Calling a foreign act

A foreign act is invoked exactly like any action — `do`, `start`, or `execute`:

```marco
the App is a script.
do OS's Key with "e"...
    when ok?  log "pressed".
    or?       log "failed".
```

### Result contract

A foreign capability may declare its result statuses through the usual compound-capability
contract (`it can Key with Text, gives KeyResult, and is Pressable.`). When it declares
none, the runtime assumes **`{ok, failed}`** — foreign code can always fail, and assuming
`ok`-only would make every `or?` arm a dead arm. Because failure floats, a bare
`do OS's Key.` at script root must handle the `failed` status (a branch group, `try`, or a
`finally`), exactly as for any failable Marco action.

A host may also return a **custom status word** (e.g. `clicked`); call sites observe it
as an ok-custom status (`when that is clicked?`), provided the declared contract permits
it.

---
## The Host interface {#host-interface}
---

A host is registered per act name at runtime construction. In Go:

```go
type Host interface {
    Invoke(call HostCall) (status string, data Value, err error)
}

type HostCall struct {
    Act    string          // owning act, e.g. "OS"
    Action string          // capability, e.g. "Key"
    Input  Value           // the `with ...` payload (may be absent)
    Out    io.Writer       // serialized with `log`; hosts that print use this
    Ctx    context.Context // canceled when the calling frame's tree is canceled
}
```

The returned `(status, data)` resolves the calling frame through the **same path** a
Marco `this is <status> with <data>!` return takes — so observers (`when ok?`,
`wait until X is ok`, status listeners) see an identical frame. A returned `err`
resolves the frame as `failed`.

When no host is registered for a foreign act, the runtime falls back to the **dryrun
host**, which logs the call and resolves `ok`. This keeps programs runnable — and golden
tests deterministic and cross-platform — without any real OS provider present.

Dryrun output:
```
[dryrun] OS's Key with e
```

---
## Host adapters {#host-adapters}
---

Two adapters implement `Host` behind the one interface:

- **Native (in-process).** Go code that calls the OS directly (e.g. Win32 `SendInput`
  for keystrokes/mouse, `Sleep`, pixel reads). Fastest; selected with `--host windows`.
- **Bridge (out-of-process).** A subprocess in any language, launched by the act and
  driven over stdio with newline-delimited JSON. Selected with `--host bridge:<path>`.
  This is how a host in AutoHotkey, Python, or Node fulfills the same act surface.

### Bridge protocol

One JSON object per line, request and response:

```json
→ {"act":"OS","action":"Key","input":"e"}
← {"status":"ok","data":null}
```

`input`/`data` are the JSON encoding of a Marco `Value` (text → string, number → number,
set → object, absent → null). One subprocess is started per act and reused for the run.

### Per-act host selection (layers)

`--host <spec>` may be repeated, and a spec may be prefixed `Act=` to bind one act to its
own host while others fall back to the default. This is how a run splits into **layers** —
the macro OS effects in one process, an overlay HUD in another:

```
marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay overlay.marco
```

A bare spec (`--host windows`) binds the default `*` host. Acts pointing at an identical
spec share a single subprocess. The runtime already dispatches per act name
(`hostFor(act)` → `hosts[act]`, else the default), so this is purely a selection concern.

### Bidirectional bridge

A bridge is normally request/response, but it may also **push events back** to a served
program (see *Events* below) by writing feed lines on the same stdout, interleaved with
responses. A single reader demuxes by shape — a line carrying `"feed"` is an event,
anything else is the response to the in-flight request:

```json
← {"feed":"Hotkeys","event":"Stop"}
← {"status":"ok","data":null}
```

This lets one process both fulfill an act *and* drive the program — e.g. the overlay HUD
renders the `Overlay` act and pushes the leader-key / typed-command events that move it.

---
## Events: host → Marco {#host-events}
---

The directions above are Marco calling *out*. Hosts also push events *in* — a global
hotkey press, a window-focus change — into a running program. These map onto Marco
**feeds**: the host writes events that feed listeners react to.

```marco
the Hotkeys is a feed of any.

when Hotkeys reads Leader?
    do OS's BeginCommand.

when Hotkeys reads Stop?
    do OS's CancelAll.
```

Events are fed in as one JSON object per line, either on the server's stdin or — for a
bidirectional bridge — on the bridge subprocess's stdout (the two are merged):

```
{"feed":"Hotkeys","event":"Leader"}
{"feed":"Hotkeys","event":"Stop"}
```

Receiving host events requires a **persistent** run (`marco serve <file>`): the program
stays alive while listeners are registered, draining inbound events until a quit event
arrives. When the run has bridge event sources, it shuts down as soon as any of them
closes (e.g. the overlay window is closed, so that process exits and its stdout ends);
with no bridge sources it shuts down when stdin closes. A stop event cancels in-flight frames via the frame's `Ctx` and the normal
cancellation tree. (One-shot `marco run` exits when the script body completes and is the
right mode for sequenced macros that do not wait on hotkeys.)
