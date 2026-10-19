---
type: experiment
status: complete
date: 2026-10-19
backend:
  - structure
fixture: "Source review of the supervised-run lifecycle as committed in `cec983d`, `7ae9dd1` and `22fd169` — all three with a green 101-package suite — followed by one reproducing test per traced condition and a mutation run on every new call site."
result: "Twelve conditions closed over four rounds. Every one was reproducible by a test that could have been written against the committed code, and not one of the committed tests described it. The first round: five timing (the moment a stop is read, the moment a target is measured, the moment a deadline applies) and two identity (which attempt a record belongs to, what a whole run's one-word outcome is). The second round, reviewing the first round's fixes, found three more — and all three were the same fix not having gone far enough: a fresh look that reached the observer and was never asked what the observer made of it, three exits past the citation that the first round had not enumerated, and a reservation that guarded the record while the pixels were already being shared. The third round found the second round's own fix one layer down: the boundary look took whatever OCR had FINISHED, which is a reading of the frame before it, admissible while the epoch is still and agreeing with the screen the plan was made on. The fourth found the edge of THAT fix: a request that completed having read nothing still satisfied the requirement, because the stabiliser counts a silent reading as a look it read. Forty-three mutations across four rounds; three survived the first pass and each named a real gap, including an ORDERING that two source-pinned strings could not hold. Two limitations traced and deliberately left open."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - cmd/director/performworkflow.go
  - cmd/director/consentdesk.go
  - cmd/director/runrecord.go
  - internal/director/workflowrun/verify.go
---

# Experiment-093 — seven conditions a green suite did not describe

## What was being tested

Not a feature. The supervised-run lifecycle had already shipped with tests, and the question was
whether those tests described the *conditions under which it has to hold* rather than the happy
path through each of its parts.

A review sweep answered no, seven times. This record is about the shape of the seven, because
the shape repeats and is worth recognising next time.

## The shape

**Five of the seven are about a moment, not a mechanism.** Each mechanism worked. The stop flag
was set and read. The target check ran and refused when the control had moved. The deadline
existed and expired. What none of the tests pinned down was *where in the sequence* the read
happened:

| Mechanism | Tested | Not tested |
|---|---|---|
| Stop key | sets a flag; the run ends | arrives between the last check and `exec.Do` |
| Stop key | ends a run with a runner | arrives before the runner is constructed |
| Consent desk | drains stale keys before asking | a stop is one of the keys it drains |
| Target check | refuses a control that moved | the look it measures against is seconds old |
| Deadline | expires a window with no evidence | expires a window with *some* evidence |

The fifth row is the one worth dwelling on. `OutcomeHeld` answering true means *the words are
there* — which feels like a reason to keep waiting, and is exactly the case where waiting is
least justified, because the whole question is whether those words STAY and readings have
stopped arriving. A plausible reason to wait is how a missing deadline check survives review.

**Two of the seven are about identity.** A second is not unique, so two attempts shared an id
and `os.WriteFile` made one of them the other. And a run's one-word outcome was being read off
its best action: `rep.VerifiedOK > 0` returned `verified` for a workflow that verified step one
and refused step two. Both are the same error in different clothes — a part standing in for a
whole.

## What was done

One reproducing test per condition, written to fail against the committed code first. Then the
fix. Then a mutation run on every new call site, as the mutation-gate discipline requires.

## The second round, which is the part worth keeping

The three conditions found by reviewing the first round's fixes are all the same shape: **the
fix did the visible half**.

`freshLook` took a real capture. That was the visible half, and it is the half a reviewer
reading the diff sees working. The other half is that the stabiliser deliberately keeps the
standing screen still while a change settles — so a look of a changed interface was taken,
offered, folded, and answered for with the SCREEN BEFORE IT, for as long as a person takes to
press F7. The fix had added a capture to a check that reads a cached claim, and the capture did
not change what the check read.

Nine is the same thing: the first round fixed the exits it had *listed*. A list is the visible
half of "every exit finalises". The guard now walks the function.

Ten likewise: `O_EXCL` on the record is the visible half of "a reference retrieves its own
attempt". The reference also names a frames directory, and the refusal arrived after a run's
worth of images had already been overwritten.

**The mutation gate is what found this in our own work.** Three of the first pass's mutations
survived, and each one named a real gap rather than a missing assertion:

| Mutation | Why it survived |
|---|---|
| Read the standing stamps *before* taking the look | Two source-pinned strings pin two statements and say nothing about which came first |
| `holdsPoint` with the lower-Y bound removed | The only off-region case also missed on X, and the X check gets there first |
| `saveRun` with `O_CREATE` restored | The runs directory did not exist in that test, so the write failed for the wrong reason |

The first is the one worth remembering: the answer was not a third pinned string but a change of
shape. `freshEvidence` now carries the standing claim, read in one statement after the
observation, and `stillThere` no longer holds the observer — so there is no longer an order for a
caller to get wrong. The second and third were vacuous tests of exactly the kind
[[Experiment-089-four-terms-passed-and-nobody-pressed-the-button]] catalogues: an assertion that
passes for a reason other than the one it names.

Twenty-five mutations across both rounds, all killed at the end.

## The third round, which is the same lesson one layer down

The second round's fix asked the stabiliser what it made of the boundary look, and gave that look
text the way the loop gives text to an ordinary look: offer the capture, drain whatever has
finished. Reviewing it found that this is **the first defect again, in the evidence rather than
in the claim**.

OCR answers about twice a second against a loop that looks ten times, so `ready()` returns a
reading of an earlier frame. The admission rule that exists to refuse stale readings cannot
refuse this one: it refuses on a moved epoch, and the epoch is exactly what does NOT move while
the stabiliser damps a change. So the boundary look of a changed panel carried the words of the
panel before it, those words agreed with the standing screen, and the one change that geometry
cannot see was confirmed by the thing it replaced.

The shape is worth naming because it is now three for three:

> a guard that asks for fresh evidence and accepts whatever evidence is lying about.

The capture was fresh. The counters were read in the right order. The text was stale, and text
was the only term that could have decided the case.

The fix is a different REQUEST rather than a different rule: this frame, by id, waited for inside
a budget, cancellable, refused when it does not arrive — and asked only where words could decide
anything, because a screen nobody has read cannot be contradicted by a reading and demanding one
would refuse presses for evidence the stabiliser never had.

Fourteen mutations on the third round, all killed. Two of the fourteen are killed only by the
source pins and are reported as such: `readNow` answers on a channel of its own, so no test in
the package can hand the boundary a foreign reading and watch the id refuse it. The id is there
for the day somebody routes this through the shared queue again.

**And one mutation survived twice before the fixture was honest enough to kill it.** `readNow`
answers on a buffered channel, because a bridge call cannot be interrupted once it is written:
the reading of an abandoned boundary finishes with nobody waiting for it, and on an unbuffered
channel it would park a goroutine for the rest of the run. Removing the buffer survived — because
the gated engine in the test honoured the cancelled context and returned an error, so the
abandoned reading never existed. The real engine cannot do that: `bridgehost.Invoke` writes a
request line and reads the answer, and no context reaches it. The fixture is now deaf in the same
place the bridge is deaf, and the mutation dies. A fixture more cooperative than the thing it
stands for cannot produce the condition it was built to describe.

## The fourth round, which is the third round's own edge

The third round's term asked two questions: did a reading come back for this capture, and did the
stabiliser count a reading on this look. Both can be yes while nothing has been established.
`VisualScreens.Read` increments `TextRead` for **every** reading it is handed, including one whose
source answered and found no words — and that is right for its own accounting, because "read and
silent" is a fact about a screen and "nobody answered" is not. The boundary was reading that
counter as the words having arrived. So OCR completing successfully with nothing on it cleared the
press, and a reading of one or two words did the same.

Four rounds, and the same pattern each time. This one sharpens the sentence:

> a request completing is not evidence arriving.

The fix asks for enough of a reading to be a claim, at the bar the text rule already applies
(`observe.TextDisagreementWords`), because below it `TextsDisagree` will not call either side a
claim — so a reading that thin could not have refused the merge however different what it said
was. Accepting it would have been accepting a measurement nothing was ever going to consult. The
stabiliser is still handed the reading either way: it is entitled to everything that was read.

Four mutations, four killed, and the bar is pinned in both directions — one word lower fails the
thin case, one word higher fails the positive control, which clears on exactly three.

## What this does NOT establish

The one-action live demonstration from [[Experiment-092-the-first-human-taught-pixel-macro]]
stands as historical evidence of the acceptance sequence. It is **not** coverage of any of these
seven conditions: none of them were exercised by it, and two of them (the stale target, the
late stop) are specifically about what a person doing something slightly differently would have
hit.

Two limitations were traced in the same sweep and left open on purpose, both larger than a
lifecycle fix: the control centre's Stop does not reach a standalone play, and a step on a
different screen can arrive while a previous step's verification is still pending. Named in
[[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] so they are not
rediscovered.

## Links

- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]
- [[Experiment-092-the-first-human-taught-pixel-macro]]
