---
type: decision
status: accepted
date: 2026-10-10
affects:
  - passive-observation
source_paths:
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
  - internal/director/observe/shadow.go
  - cmd/perceptionreplay/frames.go
---

# ADR-158 — the active list is the one that responds

## Context

[[Experiment-026-the-same-menu-seen-from-pixels]] reprocessed 864 real frames of a Trackmania
pause menu and got further than anything before it: every row discovered, a list identified
after 28 looks, and 147 of 148 named entries agreeing with the labels. And the session's final
belief was `ambiguous`.

Not because of noise. **Two real lists qualified.** The pause menu hands a mark between its
rows, and so does the track-information panel beside it — the recording's own sanitisation notes
identify it by name. Both are genuine lists. One of them is simply not the one being used, and
nothing about how it is drawn says so.

Geometry cannot separate them and never will. [[ADR-147-the-interface-in-front-of-a-session-is-a-shadow-belief]]
already settled what to do with that: abstain, because picking would be guessing and a guess
that happens to be right teaches nothing about when to trust the next one. That stands. The
problem is that abstaining was the END of the matter, on a session where the evidence to settle
it had been recorded on the same clock and never consulted.

## Decision

**Among several qualifying lists, the active one is the one whose mark moved while the person
was navigating.**

Each candidate's transfers are split by whether a directional navigation intent was observed
within `NavWindow` observations before the move was **seen**. When more than one list qualifies,
the candidate with the most input-attributed transfers is named — provided it has at least
`MinTransfers` of them and at least `NavMargin` times as many as the next candidate. Otherwise
the belief stays ambiguous, as before.

### The input labels a transfer. It never creates one.

This is the whole of what keeps the rule perception rather than inference from intent, and it is
enforced by construction rather than by care:

- Every counter the rule reads is an increment of a transfer **already measured in the pixels**
  by the exclusivity rule. A list with no visually observed transfer cannot be chosen at any
  amount of input, and an input with no visually observed transfer chooses nothing.
- The tracker is given a **count of intents and nothing else** — no key identity, no device, and
  deliberately **no direction**. Knowing that `down` was pressed *and* that a mark moved down
  would let the input predict the outcome, and an outcome that was predicted is not an outcome
  that was observed. All the rule asks is whether the person was navigating at all.
- Only directional intents count. A `confirm` or a `back` changes which screen is in front
  rather than which row is marked, so crediting them would attribute a list's own transfer to an
  input that could not have caused it.

### Ambiguity stays explicit

Three cases abstain, and they are kept apart because they call for different responses: nothing
was attributable at all (no input source, or nobody pressed anything); the presses never
coincided with a mark moving anywhere; or **two lists both answered**. The last is the one that
matters — it is exactly when picking would be guessing — and `NavMargin` exists to make it the
outcome rather than a coin toss.

With no input evidence anywhere the behaviour is exactly what it was before this existed. A
session whose input half is missing must not have its silence read as "nobody navigated, so pick
either".

### The window is a duration, in one named place

`observe.MaxResponseLatency` is 400ms: how long after a press a screen may still be repainting
it. Beyond that, a mark that moves is moving for its own reasons.

It replaced a window counted in OBSERVATIONS, which looked cadence-independent and was the
opposite — 132ms when frames are 66ms apart and 4.6s when the cycle is 2.3s, against a repaint
that takes the same couple of hundred milliseconds either way. The short end credited 4 of a real
menu's 31 transfers; the long end would credit an unrelated list's moves to the person.

The attributable interval is the gap since the last look OR the latency bound, whichever is
longer, and each covers a case the other cannot. A look's own interval is right when looks are
far apart: at 2.3s the press and the repaint are both inside one look, and the sample's inputs
ARE that interval's inputs because the producer drains per cycle. The bound is right when looks
are close together: at 66ms the response shows up five or six looks later and would otherwise be
attributed to nothing.

It is carried as a decaying debt rather than a timestamp comparison, because the two clocks here
must not be mixed: a look carries an absolute time and an `InputEvent` carries milliseconds from
its producer's own start, and joining those two numbers is a recorded trap. Subtracting one
look's time from another's is safe, and that is all it does. The replay needs no special case
any more.

## Consequences

On the 864-frame corpus, at the recording's own 66ms rate:

| | before | after |
| --- | --- | --- |
| final belief | `ambiguous`, selected −1 | **`provisional`, selected 0** |
| frames naming an entry | 148 | 155 |
| named row matched the label | 147 | **154** |

Scored against model-assisted labels, which are not independently validated.

**The margin is thin and that is worth saying plainly: 6 attributed transfers against 3.** The
recording carries 31 presses in 57 seconds, so there is not much evidence to work with, and a
different session could plausibly leave this ambiguous. The rule resolves this corpus; it is not
established as robust.

It also only matters at a fast cadence. Reprocessed at ~660ms and ~1.3s, only the menu qualifies
at all, so there is no ambiguity to resolve and the rule is inert. That is the right shape — it
acts exactly where it is needed.

### What this opens

A transfer that followed an observed input, in a list that was independently seen to move, is
the raw material for exactly the relationship [[Director]] already models: a screen state, an
observed action, and a visually confirmed change. This decision does not create that
relationship and deliberately does not touch the hypothesis layer. It establishes which list the
navigation was reaching, which is the thing that had to be known first.

## Enforced by

- `TestTheActiveListIsTheOneThatRespondsToNavigation` (`selection`) — two qualifying lists, one
  responding, and the responding one is named.
- `TestTwoListsThatBothRespondStayAmbiguous` — the case that must not be resolved.
- `TestWithNoInputObservedTwoListsStayAmbiguous` — the pre-existing behaviour, preserved.
- `TestNavigationAloneNamesNothing` — **the invariant**: sixty observations of navigation with
  no visual change names nothing, because an input cannot manufacture an outcome.
- `TestAttributionFollowsTheCallersRecencyJudgement` — this package honours the recency
  answer it is given, including for a transfer on a look carrying no press of its own.
- `TestAResponseWithinTheLatencyBoundIsStillAttributed` (`observe`) — the same press and
  the same repaint, sampled at 66ms and at 2.3s, are both attributed. The test that
  replaced a window counted in observations.
- `TestAMoveLongAfterThePressIsNotAttributed` — the other side of the bound, without
  which every move a panel makes on its own schedule belongs to whoever last touched
  the controller.
- `TestObservedNavigationReachesTheBeliefAndNamesTheActiveList` (`observe`) — the wiring,
  through `ShadowTotals.Observe`, which is the call the live cycle makes. The rule can be
  perfect while nothing tells it what the person did; that is the shape of eight recorded cases
  in this repository.
- `TestOnlyDirectionalIntentsAreCreditedWithMovingAMark` — a `confirm` is not a direction.
- `TestWithNothingPressedTheBeliefStaysAmbiguous` — the same regression at the wiring level.
- `TestTheRealMenuCorpusStillReadsAsAMenu` (`cmd/perceptionreplay`) — the measurement above on
  the real recording, skipped when the private frames are absent.
- Mutation-gated 8/8 across the rule and its wiring.

---

## Addendum, 2026-10-11 — the margin, the window, and what the rule actually compares

Three corrections, and the first undoes the result reported above.

### 1. The resolution of this corpus rested on the window being applied twice

The replay kept a 400ms lookback of its own, marking every frame within 400ms of a press as
carrying navigation. Each of those frames then RESET the watch's debt to another 400ms, so the
effective window ran to about 800ms after the press. That was never the rule; it was two copies
of one rule compounding, which is this repository's recorded worst failure mode.

The inputs are now handed to `ShadowTotals` as the `InputEvent`s they are, on the slot they
happened during, and the window is applied once. The replay owns no response window at all.

Applied once, this corpus is **ambiguous**:

| | menu | the other list |
| --- | --- | --- |
| transfers seen | 7 | 31 |
| attributed | 3 | 5 |

So the sentence above — "the rule resolves this corpus; it is not established as robust" — was
too generous. It did not resolve this corpus either.

### 2. Responding is a SHARE, not a tally

The comparison above picked the list with more attributed transfers, and that systematically
rewards whichever list moves most — by coincidence rather than by evidence. With presses spread
through a session, the fraction of any list's moves that happen to land inside a response window
is roughly the fraction of the session the windows cover, whatever that list is doing.

31 presses over 57 seconds and a 400ms window cover about 22% of this recording. So:

	the menu      3 of its 7 moves attributed    43%   — twice coincidence
	the other     5 of its 31 moves attributed   16%   — at or below it

The tally rule picked the second. It is now a cross-multiplied comparison of shares, with
`MinTransfers` kept as a floor so that one move out of one cannot win on 100%, and `NavMargin`
unchanged at a factor of two. The share rule picks the menu, and does so for a reason that holds
at any session length rather than on this recording.

### 3. The window is the interval AND the repaint, not whichever is larger

This ADR says the attributable interval is "the gap since the last look OR the latency bound,
whichever is longer" and gives the right reason. The code only ever set the bound, which at the
recording's own 66ms is invisible — 400ms spans six looks either way — and at a live cadence is
the whole thing.

Both halves are needed and they are CONSECUTIVE rather than alternative. The press happened
somewhere inside the interval the look covers, and the screen then takes up to the repaint time
to show it, so the response may become visible as late as one whole interval plus one repaint
after the window opened. Set to the interval alone it expires at the exact instant the next look
is taken: on a fixture where every transfer was the look after a press, **0 of 26 were
attributed**. With the interval plus the repaint, 26 of 26.

Bounded above by `TrackStaleness`, reusing the existing answer to how old an observation may be
and still describe the same arrangement rather than inventing a second number. Past it, "something
was pressed during the last minute and the mark is now elsewhere" is a coincidence with a long
name.

### And navigation now reaches the window on a slot the detector sat out

`InterfaceLook.Navs` is only read on a look that was OBSERVED. Live, an inference starts about
every two seconds against a session sampling twice a second, so three slots in four bring no look
at all — and a press made during one of those never opened the window, while the fast reads taken
during that same slot are exactly the ones that saw the mark move. `InterfaceWatch.Navigated` is
called on every slot that carried input and was not taken behind another window.

### What is still not attributed, and why

A move is recorded when the selection reader CONFIRMS a new row, and it refuses a row it has seen
once — so a move is confirmed a look or two after the mark arrived. Measured at a 2s cadence:
**seventeen moves retained, one of them attributed.** No defensible window reaches that far, and
widening one until it did would be exactly the kind of metric-chasing this ADR exists to avoid.

The fix is to carry the look at which the change was first SEEN out of the reader, which is a
change to the reader's interface and is not made here. It is the first thing to check in a
supervised live session, because at a live cadence it is the difference between graph evidence
and none.

## Enforced by (addendum)

- `TestTheMostResponsiveListWinsNotTheBusiest` (`selection`) — the share rule, on a fixture
  where the busy list WOULD win on a tally, with that precondition asserted so the test cannot
  pass by the fixture drifting.
- `TestTwoEquallyResponsiveListsStayAmbiguous` — the case that must not be resolved, preserved
  across the change, and asserted on the ABSTENTION REASON so it cannot pass by nothing having
  been seen.
- `TestAPressStaysAttributableAcrossACoarseCadence` (`observe`) — the window, measured on the
  CANDIDATE's attribution, which is where it does its work.
- `TestAPressIsNotAttributableAcrossAnUnboundedGap` — the upper bound, with a control assertion
  so it cannot pass against a window of zero.
- `TestALongListWhoseStyleDriftsIsStillOneList` and
  `TestAChainOfNeighboursCannotDriftIntoADifferentScale` (`selection`) — the grouping repair that
  made any of this reach a real menu, and the veto that stops a chain of neighbours walking from
  a row to a panel.
