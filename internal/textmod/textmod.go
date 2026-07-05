// Package textmod embeds the Text act surface (text.marco) so a generated route can
// `use text.` and resolve the OCR resolver's Find without a sibling file — the same
// way osmod provides the OS surface. The act is fulfilled by the out-of-process OCR
// resolver plugin (`--host Text=bridge:ocr`); when no such host is wired, the route's
// fallback to Text simply resolves failed and the click falls back to its coordinate.
package textmod

import _ "embed"

//go:embed text.marco
var Source string
