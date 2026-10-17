---
type: experiment
status: complete
date: 2026-10-17
backend:
  - accessibility
  - structure
  - ocr
  - navsource
fixture: "the owner's live desktop -- the disposable Marco Test page in an isolated Chrome profile, read through the augmented configuration of ADR-211, three separate processes for the three phases, with the production Director path as the only establishment path"
result: one-human-demonstration-became-a-durable-lesson-was-reacquired-from-a-fresh-stage-in-a-new-process-and-was-replayed-by-one-authorised-guarded-click-that-reached-the-destination-screen
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - cmd/director/surfacelearnphases.go
  - cmd/director/surfacelearn.go
---

# Experiment 067 — the whole loop ran on a real desktop

The thing the previous sixty-six experiments were for. One human demonstration became a durable
lesson, was reacquired from a fresh Stage in a new process, and was replayed by one separately
authorised guarded click that reached the destination screen.

Three phases, three processes, at `2acab72` and `580fc24`, under the augmented configuration.

## The demonstration

```
the request ended session observe_1 and watching resumed in observe_2; the
Director will establish the question itself from a reading of its own
the Director established the question in session observe_2 on state_1, at
place subj_29d46b64b789, about "chrome"
  settled: the press is inside e30 and inside none of the other 1 surface(s),
    so it tells them apart
  learned: a region containing the identity at depth 1 for "Open Archive",
    confirmed because the screen changed from subj_29d46b64b789 to
    seen_observe_3:state_1 and the crossing was attributed to one human action

  observed press ............ true
  attributed transition .... true
```

The harness established nothing. The press arrived in the session the Director had opened the
question in, on the screen state the question was about, and settled it by being inside one
candidate surface and outside the other. Nine consecutive offers agreed.

## The lesson, as stored

```json
{ "application": "chrome", "target": "subj_17e847faeb70",
  "relation": "a region containing the identity", "depth": 1,
  "shape": { "surface": {"x": 0.0372, "y": 0.2910, "width": 0.2686, "height": 0.1718},
             "identity": {"x": 0.0506, "y": 0.3168, "width": 0.0635, "height": 0.0258},
             "containment": 1, "kind": "block", "fill": 0.984 },
  "confirmed": { "level": "attributed",
                 "why": "the screen changed from subj_29d46b64b789 to
                         seen_observe_3:state_1 and the crossing was attributed to
                         one human action" },
  "by": "the person at the keyboard" }
```

The target is `{label: "Open Archive", place: subj_29d46b64b789}` by durable subject id. The
geometry is normalised fractions of the window. **There is no absolute coordinate anywhere in the
record**, which is what makes the next section possible rather than a replay of a remembered
pixel.

## The reacquisition

A new process, a new observation session, a fresh Stage:

```
foreground "chrome"; durable place "subj_29d46b64b789"
the person at the keyboard demonstrated on 17 Oct 2026 that the press for "Open
Archive" belongs on a region containing the identity, and this reading agrees
F. it would vouch for e31 and press the surface e30.
```

The element ids are this reading's own. Nothing was looked up by the ids the demonstration saw.

## The replay

One click, separately authorised, from a runtime that was the only one of the three able to reach
the desktop:

```
surface   the person at the keyboard demonstrated on 17 Oct 2026 that the press for
          "Open Archive" belongs on a region containing the identity, and this reading agrees
resolve   text "Open Archive" (0.35)
plan      click "Open Archive" (medium risk)
policy    allowed
execute   left click sent at (324,387)
settle    waited region_stable: satisfied after 2 observation(s) in 783ms
verify    the window title changed from "Marco Test — Start - Google Chrome" to
          "Marco Test — Archive Opened - Google Chrome"
record    action_1 (b13d453dcc64ffa9)
```

## The detail worth keeping

Marco pressed **(324, 387)**. The person had pressed near **(398, 387)**.

Those are different points and both are correct. The human was asked for a press that could tell
two candidate surfaces apart, so they were sent to `{213 297 371 180}` — the part of the card the
words do not cover. Marco, replaying, aimed at the middle of the surface the relation picked out:
`e30`, `{64 297 520 180}`, whose middle is (324, 387).

So the saved fact is not where the person clicked. It is **which surface the person's click
identified**, re-derived against a reading taken a minute later. That is the difference between
learning a coordinate and learning a control, and it is the only reason the replay would survive
the window moving.

Resolution confidence on the word alone was 0.35. The demonstrated surface is what carried it.

## What Marco refused to claim

```
a press was sent ......... true
the screen changed ....... true (54 Props before, 51 after)
intended destination ..... READ IT YOURSELF AND SAY. Nothing here can verify that
                           the right page opened: that is the third claim, it is
                           not implemented, and a screen change is not it.
```

Rich in observations, modest in claims. The title change is reported as a title change, and the
question of whether the right page opened is handed back to a person rather than answered.

## What it took to get here, and what that says

Four boundaries failed in sequence before this run, each one hidden behind the last:

1. A Windows precondition that could never be false, and four silent returns in `offerPresses`
   ([[ADR-212-a-silence-is-reported-as-a-silence]]).
2. A question stamped with the observation session the request itself had cancelled
   ([[ADR-213-a-question-belongs-to-the-session-that-can-answer-it]]).
3. A question outliving the twenty-second ambient session that owned it
   ([[ADR-214-a-learning-interaction-outlives-the-session-watching-it]], from
   [[Experiment-066-the-question-outlives-the-session-that-owns-it]]).
4. A `mode` parameter threaded into the two replay phases and never used, so reacquisition ran
   pixel-only and reported a screen with no durable place.

Every one was invisible to a green suite, and every one was a REPORTING or LIFECYCLE defect rather
than a failure of perception, learning or execution. The press was being captured correctly from
the first live run; nothing could say where it went. Three of the four were found by instrumenting
a refusal rather than by changing a decision.

## The residual friction, measured

The run before this one produced no press at all, and the reason is worth keeping. Ambient
watching backs its attention off while nothing changes, so the gaps between sessions grew 4s, 8s,
16s across one eighty-five-second watch — about 28s of the 85 with no question open, concentrated
at the end. The successful run landed in the first window and never paid it.

So the loop works and the invitation is still fragile: a person told to click once can be told it
at a moment when the answer cannot be received, with no signal that this is so. Driving the
supervisor's attention from whether somebody has been asked a question would hold the gap near a
second. Not done — it is a decision about what watching costs.

## Enforced by

Nothing here is a test; it is the record of a live run. The behaviour it exercises is held by the
tests named in [[ADR-212-a-silence-is-reported-as-a-silence]],
[[ADR-213-a-question-belongs-to-the-session-that-can-answer-it]] and
[[ADR-214-a-learning-interaction-outlives-the-session-watching-it]], and by
`cmd/director` `TestEveryPhaseRunsUnderTheConfigurationItWasGiven`.
