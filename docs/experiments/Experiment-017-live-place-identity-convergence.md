---
type: experiment
status: complete
date: 2026-08-28
backend:
  - production-perception
  - semantic-memory
fixture: acceptance-37g.ps1
result: one-settings-page-is-one-place-across-reflow-restart-and-sensor-richness
supersedes: []
source_paths:
  - acceptance-37g.ps1
  - cmd/director/showingcmd.go
  - internal/director/observe/reach.go
  - internal/director/observe/reachdrift_test.go
  - cmd/director/cyclelock_test.go
---

# Experiment 017 — one page, many presentations, one Place

## Question

Three separate phases had left the same measurement outstanding, and none of them could take
it offline:

> Against a REAL semantic store and the REAL production perception path, does one Windows
> Settings page resolve to one durable Place across reflow, restart, and optional sensor
> richness — while a genuinely different page stays a different Place?

[[Experiment-014-identity-variance-across-real-applications]] measured which identity-bearing
dimension varies across visits and produced the layout-role rule. It did not resize anything.
This does.

## Method

An isolated `$MARCO_HOME`, a Director serving it, and no other Marco world touched — the real
store at `%APPDATA%\marco` is hashed file-by-file before the run and again afterwards, and the
comparison is printed as a result rather than assumed.

- **Establishment** is `director learn`, the normal permitted acquisition path.
  `Episode.EstablishPlaces` is set by Learn and by nothing else, so the place the session is
  standing on becomes durable; the session is then cancelled, nothing is demonstrated and no
  play is kept.
- **Resolution** is a passive `observe-game` session — the production `StartObservation`, whose
  `Episode` is the zero value, so it recognises and cannot establish — followed by
  `director showing`, which is `ObserveShowing`: the one "where am I standing" door, answered
  by `observe.PlaceNow` against the real store.
- **Marco emits no desktop input.** Navigation is `ms-settings:` shell activation; resizing is
  `SetWindowPos`. No authority, no actuation lease.
- **Disagreements are explained by the matcher itself.** Any pair that does not resolve alike is
  handed to `identityprobe`, which re-runs `SignatureOfState` and `CompareStructure` — the
  production functions — and reports which dimension disagreed.

## Two defects found before the question could be asked

Both were invisible to a green 85-package suite, and both are recorded in
[[ADR-106-a-place-is-not-how-long-you-looked-at-it]].

**Every observation session stopped after one sample.** The escalation gate is asked from inside
`liveSampler.Sample`, which holds `Runtime.mu` for the whole collect-and-fuse; the gate then took
`Runtime.mu` to guard a timestamp.

```
pre-37E build            9 samples / 12s
HEAD, gate bypassed     14 samples / 12s
HEAD                     1 sample, then silence
```

**A window emptied because it had been watched for longer.** `ReachOfState` divided structures
inside the largest space by every structure ever seen in the state. One session over one page
nobody touched:

```
 14 samples    466 ever-seen    142 present    recognised
 27 samples    817 ever-seen    142 present    recognised
 40 samples   1024 ever-seen     88 present    UNREADABLE
183 samples   1024 ever-seen     88 present    UNREADABLE
```

## Result — reflow

`MOUSE_A = subj_71727a02470f`, established cold at 1500x1000.

| width | outcome | subject | fused elements |
|---|---|---|---|
| 1500 | recognised | `subj_71727a02470f` | 115 |
| 1200 | recognised | `subj_71727a02470f` | — |
| 1000 | recognised | `subj_71727a02470f` | — |
| 900 | recognised | `subj_71727a02470f` | — |
| 850 | recognised | `subj_71727a02470f` | — |
| 800 | not recognised | — | 84 |
| 750 | not recognised | — | 84 |
| 700 | not recognised | — | 84 |
| 600 | not recognised | — | 84 |

Across 1500→850 the durable signature is **identical**, not merely inside tolerance:

```
button 14  combo_box 3  group 14  image 13  link 6  list_item 15  menu 1
menu_item 1  pane 4  slider 2  text 29  text_field 1  unknown 1  window 4
terms [back controls settings]
```

Narrow → wide is the same answer in reverse; there is no one-directional tolerance.

## Result — where it stops, and why

Below about 850px Windows Settings removes its navigation pane. Through the one matcher:

```
mouse wide    button 14  combo_box 3  image 13  list_item 15  slider 2  text_field 1
mouse narrow  button 15  combo_box 3             list_item  3  slider 2
              terms [back controls settings] vs [back controls search settings]
```

Thirteen images and twelve of fifteen list items are the navigation. The sliders and combo boxes
— the Mouse page's own content — survive, so this is a presentation change and not a failure to
read. `Recall` answers `different`; Marco mints nothing.

The same happens to Bluetooth, for the same reasons, which is what makes it a property of the
application's responsive layout rather than a quirk of one page:

```
bluetooth wide vs narrow -> different
  role SET differs: only-in-first=[image=13 list_item=12 text_field=1]
  terms differ: [audio back display settings] vs [audio back display search settings]
```

## Result — different destinations stay different

Every pair, through `CompareStructure`, including narrow against narrow where the two pages look
most alike:

```
mouse wide    vs bluetooth wide    -> different
mouse wide    vs bluetooth narrow  -> different
mouse narrow  vs bluetooth wide    -> different
mouse narrow  vs bluetooth narrow  -> different   (combo_box+slider vs list, and the terms)
```

`SUMMARY: same=0 not-same=6` over the four readings. The store held exactly two subjects at the
end of the run: no duplicate Mouse, no duplicate Bluetooth, no alias.

## Result — restart

Two clean Director restarts against the same store. `subj_71727a02470f` and `subj_8e5637a12b8b`
both resolve exactly as before, so the identity is durable rather than rescued by session state.

## Result — sensor richness

The detector was made to run through ordinary production wiring rather than by bypassing the
policy: on a fresh session nothing is placed yet, `EscalationOf` answers "I do not know", and its
ignorance rule keeps the visual pass. `proven_providers` on the session confirms the detector
proved its target rather than reporting itself unavailable —

```
proven_providers: {"accessibility":[1], "vision":[1]}
```

— which matters, because a detector that refused cheaply and one that ran and found nothing look
identical in a result table. On this machine the bundled `plugins\vision\onnxruntime.dll` is
1.26 and the plugin asks for API 28, so the first attempt reported `vision unavailable` and
would have been recorded as a free pass; the run uses
`tools\onnxruntime\onnxruntime-win-x64-1.28.0`.

**Primary-only remembered, richer current reading: same Place.** The signature is byte-identical
with and without the detector's evidence, even though the fused world grows 115 → 135 elements
and the reading costs 617ms instead of 70ms. The detector's additions do not survive into the
structural vocabulary a Place is made of — which is [[Experiment-016-desktop-perception-corpus]]
arriving from the other side.

**Rich remembered, primary-only current reading: same Place, and one orphan beside it.** A cold
`director learn` with the detector configured left two durable Mouse subjects:

```
subj_6add57f98cf7   … icon 9  image 13 …          established first
subj_71727a02470f   … image 13 …  unknown 1       established second
```

Both later readings — one with the detector contributing, one on a Director with no detector at
all — resolve to `subj_71727a02470f`. The icon-bearing subject is never matched again.

The cause is the escalation gate acting on itself. The pass begins with nothing placed, so the
gate buys the visual pass; the composition carries nine `icon` structures and a Place is
established from it. The reading is now placed and sufficient, the gate declines, the detector
stops, the composition changes, and the segmenter calls that a new screen state — which the open
licence also makes durable.

With no `$MARCO_VISION_MODEL`, which is the default, the detector contributes nothing and one
page is one Place; the main acceptance run's store held exactly two subjects for two pages. See
[[ADR-106-a-place-is-not-how-long-you-looked-at-it]] for why this is recorded rather than fixed.

## What this does not answer

**A rich reading of a page already known to be sufficient.** Production correctly has no reason
to buy one, and manufacturing that reason would be measuring a Marco nobody ships.

**Whether the collapsed-navigation presentation *ought* to be the same Place.** It is the same
page to a person and a different composition to the matcher. Making it match would mean dropping
`image`, `list_item` and `text_field` from identity — the counts
[[Experiment-014-identity-variance-across-real-applications]] measured as what tells Settings
pages apart — so the reflow case would be bought with the false-merge case. Recorded rather than
traded.

**Any application but Windows Settings.** One application, two pages, one desktop.
