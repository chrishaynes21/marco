package widgetconfig

// THE WIDGET'S SETTINGS AS A FORM, AND THE ONE CHECK A SAVE GOES THROUGH.
//
// Studio's Widget settings pane draws itself from Fields, so the choices it offers are the ones
// the widget's own panel cycles through (Leaders, Corners, PaletteNames) and not a second copy of
// them typed into a page. Save is the one way a surface writes what a person sent: only the keys
// sent, each checked, so a corner the widget does not know or a leader it would silently replace
// with the backtick is refused with the setting's name rather than written.
//
// THE WIDGET FOLLOWS THE FILE (plugins/overlay reload.go): a change saved here reaches a running
// widget within about a second, except the ones marked AtLaunch, which it reads only when it
// starts.

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/skills"
)

// Leaders and Corners are the choices the widget's settings panel cycles through.
var (
	Leaders = []string{"`", "/", "capslock", "tab", "f8"}
	Corners = []string{"top-right", "top-left", "bottom-right", "bottom-left", "top-center"}
)

// The bounds the widget's panel holds each number to.
const (
	MinIdle, MaxIdle         = 0.2, 1.0
	MinWidth, MaxWidth       = 220, 1400
	MinMaxLines, MaxMaxLines = 1, 20
)

// Choice is one option of a choice field. Swatch, for a theme, is its ground, accent and text.
type Choice struct {
	Value  string   `json:"value"`
	Label  string   `json:"label"`
	Swatch []string `json:"swatch,omitempty"`
}

// Field is one setting as a form shows it.
type Field struct {
	Key   string `json:"key"`
	Group string `json:"group"`
	Label string `json:"label"`
	// Kind is "choice", "toggle", "number" or "text".
	Kind    string   `json:"kind"`
	Choices []Choice `json:"choices,omitempty"`
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Step    float64  `json:"step,omitempty"`
	Hint    string   `json:"hint,omitempty"`
	// AtLaunch is a setting the widget reads only when it starts.
	AtLaunch bool `json:"atLaunch,omitempty"`
}

// Fields is every setting a person changes, grouped, in the order a form reads. Height is not
// one (the widget fits its height to what it shows), nor is font, which the widget does not read.
func Fields() []Field {
	leaders := make([]Choice, len(Leaders))
	for i, l := range Leaders {
		leaders[i] = Choice{Value: l, Label: Named(l)}
	}
	leaders[0].Label = "` (backtick)"
	themes := make([]Choice, 0, len(paletteNames))
	for _, n := range paletteNames {
		_, p := PaletteFor(n)
		themes = append(themes, Choice{Value: n, Label: n, Swatch: []string{"#" + p.Bg, "#" + p.Name, "#" + p.Cmd}})
	}
	corners := make([]Choice, len(Corners))
	for i, c := range Corners {
		corners[i] = Choice{Value: c, Label: strings.ReplaceAll(c, "-", " ")}
	}
	return []Field{
		{Key: "leader", Group: "Keys and voice", Label: "Leader key", Kind: "choice", Choices: leaders,
			Hint: "Press it, then a play's key, or type a play's name."},
		{Key: "voice", Group: "Keys and voice", Label: "Act on spoken commands", Kind: "toggle"},
		{Key: "wake", Group: "Keys and voice", Label: "Activation phrase", Kind: "text", AtLaunch: true,
			Hint: "What you say before a command."},
		{Key: "theme", Group: "Look", Label: "Theme", Kind: "choice", Choices: themes,
			Hint: "Studio draws with it too."},
		{Key: "idle", Group: "Look", Label: "Opacity when idle", Kind: "number", Min: MinIdle, Max: MaxIdle, Step: 0.05},
		{Key: "border", Group: "Look", Label: "Accent border", Kind: "toggle"},
		{Key: "header", Group: "Look", Label: "Header", Kind: "toggle",
			Hint: "The clock, the date and the name at the top. Mini mode has none."},
		{Key: "mini", Group: "Look", Label: "Mini mode", Kind: "toggle"},
		{Key: "monitor", Group: "Place", Label: "Monitor", Kind: "number", Min: 1, Step: 1,
			Hint: "Counted from 1, as the widget's panel shows it."},
		{Key: "corner", Group: "Place", Label: "Corner", Kind: "choice", Choices: corners},
		{Key: "width", Group: "Place", Label: "Width", Kind: "number", Min: MinWidth, Max: MaxWidth, Step: 20},
		{Key: "maxLines", Group: "Place", Label: "Log lines shown", Kind: "number", Min: MinMaxLines, Max: MaxMaxLines, Step: 1},
		{Key: "metrics", Group: "Extras", Label: "CPU and memory", Kind: "toggle"},
		{Key: "coords", Group: "Extras", Label: "Cursor coordinates", Kind: "toggle"},
		{Key: "search", Group: "Web search", Label: "Search with", Kind: "text",
			Hint: "google, bing, duckduckgo, brave or startpage, or an https address with {query} in it. Your default browser opens it."},
		{Key: "confirmSearch", Group: "Web search", Label: "Confirm a search before it runs", Kind: "toggle"},
	}
}

// ErrRefused marks a save that was understood and not allowed: a setting that is not one, or a
// value the widget would not use as written. Anything else from Save is a write that failed.
var ErrRefused = errors.New("refused")

// refused is a refusal that reads as its reason alone and is ErrRefused.
type refused struct{ why string }

func (r refused) Error() string        { return r.why }
func (r refused) Is(target error) bool { return target == ErrRefused }

func refusef(format string, a ...any) error { return refused{fmt.Sprintf(format, a...)} }

// Save writes the settings in sent, a JSON object of setting keys, over the current ones, and
// returns the keys it wrote. Nothing is written unless every one is a setting with a value the
// widget would use as it is.
//
// Writing a key that was not sent must fail TestASaveFromAFormWritesOnlyWhatWasSent.
func Save(sent []byte) ([]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(sent, &raw); err != nil {
		return nil, refusef("the settings sent could not be read: %v", err)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if !Known(k) {
			return nil, refusef("%q is not a widget setting", k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := Load()
	for _, k := range keys {
		if err := decodeInto(raw[k], s.fields()[k]); err != nil {
			return nil, refusef("%s: %v", k, err)
		}
	}
	s.Wake = strings.TrimSpace(s.Wake)
	s.Search = strings.TrimSpace(s.Search)
	for _, k := range keys {
		if err := check(s, k); err != nil {
			return nil, refusef("%v", err)
		}
	}
	if err := Write(s, keys...); err != nil {
		return nil, err
	}
	return keys, nil
}

// check is whether the widget would use s's value for key as it is.
//
// Letting a leader through that the widget reads as the backtick must fail
// TestASaveRefusesWhatTheWidgetWouldNotUse.
func check(s Settings, key string) error {
	in := func(v string, opts []string) bool { return slices.Contains(opts, v) }
	switch key {
	case "leader":
		// The widget listens for the backtick when it does not know a leader, so a leader is
		// one it knows: a backtick spelling, or one Named does not turn into the backtick.
		l := strings.ToLower(strings.TrimSpace(s.Leader))
		if Named(l) == "`" && !in(l, []string{"`", "backtick", "grave", "tilde"}) {
			return fmt.Errorf("the widget does not know the leader key %q", s.Leader)
		}
	case "theme":
		if !in(strings.ToLower(s.Theme), paletteNames) {
			return fmt.Errorf("%q is not one of the widget's themes", s.Theme)
		}
	case "corner":
		if !in(s.Corner, Corners) {
			return fmt.Errorf("%q is not a corner the widget knows", s.Corner)
		}
	case "idle":
		if s.Idle < MinIdle || s.Idle > MaxIdle {
			return fmt.Errorf("opacity is between %.1f and %.1f, not %g", MinIdle, MaxIdle, s.Idle)
		}
	case "width":
		if s.Width < MinWidth || s.Width > MaxWidth {
			return fmt.Errorf("the width is between %d and %d, not %d", MinWidth, MaxWidth, s.Width)
		}
	case "maxLines":
		if s.MaxLines < MinMaxLines || s.MaxLines > MaxMaxLines {
			return fmt.Errorf("the log shows between %d and %d lines, not %d", MinMaxLines, MaxMaxLines, s.MaxLines)
		}
	case "monitor":
		if s.Monitor < 0 {
			return fmt.Errorf("there is no monitor %d", s.Monitor+1)
		}
	case "search":
		if s.Search != "" {
			if _, err := skills.SearchURL(s.Search, "marco"); err != nil {
				return err
			}
		}
	case "wake":
		if s.Wake == "" {
			return errors.New("the activation phrase cannot be empty")
		}
	}
	return nil
}

// Follow copies into dst every setting of next whose key is not in keep, and returns the keys
// whose value changed. It is how a running widget takes a change saved elsewhere without losing
// one made in its own panel and not yet saved (keep).
//
// Overwriting a kept key must fail TestFollowKeepsWhatThePanelHasNotSaved.
func Follow(dst *Settings, next Settings, keep map[string]bool) []string {
	have, want := dst.fields(), next.fields()
	var changed []string
	for k, p := range have {
		if keep[k] {
			continue
		}
		a, _ := json.Marshal(p)
		b, _ := json.Marshal(want[k])
		if string(a) == string(b) {
			continue
		}
		_ = decodeInto(b, p)
		changed = append(changed, k)
	}
	sort.Strings(changed)
	return changed
}
