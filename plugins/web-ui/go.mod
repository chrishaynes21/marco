module github.com/chaynes-simpleclouds/marco/plugins/web-ui

go 1.26.2

require github.com/chaynes-simpleclouds/marco v0.0.0

// The web UI decodes the Director's playbill into the DIRECTOR'S OWN TYPE rather than a
// hand-mirrored struct — the same pattern the overlay uses, and for the same reason: one
// representation, many presentations. A mirrored copy would drift the first time a field's
// meaning changed, silently, in the surface whose job is to tell somebody the truth.
//
// It costs this module nothing it was avoiding: pkg/playbill is standard library only.
replace github.com/chaynes-simpleclouds/marco => ../..
