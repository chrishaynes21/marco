// Package screenmod embeds the canonical Screen act surface (screen.marco) so a play
// can ask what is in front of it without being able to change it.
//
// The same shape as osmod and uiamod: one embedded file, the single source of truth,
// resolved by `use screen.` without a sibling copy.
package screenmod

import _ "embed"

//go:embed screen.marco
var Source string
