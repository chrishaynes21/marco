---
status: historical
---

---
# Audit {#audit}
---

Snapshot of unresolved questions, inconsistencies, undefined syntax, and rule conflicts in the v1 spec. Use this to drive cleanup. Not a normative spec page.

---
## Resolutions Since First Audit {#resolutions}
---

**Final lock — container overview:**

- **Six core containers** locked: `set`, `list`, `map`, `queue`, `feed`, `channel`. Each now has a dedicated section in [[Data Model]] with canonical declaration, rules, access, mutation, and consumption forms.
- **Set canonical uses `its`** for declaration-mode field declarations: `the User is a set. its Name is a text.` (replaces the prior `this's Name is a text.` form). This formalizes `its` (the current frame set) as the declaration-mode possessive for fields. **Open follow-up:** sweep [[Declarations]] and other files where `this's <Field> is a <Type>.` still appears as canonical for set fields.
- **List / Map iteration** locked: `for each item in <Collection>...` with `key` (index or map key) and `item` (current value). See [[Data Model#List]] and [[Data Model#Map]].
- **Map declaration** uses `map of <KeyType> to <ValueType>`: `map of id to User`, `map of text to User`, `map of text to any`.
- **Queue / Feed / Channel** moved into [[Data Model]] as first-class container sections, with cross-links into [[Iteration#Queues]], [[Iteration#Feeds]], and [[Actors#Messaging Syntax — Channels]] for behavioral detail.
- **Container Traits** section added in [[Data Model#Container Traits]] — `lockable set`, `replayable feed`, `iterable set`.
- **Container summary lock:** "Sets hold fields, lists order items, maps key items, queues backlog work, feeds stream data, and channels carry conversation."

---

**Final lock — graph model clarification:**

- **Node enumeration locked.** Nodes include: actors, actions, scenes, acts, sets, frames, contracts, templates, traits, statuses, errors, feeds, channels, queues. See [[Graph#Nodes And Edges]].
- **Edges are created by sentence structure.** Concrete sentence-to-graph mapping table locked in [[Graph#Sentences As Graph Operations]] (covers declaration, capability, implementation, execution, observation, branch, resolution, message, listener, feed read/write, queue put/dequeue, lock, import).
- **Sentence Effect rule.** Every meaningful Marco sentence does exactly one of: creates a node, creates an edge, moves data across an edge, observes an edge, resolves a Frame through an edge. No "compute-only" sentences. See [[Graph#Sentence Effect]].
- **One-line lock revised:** "Marco is a sentence-driven graph language: syntax defines nodes and edges, frames are runtime traversals of that graph." Replaces the prior graph one-liner. [[Overview]] updated to match.

---

**Final lock — contracts, templates, traits:**

- **Contract definition refactored.** Compound member form: `it can <Action> [with <Input>] [, gives <Output>] [, and is <Contract>].` Status declaration moved from `this has status <X>.` to `this allows <X> [with <Shape>].` Action contract attachment moved from `that gives a <Contract>.` to `that is <Contract>.` Actor adoption: `it is a <Contract>.` Cross-applies to actors, actions, scenes, Frames.
- **Templates** added as a new top-level kind. `the X is a template.` declares optional interface; `it uses <Template>.` adopts. Members may but need not be implemented.
- **Traits** locked. Inline modifier on data types: `lockable set`, `replayable feed`, `iterable set`. Same name spaces (`lockable`, `iterable`, `replayable`) play role-of-contract on actions/actors and role-of-trait on data — same mechanism, different application site.
- **Doctrine summary:** Contracts impose, Templates allow, Traits compose. Adoption verbs: `is` (contracts), `uses` (templates), inline modifier (traits).
- **`optional` modifier** added to inputs and status shapes (`with an optional <Type>` makes the shape non-required).
- Vocabulary updated: `template`, `optional`, `allows`, `uses`, `gives`, `takes` added; `is` reaffirmed as the contract-adoption verb.
- Spec is closed. The remaining items in this document are historical and informational; new additions require explicit spec updates.

---

**Earlier final polish — v1 vocabulary lock:**

- **Expression rules** locked in [[Data Model#Expression Rules]]: left-to-right, parentheses for nesting, prefer readable intermediate lines.
- **Value vs Frame boundary** locked in [[Data Model#Value vs Frame Boundary]]: `<binding> is <Action>.` shorthand allowed when unambiguous, with the explicit two-line form as the canonical equivalent.
- **`that` scoping** restated in [[Data Model#`that` Scoping]] — always the previous Frame.
- **`finally`** semantics refined in [[Lifecycles#Cleanup — `finally`]]: runs on all five terminal states, runs on return, must not silently override, only overrides on explicit return.
- **`replayable`** added to built-in contracts. [[Contracts#Built-In Default Contracts]].
- **Built-In Vocabulary** — comprehensive flat reference locked in [[Syntax#Built-In Vocabulary]]. Sixteen categories (types, primitive types, frame slots, statuses, execution, control, messaging, feeds, queues, data/safety, naming, debug, traversal, iteration, contracts, operators). New built-ins require explicit spec updates.

---

**Earlier closing consolidation:** all remaining language-surface gaps locked.

- **Condition chaining** — `and` / `or` inside `when`. New section [[Core Concepts#Condition Chaining]].
- **Type-check predicate** — `is <Type>` formally defined as a runtime type proof, symmetric with `has`. See [[Core Concepts#Type Checks]].
- **Message payloads** — `says <Msg> with <Data>` (with optional `to <Target>`). See [[Actors#Message Payloads]].
- **Feed payloads** — `write <Msg> with <Data> to <Feed>`. See [[Iteration#Feeds]].
- **Frame ownership** — explicit lock that every Frame except the root has exactly one parent who owns its lifetime. See [[Frames#Ownership]].
- **Cancel propagation** — locked in [[Lifecycles#Cancellation Propagation]]: parent cancel cascades to children depth-first; children's cancel does not cascade up.
- **Default return** — restated as compile error with explicit "no implicit ok" rule. [[Phrases#Resolution]].
- **Operator set** — minimal v1: `+` only (text concat / numeric add). [[Data Model#Operators]].
- **`any` type** — formalized in [[Data Model#`any` Type]].
- **No null** — explicit lock. [[Data Model#No Null]].
- **Final lock** — Marco is a strict, readable language where execution is explicit, data is proven, and behavior flows through a graph of frames and actors.

---

**Earlier consolidation (source of truth before this one):** the prior consolidation prompt's snapshot:

- **Status predicates:** canonical is `when <frame> is <status>?` (with `is`). Shorthand drops both: `when <status>?`. The half-shorthand `when <frame> <status>?` (no `is`) is **not** canonical.
- **Built-in lifecycle status set locked:** `created`, `running`, `waiting`, `ok`, `failed`, `canceled`, `exited`, `died`. Resolves the long-standing Open Question.
- **Messaging verb:** `say` → `says` (third-person conjugated). New listener filter `when <Target> hears <Msg> from <Source>?`. Speakers/listeners terminology. Channels are the conversational, ephemeral concept.
- **Translators** added as a first-class declaration kind. Partial types via `as a partial <Type>`. Auto-mapping via `it maps what it can.` Promotion via `as a <Target>`.
- **`finally...`** cleanup phrase added — runs on all terminal states; cannot swallow failures.
- **`inspect <Value>.`** added as developer-facing introspection (distinct from `log` and `show`).
- **`channel`** added to container kinds.
- **Language Principles** locked in [[Overview#Language Principles]].
- **Final lock:** "Marco is a strict, graph-based language where frames execute, sets flow, actors communicate, and correctness is proven through structure."

---

Earlier canonical syntax updates locked the following:

**Branching grammar** — `when` / `or when` / `or?` is now the canonical branch group (see [[Core Concepts#Branching: `when` / `or when` / `or?`]]). `or when` adds additional conditions to the same group. Only one branch executes. `or?` captures remaining unmatched cases. Phrase-style `when ...` (no `?`) is not allowed in v1.

**Status syntax** — 

- **`it's` is no longer canonical for status.** Removed as a documented contraction for "current Frame status."
- **Frame status predicates** are now canonical:
  - `when that <status>?` — explicit canonical
  - `when <status>?` — shorthand, implies `that <status>?` when `that` is unambiguous
  - `when this <status>?`, `when <NamedFrame> <status>?` — also valid
  - `when X is <status>?` is no longer canonical
- **Returns read like English** — `is` stays in:
  - `this is <status>!` — canonical
  - `this is <status> with <data>!` — canonical
  - `this <status>!` — optional shorthand, secondary
- **`this` in process mode** is the current Frame being resolved (the one you return from); `that` is the previous / watched Frame.
- Lock: **Checks can be short. Returns should read like English.**
- **Rule Conflict D** (`this <status>!` undefined) — RESOLVED. Now canonical (with `is`).
- **Term Inconsistency C** (`it's` double meaning) — partially resolved: removed as status syntax. Declaration-mode possessive use of `it's` still requires reconciliation against the `this's <Member>` canonical.

State updates (`it is <status>.` ending in `.`) are unchanged for now. Open Question: whether state updates should also adopt `this is <status>.` for symmetry.

---
## Open Questions {#open-questions-rollup}
---

Rolled up from every `## Open Questions` section across the spec.

### Core Concepts
- Full set of recognized contractions for shorthand (currently observed: `it's`, `that's`).
- Whether identifiers (e.g. `User`, `Items`) are case-sensitive and how they are introduced.
- Formal clause grammar for run-on sentences beyond the locked `when` shorthand examples.

### Phrases
- Whether a phrase always begins with a default status before its first sentence executes.

### Actors
- Formal grammar for `say` without a `to` target (graph-scope routing).
- Listener prioritization or explicit sequencing syntax.
- Whether actors may filter messages by content rather than name only.
- Lifetime of message Frames after all listeners have completed.
- How group routing via sets composes with contract obligations across listeners.

### Contracts
- Formal syntax for representing floated statuses in source when not handled locally.

### Data Model
- Full primitive set beyond `text`, `number`, `boolean`, `time`, `id` (e.g., binary, decimal, currency).
- Whether lists support negative indices or slice access.
- Whether map keys are restricted to primitive types or may be arbitrary sets.
- Mutation semantics for nested sets and containers.
- Whether `time` is absolute (instant), relative (duration), or both.

### Declarations
- Whether declaration Frames are always `ok` or may be surfaced as non-`ok` in validation contexts.
- Whether duplicate declarations of the same name are errors or rebinding.

### Execution Model
- Whether `start X...` requires `as Name` to make the started Frame observable.
- Whether `execute X.` may target only named phrases or also inline phrases.
- Delivery guarantees and ordering rules for observed status changes from `start X...`.

### Frames
- Whether a Frame may be named more than once during execution.
- Whether renaming an already-named Frame rebinds or creates an error.
- Scope and lifetime of named Frames after the owning phrase resolves.

### Graph
- Formal grammar for `say ... to ...!` event emission.
- Whether listener ordering may be made deterministic by declaration.
- How module boundaries appear in the graph (subgraphs, namespaces, or both).
- Visualization conventions for the graph (debug view, runtime introspection).

### Iteration
- Canonical access for `previous`/`next` entries — RESOLVED: `previous's item` / `next's item` (see [[Iteration#For Each]]).
- Whether `for each` over a feed is bounded by feed lifetime or runs indefinitely until `stop`.
- Backpressure semantics for feeds with slow listeners.
- Whether multiple consumers may share a queue (work-stealing) or each gets a private cursor.
- Replay semantics for feeds beyond v1.

### Lifecycles
- The canonical set of status values beyond `ok` and `failed`.
- Whether `.` (state update) may carry a `with that` result, or whether results are exclusive to `!` (return).
- Resolution order when multiple `when` branches match the same status event.
- Scope and lifetime of a named Frame (e.g. `SaveFlow`) after its owning phrase resolves.

### Modules
- Visibility and export rules across modules.
- How acts surface event subscription versus polling.
- Lifecycle of actors relative to the owning script.

### Reference Modes
- Whether mode is always inferable from syntactic context, or whether some constructs straddle modes.
- Formal qualification syntax when a reference is ambiguous between modes.
- Whether declarations inside a process-mode phrase (e.g., a local `the X is a set.`) switch mode for that line only.

### Scoping
- Whether shadowing emits a compiler warning by default.
- Formal grammar for kind-based disambiguation when allowed.

### Testing
- Canonical grammar for `expect:` clauses.
- Scope of a mock — per-test, per-module, or global.
- Whether tests may observe side effects beyond Frame status and result.

### Spec-wide (added by audit)
- Definition of `active` and `settled` Frame states (used in `when` listener-vs-immediate-check rule but never defined).
- Canonical syntax for `copy` (referenced in immutability rules but no defining section).
- Canonical syntax for `show`, `log` (used in examples but undefined).
- Generalization of the `the <kind> <Name> is a <kind>.` form (currently only shown for `error`).
- Canonical `<Module>'s <Action>` invocation grammar.
- Whether multiple sentences may share a single line (used in `the App is a script. do MacroMarco's Start...`).

---
## Term Inconsistencies {#term-inconsistencies}
---

### `Frame` vs `frame` capitalization
- `Overview.md` mixes `frame` (lines 23–29 in Core Pronouns list) with `Frame` elsewhere.
- Every other file capitalizes `Frame` consistently.
- **Resolution:** prefer `Frame` everywhere; treat `frame` as informal.

### `that` definition drift
- `Frames.md`, `Core Concepts.md`, `Inference.md`, `Overview.md` define `that` as "most recent completed child or observed Frame".
- `Actions.md` (lines 158, 213) calls it "the result of the most recent Frame" / "the most recent Frame result".
- **Conflict:** Frames.md `## Frame vs Result` says **Frames act. Results do not.** Actions.md still treats `that` as a result.
- **Fix:** rewrite Actions.md occurrences to align with the unified Frame-not-result definition.

### `it's` double meaning
- **PARTIALLY RESOLVED.** `it's` removed as canonical status syntax. The remaining use of `it's <Member>` as possessive in declaration-mode field declarations (e.g., `it's input is a set.`) still conflicts with the `this's <Member>` canonical from Declarations.md.
- **Remaining fix:** pick one canonical for declaration-mode field declarations (`this's` or `it's`). Update Frames.md, Reference Modes.md, Lifecycles.md custom error types accordingly.

### `this's` vs `it's` for field declarations
- `Declarations.md` canonical form: `this's <Member> is a <Type>.`
- `Frames.md`, `Reference Modes.md`, `Lifecycles.md` use `it's <Member> is a <Type>.`
- **Fix:** pick one and update consistently. `this's` is the documented canonical; `it's` is the user's preferred declaration-mode form.

### `it has` vs `this has`
- `it has SaveFile.` (Reference Modes), `it has SaveButton.` (Actors group example).
- `this has status Saved.` (Contracts) uses `this has status` — a contract-specific declaration.
- Plain `it has X.` for membership is never formally introduced.
- **Fix:** define `<Subject> has <Member>.` as a declaration form, or remove the `it has` examples in favor of `this's <Member> is a <Type>.`.

### `do save` vs `do Save` (case)
- **PARTIALLY RESOLVED.** [[Core Concepts#Naming Convention]] now establishes lowercase = built-in, Capitalized = user-defined. User-defined actions like `Save` should be capitalized; existing examples with `do save` should be normalized to `do Save` in a sweep.

### `Saving` capitalized vs `saved`/`failed` lowercase
- Status values mix cases (`it is Saving.` capitalized; `when it's failed?` lowercase) without a stated rule.
- **Fix:** define a status-naming convention.

### `hears` vs `reads` (same construct, two verbs)
- `Actors.md`: `when <Target> hears <Message>?`
- `Iteration.md` (feeds): `when <Feed> reads <Message>?`
- Both register a listener for arriving values; no rationale for the split.
- **Fix:** unify on one verb, or document why feeds use `reads` (e.g., implies order-preservation).

### "result set" vs "result" vs "returned set"
- Used interchangeably across files. No single canonical phrasing.
- **Fix:** pick one (recommended: "result").

### `that gives a <Contract>` action contract attachment
- Defined in `Contracts.md` line 43 onward.
- `Actions.md` never references this form despite being the canonical place to declare actions.
- **Fix:** add a section to Actions.md showing `that gives a <Contract>.` in context.

### `phrase` vs `Frame` in Actions.md
- Actions.md uses both terms in adjacent lines without flagging them as equivalent. Phrases.md establishes phrase = open Frame.
- **Fix:** state the equivalence in Actions.md or use `Frame` consistently.

### `commit` listed only in Frames.md
- `do its commit.` appears in Frames.md as a capability example, but `commit` has no contract and no other reference.
- **Fix:** decide whether `commit` is a v1 capability or remove the example.

---
## Undefined Syntax Used In Examples {#undefined-syntax}
---

Forms used in `marco` code blocks but lacking a defining section.

### Undefined (no defining section anywhere)
- `copy <X>.` — referenced only in immutability prose ("sets are mutable when ... copied"); no syntactic form.
- `do Compare with previous's item and item.` — `with X and Y` multi-arg form not introduced (still open).
- `when any failed?` — `any` as wildcard target undefined.
- multi-sentence single-line layout (`the App is a script. do MacroMarco's Start...`).

### Resolved
- `log <X>.` — RESOLVED. Defined in [[Observability#Logging]] as a system built-in.
- `show <X> with <Y>.` — RESOLVED. Defined in [[Modules#Show — UI Rendering]] as UI rendering, not debug.

### Partially defined (illustrated but not formalized)
- `do its <Capability>.` / `do that <Capability>.` — capability invocation grammar not formally introduced as a syntactic form. Now indirectly addressed in `Frames#Capabilities`.
- `put <Item> into <Queue>.` — appears in Iteration queues example only.
- `write <Item> to <Feed>.` — appears in Iteration feeds example only.
- `the error <Name> is an error.` — kind-prefixed declaration shown only for errors; generalization unclear.
- `it has <Member>.` — used as membership declaration; not defined.
- `this has status <Name>.` — used canonically in Contracts but never grammar-spec'd.
- `this exports <Name>.` — Modules acts example only.
- `handle <Contract> with <Handler>.` — Contracts auto-handler example, marked speculative.
- `expect: <Clause>` — Testing tests, flagged as Open Question.
- `mock <X> with <Y>.` — Testing mocks; rule list short.
- `<Module>'s <Action>` qualified invocation — RESOLVED: locked in [[Modules#Imports]] alongside the lazy/selective import rule.
- `when first?` / `when last?` / bare-name boolean predicates inside `for each`.
- `<Container> has <X>?` — Iteration queue access form.
- `it's been <N> <unit>?` — time predicate form.
- `<Frame>'s <Field>` (e.g., `SaveFlow's Path`) — implied by `'s` but not enumerated for named Frames.
- `it is ok with that as a User, and Metadata as Meta!` — multi-value return clause grammar.
- `it is failed with <ErrorType>!` — returning a custom error type as value.
- `do X, then do Y, then do Z.` — `then` as a connector; not formally listed as a chaining keyword.
- `when Ready?` (subjectless) — bare `when X?` with no subject; meaning unclear.
- `this <status>!` and `this <status> with <X>!` (no `is`) — used in Frames.md and Reference Modes.md but Lifecycles only defines `it is <status>!`.

### Undefined Frame statuses used in examples
- `active`, `settled`, `Ready`, `Running`, `Saving` — used in `when` examples but never enumerated. Lifecycles flags this as Open Question.

---
## Rule Conflicts {#rule-conflicts}
---

### A. `that` is a Frame (Frames) vs `that` is a result (Actions)
- Frames.md `## Frame vs Result`: "Frames act. Results do not." `that` is a Frame.
- Actions.md `## that Inside can`: `that` is "the result of the most recent Frame".
- Direct contradiction. Actions.md needs to be updated.

### B. `that's <field>` rule with no caveat (Core Concepts, Inference, Frames#Result) vs `that's error` exception (Frames#Frame Slots)
- Three places state `that's <field>` accesses the result set.
- Frames#Frame Slots adds: `that's error` is special-cased to `that.error`.
- Self-consistent only if the special case is explicitly noted everywhere. Currently it's only noted in Frame Slots.
- **Fix:** add the `that's error` exception note to Core Concepts and Inference where the general rule appears.

### C. `it's` semantics
- Defined as "current Frame status" (Core Concepts, Overview, Frames Reference Model).
- Used as possessive on declared subject in declaration mode (Frames#Frame As Actor lines 22–26, Reference Modes, Lifecycles custom error types).
- Two definitions of the same surface form.

### D. `this <status>!` (no `is`) vs `it is <status>!`
- **RESOLVED.** `this <status>!` is now the canonical return form. `it is <status>!` is deprecated for returns. State updates (`it is <status>.`) preserved.

### E. `or?` semantics: must resolve (Contracts) vs may continue (Execution Model)
- Contracts.md `## or? Exclusive Fallback`: "must explicitly resolve, transform, or return what it captures".
- Execution Model.md `## Branch Grouping`: "after a branch resolves, execution continues after the group" — implies continuation is normal.
- Both can be reconciled (`or?` resolves, then if it didn't return, execution continues), but the wording is not harmonized.

### F. Phrase resolution rule vs declaration Frames being implicit `ok`
- **RESOLVED.** The compiler now enforces the Phrases.md rule for action and translator bodies: every path must end in an explicit terminal return. Branch groups with a bare `or?` propagate termination when every arm terminates; `lock <X>... <body>` propagates when its body always terminates; `finally...` is a deferred cleanup hook and does not satisfy termination on its own. Declaration Frames remain implicit `ok` — they are not phrases (no `...`).

### G. `failable with error` canonical payload
- Contracts.md: "`failable with error` means `failed` carries `its error`."
- Execution Model.md uses `it is failed with that!`.
- Inference.md permits `it is failed with that!` if `that` resolves to an error.
- Not a hard conflict, but the canonical payload could be stated more clearly.

---
## Recommended Cleanup Pass {#recommended-cleanup}
---

Priority for a follow-up cleanup chunk:

1. **Lock `that` definition** — rewrite Actions.md occurrences to match Frames.md.
2. **Resolve `it's` overload** — add a note in Reference Modes explaining declaration-mode possessive vs process-mode status; pick a canonical for field declarations.
3. **Cross-link `that's error` exception** — add the special-case note to Core Concepts, Inference, and Overview.
4. **Define or drop `this <status>!` (no `is`)** — Frames.md and Reference Modes.md must align with Lifecycles.
5. **Define `copy`, `show`, `log`** — or remove them from examples.
6. **Define `put ... into ...` and `write ... to ...`** as proper queue/feed mutation operators.
7. **Enumerate canonical Frame statuses** — closes the long-running Lifecycles open question and several inference rules.
8. **Standardize case** — decide identifier case sensitivity and normalize examples.
