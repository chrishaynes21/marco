---
type: adr
status: accepted
date: 2026-10-21
decision: "ACCEPTED — Studio writes down what a trial of it showed, as the trial happens, and a diagnostic reads it back. One journal per launch under `<store>/trials`, created only when there is something to record, opening with which build is writing it and which `director.exe` it would spawn. Four facts are collected by the flow itself and none of them is a thing the owner is asked to watch for: WHICH BUILDS TOOK PART (the window on every note, the recorder from its own stream, the director that performed a run from the record it wrote — three claims, because any two of those binaries can differ and an unstamped one is neither an identity nor evidence of agreement); THE STATES A RECORDING PASSED THROUGH, taken from the value handed to the page so that what is written is what the person was shown, stamped, and written only when it changes — with the preparation countdown excluded because it ticks by construction; HOW OFTEN THE PAGE HAD TO ASK FOR EACH PICTURE, which is the whole of the blinking defect expressed as a number, tallied as a snapshot and flushed at every seam a reader cares about; and THE CANONICAL RESULT, cited by the run's own reference and read out of the run record rather than from anything the window concluded. `studio -trial <store>` prints each journal and ends on those four as four judgements, each a function returning its sentence so a test asserts the judgement and not the layout. A failure to write the journal is never a failure of the flow it is about, and reading a store leaves nothing in it."
supersedes: []
affects:
  - interface
  - execution
source_paths:
  - plugins/studio/trial.go
  - plugins/studio/trialcheck.go
  - plugins/studio/learn.go
  - plugins/studio/acts.go
  - plugins/studio/previews.go
---

# A trial collects its own evidence

## The verdict this comes from

Every defect this surface has had was found the same way. The owner used Studio, said what they
saw, and engineering worked backwards from the sentence:

> Settling is ass, it doesn't finish in time. How fast were you seeing screens settle, because
> for me it's taking like 20-30s if at all.

> Also the images blink in the ui.

> the pictures are gone

Each was true. Each took a round trip to turn into a measurement — the first one took reading the
owner's own `learnstate` stream off their machine to find that the press had reached its verdict
in 2.7 seconds and the window had gone on asking them to wait for 29.5 more
([[Experiment-095-thirty-seconds-of-settling-that-had-already-settled]]). And one of them, a
window and a recorder built a day apart, was visible only by comparing file modification times.

The Architect's instruction after that last one:

> Git identifies the source; it doesn't identify the running executable. Keep the
> protocol-version check, and make the build revision available in Studio's diagnostics and run
> evidence. **The owner should never need to compare executable timestamps.**

and then, setting up the next trial:

> Next is one normal Studio flow: learn a Scene → review its pictures → save an Act → Play.
> Engineering should collect build identity, recording states, preview persistence, and the
> canonical result automatically — **not turn them into another owner checklist.**

That is the decision. The four facts are named by the instruction; what follows is where each one
is taken from, and why it is taken from there rather than somewhere more convenient.

## What is written, and from where

**Which builds took part** — three fields, not one, because "the build" is not one thing. The
window's own revision is on every note. The recorder's is whatever its stream reported, which is
how a current window reading a day-old recorder is visible at all. The director that performed a
run comes out of the record that run wrote. An UNSTAMPED build is reported as a part of the trial
that did not say what it was, never folded in as a revision two unknowns share — the same rule
`buildid.Same` and `buildid.Differ` already hold between them, and the reason neither is the
negation of the other.

**The states a recording passed through** — taken from the value `Learning` hands to the page,
not from a second reader of the recorder's stream. The claim the evidence has to support is about
what the OWNER WAS SHOWN while they were demonstrating; "settling is ass" is a sentence about
that value, and the gap between what the stream said and what the window showed was the defect
itself. Written only when it changes, because the page polls four times every two seconds for as
long as it is open, and the preparation countdown is excluded for the same reason
`learnstate.Say` excludes its own `Left`: a sentence that differs every second by construction is
not a state that moved.

**How often the page had to ask for each picture** — counted where the picture is served. The
blinking the owner reported was 66 requests in 4.2 seconds with nothing on screen changing, and
108 with a run in flight; the page now caches by revision and label and keeps its elements, so
the expected count per label is one. Three is the threshold, which leaves room for the two honest
ways it can be more: a demonstration recorded again is a new revision, and a reopened window
starts with an empty cache. The tally is a snapshot written at most every couple of seconds while
pictures are arriving — a line per fetch would turn the defect into a megabyte a minute — and
flushed at every seam a reader cares about, including the window closing, so a Studio ended by
hand still leaves the evidence.

**The canonical result** — the run's own reference, out of the line the runner printed, and
everything else out of the record that reference names. Not from this window's bookkeeping: the
record is the canonical account and a second verdict kept here would be a second answer to "what
happened", which is the rule `history.go` already follows. A run that cited no reference, or
wrote no record, is written down as exactly that.

## What the diagnostic says

`studio -trial <store>` prints each journal — the builds, the recording with its states and the
seconds between them, the picture tally, each Act saved or refused, each run and its record — and
ends every trial on the four facts in the order they were asked for:

```
  WHAT THIS TRIAL SHOWS
    build identity      one build throughout: 34f6122 (the recorder, the window)
    recording states    reading → ready → checking → captured in 9.4s — 1 press(es) checked
    preview persistence held — 3 label(s), none asked for more than 1 time(s)
    canonical result    run-20261021-161203-7f2a — verified, 1 action(s) sent, 1 verified
```

Each of the four is a function that returns its sentence rather than printing one, so a test
asserts the judgement. The judgements are what somebody acts on — "two builds took part" and
"this picture was fetched fourteen times" have cost the owner a day each — and a test that could
only check that a line was printed would pass on either verdict.

It opens nothing, spawns nothing and changes nothing, which is the promise `-acts` makes; the
journal is lazy for the same reason, so pointing a diagnostic at somebody's store cannot leave a
file in it.

## What this deliberately is not

**Not a log.** Nothing here is written to be skimmed as it arrives. The window already tells a
person what is happening, in better words. This is evidence of one trial, in the order the window
learned it, and its only reader is the diagnostic.

**Not a second account of a run.** See above: the record is canonical.

**Not something the owner reads.** It is the instruction's own point. A journal the owner had to
open, interpret and quote back would be the checklist with extra steps.

**Not a new Studio interface.** No page, no panel, no button. A trial is collected by using the
surface that already exists.

## Enforced by

- `plugins/studio/trial_test.go` — every test enters through the verb the page calls, so the
  WIRING is what is under test and not the writer:
  `TestARecordingWritesDownTheStatesTheWindowShowed` (the states in order, through the page's own
  poll, with repeated unchanged polls writing nothing and the stamps increasing);
  `TestTheTrialSaysWhatCameOfTheRecording` (read from the store after the child exited, with the
  pictures it kept); `TestThePicturesThePageAskedForAreCountedAndJudged` (one each is held; the
  same one asked for eight times is NOT HELD — and the throttled tally before the flush is the
  reason the flush exists); `TestTheTrialSaysWhetherAnActWasSavedOrRefused` (both, because the
  refusal is the half a trial turns into a conversation without);
  `TestTheTrialCitesTheRunsOwnRecord` (the reference the runner printed, and the outcome, counts
  and performing build read out of the record it names);
  `TestTheTrialSaysWhenTwoBuildsTookPart` (a genuine mismatch: the window's revision substituted
  and the recorder's written as a recorder would, because a test binary carries no stamp);
  `TestAnUnstampedBuildIsNotEvidenceOfAgreement` (three unknowns are NOT STAMPED, not one build
  throughout); `TestTheVerdictTellsAnEmptyResultFromOneStillComing` (the three endings a
  demonstration has, and that the explanation travels with the empty one);
  `TestReadingAStoreLeavesNoTrialInIt` (`-acts` and every read-only verb leave no journal, and a
  store with no trials is not a fault); `TestTheTrialNoteShapeMovesWithItsVersion` (the shape
  pinned to the version, and a line from a shape this build does not know counted rather than
  guessed at).
- Fourteen mutations run against these: each of the eight call sites deleted in turn, the
  change-detection inverted, the countdown made a change, the record left unread, the version
  check dropped, an unknown admitted as a revision, and the tally throttle removed. All fourteen
  were killed by the named test.

## Related

[[ADR-245-one-task-at-a-time]] is the surface this collects evidence about, and carries the
build-identity change this extends from one pair to a whole trial.
[[ADR-244-pressing-play-is-the-authorisation]] is the contract a run is performed under.
