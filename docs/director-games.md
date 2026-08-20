---
type: milestone
status: historical
---

# The game capability framework

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

    The Director understands semantics.
    Game plugins contribute semantics.
    Marco executes deterministic mechanics.
    No plugin may bypass the Director.
    Every mutation still lowers through legal Marco.

Games are not special cases inside the Director. They are **capability packs**: contributed
knowledge about one application, plugged into extension points that already existed.

## The claim, and how it is checkable

> Adding a game is adding a line to `registeredPacks()` in `cmd/director/gamewiring.go` and
> nothing else.

If adding a game required editing anything under `internal/director`, the framework would
have failed. `internal/director/boundary_test.go` forbids the Director importing
`internal/gamepacks`, so the claim is a property of the build rather than a discipline.

```
internal/director/game      what a capability pack IS          (Director-side)
internal/gamepacks/palworld one particular pack                (never imported by the Director)
cmd/director/gamewiring.go  where the two meet                 (composition root)
```

The same shape as the platform adapters, enforced the same way.

## What a pack contributes, and where each piece plugs in

| a pack contributes | it becomes | run by |
|---|---|---|
| `Interpreters()` | an `EntityIdentity` on an element | the enrichment pass, after fusion |
| `Procedures()` | `goal.Procedure` | the ordinary registry and expander |
| `ControlRoles()` | `goal.ContributedRole` | the ordinary alias table |
| `Conditions()` | `conditions.WorldCondition` | the ordinary wait engine |
| `Verifiers()` | `verify.EvidenceSource` | the ordinary verifier, weighed and capped |
| `Policies()` | `policy.Rule` | the ordinary policy engine, narrowing only |

That table is the design. A pack that wanted to do something not on it would be asking for
a second execution path, and the answer is no: the Director's value is that one pipeline
observes, plans, confirms, lowers to Marco, executes and verifies.

### Packs reason over belief, never evidence

A pack does **not** contribute a perception provider. `internal/director/perception` is the
only package that may see observations — that is what lets a new source be added without
touching anything that reasons — so a pack is handed the FUSED elements and says which of
them are inventory slots.

The consequence is the right one: a pack cannot capture a screen, run a model, or
contribute evidence nobody weighed. It annotates what the ordinary pipeline established.

A pack that genuinely needs a new SOURCE — a vision model reading a full-screen game that
exposes no accessibility tree — contributes it as an ordinary perception provider wired at
the composition root, and its `Interpreter` turns that output into entities exactly the
same way.

## Entities

`directorapi.EntityIdentity` is what a control is inside the **application's own model**,
where `ResourceIdentity` is what it is outside one.

    item · slot · container · equipment · queue · recipe · meter · creature · station · marker

A closed vocabulary, and nothing in it says "game": a slot, a container and a queue are the
shapes a mail client's folder list and a build system's job list have too. What is
game-specific is which controls carry which entities, and that is what a pack contributes
and the Director never learns.

**Quantity is a pointer, and that is load-bearing.** An empty slot holds zero; a slot
nobody could read holds an unknown number. Conflating them is how "deposit everything"
silently leaves four things behind, so `Inventory.Full()` returns *(full, known)* and every
selection reports what it skipped.

No memory reading. Nothing in this design reads a game's memory, and there is no field in
`EntityIdentity` that would carry the result if something did.

## Detection

Four signals, because each is wrong alone:

| signal | weight | why it is not enough |
|---|---|---|
| process / executable | 0.40 | mod loaders and launchers rename executables |
| window title | 0.35 | overlays and streaming software decorate titles |
| interface labels | 0.60 | arrives only once the game draws something |
| the pack's own entities | 0.70 | the strongest: the interface was *modelled* |

Weights combine probabilistically, never add, so no accumulation of guesses reaches
certainty. The threshold is 0.60: **no single weak signal reaches it and any two do** —
arithmetic, asserted by a test, not intention.

A tie between two packs **detects nothing**. Picking by registration order is how a user's
Minecraft gets another game's procedures.

## Safety

    Plugins declare: online, competitive, protected, automation restrictions.
    Supportive automation is preferred.
    The framework should make these policies explicit rather than hiding them.

### The vocabulary does the work

There is no filter here that tries to recognise an aimbot. Filters get argued with. Instead
the `Automation` vocabulary contains **no value** for combat, aiming, movement or player
interaction:

    inventory · crafting · menus · organization · accessibility · reminders

A pack cannot permit what it cannot write down. Adding a permission is a change to
`internal/director/game/game.go`, reviewed there, rather than a line in a pack nobody reads.

### Three declarations, three answers

- **Protected** → every action refused, absolutely. An application that ships measures
  against automation has said what it wants. There is deliberately nothing configurable
  here.
- **Nothing permitted** → refused. A pack that recognises an application and permits
  nothing has said something useful.
- **Competitive** → supportive automation is allowed and **every action is confirmed**,
  because it affects other people and the player should be the one deciding, each time.

### Procedures declare what they are

`game.Procedure` pairs a `goal.Procedure` with the `Automation` it declares itself to be,
and registration **refuses a pack whose procedure declares something it does not permit**.
That is what makes Part 11's distinction structural: the Director does not start, rather
than the fault being discovered when a user asks for that procedure on a live game.

### A contributed rule can only narrow

`policy.Rule` returns a `Verdict` with `Refuse` and `Confirm` and no field meaning "allow".
Adding a pack can make the Director more cautious and can never make it less — and a pack
written to unlock something does not compile.

## Verification stays semantic

    Craft → evidence: craft queue changed, inventory changed, item count increased.
    Not: pixels moved.

The Director sees "the element count changed", which is weak evidence of something. It
cannot see that the player's inventory went from eleven filled slots to none while the
container went from none to eleven — which is not weak evidence, it is what depositing IS.

So a pack contributes evidence sources, with three properties that make it safe:

- **Additive.** A source appends; there is no return value that could remove the Director's
  own evidence.
- **Capped.** `verify.MaxContributedWeight` is 0.7, below the Director's strongest signals,
  so a pack cannot declare its own action verified by asserting a large enough number.
- **Two worlds and nothing else.** A source cannot act, observe or reach a host.

An **inconclusive** verdict that contributed evidence did not rescue stays inconclusive.
Turning "I could not tell" into "it did not happen" would be the worst possible use of the
seam: the caller would retry an action that may have landed.

## Two general outcomes arrived with the first pack

`sort` and `craft` are now in the Director's goal vocabulary, with generic procedures.
Neither is a game concept — a file manager, a mail client and a music library all sort, and
"choose what to make, then start making it" is a build tool's target list as much as a
workbench. They are in the Director because they are general; Palworld's versions are
ordinary application overrides.

That is the test for whether something belongs in the vocabulary or in a pack, and the
first pack is what surfaced it.

## The Palworld pack

Scope: **inventory · crafting · storage · base UI · Pal management.** Not combat, not
movement, not capture — and not because the pack has not got round to it, but because the
`Automation` vocabulary cannot express it.

```
director game               Palworld, 84% confident, mode: palbox
director capabilities       what the pack contributes, and what it permits
director explain game       why it was chosen and what else was considered
director explain inventory  what the Director can see of the player's holdings
marco games                 the same, from the engine's own CLI
```

## Known gaps

- **Nothing in this milestone has been run against a running game.** Every test is against
  fixtures. The framework is proven — registration, detection, refusal, enrichment,
  expansion, evidence — and the Palworld pack's label tables are written from the game's
  documented English interface and have never been compared to a live window.
- **Whether Palworld exposes an accessibility tree at all is unknown.** Most full-screen
  games expose nothing, in which case the pack's interpreter will see no elements and
  detection will fall back to the process and title signals. That is the honest expected
  outcome of the first live run, and the fix is a vision or OCR provider wired at the
  composition root — which the framework already supports and this pack does not yet use.
- **`Detect` sees no real process metadata yet.** It reads the active application's
  executable and PID from the window-system provider; whether that provider populates them
  on Windows is untested.
- **The incubator procedure was dropped.** "Start another incubator" had no goal in the
  vocabulary that fits, and inventing one to hold a single pack's step would have been the
  game-specific hack this milestone forbids. It is a good candidate for Learn:
  demonstrate it once and the learned procedure joins the same registry.
- **`Interpreter` reads a control's parent label from an attribute.** Which container a
  slot belongs to comes from `parent_label`, which the accessibility bridge may or may not
  set. Where it is absent, a slot is recognised but placed in no container — and a
  container-scoped query then finds nothing rather than the wrong thing.
- **Categories are a partial table.** An item the pack has not classified gets no category,
  so "everything except food" includes it. That is the right failure: the alternative is
  guessing an unknown item is food and leaving it behind.
- **One pack.** The framework's claim that adding a game touches nothing in the Director is
  argued from its structure and the boundary test, and has not yet been demonstrated by
  adding a second.
