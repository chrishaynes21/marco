---
type: adr
status: accepted
date: 2026-10-17
decision: a literal match whose role conflicts with the kind of control the person's own sentence named is a CANDIDATE rather than an answer — resolution asks again for a control that fits, and keeps the literal answer untouched when there is none; nothing about scoring, MinScore, substring matching or ambiguity changes
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - internal/director/target/target.go
  - internal/director/rephrase/rephrase.go
---

# ADR-219 — a match is not an answer

## The defect

Measured on the Settings Bluetooth page by the oracle in
[[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]]:

```
"open bluetooth settings"  →  RESOLVED   button "Bluetooth"   0.485
                               (the on/off switch, not the page)
```

Nothing is wrong with the match. `textScore` scores a label CONTAINED IN the request at 0.5,
"Bluetooth" is contained in "bluetooth settings", the switch is the only usable candidate and
nothing contests it. The resolver did exactly what it was built to do.

What is wrong is that a plausible match ENDED THE SEARCH. Asked again with "settings" set
aside and destinations tried first, the navigation item resolves and the switch is rejected as
the wrong kind of control — so the reading that would have answered correctly existed, and
never ran, because a confident wrong answer reached the person first.

**Marco could mistake a match for an answer.** That is a different defect from missing
semantic intelligence, it sits upstream of it, and an interpreter added in front of it would
have been overruled by it.

## The decision

A literal RESOLVED answer is reconsidered when, and only when, the request named a KIND of
control and the chosen control's role conflicts with it. `Resolver.reconsider`.

```
literally ──► RESOLVED ──► did the request name a kind of control?
                               no  ─────────────────────────────► unchanged
                               yes ─► does the chosen role fit it?
                                        yes ───────────────────► unchanged
                                        no  ─► ask again, and take a RESOLVED,
                                               role-fitting, DIFFERENT control
                                               ──► otherwise unchanged
```

### Why the person's own noun, and nothing else

Because it is evidence rather than preference, which is the line
[[ADR-218-a-request-reworded-in-the-screens-own-words]] draws and this must not cross.

"A switch is a poor answer to open settings" is knowable here only because the person SAID
settings. A rule that preferred navigation items in general would be Marco's taste standing in
for the person's intent, and the *Apps* case this project has been protecting since
Experiment-068 says plainly what that would cost: two controls the person did not distinguish
must stay undistinguished.

The kind-noun table is the one `rephrase` already reads — UI vocabulary, not application
vocabulary, and one list rather than two that could drift. It is exported as
`rephrase.KindNamed` for exactly this.

### What it cannot do, which is why it is safe in front of every resolved request

- **It cannot refuse.** A reconsideration that finds nothing better leaves the literal answer
  exactly as it was, word for word and number for number. The held-out case `"the rotate
  panel"` in Paint proves it: "panel" names a destination, Paint has none, and the answer the
  words gave stands.
- **It cannot raise a new question.** Only a RESOLVED re-asking is accepted. Existing
  ambiguity behaviour is untouched.
- **It cannot choose the same control twice.** A re-asking that lands on the control the words
  already chose returns the literal answer untouched rather than relabelled as reworded —
  which keeps `"the Storage section"` (a LINK, where the person said "section") bit-identical.
- **It cannot accept a replacement that conflicts too.** The replacement must fit the kind that
  was named, or there was no reason to prefer it.

### What was deliberately NOT done

- `MinScore` is unchanged. Raising it would refuse the ordinary extra-word requests the same
  substring clause makes work — `"time and language"` resolves at 0.259 against a floor of
  0.25.
- Substring matching is unchanged. It is doing a great deal of load-bearing work, and what
  changes here is not whether a partial match SCORES but whether that kind of match is allowed
  to end the search.
- Ambiguity is unchanged, including where the person's own noun could have settled it. Paint's
  `Zoom` slider beside its `Zoom` combo box, asked for as `"the zoom slider"`, still asks —
  recorded as a measured gap in the held-out set rather than quietly fixed, because the
  instruction for this work was to preserve ambiguity behaviour.

## What it cost and bought

Over the expanded regression set, with the oracle written down first:

| | Settings | held out |
|---|---|---|
| correct, before | 24 / 33 | 31 / 39 |
| **correct, after** | **27 / 33** | **31 / 39** |
| wrong control | 0 | 0 |
| literal answers reconsidered | 3 | 0 |
| extra-words phrasing | **11/11** | **11/11** |

Three answers changed on Settings, all three the Bluetooth case and all three to the control
the oracle named. Nothing changed on the held-out applications, which is the correct result
there: the defect was real and specific, not everywhere.

## One thing the evaluation had to become

The no-regression rule in `director assess-rewording` was absolute — no literal answer may
change. It is now precise: a literal answer may change only where its role conflicted with the
kind the request named and the replacement fits. Both halves are checked, in the harness,
against the resolver's own `target.RoleFits` rather than a second copy of the rule. Anything
else still fails the run.

## Enforced by

- `internal/director/target` `TestAMatchIsNotAnAnswerWhenItsRoleConflictsWithWhatWasSaid`
  (both directions: "bluetooth settings" reaches the page, "the bluetooth switch" reaches the
  switch), `TestARequestThatNamedNoKindOfControlIsNeverReconsidered`,
  `TestReconsideringLeavesAnAnswerItCannotBetterExactlyAsItWas`.
- `cmd/director` `TestRewordingNeverChoosesTheWrongControl` over the expanded sets.

Mutation: deleting the RESOLVED branch, deleting `KindNamed`'s reading of the request, and
letting a reconsideration re-choose the same control are all killed. One clause is not
independently killable and is reported rather than papered over — see the experiment record.
