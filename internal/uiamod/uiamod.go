// Package uiamod embeds the canonical Accessibility act surface (accessibility.marco), so a
// route's `use accessibility.` import resolves without every routes/ directory needing its
// own copy — the same arrangement osmod has for the OS act.
//
// # Why the module is `accessibility` and this package is not
//
// Because they name different things, and Marco's half is the one that has to read as English.
// A program says `use accessibility.` — what the capability MEANS. UIA is how Windows happens
// to provide it, which is the host's business and belongs in the host's name; a Marco program
// that opened with an acronym for a Microsoft API would be describing an implementation to its
// reader. The act inside has always been called `Accessibility`; only the import disagreed.
//
// This embedded copy is the single source of truth for what the Accessibility act
// exports. If a capability is not listed there, no Marco program can call it, however
// much the host underneath happens to implement.
package uiamod

import _ "embed"

//go:embed accessibility.marco
var Source string
