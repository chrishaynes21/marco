---
type: experiment
status: complete
date: 2026-10-13
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1"
result: five-of-eight-bound-correctly-two-bound-confidently-wrong-and-no-threshold-separates-them
supersedes: []
source_paths:
  - cmd/perceptionreplay/binding.go
  - internal/director/observe/arrangement.go
  - internal/director/fastobserve/fastobserve.go
---

# Experiment 029 — binding a screen across two independent folds

**Status: RUN offline, 2026-10-13, on the existing recording. No new capture, nothing executed,
nothing stored.**

## The question

[[ADR-183-remembering-recognising-and-being-allowed-to-act]] proposes a durable workflow that
carries each screen's **arrangement** so a later session can work out where it is without
inheriting another session's counters. The whole design rests on an assumption nobody had tested.
The owner named it as the prerequisite:

> Take the existing recording, split it into independent folds, and determine whether screen
> arrangements learned from one segment can be recognized in another without sharing session-local
> IDs. Importantly, don't allow the two folds to share the registry or screen identities.

## Method

`perceptionreplay -frames -bind`. Three completely separate `fastobserve.Observer`s over the same
pixels:

- **fold A** sees looks 1–450 and nothing else;
- **fold B** sees looks 451–900 and nothing else;
- **the oracle** sees all 900, and its only job is to say which screens are really the same.

No counter, registry, signature or binding passes between A and B. Their screen names collide by
coincidence and mean nothing to each other — which the results show directly: fold B's `visual_4`
is the oracle's `visual_6`, and fold B's `visual_7` is the oracle's `visual_1`.

The proposals are computed once and shared, so a difference in binding cannot be a difference in
segmentation. The relation is **production's own** — `ArrangementSimilarity >= VisualScreenMatch`
(0.55) — so this measures what the screen model already believes rather than a rule invented for
the experiment.

### What the ground truth is, and is not

The oracle is the same stabiliser run with continuity. It says two folds are looking at what one
uninterrupted observer would have called one screen. It is not an independent labelling, and none
exists without a person. The owner's caveat is on the report itself rather than under it:

> splitting a single recording creates independent folds, not genuinely independent application
> sessions. It can validate the binding mechanism, but not prove robustness across restarts, UI
> changes, or display configurations.

Same resolution, same window, same build, same theme. What it does test is whether the
representation survives a different observer arriving at a different moment with a different
history.

## Result

```
fold A saw 8 screens, with 8 distinct oracle names
fold B saw 8 screens, with 6 distinct oracle names
```

**Fold B split one screen into three.** The oracle's `visual_1` arrives in fold B as three separate
identities — 177, 51 and 22 looks — at mean proposal yields of 258, 269 and 283. Fold A saw it as
one. So a screen's identity is not stable across observers even inside one recording, and the
yields at which it fragmented are close enough together that proposal count is not an obvious
explanation.

```
A          oracle       result       best    2nd   bound to
visual_1   visual_1     ambiguous   0.787  0.653   visual_1   two above the floor
visual_6   visual_6     bound       0.688  0.515   visual_4   CORRECT
visual_3   visual_3     bound       0.886  0.467   visual_3   WRONG — B never saw this screen
visual_2   visual_2     bound       0.901  0.407   visual_2   CORRECT
visual_7   visual_7     bound       0.556  0.494   visual_5   CORRECT
visual_4   visual_4     bound       0.611  0.423   visual_3   CORRECT
visual_5   visual_5     bound       0.559  0.416   visual_6   WRONG — B never saw this screen
visual_8   visual_8     bound       0.833  0.450   visual_6   CORRECT

7 bound: 5 CORRECT, 2 WRONG (both to a screen fold B never saw at all,
  where the only right answer was to refuse)
1 ambiguous
0 unobservable
```

### No threshold separates right from wrong

```
correct bindings scored   0.556  0.611  0.688  0.833  0.901
wrong   bindings scored   0.559  0.886
```

The best **wrong** binding scored 0.886 — above four of the five correct ones. A floor that
excluded it would throw those four away too. **The score does not carry the distinction, and
raising the threshold cannot make it carry it.**

### The model has no way to say "not here"

Both wrong bindings are the same failure: fold A knew a screen that fold B never saw, and the only
correct answer was to refuse. `VisualScreens.promote` takes the best match above the floor and
never reports that it was choosing. That is defensible for a session-local stabiliser, where the
alternative is minting a screen that already exists. It is not defensible for a durable binding,
where the alternative is performing a saved workflow somewhere else.

The one ambiguity found is the same gap seen from the other side: fold A's `visual_1` had two
candidates above the floor, and production would have silently taken the better.

## What this means for ADR-183

The prerequisite **fails as the design assumed it**. An arrangement — a count of rectangles in
nine boxes — binds the right screen more often than not, and it binds the wrong one confidently
enough that no score separates the two. Carrying arrangements in a durable workflow and binding by
similarity would produce a system that is usually in the right place and occasionally certain it is
somewhere it has never been.

Three things follow, and none of them is "raise the threshold":

1. **The Binding layer needs an abstention that the screen model does not currently have.** "No
   counterpart" has to be an available answer, scored on something other than being the best of
   what is lying around. The experiment's own `ambiguous` result is a sketch of it and is not
   enough on its own: the two wrong bindings here were not ambiguous, they were unopposed.
2. **An arrangement is not sufficient evidence for a durable claim.** It was built as a
   session-local stabiliser cue, is described in its own file as "a far weaker claim", and this is
   what that weakness costs when it is asked to do more.
3. **ADR-183's separation survives and is reinforced.** Persistence does not imply recognition is
   exactly the right boundary, because this measurement says recognition is the part that is not
   ready. A workflow could be remembered today and still refuse to say where it is.

The durable Workflow and Binding stay unimplemented, which is what the owner asked for. The
evidence now says which half of the design needs work first, and it is not the half that stores
things.

## Enforced by

- `cmd/perceptionreplay` — `TestTheBindingFoldsShareNothing` holds the one property the whole
  experiment depends on: that the folds are independent.
- `internal/director/fastobserve` — `Arrangement` forwards to `observe.VisualScreens.Arrangement`,
  so the experiment reads the stored arrangement rather than recomputing one.
