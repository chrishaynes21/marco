---
type: subsystem
status: active
owners:
  - director
depends_on:
  - passive-observation
used_by:
  - passive-observation
updated: 2026-08-09
source_paths:
  - internal/platform/navsource
  - internal/director/observe/input.go
  - internal/director/observe/screenstate.go
  - cmd/director/observewiring.go
  - cmd/director/navcompositionroot_test.go
  - internal/director/observe/menulike.go
  - cmd/director/conditionalwiring_test.go
---

# Navigation

Observing what the player *did*, so the changes on screen have something to be correlated with.
It is what gives the state graph edges. It never acts, and it never learns a key.

## The path

```
physical key  →  bounded queue  →  classify  →  NavIntent  →  subscription  →  ShadowSample.Inputs
  (hook thread)     (256)          (worker)     (closed)      (session)         (correlation)
```

The raw key exists only between the first and third steps. Everything downstream of `classify`
is meaning. See [[ADR-013-navigation-is-meaning-not-keys]] for why the boundary is there rather
than further in, and what makes it structural rather than a promise.

## The vocabulary is closed and has no string in it

`up`, `down`, `left`, `right`, `confirm`, `back`, `pause`, `point`. There is no free-form field
on `InputEvent` — not a key name, not a label, not a note — so the type cannot express "the user
typed p" whatever a future caller wants. The ignore-reason vocabulary is closed for the same
reason and defined by the consumer, because a diagnostic "reason" is the obvious way a key name
would eventually travel.

**What is deliberately not captured: character keys.** Not letters, not digits, not punctuation,
not even as opaque codes. The classifier has no branch that could admit one.

## Admission is conservative, and refusals are counted

| keys | outcome |
|---|---|
| arrows | `up` / `down` / `left` / `right` |
| Enter | `confirm` |
| Backspace | `back` |
| Escape | `pause` — it opens *or closes* a menu, and this layer cannot see which |
| **WASD, Space** | `up`/`down`/`left`/`right`/`confirm` **only while a set of choices is on screen**; refused and counted as `ambiguous_gameplay_key` otherwise |
| everything else | refused, counted as `unsupported_navigation` |

W is "up" in a menu and "drive forwards" in a car, so the key alone cannot settle it — the
answer is CONTEXT, never a per-game key table.

`observe.MenuLike` reads one inference's RAW detections and asks how many carry a role whose
name may be said. Three or more is a set of choices. It deliberately does NOT measure spacing:
Rocket League's menu column scores uniformity 0.97 and every recurring group in Schedule I
scores 0.00–0.01, so a spacing rule would be a Rocket League rule wearing a costume.

Admission is state-conditional and every intent it produces is MARKED — see "Weaker evidence"
below. With no context the behaviour is unchanged: refused, and counted.

## The hook callback does almost nothing

A non-blocking offer into a bounded queue, and return. Windows silently drops a low-level hook
that overruns its timeout and takes the overlay's F12 stop key with it, so the constraint is
honoured structurally rather than measured. When the queue is full the event is **dropped and
counted** — blocking is not an option, and an uncounted loss reads as a player who pressed
nothing.

It never suppresses, and it ignores injected events so Marco cannot correlate its own input with
the changes it caused.

## What an edge is entitled to claim

Each transition carries: how often it was observed, which intents preceded it and how often each
did, how often it had **no** navigation before it, and the ordered runs that preceded it.

- **Correlation, never cause.** `Dominant()` returns its support with the intent, so one
  observation and four-of-four cannot render identically. Competing intents survive beside it.
- **Unattributed changes are reported**, because an edge that usually has no input before it is
  evidence against reading the correlated ones as caused.
- **Order is kept separately from the set.** `down, down, confirm` and `confirm, down, down`
  have identical unordered evidence and describe different interactions, and the difference is
  not recoverable later. Bounded — an edge must not become a transcript.
- **A held key counts once.** Repeat suppression happens at the producer, and the correlation
  counts each distinct intent once per change, so a repeat rate is never evidence strength.
- Navigation that preceded no change produces **no edge**. Evidence is consumed by the inference
  it was observed during; nothing is held in reserve to explain a later change.

## Weaker evidence stays weaker

An intent admitted on context is real evidence and *weaker* evidence: the key also means
movement, and whether it was navigation rests on an assessment of the screen made up to one
observation earlier. That doubt travels with it rather than being resolved silently at the
boundary.

- `InputEvent.Conditional` marks the intent. It says nothing about which key produced it —
  `up` from an arrow and `up` from W are indistinguishable downstream — so the privacy boundary
  is unchanged and only the strength of the claim is qualified.
- `InputStats.Conditional` counts them beside `Classified`.
- `ScreenTransition.ConditionalOnly` counts the attributed observations of an edge whose
  navigation was **entirely** context-admitted. One unambiguous key among them carries the
  observation on its own — the rule is "entirely", not "any", because discounting an edge
  established by Escape merely because a W was also pressed would throw away good evidence to
  punish the doubtful.
- A hypothesis whose every attributed observation is context-admitted is **contested** and can
  never reach `supported`. A partially context-admitted edge states the caveat in its support.

## The freshness bounds

Two, and the second is easy to miss:

- The assessment must not be **newer** than the press. A key pressed during play, immediately
  before a menu appeared for another reason, must not become navigation retroactively.
- It must not be older than `ScreenContextTTL`. That is a backstop against inferences *stopping*
  — a run of skipped slots, a stalled sampler. The real bound is **flip-off**: the composition
  root sets the context on every valid inference, including the ones that are not menu-like, so
  leaving a menu turns admission off at the next observation. The error window is one inference
  gap, measured at a 3807ms median.

A skipped inference leaves the assessment standing. Unknown is not false
([[ADR-006-unknown-is-not-false]]) — the detector sitting a slot out is not evidence the menu
closed, and flipping off there would lose admission every time the cadence gate declined, which
it did for 13 of 65 slots in the live session.

## Read the producer's counters before reading an empty correlation

`ShadowTotals.Input` reports events received, intents classified, refusals by reason, events
dropped to backpressure, and why the producer is unavailable when it is. A correlation with no
attributed edges has two explanations that call for opposite responses — the player pressed
nothing, or nothing was listening — and this is what separates them. The CLI prints it under
`navigation` in both the live report and `director shadow-trace`.

## Not built, deliberately

- **Controller / gamepad.** There is no XInput or GameInput infrastructure in the repository to
  reuse, so this would be new platform code rather than a normalisation of existing plumbing.
  Keyboard alone proves the loop. If added: emit edges, never held state, and keep button masks
  inside the adapter — after normalisation, downstream correlation must not be able to tell which
  device produced an intent.
- **Pointer.** `NavPoint` is in the vocabulary with its admission rules enforced (a position may
  ride only on a pointer intent, and is zeroed on every other) and **no producer behind it**.
  `director.exe` installs no mouse hook, and a click would additionally need window-relative
  normalisation against the validated target so a click outside it cannot become target-relative
  evidence. Not trivial, so not in scope.
- **Text entry.** A separate explicit privilege on its own channel — see the seam described in
  [[ADR-013-navigation-is-meaning-not-keys]] and at the foot of `navsource.go`. A mode flag on
  this producer would mean the capability existed and was merely switched off.

## Related systems

- [[Passive-Observation]] — the session this rides on, and the state graph it attributes
- [[Perception]] — deliberately NOT a provider: input is not evidence about the screen
- [[Game-Packs]] — where naming a state, and therefore an edge, will eventually belong

## Decisions

- [[ADR-013-navigation-is-meaning-not-keys]]
- [[ADR-042-a-click-is-a-place-in-a-window]] — the pointer half, and why movement is not observable
- [[ADR-058-a-demonstrated-target-may-keep-its-name]] — a placed press resolves to the
  control under it at event time, from the actionable-control index each valid inference
  pushes; a confirm resolves to the focused control. Enrichment, never a capture
  prerequisite.
- [[ADR-010-passive-observation-cannot-execute]] — unweakened; this adds an observation path
- [[ADR-012-presence-is-state-relative]] — the states these edges connect
- [[ADR-006-unknown-is-not-false]]

## Validated by

The full list is in [[ADR-013-navigation-is-meaning-not-keys]]. The three that matter most:

- `TestTheCompositionRootSubscribesTheSessionToNavigation` (`cmd/director`) — the wiring.
  Deleting `navSource.Open` from `Runtime.newObservationSampler` fails it, **and used to fail
  nothing at all**: the earlier wiring test built its own subscription, so it proved the producer
  and the correlation while saying nothing about whether the Director ever subscribed. See
  [[Wiring-Tests]].
- `TestRawKeyIdentityCannotCrossTheBoundary` plus the Result- and trace-wide sweeps — the privacy
  boundary, asserted from the shape of the types rather than from intent.
- `TestACapturedTraceReplaysToTheSameAttributedGraph` — production and replay agree on
  attribution, so an offline analysis of a captured session is measuring the tracker rather than
  the recorder.

## Known gaps

- **Confirmed live 2026-08-09** against an application Marco had never been taught. See
  [[Experiment-008-unknown-game-discovery]].
- **The gain from state-conditional admission is not measured.** It cannot be, from any trace
  captured before it existed: admission is decided inside the producer at the moment of the
  press, and a trace records the outcome rather than the keystroke — raw key identity dies in the
  adapter by construction. What the Schedule I trace CAN say is the upper bound: **17 of its 52
  valid inferences (33%) showed a set of choices**, so a third of that session was eligible.
  Whether the intents that would now be admitted actually correlate with anything is the one
  question a new session would answer.
- **Sub-sample ordering is unavailable.** Intents are ordered within a drain by their session
  timestamps, but the observation cadence is ~3.5s; two intents and a screen change inside one
  interval cannot be ordered against each other, only against the change. Reconstructing a long
  procedure will need this and does not have it.
- **The predicate is a count, and a HUD of several buttons would satisfy it.** An action bar of
  four ability buttons reads as a set of choices, and a player walking past it with W held would
  produce context-admitted intents. That is why they are marked and why an edge resting entirely
  on them is contested — the mitigation is the hedge, not a cleverer predicate. A tighter rule
  would need evidence from a third application, and inventing one from two would be the overfit
  this design already refused once.
