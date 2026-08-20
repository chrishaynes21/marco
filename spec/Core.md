---
status: normative
---

# Marco Core v1 {#core}

This is the language. Everything a Marco program needs to say, and nothing else.

The other pages in `spec/` describe the same language in more detail, or describe extensions,
or record thinking that was never built. **This page is the promise.** If a construct is not
here and not listed under [[Core#Supported Extensions]], a Marco program should not rely on it
and Director must not generate it.

---
## The one-line rule {#rule}
---

> Marco is a play. The audience reads the script; the machinery stays backstage.

A person who has never programmed should be able to read a Marco program and answer:

- What thing is being acted on?
- What is being done to it?
- What information is being supplied?
- What happens if it works, and if it doesn't?
- What repeats, and what branches?

If a construct makes those questions harder to answer, it does not belong on stage. See
[[Core#Governance]].

---
## The four nouns {#nouns}
---

Marco is a play, and the words are the words a play uses.

> **Acts** organise what the play can do.
> **Scenes** describe where things happen.
> **Actors** are the things in the play.
> **Verbs** are what they do.

That is the whole vocabulary. Everything below is those four sentences said more slowly.

| Word | What it is |
|---|---|
| `act` | the way in. It offers capabilities outwards, and a host or another file fulfils them. `use` brings one in. |
| `scene` | a place in the play. It holds the actors that are there and knows verbs of its own. |
| `actor` | a thing in the play. It holds what belongs to it and does what it can do. |
| verb | something an act, a scene or an actor does — written `this can <Name>.` |
| `script` | the play itself: what happens when you run it |
| `set` | a shape: a thing with named parts |

An act, a scene and an actor behave the same way once the play is running — each holds things,
knows verbs, and can be asked to do them. They are different words because they mean different
things to whoever is reading, and the compiler holds them to it: **only an act offers
capabilities outwards.** A scene or an actor that tried to would be claiming to be a way in.

```marco
the Greeter is an actor.

this can Run.
this's Run does...
    log "hello".
    this is ok!

the App is a script.

do Greeter's Run...
    when ok?
        log "done".
    or?
        log that's error.
```

That program is complete. It parses, compiles and runs.

### act — the world's surface

An `act` declares capabilities it does not implement. A host fulfils them: real keystrokes on
Windows, printed lines under `--host dryrun`, an external program under `--host bridge:…`. See
[[Hosts]].

```marco
the Keyboard is an act.
this exports Type.      // type a text string
this exports Key.       // press a key or chord
```

An act is the only place in Marco where a capability may have no body, and the only door
through which a program affects anything outside itself.

### set — a shape

```marco
the Point is a set.
this's X is a number.
this's Y is a number.
```

Primitive types: `text`, `number`, `boolean`, `time`, `id`, `any`.

---
## Saying what an actor can do {#capabilities}
---

```marco
the Mover is an actor.

this can Run.
this can Measure.

this's Run does...
    this is ok!

this's Measure does...
    this is ok with 1!

the App is a script.

do Mover's Run...
    when ok?
        log "moved".
    or?
        log that's error.
```

`this can …` declares the capability. `this's <Cap> does…` gives it a body. That is the whole
declaration — a capability does not announce its shapes, because most of the time the sentence
that calls it already says everything a reader needs:

```marco-body
do Mover's Measure...
    when ok?
        log that's result.
    or?
        log that's error.
```

Inside a body, `input` is what was passed in, and `this is ok with <value>!` is what comes back.

When shapes DO need to be written down and checked — a shared surface, several implementers — a
contract says so, and that is an extension rather than everyday Marco. See [[Contracts]].

---
## Doing things {#doing}
---

```marco-body
do Keyboard's Type with "hello".
```

`do <Actor>'s <Capability>` — optionally `with <value>`. That is the only way one part of a
program reaches another, and it reads the same whether the capability is one line away or on
the other side of a host bridge.

To act on the result, open the sentence with `...` and answer it:

```marco-body
do Mover's Measure...
    when ok?
        log that's result.
    or?
        log that's error.
```

---
## Finishing {#finishing}
---

A capability body must finish. There is no implicit success.

```marco-body
this is ok with 42!
```

The four endings:

| Sentence | Means |
|---|---|
| `this is ok!` | done, nothing to hand back |
| `this is ok with <value>!` | done, and here is the answer |
| `this is failed with error "…"!` | did not work, and here is why in plain words |
| `this is that!` | same as that one — adopts the last outcome verbatim |

The last is how a person actually talks, and it is why a passthrough does not need a variable.

---
## Naming things as you go {#binding}
---

```marco-body
the p1 is a Point with X 308, Y 478.
the total is 3.
do Keyboard's Type with "x".
```

`the <name> is <value>.` names something for the rest of the block. A lowercase name is a
local; a capitalised one is a declaration. That is the whole rule.

---
## Branching {#branching}
---

```marco-body
when that is ok?
    log "worked".
or?
    log "did not".
```

`when <question>?` opens an arm; `or?` is the fallback. Questions may ask about status
(`when ok?`), about data (`when input's Count exists?`), or be joined with `and` / `or`.

---
## Repeating {#repeating}
---

```marco-body
for each item in Names...
    log item.

repeat 3 times...
    do Keyboard's Key with "e".

while that is ok...
    do Mover's Measure.
```

`stop.` leaves the loop. `skip.` goes to the next turn.

---
## Waiting {#waiting}
---

```marco-body
wait until that is ok...
    log "there".
```

Marco waits for a condition, not for a duration. Sleeping is a capability an act provides, not
a language feature — because "wait 500ms and hope" is a thing a program does, not a thing a
language should make easy.

---
## Cleanup {#cleanup}
---

```marco-body
finally...
    do Keyboard's KeyUp with "shift".
```

Runs however the surrounding work ended, including cancellation. It cannot turn a failure into
a success by accident.

---
## Speaking about things {#pronouns}
---

| Word | Means |
|---|---|
| `this` | the capability currently being carried out |
| `this's X` | something belonging to it |
| `that` | the last thing that finished |
| `that's X` | something belonging to that — `that's result`, `that's error` |
| `it`, `it's`, `its` | the current frame, its status, its data |
| `input` | what was passed in |

`this's` and `that's` are settled and are not open for redesign. They exist so a program can
refer to the obvious thing without inventing a variable to hold it, and they read as English
possessives because that is what they are. See [[Core Concepts]].

---
## Bringing in another file {#modules}
---

```marco
use os.

the App is a script.

do OS's Key with "e"...
    when ok?
        log "pressed".
    or?
        log that's error.
```

---
## Comments {#comments}
---

```marco-body
// A note for whoever reads this next.
```

---
## Supported Extensions {#extensions}
---

These are implemented, tested, and normative — but a program does not need them, and Director
does not currently generate any of them. They are listed here so a reader knows they are real.

| Area | Constructs | Page |
|---|---|---|
| Collections | `list`, `map`, `queue`, `feed`, `channel` | [[Data Model]] |
| Contracts | `the X is a contract.`, `it can …`, `this allows …`, `that is <Contract>.` | [[Contracts]] |
| Messaging | `says`, `hears`, channels | [[Actors]] |
| Concurrency | `start … as <Name>`, `execute`, `cancel`, `lock` | [[Lifecycles]], [[Locking]] |
| Conversion | `translator`, `as a <Type>`, `it maps what it can.` | [[Translators]] |
| Optionality | `template`, `it uses <Template>.`, `optional`, `partial` | [[Contracts]] |
| Traits | `lockable set`, `replayable feed`, `iterable set` | [[Data Model]] |
| Testing | `the X is a test.`, `expect …`, `mock … with …` | [[Testing]] |
| Diagnostics | `show`, `inspect`, `what <ref>?` | [[Observability]] |

---
## Governance {#governance}
---

Three rules, and they exist because the pressure runs the other way.

**Director may grow intelligence. Marco may grow expressiveness only when Director needs a new
legal effect or orchestration primitive that cannot already be expressed clearly.** Perception,
semantic memory, hypotheses, evidence, learning, tracking, planning and verification are
Director's business. None of them is a Marco noun, and none of them should become one because
it happens to be representable.

**Generated Marco must remain understandable without exposing backstage machinery.** Director
is allowed to know *why* it chose an action. Marco says *what it intends to do*. A generated
program that reads like a data dump has failed even if it runs.

**New syntax requires a demonstrated language-level need, not an implementation concept that
could be represented in the language.** Every permanent word is a cost paid by every future
reader. The question is never "could this be a keyword" but "does a person reading a play need
this word".

Mechanically: every fenced `marco` block on a page marked `status: normative` is compiled by
`internal/spectest`, and generated Marco is checked against this page's vocabulary. Both fail
loudly. See [[Core#Drift]].

---
## Drift {#drift}
---

Every page in `spec/` declares its status in frontmatter:

| Status | Means |
|---|---|
| `normative` | this is Marco. Examples compile, and a test proves it. |
| `reference` | detail about normative constructs. Examples compile. |
| `historical` | a record of how a decision was reached. Not a promise. |
| `experimental` | an idea. **Not implemented, or not settled. Do not build to this.** |

A future session reading `spec/` should never have to guess which of those it is looking at.

---
## Where things happen {#scenes}
---

```marco
the Knight is an actor.
this can Charge.
this's Charge does...
    log "the knight charges".
    this is ok!

the Battlefield is a scene.
this's Hero is a Knight.
this can Begin.
this's Begin does...
    do Knight's Charge.
    this is ok!

the App is a script.

do Battlefield's Begin...
    when ok?
        log "the battle begins".
    or?
        log that's error.
```

A scene says where something happens and what is there. `this's Hero is a Knight.` is the
ordinary sentence for saying a thing has a thing — a scene saying it about an actor is a scene
holding an actor, and it needs no special word.

A verb is what one of them does: `this can Begin.` names it, `this's Begin does…` says how, and
it has to finish. That is true of an act's verbs, a scene's verbs and an actor's verbs alike.
