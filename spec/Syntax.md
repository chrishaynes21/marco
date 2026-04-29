---
# Syntax {#syntax}
---

Single-page reference for canonical Marco v1 forms. Each entry links to the defining section. Forms marked **partial** appear in canonical examples but lack a fully formalized grammar — see [[Audit#Undefined Syntax]].

---
## Sentence Punctuation {#sentence-punctuation}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `.` | State update / completed sentence | [[Core Concepts#Sentences]] |
| `!` | Return from owning phrase | [[Core Concepts#Sentences]] |
| `?` | Question / listener | [[Core Concepts#Sentences]] |
| `...` | Open phrase | [[Phrases#Definition]] |

---
## Pronouns {#pronouns}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `it` | Current Frame | [[Core Concepts#`it` — Frame]], [[Frames#Reference Model]] |
| `its` | Current Frame set | [[Core Concepts#`its` And `it's`]] |
| `input` | Current Frame input set | [[Core Concepts#`input` — Received Set]] |
| `this` | Current Frame being resolved (process mode) / declared subject (declaration mode) | [[Reference Modes]] |
| `that` | Previous / watched Frame (most recent completed child or observed) | [[Frames#Frame vs Result]] |
| `that's <field>` | Field on `that`'s result | [[Frames#Frame Slots]] |
| `that's error` | Frame's `error` slot (special case) | [[Frames#Frame Slots]] |

`it's` as "current Frame status" is no longer canonical. Use the [[#Frame Status Predicates|status predicate]] forms below.

---
## Declarations {#declarations}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `the <Name> is a <Type>.` | Declare named subject | [[Declarations#Canonical Form]] |
| `the <Name> is a contract.` | Declare contract | [[Contracts#Definition]] |
| `the <Name> is a template.` | Declare template | [[Contracts#Templates]] |
| `the error <Name> is an error.` | Declare error type (kind-prefixed) | [[Lifecycles#Custom Error Types]] |
| `this's <Member> is a <Type>.` | Declare member field | [[Declarations#Field Declarations]] |
| `this can <Capability>.` | Declare capability | [[Actions#Action Declaration Forms]], [[Contracts#Capabilities]] |
| `it can <Action> with <Input>, gives <Output>, and is <Contract>.` | Compound contract member | [[Contracts#Definition]] |
| `this is <Contract>.` | Adopt a contract | [[Contracts#Built-In Default Contracts]] |
| `it is a <Contract>.` | Actor adopts contract | [[Contracts#Definition]] |
| `that is <Contract>.` | Action adopts contract | [[Contracts#Action Contract Declaration]] |
| `it uses <Template>.` | Adopt a template | [[Contracts#Templates]] |
| `<Trait> <BaseType>` | Compose trait into data type (e.g., `lockable set`) | [[Contracts#Traits]] |
| `this allows <Status> [with <Shape>].` | Declare allowed status in contract | [[Contracts#Definition]] |
| `this allows <Status> with an optional <Shape>.` | Declare allowed status with optional shape | [[Contracts#Definition]] |
| `<Set>'s <Member> is <Value> by default.` | Default value | [[Data Model#Defaults]] |

---
## Phrase Invocation {#phrase-invocation}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `does...` | Open behavior phrase | [[Actions#Canonical Behavior Opener]] |
| `do <Subject>.` | Invoke (completed) | [[Phrases#Definition]] |
| `do <Subject>...` | Invoke (blocking phrase) | [[Execution Model#Blocking]] |
| `do <Subject> with <Input>...` | Invoke with input | [[Frames#Input Model]] |
| `start <Subject>...` | Async observable | [[Execution Model#Asynchronous And Observable]] |
| `start <Subject> as <Name>...` | Async with named Frame | [[Execution Model#Asynchronous And Observable]] |
| `execute <Subject>.` | Fire-and-forget | [[Execution Model#Fire-And-Forget]] |
| `wait until <Condition>...` | Observe until true | [[Iteration#Waiting]] |
| `finally...` | Cleanup phrase — runs on all terminal states | [[Lifecycles#Cleanup — `finally`]] |

---
## Branching {#branching}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `when <Condition>?` | Question / listener | [[Core Concepts#`when` — Question Mode]] |
| `when <C1> and <C2>?` | All conditions must match | [[Core Concepts#Condition Chaining]] |
| `when <C1> or <C2>?` | Any condition matches | [[Core Concepts#Condition Chaining]] |
| `or when <C>?` | Add condition to branch group | [[Core Concepts#Branching: `when` / `or when` / `or?`]] |
| `or?` | Exclusive fallback | [[Contracts#`or?` Exclusive Fallback]] |
| `when <ref> is <Type>?` | Type-check predicate | [[Core Concepts#Type Checks]] |
| `do X, when <C>, do Y!` | Run-on inline `when` | [[Core Concepts#Run-On `when`]] |
| `if <Condition>, <Return>!` | Compact conditional return | [[Lifecycles#Returns]] (Branch-Return Shorthand) |
| `or, <Return>!` | Compact fallback return | [[Lifecycles#Returns]] (Branch-Return Shorthand) |

---
## Frame Status Predicates {#frame-status-predicates}
---

A status check is `when <frame> is <status>?` (canonical) or `when <status>?` (shorthand). The half-shorthand `when <frame> <status>?` is **not** canonical.

| Form | Meaning | Defined in |
|------|---------|------------|
| `when that is <status>?` | **Canonical** — previous / watched Frame | [[Lifecycles#Frame Status Predicates]] |
| `when this is <status>?` | **Canonical** — current Frame | [[Lifecycles#Frame Status Predicates]] |
| `when <NamedFrame> is <status>?` | **Canonical** — named Frame | [[Lifecycles#Frame Status Predicates]] |
| `when <status>?` | **Shorthand** — implies `that is <status>?` | [[Lifecycles#Frame Status Predicates]] |

Examples: `when that is failed?` / `when failed?` (shorthand); `when SaveFrame is ok?`; `when this is canceled?`.

---
## Returns And Status {#returns-and-status}
---

Returns read like English. The canonical form keeps `is`.

| Form | Meaning | Defined in |
|------|---------|------------|
| `it is <Status>.` | State update (no return) | [[Lifecycles#State Updates]] |
| `this is <Status>!` | **Canonical** status return | [[Lifecycles#Returns]] |
| `this is <Status> with <Data>!` | **Canonical** status return with data | [[Lifecycles#Returns]] |
| `this <Status>!` | Optional shorthand (secondary) | [[Lifecycles#Returns]] |
| `this is failed with error <Text>!` | Error shorthand | [[Lifecycles#Errors]] |
| `this is failed with its error!` | Return current error | [[Lifecycles#Errors]] |
| `this is failed with that's error!` | Return previous frame's error | [[Lifecycles#Errors]] |
| `this is failed with <ErrorType>!` | Return with custom error **partial** | [[Lifecycles#Custom Error Types]] |
| `this is ok with that as a <Type>!` | Cast on return | [[Inference#Result Inference]] |

Rule: **Checks can be short. Returns should read like English.**

`it is <Status>!` and `it's <Status>!` are no longer canonical for returns.

---
## Frame Capabilities {#frame-capabilities}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `do its <Capability>.` | Invoke capability on current Frame **partial** | [[Frames#Lifecycle Behavior]], [[Frames#Capabilities]] |
| `do that <Capability>.` | Invoke capability on child Frame **partial** | [[Frames#Capabilities]] |

Built-in capabilities discussed in examples: `rollback`, `commit`, `cancel`, `retry`. (Note: `commit` only appears in Frames.md.)

---
## Built-In Contracts {#built-in-contracts}
---

| Contract | Adds | Defined in |
|----------|------|------------|
| `failable` | `failed` status | [[Contracts#Built-In Default Contracts]] |
| `failable with error` | `failed` carries `its error` | [[Contracts#Built-In Default Contracts]] |
| `waitable` | active/waiting/finished states | [[Contracts#Built-In Default Contracts]] |
| `cancelable` | `canceled` status | [[Contracts#Built-In Default Contracts]] |
| `lockable` | locked/unlocked states | [[Contracts#Built-In Default Contracts]], [[Locking]] |
| `retryable` | retry/retried states | [[Contracts#Built-In Default Contracts]] |
| `iterable` | `for each` traversal | [[Contracts#Built-In Default Contracts]], [[Iteration#Iterable Contract]] |

---
## Data {#data}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `"<text>"` / `'<text>'` | String literal | [[Data Model#Literals]] |
| `42`, `true`, `false` | Number, boolean literals | [[Data Model#Literals]] |
| `\"`, `\'`, `\\`, `\n`, `\t` | Escape sequences | [[Data Model#Literals]] |
| `<Subject>'s <Member> is <Value>.` | Assignment | [[Data Model#Assignment]] |
| `<Type> with <Field> <Value>, ...` | Inline construction | [[Data Model#Inline Construction]] |
| `the <Name> is <Source>.` | Destructuring / binding | [[Data Model#Destructuring]] |

---
## Containers {#containers}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `the <Name> is a set.` | Set declaration | [[Data Model#Sets]] |
| `the <Name> is a list of <Type>.` | List declaration | [[Data Model#List]] |
| `the <Name> is a map of <KeyType> to <ValueType>.` | Map declaration | [[Data Model#Map]] |
| `the <Name> is a queue.` | Queue declaration | [[Iteration#Queues]] |
| `the <Name> is a feed.` | Feed declaration | [[Iteration#Feeds]] |
| `the <Name> is a channel.` | Channel declaration | [[Actors#Messaging Syntax — Channels]] |
| `<Set>'s <Member>` | Set / map access | [[Core Concepts#Access]], [[Data Model#Map]] |
| `<List> at <Index>` | List access | [[Core Concepts#Access]], [[Data Model#List]] |

---
## Comparison And Existence {#comparison}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `when <X> is <Value>?` | Equality | [[Data Model#Comparison]] |
| `when <X> is not <Value>?` | Inequality | [[Data Model#Comparison]] |
| `when <Path> exists?` | Path-style existence predicate | [[Data Model#Existence]] |
| `when <Container> has <Field>?` | Presence proof — gates safe access in branch body | [[Data Model#Presence and Safe Access]] |

---
## Iteration {#iteration}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `for each <Name> in <Collection>...` | Named iteration | [[Iteration#For Each]] |
| `for each in <Collection>...` | Anonymous iteration | [[Iteration#For Each]] |
| `while <Condition>...` | While loop | [[Iteration#While Loops]] |
| `skip.` | Continue to next iteration | [[Iteration#Loop Control]] |
| `stop.` | Exit loop | [[Iteration#Loop Control]] |
| `item`, `key`, `previous`, `next`, `first`, `last` | Iteration state | [[Iteration#For Each]] |
| `for each item in <Collection>...` | Canonical iteration form | [[Iteration#For Each]] |
| `when <ref> is <Type>?` | Runtime type proof — gates safe access | [[Iteration#Typed and Untyped Collections]] |
| `the <Name> is a <Container> of <Type>.` | Typed collection | [[Iteration#Typed and Untyped Collections]] |
| `the <Name> is a <Container> of any.` | Untyped collection (requires type proof) | [[Iteration#Typed and Untyped Collections]] |

---
## Messaging {#messaging}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `says <Message> to <Target>!` | Speak on channel | [[Actors#Messaging Syntax — Channels]] |
| `says <Message> with <Data>.` | Speak with payload (graph-scope) | [[Actors#Message Payloads]] |
| `says <Message> with <Data> to <Target>.` | Speak with payload to target | [[Actors#Message Payloads]] |
| `when <Target> hears <Message>?` | Listen on channel | [[Actors#Messaging Syntax — Channels]] |
| `when <Target> hears <Message> from <Source>?` | Listen, filtered by speaker | [[Actors#Messaging Syntax — Channels]] |
| `write <Item> to <Feed>.` | Write to feed | [[Iteration#Feeds]] |
| `write <Message> with <Data> to <Feed>.` | Feed write with payload | [[Iteration#Feeds]] |
| `put <Item> into <Queue>.` | Add to queue **partial** | [[Iteration#Queues]] |
| `<Queue> has next?` | Queue availability check **partial** | [[Iteration#Queues]] |
| `write <Item> to <Feed>.` | Write to feed **partial** | [[Iteration#Feeds]] |
| `when <Feed> reads <Message>?` | Feed listener | [[Iteration#Feeds]] |

---
## Locking {#locking}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `lock <Set>...` | Open exclusive lock | [[Locking]] |

---
## Modules {#modules}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `the <Name> is a script.` | Script declaration | [[Modules#Scripts]] |
| `the <Name> is an act.` | Act declaration | [[Modules#Acts]] |
| `the <Name> is a scene.` | Scene declaration | [[Modules#Scenes]] |
| `the <Name> is an actor.` | Actor declaration | [[Actors#Declaration]] |
| `use <Module>.` | Lazy/selective import — only referenced names enter the namespace | [[Modules#Imports]] |
| `<Module>'s <Name>` | Qualified access (always works) | [[Modules#Imports]] |

---
## Frame Naming {#frame-naming}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `... as <Name>` | Name without focus change | [[Frames#Naming Without Focus: `as`]] |
| `the <Name> is <Expression>.` | Name with focus | [[Frames#Naming With Focus: `the`]] |

---
## Operators {#operators}
---

| Operator | Meaning | Defined in |
|----------|---------|------------|
| `+` | Text concatenation / numeric addition | [[Data Model#Operators]] |

`and` and `or` exist only inside `when` conditions — see [[Core Concepts#Condition Chaining]]. Marco has no boolean operators in expression position.

---
## Comments And Formatting {#comments-and-formatting}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `// ...` | Line comment | [[Core Concepts#Comments]] |
| Indentation | Phrase structure | [[Core Concepts#Formatting And Punctuation]] |
| `,` | Run-on chaining | [[Core Concepts#Run-On `when`]], [[Execution Model#Execution Order]] |

---
## Naming Convention {#naming-convention}
---

| Case | Meaning | Defined in |
|------|---------|------------|
| `lowercase` | Built-in actions, keywords, statuses | [[Core Concepts#Naming Convention]] |
| `Capitalized` | User-defined names; may override lowercase built-ins | [[Core Concepts#Naming Convention]] |

---
## Observability {#observability}
---

| Form | Meaning | Defined in |
|------|---------|------------|
| `log <Value>.` | Built-in logging — record for trace / monitoring | [[Observability#Logging — `log`]] |
| `inspect <Value>.` | Developer-facing introspection — for REPL / dev tools | [[Observability#Inspect — `inspect`]] |
| `show <Component>.` | UI rendering — **not debug** | [[Modules#Show — UI Rendering]] |
| `show <Component> with <Data>.` | UI render with data | [[Modules#Show — UI Rendering]] |
| `what is <Frame>?` | Debug — inspect Frame | [[Observability#Debugging — `what`]] |
| `what's <Frame>?` | Debug — inspect Frame's result | [[Observability#Debugging — `what`]] |
| `what happened?` | Debug — recent Frame activity / callstack | [[Observability#Debugging — `what`]] |
| `what was previous that?` | Debug — inspect parent Frame | [[Observability#Debugging — `what`]] |
| `what can that do?` | Debug — list capabilities from contract | [[Observability#Debugging — `what`]] |
| `callstack`, `previous`, `root` | Trace access **preliminary** | [[Observability#Tracing]] |
| `when that is failed? log that's error.` | Observation pattern | [[Observability#Observation]] |

---
## Reference Modes {#reference-modes-summary}
---

| Mode | Anchors | Defined in |
|------|---------|------------|
| Declaration | `it` = current declared subject; `the` updates `this` | [[Reference Modes#Declaration Mode]] |
| Process | `this` = Frame being resolved; `that` = child / observed Frame | [[Reference Modes#Process Mode]] |

---
## Built-In Vocabulary {#built-in-vocabulary}
---

The complete v1 built-in word list. New built-ins require explicit spec updates.

### Types
`actor`, `act`, `action`, `scene`, `script`, `set`, `list`, `map`, `queue`, `feed`, `channel`, `contract`, `template`, `status`, `error`, `translator`

### Primitive Types
`text`, `number`, `boolean`, `time`, `id`, `any`

### Frame Slots
`input`, `status`, `result`, `error`

### Frame Statuses
`created`, `running`, `waiting`, `ok`, `failed`, `canceled`, `exited`, `died`

### Execution
`do`, `start`, `execute`, `wait`, `lock`

### Control / Branching
`when`, `or when`, `or`, `or?`, `finally`, `skip`, `stop`

### Messaging
`says`, `hears`

### Feeds
`write`, `reads`

### Queues
`put`, `has next`

### Data / Safety
`has`, `is`, `is not`, `exists`, `copy`, `shared`, `partial`, `optional`

### Naming / Structure
`the`, `it`, `this`, `that`, `its`, `that's`, `input`, `as`, `with`, `use`, `uses`, `export`, `allows`, `gives`, `takes`

### Debug / Observability
`log`, `show`, `inspect`, `what`

### Traversal
`previous`, `root`, `last`, `callstack`, `at`

### Iteration
`for each`, `while`, `item`, `key`, `first`, `last`, `previous`, `next`

### Built-In Contracts
`failable`, `waitable`, `cancelable`, `lockable`, `retryable`, `iterable`, `replayable`

### Core Operators
`+`

### One-Line Lock

This is the v1 built-in vocabulary; new built-ins require explicit spec updates. Per [[Core Concepts#Naming Convention]], all v1 built-ins are lowercase; user-defined names use Capitalized form to override.
