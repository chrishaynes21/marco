// Package theatermod embeds the canonical Theater act surface (theater.marco), so a route's
// `use theater.` import resolves without every routes/ directory needing its own copy — the same
// arrangement osmod has for the OS act.
//
// This embedded copy is the single source of truth for what the Theater act exports. If a
// capability is not listed there, no Marco program can call it, however much the Director
// underneath happens to be able to do.
package theatermod

import _ "embed"

//go:embed theater.marco
var Source string
