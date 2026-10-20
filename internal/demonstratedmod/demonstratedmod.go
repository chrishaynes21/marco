// Package demonstratedmod embeds the canonical Demonstrated act surface (demonstrated.marco), so
// a play's `use demonstrated.` import resolves without every routes/ directory needing its own
// copy — the same arrangement osmod has for the OS act and theatermod for the Theater.
//
// This embedded copy is the single source of truth for what the Demonstrated act exports. If a
// capability is not listed there, no Marco program can call it, however much the Director
// underneath happens to be able to do.
package demonstratedmod

import _ "embed"

//go:embed demonstrated.marco
var Source string
