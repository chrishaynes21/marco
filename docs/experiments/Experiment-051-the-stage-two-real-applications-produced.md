---
type: experiment
status: complete
date: 2026-10-15
backend:
  - ocr
  - vision
  - accessibility
game: trackmania
fixture: "the tm-menu-steps-sanitized recording through the real OCR and ScreenParser providers, the real collector and the real fusion engine; and a live Notepad through director execute --dry-run"
result: the-integration-seam-works-and-the-first-missing-connection-is-upstream-no-source-on-either-surface-produced-an-observation-of-an-OBJECT-so-the-stage-had-no-props-and-the-licence-was-never-reached
supersedes: []
affects:
  - perception
  - execution
---

# Experiment 051 — the Stage two real applications produced

**Status: RUN 2026-10-15. No game inputs. The one live action was opening Notepad and a
`--dry-run` request, which plans and verifies and sends nothing.**

## The question

[[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] was validated
against hand-built Stages. The owner asked for the integration instead:

> Goal: establish whether the actual Collector → Fusion → WorldState → Director Pipeline can
> resolve and execute an explicitly requested action, including on a pixel-only surface.

## The seam, which needed no new code

`capture.WindowCapture` is one method — `CaptureWindow(ctx, window) (Image, error)`. A recorded
PNG is served through it and **nothing downstream can tell the difference**, so the real
`ocr.Provider` and the real `vision.Provider` read a recording exactly as they read a screen.
That is the whole adapter, and it lives in the test.

Three things had to be got right, and each was a real integration finding before it was a fix:

```
  the capture needs a TIMESTAMP     a capture older than 2s is refused as stale, and an
                                    image with no time reads as 2562047h old
  the request must ASK for them     an empty Sources list means "what the providers can
                                    CHEAPLY see", and neither OCR nor a detector is cheap
  the detector needs a LABEL READER `rt.vision.Reader = newLabelReader(textEngine)` in the
                                    runtime; without it every non-structural detection is
                                    rejected as "nothing readable inside"
```

## Trackmania — a surface with no accessibility at all

One frame of the pause menu: RESUME, CHANGE OPPONENTS, ADD TRACK TO FAVORITES, SCOREBOARD,
VIEW RECORDS, SETTINGS, REPORT, UBISOFT CONNECT, EXIT.

```
  ocr      available, 1920x1080, accepted 43, rejected on confidence 10
  vision   available, model vision-bridge, 1.19s
           classes found   button 3   menu 1   text 23   image 2
           accepted 3 — ALL of them text
           rejected 25 "nothing readable inside a non-structural class" (before the
                       label reader was wired), 4 on confidence

  the Stage   46 observation(s)  by source {ocr 43, vision 3}
                                 BY KIND   {text 46}
  fusion      46 observations -> 0 elements, 34 text standalone
  Director    unobservable — "nothing in this window can be operated, so no control
              could be matched"
```

**Every observation on this Stage is text.** The detector does propose three buttons and a menu
— it can see the menu — but below the 0.15 acceptance floor and far below the 0.25 a structural
class needs, so they never become observations. Its confident detections are all `text`, which
the provider emits as text evidence.

So fusion receives no observation of an OBJECT, builds no Element, and **the Stage has no Props
on it**. The licence is never reached, because there is nothing to license an action on.

### Which layer, named

**A perception gap**, upstream of everything ADR-202 added, and consistent with ADR-202's own
rule: read text does not describe an object. The resolver is not at fault and neither is the
policy; there was nothing to resolve.

**Not fixed here, deliberately.** Lowering the detector's floor to make this frame work is
tuning a threshold to one recording of one game, which is the move this subsystem has paid for
repeatedly. The honest options are a better model, a structural proposer that is allowed to
produce objects on pixel-only surfaces, or a demonstration — and choosing between them is a
product decision, not a test fix.

## Notepad — the accessibility control

Opened live, `director execute --dry-run --json "click File"`, which plans and verifies and
sends no input:

```
  intent   click "File"                                                    ok
  observe  1 elements, coverage 0.00, actionable 0.00                      ok
  resolve  unobservable: the application exposed only containers and no
           content, so its interior was never visible — this is not
           evidence the target is absent                                   refused
  observe  the reading did not represent the window — looking again
           before refusing                                                 ok
  resolve  (the same, and final)                                           refused
```

The whole path ran: real collector, real fusion, real resolver, and a **re-observation before
refusing**. The refusal is the correct one and says the right thing — *this is not evidence the
target is absent* — which is the Coverage/Actionability rule of `policy` working.

**It is a weaker control than intended, and that should be said plainly.** This Notepad exposed
one element and no interior, so the run confirms the path is intact and unchanged; it does not
demonstrate a successful accessibility-backed resolution. A classic Win32 menu bar would.

## What this establishes

| | |
|---|---|
| recorded pixels reach the real providers | yes, through one existing interface |
| the real collector and fusion run over them | yes |
| the Director reaches a decision and explains it | yes, on both surfaces |
| a pixel-only Prop is resolved under licence | **not reached** — there were no Props |
| the accessibility path is unchanged | consistent with it; not positively demonstrated |

The first missing connection is **not** the one ADR-202 addressed. It is that on a pixel-only
surface, nothing currently produces an observation of an object at all.

## Related

- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence
  this set out to exercise.
- [[ADR-192-a-second-source-in-the-live-loop]] — the detector, and what it is and is not for.
- [[Experiment-049-the-control-it-could-see-and-could-not-name]] — the other way a pixel-only
  control is reached, which is a demonstration.
