---
type: adr
status: accepted
date: 2026-11-07
decision: "ACCEPTED — STUDIO HAS A WIDGET SETTINGS PANE, AND THE WIDGET FOLLOWS ITS SETTINGS FILE. Studio's rail gains Widget settings, under Open widget, offered without a flag. The pane is drawn from internal/widgetconfig Fields: the settings a person changes, grouped, with the choices the widget's own panel cycles through (Leaders, Corners, PaletteNames) and the theme as a grid of the widget's palettes drawn in their own colours. Each setting is saved as it changes, alone, through widgetconfig.Save: the one check the control centre's save now goes through too. Save writes only the keys sent and refuses, writing nothing, a value the widget would not use as written: a leader it would read as the backtick, a theme or corner it does not know, a number outside the panel's bounds, an empty activation phrase, a search address that is not https. The monitor is counted from 1 in the pane and from 0 in the file. A saved theme redraws Studio at once. A running widget looks at the file about once a second and takes every changed setting except one changed in its own panel and not yet saved (widgetconfig.Follow), applying each as the panel does; the activation phrase is read when the recogniser starts, and the pane says it applies the next time the widget opens."
supersedes: []
affects:
  - interface
source_paths:
  - internal/widgetconfig/form.go
  - plugins/studio/widgetsettings.go
  - plugins/studio/help.go
  - plugins/studio/frontend/index.html
  - plugins/overlay/reload.go
  - plugins/overlay/view.go
  - cmd/marco/oconfig.go
---

# Widget settings in Studio

Builds on [[ADR-307-studio-draws-the-widgets-theme]] and [[ADR-330-the-widget-anchors-to-the-chosen-display]].

## Why

The owner: "align the color scheme of the studio to match your widgets config, and add the
settings for the widget to a widget settings pane in studio". Studio already drew the widget's
theme ([[ADR-307-studio-draws-the-widgets-theme]]); on the owner's machine `theme` is `dracula`
and the running Studio was built from a commit that includes that. What it lacked was a place to
change the widget's settings, the theme among them, without the widget's in-HUD panel or the
control centre.

## Decision

- **One description of the settings** (`widgetconfig.Fields`), so the pane offers the widget's
  own choices rather than a copy typed into a page. Height is not offered (the widget fits it to
  its content) and neither is font, which the widget does not read.
- **One save** (`widgetconfig.Save`), which `marco edit`'s Config view now uses as well, so the
  two surfaces cannot disagree about what is allowed. A refusal is `ErrRefused` and reads as its
  reason alone; anything else is a write that failed.
- **Saved as changed, one setting at a time**, so a save writes only what somebody touched and a
  refusal is said under that setting with the control put back.
- **The widget follows the file** (`plugins/overlay` `reload.go`): the frame loop stats
  `overlay.json` every 60 frames and, when it changed, takes what changed through
  `widgetconfig.Follow`, keeping the panel's unsaved edits. Theme, leader, window placement and the
  voice gate are applied the way the panel applies them. This replaces "the widget reads its
  settings only at launch" in [[ADR-307-studio-draws-the-widgets-theme]].

## Not established

- Not tried live: the pane was rendered in headless Edge, not WebView2, and the widget's following
  was tested through `view.Update` with the fake Ebiten, not on the desktop.
- The control centre's form still keeps its own list of fields; only its save is shared.

## Enforced by

- `internal/widgetconfig`: `TestASaveFromAFormWritesOnlyWhatWasSent`, `TestASaveRefusesWhatTheWidgetWouldNotUse`, `TestTheFormOffersTheWidgetsChoicesAndEachSaves`, `TestFollowKeepsWhatThePanelHasNotSaved`.
- `plugins/overlay`: `TestTheWidgetTakesASettingSavedElsewhere`, `TestTheViewFollowsACornerSavedElsewhere`.
- `plugins/studio`: `TestTheWidgetSettingsCountMonitorsFromOne`, `TestTheWidgetSettingsRefuseInASentence`, `TestTheWidgetSettingsPaneSavesAndRedraws`, `TestTheAlphaRailOffersOneJourney`.
- `cmd/marco`: `TestTheControlCentreRefusesWhatItCannotSave`.
