---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — THE WIDGET'S HEADER CAN BE HIDDEN. A `header` setting (on by default, so a settings file that says nothing about it keeps the header) shows or hides the crown at the top of the full widget: the clock, the date and `Marco`. With it off, the widget starts at CPU and memory or the cursor coordinates when those are on, with the rule under them, and otherwise at its hint line with no rule. The window shrinks by what is no longer drawn: Draw and layoutHeight both start from crownTop. Mini mode has no header either way. It is a row of the widget's own panel (the last, so no other row moves), a toggle in Studio's Widget settings under Look, and a row in the control centre's Config view. A running widget follows it within about a second, as it follows every setting. Two guards came with it: Studio's form must offer every setting but height and font, and the control centre must offer every setting the form does."
supersedes: []
affects:
  - interface
source_paths:
  - internal/widgetconfig/settings.go
  - internal/widgetconfig/form.go
  - plugins/overlay/config.go
  - plugins/overlay/view.go
  - cmd/marco/edit.go
---

# The widget's header can be hidden

Builds on [[ADR-331-widget-settings-in-studio]].

## Why

The owner: "add an option to display or hide the header in the widget".

## Not established

- Not seen on the desktop. Ebiten cannot draw without a window, so the layout is held by its height
  (`layoutHeight`) and the drawing by the widget's own suite, not by a picture.

## Enforced by

- `plugins/overlay`: `TestHidingTheHeaderShrinksTheWidget`.
- `internal/widgetconfig`: `TestTheFormOffersEverySettingButHeightAndFont`.
- `cmd/marco`: `TestTheControlCentreOffersEverySettingTheFormDoes`.
- `plugins/studio`: `TestTheWidgetSettingsPaneSavesAndRedraws` (counts a row for every field of the form).
