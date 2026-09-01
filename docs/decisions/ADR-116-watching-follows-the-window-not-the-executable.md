---
type: decision
status: accepted
date: 2026-08-30
affects:
  - passive-observation
  - perception
source_paths:
  - internal/director/perception/windowref/selector.go
  - cmd/director/observeambient.go
---

# ADR-116 — watching follows the window in front, not the executable

## Context

A person turned Watch & Learn on and walked `Home → Bluetooth & devices → Mouse` **three times**.
Marco noticed nothing, for twenty minutes, and said nothing at all.

The Director's own session report had the answer:

```
State: target_unavailable
Target: application applicationframehost
Samples: 0   skipped: 39
Provenance: no target-scoped provider contributed evidence

applicationframehost has more than one window ... select one by --window-id (ambiguous)
```

`currentWindowSelector` built `Selector{Application: winctx.Active()}`. Windows hosts Settings,
XBOX and Realtek Audio Console inside one `applicationframehost` — and this desktop had **Settings
and Realtek Audio Console open at once, on one PID**. So the selector was ambiguous, `one()`
refused it, and every reading was skipped.

**And nothing reported a failure.** Starting the session succeeded, so `ambientObserveNow` returned
no error; the skipping happened afterwards, inside a session nobody read the state of. Twenty
seconds later the supervisor did the whole thing again. Perception was fine the entire time — a
session pointed at the same window by TITLE read it immediately, 8 samples, 31 controls, pointer
resolving to named controls.

## Decision

### The foreground is a primary selector

`Selector.Foreground` names **the window in front, whichever it is**, resolved once through the
existing `windowref.Foreground` and pinned by the caller exactly like every other selector. It does
not follow focus, and the stale-capture rule is untouched: what changes is only how a starting
window is CHOSEN.

Ambient watching uses it. The supervisor has just asked the desktop what is in front in order to
decide whether to start a session at all; asking the resolver to then find "that application" is a
different question, and it has a different answer whenever one executable owns two windows. The
application is still passed and still decides when the session has been left behind — that check
is unchanged inside `watchOnce`.

### A session boundary is not a crossing

A screen Marco does not recognise is keyed on `session:state`, and the session belongs in the key:
`state_2` in two sessions are unrelated screens, and without it a real crossing would be lost.

The cost was the mirror image. The **same** screen either side of a boundary compared unequal, so
every rollover recorded a step from a screen to itself. Ambient sessions last twenty seconds, so
this fired every twenty seconds forever. Measured on an untouched Chrome window: **sixteen
transitions in five minutes**, one every ten polls of a two-second poller, metronomic.

```
transitions:1  ×7 polls
transitions:2  ×10      one per 20s
transitions:3  ×10      = ambientSession
...
```

They never became knowledge — `drain` clears the pending acts at the same boundary, so the step
carried no action and `noticed` dropped it at `len(s.Did) != 1`. But a count somebody reads is a
count they are entitled to trust, and "16 moves noticed" for an afternoon of stillness is Marco
reporting something that did not happen.

So a boundary is no longer taken at its word: the two readings are compared through
`observe.CompareStructure`, the same identity test the candidate ledger already uses to unify a
wide and a narrow reading of one screen. Never a second opinion about identity. Two DIFFERENT
screens either side of a boundary still cross, which is what
`TestTwoScreensEitherSideOfASessionAreNotOneScreen` has always required and what this must not
break.

### One subtlety, found by a stray line in a test

The first version of this suppressed the crossing and kept the OLD key. That is wrong in a way
that hides itself: the first reading of the new session is suppressed, and the SECOND reading of
that same session then compares against the old session's key, finds it different, and crosses
anyway. The phantom is delayed by one reading rather than removed — which is exactly why it
survived a live measurement (seven of twelve rollovers still crossed) and a test that only read
once per session. The stored key moves forward; only the CONCLUSION is withdrawn.

Measured after the correction: an untouched desktop crossed five session boundaries in 132 seconds
and recorded no transitions at all.

## Consequences

- Ambient watching can observe any window, including one whose executable owns several. The
  `applicationframehost` family — Settings, XBOX, Realtek Audio Console — was entirely
  unobservable whenever more than one of them was open.
- The transition count now reflects screens somebody actually crossed.
- **Not fixed here, and worth knowing:** `drain` still discards pending actions at every session
  boundary, deliberately — an action whose destination never resolved cannot honestly be placed.
  With twenty-second sessions that can still cost a click whose page had not settled in time.
- The failure was invisible because a session that starts and then skips every reading is not an
  error anywhere. Nothing in this ADR fixes that; it is the next thing to look at.

## Enforced by

- `windowref` `TestTheWindowInFrontCanBeSelected` — two windows of one executable, by name and by
  foreground
- `windowref` `TestTheForegroundSelectorIsOneOfTheChoices`
- `cmd/director` `TestWatchingTheWindowInFrontIsNotAmbiguous`
- `cmd/director` `TestASessionBoundaryIsNotACrossing`

## Related

- [[ADR-115-one-experiment-and-the-desktop-given-back]]
- [[ADR-114-watching-and-learning-may-keep-the-name-of-what-you-clicked]]
- [[ADR-071-a-window-is-not-a-place]]
- [[Experiment-022-the-first-dogfood]]

## Addendum — watching forbade everything, including its own proposal

Two further blockers, reported the moment the selector fix let watching actually work.

### Marco's own attention is not somebody else's session

```
BLOCKED: an application is under passive observation by session observe_3. Director actions
are refused while it is being watched ... Stop it first: director cancel-observation observe_3
```

`refuseWhileObserved` and `watchingElsewhere` both treat any active session as a reason to refuse.
The reasoning is right for a session somebody SET UP — an observe-game, a demonstration — where an
action Marco took would silently corrupt evidence about how a person uses an application.

Ambient watching is not that, and it runs **continuously**. So with Watch & Learn on, every door
that moves the desktop refused — including the one that mode exists to offer: the control centre
proposes *test what I learned* from what ambient noticed, and pressing it was blocked by the
watching that produced the proposal, naming a session the person had never heard of.

`ambientObserver.held` already draws the distinction — "a passive observe-game somebody set up
deliberately is not Marco's to cancel" — and `standAside` already acts on it. The three acting
doors now ask the same question in one place, `standAsideForAction`. Somebody else's session is
still refused, unchanged. Marco's own ENDS, so nothing the Director does is folded into it, and
the supervisor opens a fresh one afterwards — the same shape as an explicit Learn taking the slot,
which is why it is not the pause-and-resume the original note rejects: there is no second half to
stitch.

### A place is not something you can ask for

```
learned place Home
Home                      [ Go there ]
failed — FAILED: I only understand click, focus, move, repeat and text editing so far
```

The phrase box was filled from whatever the newest change described, and a `learned place`
describes a PLACE. `marco do "Home"` matched no play and no goal, fell through to the Director's
read-it-against-the-screen path, and blamed the action vocabulary for a request nobody could have
answered.

Knowing where somewhere is and being able to be ASKED for it are different things, and the second
is a name a person gives. Go there is now offered only for a `goal` event; when there is none, the
gap is named and points at *Name what I just did* — which is the step that closes it.

### And two more, from the same session

**A subject id is not an answer.** The feed fell back to `[unnamed subj_c3e77b6f]`, defended in a
comment as an honest admission of a real outcome. It is honest and it is not an answer — reported
verbatim: *"I have no clue what that is telling me."* The outcome is real and is not hidden now; it
is WORDED, through `observe.PlaceWords`, the one function every other surface names a place with,
whose floor is what the screen is made of rather than a hash. The feed had a second, worse copy of
that function sitting beside it. A diagnostic's honesty and a product's answer are not the same
thing, and this surface is the second one.

**Hidden did not hide.** `hidden` is only `display:none` in the browser's own stylesheet, and
`.bar{display:flex}` beats it — so every `show(id, false)` on a bar was a no-op. The Go there box
and the sentence explaining why Go there was absent rendered together, one above the other. Not
cosmetic: `show()` is how this page says a thing does not apply, and a page that cannot say that
offers actions it has just explained are impossible.

**A name that arrived was never shown.** The Director resolves a place's name at READ time,
deliberately — a Place is established on one pass and NAMED on a later one, so an event carrying
the words it had at the instant of the write would describe a screen that is now perfectly well
named. The page defeated it: it asked from a moving cursor, received only what was new, and cached
the rendered line. So `learned place about back, settings, 96 things on it` stayed on screen after
the naming sweep had already replaced it with a name. The page now re-reads the ring, which is what
makes read-time resolution mean anything; the cursor belongs to a FOLLOWER, and `marco observe
--follow` keeps its own and is untouched.

**And the headline was the oldest of the newest batch.** The Director returns the ring oldest to
newest and the page read index 0 as the headline, concatenating batches. It showed on a burst and
hid on a quiet desktop — invisible exactly when it was easiest to notice.

### Enforced by

- `cmd/director` `TestWatchingStandsAsideForMarcosOwnActions` — both halves: somebody else's
  session still refuses, Marco's own stands aside and actually gives up the substrate
- `cmd/marco` `TestGoThereIsOnlyOfferedForSomethingYouCanAskFor`
- `cmd/director` `TestTheFeedNeverShowsASubjectId`
- `cmd/marco` `TestHiddenActuallyHides`
- `cmd/marco` `TestTheFeedRefreshesNamesRatherThanCachingThem`, `TestTheHeadlineIsTheNewestEvent`
