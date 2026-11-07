---
type: adr
status: accepted
date: 2026-11-07
decision: "ACCEPTED — THE WIDGET IS PLACED IN ITS CORNER ONLY WHILE EBITEN'S CURRENT MONITOR IS THE DISPLAY THE PERSON CHOSE, ON A MONITOR SWITCH AND ON EVERY AUTO-FIT RESIZE, AND THE SETTINGS PANEL COUNTS DISPLAYS FROM 1. In Ebiten v2.9.9 SetWindowPosition is measured from a cached current monitor that SetMonitor does not update, so placing straight after SetMonitor put the corner on the screen just left. The view's loop now asks for the switch, keeps the size, and gives SetWindowPosition the corner of the chosen display only once ebiten.Monitor() names it; if Ebiten has not agreed after twice its cache's life it asks again, three times at most, and then leaves the window where SetMonitor put it. The panel shows Monitor 1, Monitor 2, … (the index + 1, the primary first, as Ebiten lists them); overlay.json keeps the zero-based index."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/overlay/anchor.go
  - plugins/overlay/config.go
  - plugins/overlay/view.go
---

# The widget anchors to the chosen display

Builds on [[ADR-300-one-owner-for-the-widgets-settings]], which owns the `monitor` setting this
places by.

## What was wrong

The owner reported the monitor toggle could look like it did nothing. `placeWindow` called
`ebiten.SetMonitor` and then `ebiten.SetWindowPosition` straight after. In the Ebiten the widget
ships (v2.9.9):

- `internal/ui/ui_glfw.go` `currentMonitor()` returns `cachedCurrentMonitor` until it is one
  second's worth of ticks old;
- `setWindowMonitor` (same file) moves the window to the new display with `window.SetPos` and does
  not update that cache;
- `internal/ui/window_glfw.go` `glfwWindow.SetPosition` adds the cached monitor's origin to the
  position it is given.

So the corner was worked out from the chosen display's size and measured from the old display's
origin: the window went back to the screen it had just left. The auto-fit resize
(`view.Update`) re-placed it through the same call every time the height changed.

## Decision

- **Place only when Ebiten agrees** (`planPlacement`). A position is given to SetWindowPosition
  only while `ebiten.Monitor()` — the cache SetWindowPosition reads — is the chosen display, so
  "relative to the current monitor" and "relative to the chosen display" are the same. If the
  cache expires between the two calls it is refilled from the window's real display, which after
  SetMonitor is the chosen one. A quick toggle back, while the cache still names the first display,
  makes SetMonitor do nothing and the placement measures from the first display, which is where
  it is meant to go.
- **The same rule on resize.** An auto-fit height change applies the size at once and re-places
  under the same rule, so a resize never measures from a display the widget is not meant to be on.
- **Bounded.** Not agreed after `2 × TPS` frames (twice the cache's life): SetMonitor again, at
  most three times; then the window is left where SetMonitor put it — centred on the right screen
  rather than in the right corner of the wrong one.
- **The window calls are made by the view's loop.** `applyWindow` (the settings panel, startup)
  only sets a request; `anchor.frame` makes the calls, behind a small `windowSystem` interface so
  the loop is tested against a stand-in that keeps Ebiten's stale cache.
- **Counted from 1.** The panel shows `Monitor 1`, `Monitor 2`, … from the saved zero-based index,
  which is unchanged, so an existing `overlay.json` still names the same screen. Ebiten lists the
  primary first; the numbers are not Windows Display settings' numbers, which Ebiten does not know.

## Rejected

- **Compute an absolute position.** v2.9.9's `MonitorType` has no origin (Name, DeviceScaleFactor,
  Size), so the offset between two displays cannot be had from Ebiten.
- **Place the window through Win32.** It would need Ebiten's window handle found by title, work in
  physical pixels against a width configured in DIPs, and race Ebiten's own `SetPos`.

## Not established

- **Not tried on a real desktop.** The tests run against a stand-in written from Ebiten's source.
- Mixed-DPI displays: the size is set again when the position is given, so Ebiten converts it with
  the chosen display's scale. The stand-in records which scale a size was converted with, and that
  is all it models of DPI.
- A display configuration change (a monitor unplugged) mid-placement is left to Ebiten's own
  monitor list; the setting falls back to the primary when it names a display that is not there.

## Enforced by

- `plugins/overlay` `TestNothingIsPlacedWhileEbitenNamesAnotherDisplay`,
  `TestTheToggleLandsOnTheChosenDisplay`, `TestAQuickToggleBackStillLands`,
  `TestAResizeOnTheSecondDisplayStaysInItsCorner`,
  `TestAWindowEbitenNeverAgreesAboutIsLeftOnTheChosenScreen`,
  `TestAWindowThatEndsUpElsewhereIsAskedOverAgain`,
  `TestTheViewAnchorsOnStartupAndOnResize`, `TestTheCornerIsMeasuredFromTheChosenDisplay`,
  `TestTheMonitorRowCountsFromOne`.
