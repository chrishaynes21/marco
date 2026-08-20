---
type: guide
status: active
updated: 2026-08-07
source_paths:
  - internal/director/visionbench/groundtruth.go
  - internal/director/visionbench/fixture.go
  - fixtures/vision
---

# Building a vision benchmark corpus

The measuring stick is rebuilt ([[Experiment-004-vision-corpus-v2]]); the evidence is not.
This is what producing it involves, and where a person has to decide something a tool must
not decide for them.

## The lifecycle

```
captured_private → reviewed → sanitized → benchmark_approved
```

Only `benchmark_approved` frames enter the durable corpus. The promotion step is **explicit
and manual by design**: a live observation session produces evidence, and a benchmark fixture
is *curated* evidence. Letting passive observation quietly become benchmark data would mean
the corpus grew without anyone choosing what was in it.

## 1. Capture

Full-resolution frames of one window, in ordered sequences. Not crops — a crop's "fraction of
frame" is not a screen's, and calibrating a normalised threshold on crops is the mistake that
cost [[Experiment-003-classical-cv-tuning]].

Useful sequences, 10–30 frames each:

| sequence | what it tests |
|---|---|
| gameplay, camera still | baseline false-positive rate |
| gameplay, **camera moving** | screen-relative HUD vs world-relative geometry — the one that separates a real detector from a texture tracker |
| menu opening | transition handling |
| menu stable | true structure, densely |
| menu closing | disappearance |
| overlay appears / remains / disappears | temporal recall |
| no interface at all | pure false-positive measurement |

**Play normally.** Do not script the game to produce fixtures — a corpus of synthetic
interactions measures the script.

## 2. Privacy review — the step that cannot be automated

Game frames carry account names, party lists, chat, notifications, Spotify titles, Discord
overlays. A tool can check a manifest for markers; **it cannot look at a picture and decide it
is safe to publish.** That judgement is the user's, and every corpus that has stalled in this
project stalled here.

Review each frame for: account or person names, party or friend lists, chat, server
identifiers, notifications, external application overlays.

## 3. Sanitize, or exclude

Prefer **exclusion**. Masking alters the very geometry being benchmarked, and a blur is worse
than a mask — it invents soft edges that a classical detector will happily find. Where a mask
is justified, use solid neutral fill, and record it:

```json
{ "sanitized": true,
  "operations": [{ "kind": "mask", "reason": "account identifier",
                   "region": [0.02, 0.02, 0.20, 0.05] }] }
```

Record the *reason*, never the private text itself.

## 4. Annotate

Smallest workable mechanism: hand-written JSON with normalised bounds. No annotation tool is
worth building until the corpus proves it needs one.

```json
{ "schema": 1,
  "frame": "rl-freeplay-004", "sequence": "rl-freeplay", "index": 4,
  "interface_present": true,
  "regions": [
    { "kind": "button", "bounds": {"X":0.42,"Y":0.31,"W":0.18,"H":0.05},
      "identity": "resume" }
  ],
  "negative_regions": [
    { "kind": "scene_texture", "bounds": {"X":0.05,"Y":0.60,"W":0.30,"H":0.25} }
  ] }
```

Three rules that matter more than completeness:

- **Annotate partially.** Mark what the benchmark scores; an unmarked box is reported as
  `unmatched` and counted neither for nor against.
- **`identity` ties the same control across frames.** Without it temporal recall cannot be
  measured — the benchmark can see a button in frame 3 and a button in frame 4 and cannot
  tell whether they are the same button.
- **Mark negative regions.** Declared scenery is the single most valuable annotation in a
  game corpus and the one no proxy can infer.

Keep the vocabulary generic. `button`, not `resume button`; `bar`, not `boost meter`. Game
semantics belong to game packs; this corpus measures visual structure.

## 5. Approve

Set `"corpus": "vision-corpus-v2"` in the manifest and a `privacy_review` line naming who
checked and for what. `CorpusVersion.Calibrating()` is false for anything else, and an
unmarked corpus defaults *down* to legacy — v2 has to be claimed deliberately.

## Bounds

Keep it small: **6–10 sequences, ~100–200 frames**, under a soft ceiling of 500 approved
frames before reconsidering tooling. This is a measuring stick, not training data. A corpus
large enough to train on is a corpus large enough to overfit to, and nobody will read it.

## Related

- [[Experiment-004-vision-corpus-v2]], [[Experiment-003-classical-cv-tuning]]
- [[Vision]], [[Roadmap]]
