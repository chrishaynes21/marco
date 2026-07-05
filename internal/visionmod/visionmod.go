// Package visionmod embeds the Vision act surface (vision.marco) so a generated route
// can `use vision.` and resolve the semantic UI-element detector's Detect/Locate without
// a sibling file — the same way osmod and textmod provide their surfaces. The act is
// fulfilled by the out-of-process vision resolver plugin (`--host Vision=bridge:vision`);
// when no such host is wired, a route's fallback to Vision simply resolves failed and the
// click falls back to its coordinate (or another resolver in the chain).
package visionmod

import _ "embed"

//go:embed vision.marco
var Source string
