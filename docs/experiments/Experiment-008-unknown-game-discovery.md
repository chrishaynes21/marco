---
type: experiment
status: complete
date: 2026-08-09
backend:
  - screenparser
game: unknown-to-marco (Schedule I)
fixture: live capture, 3m, privacy-safe trace
result: generalisation-confirmed-navigation-limited
supersedes: []
source_paths:
  - internal/director/observe/hypothesis.go
  - internal/director/observe/screenstate.go
  - internal/platform/navsource/navsource.go
  - cmd/director/observewiring.go
---

# Experiment 008 — a game Marco had never been taught

## Question

Every previous measurement in this project used Rocket League, and the tuning that came out of
them was therefore free to fit one game's interface. The discovery stack now claims to be
game-agnostic. Does it produce anything on an application it has never seen, with no
configuration, and does it refuse to over-claim when the evidence is thin?

## Method

The user played **Schedule I** normally for three minutes. Nothing about it was configured: the
target was discovered at run time from `director windows` (foreground window, application
`schedule i`), and the application name reaches only the session's selector — the hypothesis
generator cannot see it ([[ADR-014-hypotheses-are-evidence-not-identity]]).

ScreenParser in shadow at frozen settings (1280px, conf 0.15, NMS 0.45), 2s cadence, trace
captured via `$MARCO_SHADOW_TRACE`. **OCR was unavailable** (no tesseract on the machine) and
accessibility reported the application `unobservable (1 elements, nothing operable)` — so this
measures the **structure + navigation** path with the text leg absent.

## Result — discovery generalises

65 slots, 52 valid inferences, 13 skipped, 0 failures, 0 unproven, one window generation, 0
regions outside `[0,1]`. 215 detections: 134 button, 74 icon, 4 group, 3 progress_bar.

**Ten screen states** were discovered and **48 tracks**, of which 16 stable and **26 stable
within their own screen state**. The state-relative correction of
[[ADR-012-presence-is-state-relative]] reproduced on a second game without retuning:

| track | global | state-local |
|---|---|---|
| shadow_1 (button) | 43/52, `bursty` | **18/18 (100%) `persistent` in state_3** |
| shadow_2 (icon) | 14/52, `bursty` | **7/7 (100%) `persistent` in state_2** |
| shadow_9 (button) | 11/51, `bursty` | **7/7 (100%) `persistent` in state_2** |

**Production and replay agreed exactly** — same ten states, same inference and episode counts,
same transitions, same navigation attribution, and the ordered run `pause → pause ×1` survived
the round trip. This is the parity property confirmed on live data rather than on a fixture.

## The hypotheses it formed

One `supported`, four `contested`, and it never named a screen:

```
possibly a menu-like screen   [supported]
  a recurring screen of 6 grouped controls, seen in 3 visit(s) over 7 observation(s)
  structure    dominated by 6 controls presented as a set
  recurrence   recurred 3 separate times

possibly a set of choices presented together   [contested]
  6 controls recurred together at the lower left across 3 visit(s)
  AGAINST      the spacing is irregular (uniformity 0.01)
```

That pair is the result worth keeping: the same screen is *supported* as menu-like and its
control group is *contested* on arrangement, simultaneously and correctly. Nothing claimed to be
settings, because there was no text — which is the ceiling working, not a gap.

## What was actually wrong

**Navigation admission is the limiting factor, and it is measurable.** The producer saw 1086
physical events in three minutes and classified **7 intents**:

| outcome | count |
|---|---|
| classified | 7 |
| `repeat` (held keys) | 765 |
| `ambiguous_gameplay_key` (WASD/Space) | 117 |
| `unsupported_navigation` | 37 |
| dropped to backpressure | **0** |

Exactly **one** of the 21 observed edges carried any attribution (`state_2→state_3, after pause
in 1/2`); every other edge reads `no input in N`. The conservative admission policy is behaving
exactly as [[ADR-013-navigation-is-meaning-not-keys]] specifies — and on a WASD-driven game that
policy refuses nearly everything the player did. The fix that ADR already names,
**state-conditional admission**, is now unblocked because screen states exist and are proven
live.

**The uniformity heuristic does not generalise.** Rocket League's evenly stacked menu column
scored 0.97; every recurring group here scores **0.00–0.01**, so `possible_choice_group` is
contested on spacing for essentially every real group in this game. The measure describes a
vertical list, and this application does not lay out that way. Reported rather than retuned: one
game is not a basis for replacing a rule fitted to another.

**Cadence is tighter than it was.** Median inference 1218ms (against 850ms in
[[Experiment-007-state-relative-tracking]]) and a median recorded gap of 3807ms against a 2s
interval; 13 of 65 slots skipped and 21 samples late.

**Segmentation looks loose.** Ten states from three minutes, six of them with a single inference,
and `state_unknown` involved in seven of the twenty-one edges. Not diagnosed here.

## Privacy

Held completely. 1086 physical events produced no key identity anywhere: the trace schema has no
field for one, and the report prints only counts and closed-vocabulary intents. No OCR text was
available and none would have been retained — only matched interface concepts, of which there
were none.

## Conclusion

The stated bar was *"Marco discovered a recurring interface/state and formed a useful semantic
hypothesis in a game it had never been taught"*, and it cleared it: one supported menu-like
screen, correctly hedged, with four contested claims beside it and no application-specific code
anywhere in the path.

The product goal — *"want me to learn how to get here?"* — is **not** reachable on this evidence,
and the reason is now measured rather than guessed: Marco can see the screen and cannot see how
the player reached it.

## Enforced by

The generalisation property is held by `TestHypothesesDoNotDependOnApplicationIdentity`; the
ceilings by `TestGeometryAloneNeverNamesAScreen`, `TestTextAloneNeverNamesAScreen` and
`TestNavigationAloneNeverNamesAScreen`. This session is evidence, not a gate.
