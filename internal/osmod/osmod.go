// Package osmod embeds the OS act surface (os.marco) so the teach orchestrator
// can seed a routes/ directory with it, letting generated routes' `use os.`
// import resolve without a checkout of programs/.
package osmod

import _ "embed"

//go:embed os.marco
var Source string
