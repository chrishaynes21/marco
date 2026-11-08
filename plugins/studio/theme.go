package main

// THE WIDGET'S THEME, ON STUDIO'S PAGE (S10, docs/S10-shared-themes-review-2026-11-04.md).
//
// Studio has no theme preference of its own. It reads the one `theme` setting the widget and the
// control centre use, and draws with the token values internal/widgetconfig derives from that
// palette, so the default is the widget's default dark palette rather than whatever the desktop
// is set to. The page asks at start-up and again on its slow readiness poll, so a theme changed
// in the widget's panel or the control centre reaches an open Studio within that poll.
//
// BOTH SURFACES FOLLOW THE FILE: a running widget takes a changed setting within about a second
// (plugins/overlay reload.go, ADR-331), and Studio redraws at once with a theme saved in its own
// Widget settings pane (widgetsettings.go).

import "github.com/chaynes-simpleclouds/marco/internal/widgetconfig"

// Theme is the theme Studio draws with: the widget's `theme` setting, resolved as the widget
// resolves it, and the page tokens for it.
func (a *Acts) Theme() widgetconfig.StudioTheme {
	return widgetconfig.StudioTokens(widgetconfig.Load().Theme)
}
