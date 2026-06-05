# Engine — MVP Slice

Smallest engine that runs an actual Marco program. Pick a target program, work backwards to the minimum runtime that supports it.

---

## Target program for MVP

A trimmed version of the canonical SaveApp:

```marco
the SaveData is a set.
this's Path is a text.

the Game is an actor.
this's State is a SaveData.

this can Save.
this's Save does...
    when this's State exists?
        this is ok with this's State!
    or?
        this is failed with error "no state"!

the App is a script.
do Game's Save...
    when ok?
        log that's Path.
    or?
        log that's error.
```

This exercises:

- Set declaration with a typed field
- Actor declaration with state
- Action declaration + capability + body with branch group
- Process-mode return + branch + RESOLVE
- `exists?` predicate on a set field
- Status predicate (`when ok?` shorthand)
- `that's Path` and `that's error` (slot vs result)
- Script entry point
- `log` built-in

What it does NOT exercise (deliberately deferred):

- `start` async
- `lock`
- `say`/`hears` actor messaging
- `for each` iteration
- `wait until`
- `with that as a <Type>` casts
- Custom errors / contracts
- Imports
- Capabilities (`retry`, `cancel`, etc.)
- Feeds, queues
- Tracing / `what` queries
- REPL

That's a long deferred list, but the MVP isn't trying to be useful — it's trying to prove the core graph-walker with sentence edges works end-to-end.

---

## Phases

### Phase 1: Lexer

Tokenize:

- words (identifiers, keywords)
- punctuation: `.`, `!`, `?`, `,`, `...`
- string literals: `"..."`, `'...'`, escapes
- numbers, booleans
- whitespace as indentation marker

Output: token stream with line/col info.

### Phase 2: Parser

Per the Marco spec, sentences are line-oriented. Each line is a sentence. Indentation creates phrase nesting.

- One pass: lines → sentence trees
- Sentence trees retain structure: subject + verb + clauses + terminator

### Phase 3: Graph builder (declaration mode)

Walk top-level declaration sentences:

- `the X is a Y.` → create Node(name=X, kind=fromY)
- `this's F is a T.` → add Field to current subject
- `this can C.` → add Capability declaration
- `this's C does...` + body → set Body on capability action node

After this phase: a static graph. Nothing is running yet.

### Phase 4: Phrase compiler (process mode)

For each action body, walk the sentences and emit Edges. Resolve names against scope (compile-time).

Validation pass (per Compiler spec):

- All referenced names resolve.
- Branch groups are exhaustive against the relevant contract (or compiler infers a contract from the body).
- Return validity (must include a status).
- Capability invocations dispatch to declared caps.

Emit a Block per action body.

### Phase 5: Runtime

- Allocate root Frame (origin = the script).
- Walk script body: for each top-level sentence, dispatch edge.
- Single cursor for v1.
- Implement minimal edge dispatch: INVOKE_PHRASE, RESOLVE, BRANCH_OPEN, BRANCH_FALLBACK, RESULT_READ, SLOT_READ, INPUT_BIND, STATE_FAIL_SHORT (the failed-with-error string shorthand).
- Implement `log` as a side-effect built-in.

### Phase 6: Wire it together

CLI: `marco run save.marco` runs the file. Outputs:

```
"/save/path/here"
```

(or the error message if State doesn't exist).

---

## Estimated scope

Rough Go LOC estimate to hit MVP:

- Lexer: ~300 LOC
- Parser: ~600 LOC
- Graph builder: ~400 LOC
- Phrase compiler: ~500 LOC
- Runtime / dispatch: ~600 LOC
- Built-ins (log, sets): ~200 LOC
- Plumbing, errors, tests: ~400 LOC

Total: ~3000 LOC. Reasonable for one developer over a few weeks.

---

## What gets us to "real"

After MVP, here's the priority order to climb:

1. `start` + cursor scheduler (multi-frame execution)
2. `say`/`hears` (actor messaging)
3. `lock`
4. `for each`, `while`, `wait until`
5. Capabilities (`retry`, `cancel`)
6. Imports + namespace
7. Custom error types
8. Contract validation (full obligation propagation)
9. Tracing + `what` queries
10. REPL
11. Acts (FFI to OS / external systems)

Each step opens up real use cases. Step 2 (messaging) makes Marco event-driven, which is when the language design starts paying off. Step 10 (REPL) is when iteration becomes fast.

---

## Test strategy

Golden tests against full Marco programs:

```
testdata/
  save_basic/
    program.marco
    expected.txt
    expected-error.txt   # optional
  save_with_branch/
  ...
```

Each test runs the program, compares stdout to `expected.txt`. Failures dump the trace tree for inspection.

Unit tests for edge dispatch, scope resolution, branch group resolution. Property tests for "every well-typed program produces *some* output without panicking".

Static-error tests are equally important — programs that should fail to compile, with expected diagnostic strings. The Compiler spec demands quality diagnostics; we should test for them.

---

## What I'd build first, in order

1. **Lexer** — tight loop with explicit state machine. Tests on token stream snapshots.
2. **Parser to sentence trees** — no semantics yet. Round-trip tests: parse → re-emit → compare.
3. **Graph builder for declarations only** — no bodies yet. Just `the X is a Y.` and field decls. Verify the graph shape.
4. **Phrase compiler for the simplest body** — `this's Save does... this is ok!`. Tiny.
5. **Runtime for that simplest body** — invoke the action, watch it return.
6. **Add branch groups** — the next leap, maybe the hardest single piece.
7. **Add `that` and `that's` resolution** — needs the runtime to maintain bindings as it walks.
8. **Add `log`** — first real side-effect.
9. **Wire up the CLI** — read file, run script.
10. **Write the SaveApp test** — should pass.

Each step should produce a runnable artifact. Don't go more than a day without something running, even if minimal.

---

## What I'd skip in MVP

- Performance work (no benchmarking yet)
- Concurrent / multi-thread (single thread, in-process)
- Source maps / IDE protocol (just CLI errors)
- Persistence / serialization
- Module loading from disk (single file only for MVP; imports come later)

---

## Next file

`99-open-questions.md` — running list of engine-specific questions that need answers before/during build.
