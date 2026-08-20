---
type: milestone
status: historical
---

# Accessibility targeting — the audit

> **Historical record.** This describes the state of the system when it was written. It is
> kept for the reasoning, not as current truth: where it disagrees with a note in `subsystems/`
> or an ADR in `decisions/`, **they win**. See [[AI-CONTEXT]].

> **RESOLVED.** The defect described below was fixed. `cmd/director/observewiring.go` now sets
> `out.Window` (and `out.Target`) on every sampled request, and the code comment there points back
> at this page. Read this for the *method* — an end-to-end path trace that found a mechanism
> production never invoked — not for the state of the code.

**Finding (at the time): the targeting mechanism already exists, complete and correct, at every
layer. The passive observation sampler simply never uses it.** One unset struct field was the
whole defect, and it had blocked three milestones.

This is the Part 1 audit: the path traced end to end, with no behaviour changed.

---

## The path, layer by layer

```
liveSampler.Sample                      cmd/director/observewiring.go:74
  ├─ rt.pinnedWindow = &req.Window      ← vision + OCR pinned HERE
  └─ collector.Collect(ctx, s.request(req))
        └─ s.request(req)               observewiring.go:124
              returns observation.WithVision(nil)   ← Window never set
                    ↓
Accessibility.Observe(ctx, req)         providers/providers.go:71
  scope := ""
  if req.Window != nil { scope = *req.Window }      ← nil, so ""
        ↓
uiaclient.Provider.Snapshot(ctx, scope) platform/uiaclient/uiaclient.go:154
  if scope != "" { in.Put("Window", …) }            ← omitted
        ↓
plugins/uia Program.cs:132
  if (w.Length == 0) return opts;                   ← "the foreground window"
```

## The eleven audit questions

| # | Question | Answer |
|---|---|---|
| 1 | How is the accessibility window selected? | `observation.Request.Window`, falling back to foreground when nil |
| 2 | Does it accept an explicit window reference? | **Yes** — `Snapshot(ctx, scope directorapi.WindowID)`, plumbed the whole way |
| 3 | Where does foreground selection occur? | `plugins/uia/Program.cs:133`, only when `Window` is absent |
| 4 | Is an HWND passed? | Yes, as the opaque string `"hwnd:<handle>"`; the bridge calls `long.TryParse` and builds an `IntPtr` |
| 5 | Is a process ID available? | Yes on `windowref.Ref.ProcessID`, but it is **not** sent to the bridge |
| 6 | Is window generation available? | Yes on `windowref.Ref.Generation`, but it is **not** sent, and observations do not carry it |
| 7 | Is provider lifetime tied to one window? | **No.** The bridge is a long-lived process; each `Snapshot` is stateless and resolves its own target |
| 8 | Does Chromium hydration need persistent attachment? | Yes — this is why `director serve` exists. The attachment is per **process**, not per window |
| 9 | Can it observe a background window? | **Yes.** `AutomationElement.FromHandle` does not require foreground |
| 10 | Can it detect ownership changes? | **No.** The bridge validates the handle *shape*, never that it still belongs to the expected process |
| 11 | Do ID formats match? | **Yes.** `windowref` mints `"hwnd:661516"`; the bridge parses exactly that |

## Why vision and OCR work and accessibility does not

Two different pinning mechanisms, and the sampler only uses one.

- **Vision and OCR** read `Runtime.activeWindow`, which returns `Runtime.pinnedWindow`
  while a sample is in flight. `liveSampler.Sample` sets that field.
- **Accessibility** reads `observation.Request.Window`. `liveSampler.request` never sets it.

The comment at `observewiring.go:80` says the window is *"pinned for the duration of this
cycle. Without this the collector would look at whatever is in front."* That is true of the
`pinnedWindow` field and false of the request beside it, which is exactly how the gap
survived review: the intent is written down, and only two of the three providers obey it.

## What this predicts, and it matches the evidence

Experiment-002 calibrated against VS Code and got **46 entities, all `role=icon`** — not one
accessibility element. That was read as "a targeted session is vision-only by construction".
The construction is one missing field, not an architectural limit.

Two live VS Code sessions in the previous milestone reported **0 stable entities** across 31
samples, for the same reason.

## The fix, and what it does not cover

Setting `Request.Window` fixes selection. It does **not** deliver the rest of the milestone:

- **Generation is still not carried on observations** (Part 6). Nothing can currently reject
  evidence from a stale generation because nothing knows which generation produced it.
- **Ownership is never re-validated** (Part 4, item 2–3). The bridge trusts the handle. A
  recycled handle would be walked without complaint — the same class of defect
  [[ADR-008-no-stale-window-geometry]] fixed for capture, still open for accessibility.
- **Provider outcomes are collapsed** (Part 7). "Unobservable" and "empty" are both zero
  observations today.
- **Fusion has no context guard** (Part 8).

So the one-line change is necessary and nowhere near sufficient. It is worth making first
because it is what unblocks measurement: until accessibility reaches a targeted session at
all, none of the remaining guards can be evaluated against real evidence.

---

# Part 2 audit — the vision capture path

**Finding: vision's post-capture ownership check exists, but it cannot detect the race this
gate is about, and its result is thrown away as an error rather than surfaced as evidence.**

## Where each thing lives today

| what | where | usable as provenance? |
|---|---|---|
| expected application | `directorapi.Window.Application`, passed to capture | intent only |
| pre-capture ownership | `wincapture.go` — `owner(w.ID)` before the grab | yes, but discarded |
| post-capture ownership | `owner(w.ID)` again after the grab | yes, but discarded |
| `window_changed_during_capture` | raised as an **error** | not a provenance value |
| live bounds re-read | `c.Bounds(w.ID)` before capture | yes |
| frame metadata | `CapturedImage{Bounds, ContentOrigin, Scale, CapturedAt}` | **no target field** |
| expected process | — | **absent from the capture path** |
| expected generation | — | **absent from the capture path** |

## The two problems

**1. The check compares application NAMES, not identity.**

```go
ownerBefore, ownerKnown := c.owner(w.ID)
if ownerKnown && w.Application != "" && !strings.EqualFold(ownerBefore, w.Application) { … }
…
if ownerAfter, known := c.owner(w.ID); known && ownerKnown &&
    !strings.EqualFold(ownerAfter, ownerBefore) { … }
```

It catches a window changing hands to a *different application*. It does **not** catch a
window of the same application being replaced — which is precisely generation 7 → 8 of VS
Code, the race the whole gate exists for. Both comparisons would pass.

So Part 3's premise — "use the post-capture validated target as ObservedTarget" — does not
hold as written. The existing check establishes *an* ownership fact, not the Director's
semantic generation. Vision needs the same `TargetResolver` treatment accessibility got:
re-resolve the captured window through `windowref` after the grab.

**2. The result is an error, not a value.**

Every ownership outcome is `return ocr.CapturedImage{}, fmt.Errorf(…)`. On success,
`CapturedImage` carries no target at all. There is nothing for a provider outcome to inherit
and nothing for OCR to inherit from it (Part 9's whole chain).

`FrameProvenance` (Part 2) is therefore a real addition, not a rename of something existing.

## What this means for sequencing

Accessibility was one missing field because the bridge already reported what it walked.
Vision is not: the capture layer must start *producing* provenance before vision can report
it and before OCR can inherit it. Parts 2, 3 and 9 are one change, not three.

## Related

- [[Perception]] — the pipeline and its known gaps
- [[Windows]] — window identity, and the capture-side guards this path still lacks
- [[Experiment-002-dnfc-observation-baseline]] — the measurement this defect invalidated
- [[ADR-008-no-stale-window-geometry]], [[ADR-009-window-identity-is-ephemeral]]
