package main

// THE WIDGET'S THEME, ON STUDIO'S PAGE (S10, docs/S10-shared-themes-review-2026-11-04.md).
//
// Studio has no theme preference of its own. It reads the one `theme` setting the widget and the
// control centre use, and draws with the token values internal/widgetconfig derives from that
// palette, so the default is the widget's default dark palette rather than whatever the desktop
// is set to. The page asks at start-up and again on its slow readiness poll, so a theme changed
// in the widget's panel or the control centre reaches an open Studio within that poll.
//
// THE TWO SURFACES DO NOT BOTH UPDATE LIVE. The widget reads overlay.json once, at launch
// (plugins/overlay loadConfig), and applies at once only a change made in its own panel. A theme
// changed in the control centre or in the file reaches Studio on its next poll and the widget
// only when it is restarted. This does not add live reload to the widget.

import "github.com/chaynes-simpleclouds/marco/internal/widgetconfig"

// Theme is the theme Studio draws with: the widget's `theme` setting, resolved as the widget
// resolves it, and the page tokens for it.
func (a *Acts) Theme() widgetconfig.StudioTheme {
	return widgetconfig.StudioTokens(widgetconfig.Load().Theme)
}
