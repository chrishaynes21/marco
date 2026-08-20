---
type: reference
status: active
updated: 2026-08-10
source_paths:
  - spec/Core.md
  - internal/screenmod/screen.marco
  - internal/director/marcoexec/play.go
---

# Reading Marco

Enough of the language to read any route or generated play in this repository, in one page.
`spec/Core.md` is normative; this is the on-ramp. Every other page in `spec/` declares a `status:`
— `normative`, `reference`, `historical` or `experimental` — so you can tell *this is Marco* from
*this was an idea for Marco* without reading the code.

## The idea

A Marco program is **a play somebody reads**. That is not a metaphor about the implementation, it
is the constraint the language is designed around: if a sentence would not make sense read aloud
to the person whose computer it is about to drive, it does not belong in the language.

## The four kinds of declaration

```marco
the Screen is an act.        // the way IN: offers capabilities to the outside world
the Settings is a scene.     // where things happen; holds actors, knows verbs
the Volume is an actor.      // a thing in the play; has its own things and verbs
the App is a script.         // the entry point
```

`act`, `scene` and `actor` are **settled and distinct**, and they share one representation on
purpose. Only an act offers capabilities outwards. `internal/spectest` holds that line.

## Saying what a thing can do

```marco
this can Mute.               // declare the capability
this's Mute does...          // define it
    ...
```

`this's` is *the thing being defined*. `that's` is *the last outcome* — as in `log that's error.`
Both are settled; do not look for a third pronoun.

## Doing something

```marco
do OS's Navigate with "down".            // fire and forget
do Screen's Showing with "the pause menu"...   // ... means "and then branch on it"
    when ok?
        ...
    or?
        ...
```

The `...` at the end of a `do` means the sentence continues into arms. `when ok?` is the success
arm; `or?` is everything else.

## The four endings

Every path through a body must end in one of these. A body that falls off the end is a compile
error, not a warning.

| | |
|---|---|
| `this is ok!` | done, nothing to hand back |
| `this is ok with <value>!` | done, and here is the answer |
| `this is failed with error "…"!` | did not work, and here is why in plain words |
| `this is that!` | same as that one — adopts the last outcome verbatim |

They are **exclamations**. A full stop instead of `!` leaves the arm without an ending, which the
compiler rejects.

## The one thing that trips everybody up

> A return inside an arm ends **the arm**, not the capability.

So this compiles and does **not** guard — execution walks straight on to the effects:

```marco
do Screen's Showing with "the pause menu"...
    or?
        this is failed with error "wrong screen"!
do OS's Navigate with "down".        # ← still runs
```

The correct shape nests the effects inside the success arm, so there is no line after the block
for control to fall through to:

```marco
do Screen's Showing with "the pause menu"...
    when ok?
        do OS's Navigate with "down".
        this is ok!
    or?
        this is failed with error "wrong screen"!
```

**Compiling is not behaving.** Any review of generated Marco that only reads the source, or only
checks that it compiles, will miss this class of bug — it already did once. See [[Learned-Plays]].

## Reading a generated play

```marco
// Marco learned this by watching. Rename it and change it however you like.
use os.
use screen.

the Volume is an actor.

this can Mute.
this's Mute does...
    do Screen's Showing with "the pause menu"...
        when ok?
            do OS's Navigate with "down".
            do OS's Navigate with "confirm".
            do Screen's Showing with "the audio page"...
                when ok?
                    this is ok!
                or?
                    this is failed with error "this play expected to finish on the audio page"!
        or?
            this is failed with error "this play starts on the pause menu"!

the App is a script.

do Volume's Mute...
    when ok?
        log "done".
    or?
        log that's error.
```

Out loud: *this play only starts on the pause menu; it presses down then confirm; it expects to
finish on the audio page.* Three claims, all visible, none hidden in a sidecar.

## The governance rule

**Director may grow intelligence. Marco may grow expressiveness only when Director needs a legal
effect or orchestration primitive that cannot already be expressed clearly.**

Perception, memory, hypotheses, evidence, learning, planning and verification are Director's
business and are **not Marco nouns** — not because they could not be represented, but because a
Marco program is a play somebody reads. Every permanent word is a cost paid by every future
reader.

Language work is closed. If a review concludes Marco needs new syntax, that is a significant
finding and needs a demonstrated language-level need, not an implementation concept that could be
represented with what exists.

## Related

- `spec/Core.md` — normative; `spec/Hosts.md` — the host-FFI design
- [[Learned-Plays]] — what generated Marco is allowed to say, and why
- [[Marco-Boundary]] — every desktop effect lowers to legal Marco source
- [[Audit]] — the invariants a reviewer should try to break
