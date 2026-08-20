---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - navigation
  - passive-observation
  - perception
---

# A navigation observer records what the player meant, and cannot record what they typed

## Context

Screen-state segmentation could already say that the composition changed and came back:
`state_1 → state_2 ×4`. It could not say what made it change. Correlating those changes with the
player's own navigation is what turns a list of screens into a graph with **edges**, and a graph
with edges is the first thing a capability could be learned from rather than merely reported.

Getting there means observing the keyboard, which makes this the highest-privacy surface in the
system. A global low-level keyboard hook sees every keystroke on the machine — passwords, chat,
private messages — and it sees them in a background service the user is not looking at.

Two further constraints are not negotiable. The hook callback shares the install thread with the
overlay's, and Windows silently drops a low-level hook that overruns `LowLevelHooksTimeout`,
taking the F12 stop key with it (see `CLAUDE.md`). And [[ADR-010-passive-observation-cannot-execute]]
must not be weakened: an observation path is exactly what that decision exists to protect, but
only for as long as nothing on it can act.

The naive design — record key codes now, decide what they mean later — is wrong in a way that
cannot be corrected after the fact. Retained key codes are keylogging material whatever the
intention, and "we only ever look at the arrow keys" is a promise about a future reader.

## Decision

**Physical input is classified into a closed vocabulary of navigation MEANINGS inside the
platform adapter, and the physical representation is destroyed there.** The only value that
crosses out is `observe.InputEvent`: a `NavIntent` from a closed set, a session-relative
millisecond offset, and a normalised pointer position that only a pointer intent may carry.

The guarantee is structural, not procedural:

- `rawEvent` is unexported, has no `String` method, is never logged, and appears in no type that
  leaves the package. Leaking a key code would require exporting a new type — a visible act.
- The classifier has **no branch that can admit a character key**. It cannot be configured into
  a keylogger because it has nowhere to put the character. Text-entry learning is deliberately a
  different privilege on a different channel; see "The text-entry seam" below.
- The vocabulary of ignore reasons crosses the boundary too, so it is defined by the CONSUMER
  (`observe.IgnoreReason`). A producer cannot invent a diagnostic string, because a free-form
  "reason" is the obvious way a key name would eventually travel.
- `NavIntent`'s underlying type is a string, so admission — not typing — is what closes the door:
  an unrecognised intent is dropped rather than carried.

### The hook callback does one thing

Bounds-check, read the struct, non-blocking offer into a bounded queue, chain. No lock, no
allocation, no map, no classification, no logging, no Director call. Classification, repeat
suppression, session gating and timestamps all happen on a worker goroutine.

When the queue is full the offer **fails and the event is dropped**, and the drop is counted.
Blocking is not an option: a stalled hook is a hook Windows removes. Losing a navigation event
is strictly better, and a loss nobody can see is worse than either — a correlation thinned by
backpressure reads exactly like a player who pressed nothing.

The hook **never suppresses**. Returning 1 would eat the user's keystroke, and a passive
observer that changed what the game received would not be passive. Injected events are ignored,
so Marco cannot correlate its own input with the changes it caused and call it discovery.

### Admission is conservative, and refusals are counted

Arrows, Enter, Backspace and Escape are admitted. WASD and Space are **refused** and counted as
`ambiguous_gameplay_key`: the same key navigates a menu and drives a car, and this layer cannot
tell which. Admitting them would attribute screen changes to navigation the player never made,
in precisely the sessions where they were driving around — which is most of the session.

Escape maps to `pause`, not `back`: `NavPause` is defined as the intent that opens **or closes**
an application's own menu, which is what Escape does. Calling it `back` claims more than the key
supports.

The right fix for the ambiguous keys is not a per-game key table, which would put game knowledge
in the platform adapter. It is state-conditional admission — the same key may be navigation when
the screen is structurally menu-like and movement when it is not — which needs screen state on
this side of the boundary. Refusing is the conservative answer until then.

#### Amended 2026-08-09: admission is now state-conditional

The precondition arrived, and the cost of refusing was measured: a live session of a WASD-driven
game classified **7 intents from 1086 physical events**, with 117 refused as ambiguous, and one
of twenty-one screen changes carried any attribution at all
([[Experiment-008-unknown-game-discovery]]).

W/A/S/D and Space are now admitted as `up/down/left/right/confirm` **while, and only while, the
last observation showed a set of choices on screen**. Everything above still holds when there is
no such context, which remains the default: with no screen evidence the behaviour is exactly
what it was.

Four properties make this safe enough to do, and each is held by a test:

- **The predicate reads raw detections only** — `observe.MenuLike`, over one inference's regions,
  never over tracks, states, groups or hypotheses. Admission decides what input evidence exists
  and the tracker consumes admitted input; if the predicate read anything the tracker had
  concluded, the two would define each other.
- **It does not measure spacing.** Rocket League's menu column scores uniformity 0.97 and every
  recurring group in Schedule I scores 0.00–0.01. A predicate keyed on even spacing would admit
  navigation in one game and refuse it in the other — a Rocket League rule wearing the costume of
  a screen-shape rule. The signal used instead is how many controls carry a role whose name may
  be said, which is layout-independent.
- **Context is read on the worker, never in the hook callback**, and freshness is judged against
  the event's own timestamp. An assessment newer than the press cannot justify it, and one older
  than `ScreenContextTTL` cannot either. The real bound is flip-off — the composition root sets
  the context on every valid inference, including the ones that are not menu-like — and the TTL
  is only a backstop against inferences stopping altogether. A skipped inference leaves the
  assessment standing, because unknown is not false ([[ADR-006-unknown-is-not-false]]).
- **Every such intent is marked, and is weaker evidence.** `InputEvent.Conditional` says the
  reading rested on a judgement about the screen. An edge whose every attributed observation is
  context-admitted is **contested** and can never be `supported`; a partially context-admitted
  edge states the caveat in its support. One unambiguous key among them carries the observation
  on its own.

This required widening the type allowlist in `TestRawKeyIdentityCannotCrossTheBoundary` by one
entry, `bool`, deliberately and with the reasoning recorded in the test. The privacy boundary is
otherwise unchanged: the flag says nothing about which key was pressed — `up` from an arrow and
`up` from W are indistinguishable downstream — and no character key can reach the classifier
under any context.

### One press is one intent

A low-level hook reports a held key as a stream of key-downs with no repeat flag. Held state is
tracked so leaning on a direction for 800ms produces one intent rather than forty, and cannot
drown out a single deliberate confirm in the correlation.

### Retention is session-bounded

Navigation observed while no session is watching is discarded at classification, not buffered.
Opening a session clears held state; a second Open retires the first; ending a session detaches
the subscription however it ends. A Director that accumulated intents whenever it happened to be
running would be keeping a record of the user's keyboard for no session's benefit.

### One timebase

Input timestamps come from the session's own clock, not from `time.Now()`. Two independent bases
would attribute a keypress to whichever transition was nearest in a drifting frame of reference
— and the drift is invisible in production, where both are the wall clock, so only an injected
clock could ever reveal it.

### Correlation, never cause

An edge records which intents were **observed before** the change, how often each was, and how
often the change had no navigation before it at all. `Dominant()` reports its support alongside
the intent so one observation and four out of four cannot render identically, and the competing
intents survive beside it. Unattributed changes are reported rather than omitted: a transition
that usually has no input before it is evidence AGAINST reading the correlated ones as causes.

Order is retained separately from the intent set, bounded, because `down, down, confirm` and
`confirm, down, down` produce identical unordered evidence and describe different interactions.
The information is not recoverable later.

### The text-entry seam

Passive discovery never retains character identity, and nothing above can be configured to.
Learning a command like `invite user <name>` needs the typed value, which is a different
privilege with a different lifecycle: an explicit Teach/Text mode emitting `text_entry_started`
/ `parameter_candidate` / `text_entry_committed`, entered by the user on purpose and visible
while it runs. Keeping it on its own channel is what lets passive observation stay structurally
incapable of reconstructing typed content — a mode flag on THIS producer would mean the
capability existed and was merely switched off.

## Consequences

The discovery graph has attributed edges for the first time, and the substrate the hypothesis
milestone needs — from, to, observed count, per-intent support, dominant with support,
unattributed count, ordered runs — is complete and bounded.

Nothing here reaches World State, fusion, planning, policy or execution. Input rides the
**shadow** sample, beside evidence rather than in it, so it cannot become something Marco
believes its way into acting on.

Controller and pointer input are **not** implemented, and that is a scoping decision rather than
an oversight: there is no XInput or GameInput infrastructure in the repository to reuse, and
`director.exe` installs no mouse hook. `NavPoint` exists in the vocabulary with admission rules
enforced and no producer behind it. Keyboard alone proves the loop.

## Enforced by

- `TestRawKeyIdentityCannotCrossTheBoundary` — the shape of what escapes, by field name AND by
  type, so `RawKey int` fails even though an int is not text.
- `TestNoPhysicalKeyIdentitySurvivesIntoASessionResult` / `...IntoACapturedTrace` — the same rule
  swept over the whole terminal record and the durable on-disk artifact, not one type.
- `TestTheKeyIdentityWalkerCatchesAPlantedViolation` — the sweep is proven to catch a violation
  planted three types deep, so a green result means something.
- `TestCharacterKeysProduceNoIntent` — typing produces nothing, and is counted as unsupported.
- `TestGameplayAmbiguousKeysAreRefusedAndCounted` — the refusal is visible, not silent.
- `TestAHeldKeyProducesExactlyOneIntent` / `TestAHeldKeyCountsOncePerTransition` — repeat
  suppression at the producer and at the correlation.
- `TestAStalledConsumerCannotBlockTheHook` — a stalled worker cannot block the hook thread, and
  the shed events are counted.
- `TestTheProducerSurvivesRepeatedLifecyclesAndLateCallbacks` — 25 start/stop rounds with the
  callback racing shutdown, no panic, no send on a closed channel, no leaked worker.
- `TestInputBeforeASessionDoesNotLeakIntoIt` / `TestASecondSessionDoesNotSeeTheFirstsInput` /
  `TestOpeningASecondSessionRetiresTheFirst` — session-bounded retention.
- `TestTheCompositionRootSubscribesTheSessionToNavigation` — THE wiring test. Deleting
  `navSource.Open` from `Runtime.newObservationSampler` fails it, and used to fail nothing.
- `TestAnEdgeWithCompetingIntentsKeepsAllOfThem` / `TestRepeatedTransitionsAccumulateTheirSupport`
  — support arithmetic, including the unattributed counter-example.
- `TestNavigationWithNoScreenChangeInventsNoEdge` / `TestAnOldKeypressIsNotForcedOntoALaterEdge`
  — no fabricated edges, no nearest-neighbour attribution.
- `TestTheOrderOfNavigationSurvivesOnTheEdge` — order is not flattened into a bundle.
- `TestACapturedTraceReplaysToTheSameAttributedGraph` — production and replay agree on
  attribution, not only on geometry.
- `TestAmbiguousKeysBecomeNavigationWhileChoicesAreOnScreen` / `TestWithNoContextAmbiguousKeysAreStillRefused` — the relaxation, and the unchanged default.
- `TestUnambiguousKeysAreNeverConditional` — context never weakens an arrow key.
- `TestAmbiguousKeysAreRefusedOnceTheScreenIsNoLongerChoices` — flip-off.
- `TestAStaleScreenContextStopsAdmitting` / `TestAnAssessmentMadeAfterThePressCannotJustifyIt` / `TestFreshnessIsJudgedAgainstTheEventNotTheClock` — the two freshness bounds.
- `TestContextDoesNotAdmitCharacterKeys` — the relaxation did not widen the classifier's reach.
- `TestTheAdmissionPredicateSeparatesChoicesFromPlayInBothRecordedGames` / `TestTheAdmissionPredicateIgnoresArrangement` — one rule, two games, no spacing.
- `TestAdmissionContextDoesNotDependOnTracking` — the non-circularity guard.
- `TestTheProductionSamplerPushesTheAdmissionContext` — THE wiring test; deleting the `SetScreenContext` call fails it. `TestASparseScreenDoesNotLicenseAmbiguousKeys`, `TestTheProductionSamplerFlipsAdmissionOffWhenChoicesLeaveTheScreen` and `TestASkippedInferenceLeavesTheAssessmentStanding` cover the other three production behaviours.
- `TestAnActionRestingOnlyOnContextAdmittedKeysIsContested` / `TestPartialContextAdmittedSupportIsStatedNotContested` / `TestOneUnambiguousKeyMakesAnObservationUnconditional` — weaker evidence stays weaker.
- `TestConditionalEvidenceSurvivesTheTraceAndReplaysIdentically` — production and replay agree on the new class of evidence.
