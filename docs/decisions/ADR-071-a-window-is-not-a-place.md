---
type: decision
status: accepted
date: 2026-08-18
supersedes: []
affects:
  - semantic-memory
  - perception
  - passive-observation
source_paths:
  - internal/director/perception/observation/chrome.go
  - internal/director/observe/screenstate.go
  - cmd/director/observesnapshot.go
  - plugins/uia/Uia.cs
  - pkg/directorapi/world.go
---

# ADR-071 — a window is not a place

Windows Settings minted a fresh durable subject for the same page on almost every visit. Three
families of twins in one live store each differed from their named original by **exactly three
buttons** and every other role count was identical:

```
61ff vs Mouse      button 16 vs 13
543  vs Bluetooth  button 13 vs 10
235d vs Home       button 18 vs 15
```

`director inspect -chrome` named them: the frame's own **Minimize, Restore and Close**. A route
learned on one of these twins is structurally valid and practically unreachable, because the
place it starts from never comes back.

## Decision

**A window's own machinery is not part of what makes a screen that screen.** An element is chrome
when it IS, or descends from, a window-frame part — today a `title_bar` or a `scroll_bar`.

Classified by **hierarchy**, from the `ParentNativeID` the accessibility provider already supplies.
One classifier, `observation.Chrome`, which `director inspect -chrome` reports with and the
observation pipeline carries forward.

### What it is deliberately not

- **Not geometry.** An earlier attempt excluded zero-area elements. It was measured and reverted:
  Settings reports legitimate page content — a combo box, links, nineteen pieces of text — with
  `0,0 0x0` bounds, and excluding those broke recall on real screens. Geometry describes how
  something is laid out at one instant; ownership describes what it belongs to.
- **Not names.** "Minimize", "Restore" and "Close" are one operating system's words in one
  language.
- **Not a process, a title, or a position in the window.**
- **Not a matcher change.** `RoleCountTolerance`, the role-set rule and the term rules are
  untouched. This fixes what evidence reaches the matcher, not how the matcher weighs it.

### It labels, it does not remove

Chrome is still observed, still fused, still addressable, still shown by `inspect` and by Sight,
and a play may still press Close. Exactly one consumer reads the label: `NewScreenSignature`, the
choke point where a composition becomes a durable place. Window identity is unaffected — that is
`windowref`'s question and it still gets the whole window.

## An upstream defect this exposed

The UIA bridge had no `ControlType.TitleBar` case, so every title bar degraded to `unknown` before
the Director saw it. The hierarchy was intact; the *semantics* were lost at the provider boundary.
Fixed in `plugins/uia/Uia.cs`, with `RoleTitleBar` added to `directorapi` and to the client's
`knownRoles`. Without it no hierarchy rule could have named the frame at all.

## Known, and not settled here

`cmd/director/observesnapshot.go` has **a pre-existing geometric filter** on the fused identity
path: an entity with no extent is dropped before it becomes a region. It predates this work and it
is why the zero-area caption buttons were not the live driver on that path — the scroll bar's
arrow buttons, which have real area, were. That filter sits uneasily beside the rule above and
should be revisited: it is exactly the geometric exclusion this ADR rejects, applied one layer
earlier.

## Enforced by

- `internal/director/perception/observation/chrome_test.go` —
  `TestChromeIsEveryDescendantOfAWindowFramePart` (the ancestor walk, not just the frame parts);
  `TestGeometryDoesNotDecideChrome`; `TestChromeIsNotDecidedByLabels`;
  `TestAnOrphanIsTreatedAsPageContent` (a broken hierarchy fails OPEN, never dropping structure);
  `TestAMalformedTreeDoesNotHang`.
- `internal/director/observe/chromeplace_test.go` —
  `TestAWindowsOwnMachineryIsNotPartOfThePlace`;
  `TestTheSamePageWithAndWithoutItsFrameRecallsTheSame`;
  `TestPageContentWithNoRectangleIsStillPartOfThePlace` (the reverted geometric rule may not come
  back); `TestTwoDifferentPagesRemainDistinct` and `TestThreeRealSettingsPagesStayThreePlaces`
  (over-merging is the worse failure); `TestChromeIsStillObservedAndOnlyLeftOutOfIdentity`.
- `cmd/director/chromewiring_test.go` — `TestTheShadowSampleCarriesTheChromeClassification` and
  `TestChromeIsStillPresentInTheRawSample`: this binary asks the classifier and carries the answer,
  and does not delete chrome from what it saw.

## Corrected by

The pre-existing geometric filter flagged above as "sits uneasily beside the rule above" was the
last twin generator, and [[ADR-072-a-place-is-not-its-viewport]] removes it: extent is not a
membership test for place identity.

## Related

[[ADR-062-a-scroll-bar-is-not-a-screen]] — the same reasoning, applied to one role and by name;
this generalises it to ownership. ·

[[ADR-016-cross-session-identity-is-structural-and-conservative]] ·
[[Semantic-Memory]] · [[Perception]]
