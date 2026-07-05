// Package osmod embeds the canonical OS act surface (os.marco) so the teach
// orchestrator can seed a routes/ directory with it, letting generated routes'
// `use os.` import resolve. This embedded copy is the single source of truth.
package osmod

import _ "embed"

//go:embed os.marco
var Source string
