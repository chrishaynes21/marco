package widgetconfig

// THE WIDGET'S PALETTES, AS DATA, SO A SECOND SURFACE CAN USE THE SAME ONES (S10,
// docs/S10-shared-themes-review-2026-11-04.md).
//
// These were a map of colour structs in plugins/overlay/view.go. They moved here unchanged: the
// widget builds its drawing colours from these strings, and Studio derives its page tokens from
// them (StudioTokens). The `theme` setting picks one by name, and both surfaces resolve the name
// the same way, through PaletteFor.
//
// The values are the widget's and are not to be tuned for Studio. A colour Studio cannot use as
// it is gets adjusted in the mapping, never here: changing a value here changes what the widget
// draws. TestThePalettesAreTheWidgetsOwn holds them to the values the widget shipped with.

import "strings"

// Palette is one widget theme: eleven colours, each six hex digits with no '#'.
//
//   - Bg is the panel; Cmd the command text; LastCmd the previous command; Sep the dividers;
//     Desc the secondary text; Name the routine names (the widget's accent).
//   - Idle, Listen, Run and Errc are the state colours: ready, listening, running or done, and
//     error. LogInfo is the log's ordinary lines.
type Palette struct {
	Bg, Name, Cmd, LastCmd, Sep, Desc string
	Idle, Listen, Run, Errc           string
	LogInfo                           string
}

// DefaultPalette is the name every unknown, empty or misspelt theme resolves to.
const DefaultPalette = "default"

// paletteNames is the order the widget's settings panel cycles through, and the control centre
// offers.
var paletteNames = []string{
	"default", "dracula", "solarized-dark", "monokai", "nord",
	"tokyo-night", "catppuccin-mocha", "gruvbox-dark", "rose-pine", "light",
}

var palettes = map[string]Palette{
	"default": {
		Bg: "0A0A0A", Name: "00E5CC", Cmd: "DDDDDD", LastCmd: "4A8A8A",
		Sep: "2A4A4A", Desc: "6A9A98",
		Idle: "A0A8A5", Listen: "5AB4F5", Run: "4DC94D", Errc: "F56565",
		LogInfo: "8AC8DE",
	},
	"dracula": {
		Bg: "282A36", Name: "BD93F9", Cmd: "F8F8F2", LastCmd: "6272A4",
		Sep: "44475A", Desc: "6272A4",
		Idle: "9291CD", Listen: "8BE9FD", Run: "50FA7B", Errc: "FF5555",
		LogInfo: "8BE9FD",
	},
	"solarized-dark": {
		Bg: "002B36", Name: "2AA198", Cmd: "EEE8D5", LastCmd: "2AA198",
		Sep: "0F4A5A", Desc: "586E75",
		Idle: "6F979B", Listen: "268BD2", Run: "859900", Errc: "DC322F",
		LogInfo: "93A1A1",
	},
	"monokai": {
		Bg: "272822", Name: "A6E22E", Cmd: "F8F8F2", LastCmd: "75715E",
		Sep: "3E3D32", Desc: "75715E",
		Idle: "9AA470", Listen: "66D9E8", Run: "A6E22E", Errc: "F92672",
		LogInfo: "F8F8F2",
	},
	"nord": {
		Bg: "2E3440", Name: "88C0D0", Cmd: "ECEFF4", LastCmd: "4C566A",
		Sep: "434C5E", Desc: "4C566A",
		Idle: "7287A0", Listen: "81A1C1", Run: "A3BE8C", Errc: "BF616A",
		LogInfo: "88C0D0",
	},
	"tokyo-night": {
		Bg: "1A1B26", Name: "BB9AF7", Cmd: "C0CAF5", LastCmd: "565F89",
		Sep: "292E42", Desc: "565F89",
		Idle: "6A6A97", Listen: "7AA2F7", Run: "9ECE6A", Errc: "F7768E",
		LogInfo: "7AA2F7",
	},
	"catppuccin-mocha": {
		Bg: "1E1E2E", Name: "CBA6F7", Cmd: "CDD6F4", LastCmd: "6C7086",
		Sep: "313244", Desc: "7F849C",
		Idle: "8E8BAA", Listen: "89B4FA", Run: "A6E3A1", Errc: "F38BA8",
		LogInfo: "89DCEB",
	},
	"gruvbox-dark": {
		Bg: "282828", Name: "FE8019", Cmd: "EBDBB2", LastCmd: "7C6F64",
		Sep: "3C3836", Desc: "928374",
		Idle: "A78461", Listen: "83A598", Run: "B8BB26", Errc: "FB4934",
		LogInfo: "83A598",
	},
	"rose-pine": {
		Bg: "191724", Name: "C4A7E7", Cmd: "E0DEF4", LastCmd: "6E6A86",
		Sep: "26233A", Desc: "908CAA",
		Idle: "7F76A0", Listen: "9CCFD8", Run: "31748F", Errc: "EB6F92",
		LogInfo: "9CCFD8",
	},
	"light": {
		Bg: "FAFAFA", Name: "6F42C1", Cmd: "1A1A2E", LastCmd: "606060",
		Sep: "A0A8B0", Desc: "3A4A54",
		Idle: "505050", Listen: "0969DA", Run: "1A7F37", Errc: "CF222E",
		LogInfo: "0969DA",
	},
}

// PaletteNames is every palette, in the order the widget cycles them.
func PaletteNames() []string { return append([]string(nil), paletteNames...) }

// PaletteFor resolves a theme name the way the widget always has: case-insensitively, and to
// DefaultPalette when the name is not one of PaletteNames. It returns the name it resolved to.
func PaletteFor(name string) (string, Palette) {
	n := strings.ToLower(name)
	if p, ok := palettes[n]; ok {
		return n, p
	}
	return DefaultPalette, palettes[DefaultPalette]
}
