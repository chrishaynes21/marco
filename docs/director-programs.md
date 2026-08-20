---
type: milestone
status: historical
---

# Sequential semantic execution

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> A program is not a batch of actions. It is a sequence of independently observed,
> independently compiled, independently verified semantic operations.

## Why not a batch

A batch is submitted once and comes back once. Nothing between the first action and the
last can notice that the world moved, that a dialog appeared, or that the second target
no longer exists.

That matters immediately. In `open File then click Save`, **Save does not exist** when
the request arrives — it is inside a menu that is closed. A design that planned and
resolved everything up front would resolve Save against a world with no Save in it, and
either fail at planning time or click a stale coordinate.

So the Director regains control after every step:

```
Observe -> Resolve -> Compile one legal Marco program -> parser -> compiler
        -> runtime -> Observe -> Verify -> Continue
```

Live, against Notepad:

```
[1/2] open File - verified        1/2 resolve  menu_item "File" (0.97)
[2/2] click Save - verified       2/2 resolve  menu_item "Save" (0.97)
COMPLETED: 2 of 2 steps verified
```

## The model

A `Step` carries an unresolved **Intent**, not a plan and not a resolved target. That is
the load-bearing choice: resolution happens against the world as it is when the step
runs, so a step can never carry a handle from before the previous step changed the
screen.

`FailurePolicy` has exactly one value — `stop`. Retry belongs to the single-step layer,
which already knows when repeating is safe (an unchanged screen) and when it is not (an
edit that may have landed). Continue-on-failure would run step 3 against a world step 2
failed to produce. Rollback would need an inverse for every operation, and there is no
inverse for "press Enter".

## The single-step path is unchanged

A request that decomposes into ONE clause goes down the existing path, byte for byte.
The check is on the **split**, not on the parse — program validation rejects verbs the
ordinary parser handles gracefully, and "open the file menu" should still get its
helpful clarification rather than "unsupported operation".

`handleParsed` is the shared core: a program step gets exactly the retry guard, the
settle-by-condition, the Marco lowering and the verification a single request gets,
because it is the same function. A separate sequence executor with its own simplified
copy is how two paths drift until one of them is quietly less safe.

## Decomposition splits, it never infers

Split on ` and then `, ` then `, `, and `, ` and `, `; `, `, ` — longest first, so a
single conjunction does not become two empty clauses. No reordering, no inserted focus
step, no deciding that "save the file" means Ctrl+S. Every step corresponds to a clause
the user actually said, in the order they said it, which is what makes `director plan`
an honest preview.

Two rules earn their keep:

- **Never split inside a quoted run.** `type "save and exit" into the box` is one
  instruction whose text contains a conjunction. Splitting would type half of it and
  try to execute the other half.
- **A clause that does not start with an instruction word rejoins the one before it.**
  `type hello and goodbye` is one instruction; "goodbye" is data, not a command.

`instructionStarters` also lists verbs the Director does **not** implement — `scroll`,
`drag`, `close`, `delete`, `rename`. That is deliberate: a clause beginning with one is
a real instruction, so it becomes its own step and validation rejects the whole request,
rather than quietly rejoining into a click on a control labelled "Save and scroll down".

## Whole-request validation

All of it, before any of it. A partially validated plan that starts executing and then
discovers step 4 is unsupported has already done steps 1-3, and there is no undo.

| rejected | why |
|---|---|
| control flow (`if`, `when`, `until`, `unless`, `for each`, `while`) | executing a gated action as an ungated one is the most dangerous thing this layer could do |
| unsupported operation | a verb with no implementation would fail after earlier steps had changed the screen |
| unfilled placeholder | `{{target}}` never reaches a desktop |
| more than 10 steps | **rejected, never truncated** — running a prefix does something the user did not ask for |
| any failure policy but `stop` | there is only one |

Conditionals are checked against the **whole request before splitting**, because
"if it is open, click Save and close it" would otherwise split into clauses that each
look unconditional.

## Verification

Program success is the conjunction of verified steps. `FAILED`, `UNSAFE`, `AMBIGUOUS`
and `UNOBSERVABLE` stop the program immediately; later steps do not run.

`UNVERIFIED` stops too — **unless** the planner marked the step best-effort.

### Best-effort is a closed list, not a judgement

Some operations make no claim the World Model can check. A selection is not in the World
State. A copy changes the clipboard, not the screen. Enter usually leaves a trace — a
newline, a cleared box, the control going away — but a search submitted without changing
the field leaves none, and that is indistinguishable from an Enter that went nowhere.

`press_enter`, `select_all` and `copy_selection` are therefore `VerifyBestEffort`, set
**in the planner**. The execution loop trusts that list completely and makes no judgement
of its own, so a step cannot talk its way past verification by being hard to check.

This was found live: `type Marco and press enter` — validation #1 of this milestone —
stopped at step 2 because `press_enter` had nothing to verify against.

## Cancellation

Checked before every step. **Never mid-program**: a Marco program that has started runs
to its end, because abandoning it halfway leaves the desktop in a state nobody planned —
a key held down, half a string typed. Cancellation prevents the *next* one.

```
[1/3] open File - verified
      1/3 settle waited region_stable: cancelled after 1 observation(s) in 446ms
CANCELLED: stopped before step 2 of 3 (click Save)
```

## Program context

Three fields: the last resolved target, the last window, the last action node. Enough
for `it`, `that field`, `that window`, `the same control` — a real and narrow need
("clear it, then type Director").

A back-reference rewrites the **query**, not the target: the step still looks the element
up in a fresh world by label. Inheriting the resolved handle would be exactly the stale-
handle bug this design exists to prevent.

General conversational memory is a different feature with different failure modes — it
lets a request three turns ago silently decide where today's text goes — and this is
deliberately not it.

## Action graph

Every completed step produces an ActionNode, chained to the previous step's, so a program
reads as a chain rather than as unrelated events. The Program itself is **not** a node —
it is an orchestration object.

## Busy

One mutating program at a time. Status, history and cancel stay answerable throughout.

```
BUSY
current program: "open File then click Save then click Cancel"
step 1 of 3
say "stop" to cancel it, or wait for it to finish
```

The step position is the part that matters: "already running X" leaves the user
wondering whether it has hung.

## Diagnostics

```
director plan "<request>"        the steps a request becomes - never runs them
director plan "<request>" --json
```

```
Goal: focus the search box, clear it, type Director and press enter

[1/4] focus "search box"
        marco         Accessibility's Focus
        verification  required
        on failure    stop
...
Executable: yes (4 steps, each independently verified)
```

An edit's capability is reported as *"chosen at execution from the control's
capabilities"* rather than guessed: the strategy ladder decides against a real control,
and printing a guess would invent an answer the Director does not have yet.

## Known gaps

- **Mid-program clarification pauses but cannot yet be answered.** The pause is correct
  and complete — `pausedProgram` retains the program, the step index and the context, and
  `resumeProgram` continues from that step without re-running completed ones (covered by
  `TestResumingFromAStepDoesNotRerunCompletedOnes`). But the clarification question is
  not reaching the client, so the answer is parsed as a fresh request.
- **`focus <control>` fails against Notepad's editor**, and does so as a single step too:
  Notepad does not report the editor as focused after a UIA SetFocus. Pre-existing, not
  caused by this milestone.
