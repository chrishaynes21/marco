package widgetconfig

// ONE READER AND ONE WRITER FOR overlay.json (S5 of docs/Surfaces-alignment-proposal-2026-11-02.md,
// finding F5).
//
// Three surfaces show or change the widget's settings: the widget's own settings panel, the
// control centre's Config view (`marco edit`), and Studio, which shows the leader in front of
// every shortcut. Each used to have its own copy. The widget's copy had no `wake` field and
// saved by writing its whole struct back, so a save from the widget dropped the activation phrase
// the control centre had set. The control centre's copy read the file but not the widget's
// environment seed, so it showed the default leader to somebody whose widget listened for
// CapsLock, and a save of anything wrote that default leader back.
//
// # The rules
//
//   - **Reading has one precedence, the widget's**: the defaults, then `$MARCO_OVERLAY_*`, then the
//     file. Each key in the file is read on its own, so a key of the wrong type keeps the value
//     before it and does not spoil the others.
//   - **Writing is a read-modify-write of the file's own fields** (Write), never a struct written
//     back. Only the keys the caller names are written. Every other field stays exactly as it was,
//     including ones no Marco surface knows, under the same cross-process lock the route store
//     uses, and the file is replaced atomically.
//   - **A file that cannot be read is not overwritten.** A save over a file that does not parse
//     would replace everything in it with the few keys the caller changed, so Write refuses and
//     says so.
//
// Library and resolver settings are not here; they stay where they are (proposal §5.6).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Settings is every widget setting a Marco surface knows. The json names are the file's.
type Settings struct {
	Theme    string  `json:"theme"`   // default | dracula | …
	Idle     float64 `json:"idle"`    // idle panel opacity
	Monitor  int     `json:"monitor"` // monitor index (0 = primary)
	Corner   string  `json:"corner"`  // top-right | top-left | bottom-right | bottom-left | top-center
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Leader   string  `json:"leader"`   // see Named for what the widget listens for
	Font     string  `json:"font"`     // path to a .ttf, or "" for the built-in
	MaxLines int     `json:"maxLines"` // log lines shown
	Border   bool    `json:"border"`
	Mini     bool    `json:"mini"`
	Metrics  bool    `json:"metrics"`
	Coords   bool    `json:"coords"`
	Voice    bool    `json:"voice"` // act on spoken commands
	Wake     string  `json:"wake"`  // the voice activation phrase, read at launch as $MARCO_VOICE_WAKE
	// Search is where "search the web for …" searches: google | bing | duckduckgo | brave |
	// startpage, or an https address with {query} in it; "" is google (internal/skills). Which
	// BROWSER opens it is Windows' default, a separate setting that is not Marco's.
	Search string `json:"search"`
	// ConfirmSearch makes "search the web for …" wait for its read-back and a second Enter before
	// it runs, as a routine with a value does; off, it runs on the first Enter like Focus app.
	ConfirmSearch bool `json:"confirmSearch"`
}

// Defaults are the widget's own defaults, and the control centre's activation phrase.
func Defaults() Settings {
	return Settings{Theme: "default", Idle: 0.72, Corner: "top-right", Width: 340, Height: 270,
		Leader: DefaultLeader, MaxLines: 5, Voice: true, Wake: "marco"}
}

// fields is each key of the file, with where its value lives in s.
func (s *Settings) fields() map[string]any {
	return map[string]any{
		"theme": &s.Theme, "idle": &s.Idle, "monitor": &s.Monitor, "corner": &s.Corner,
		"width": &s.Width, "height": &s.Height, "leader": &s.Leader, "font": &s.Font,
		"maxLines": &s.MaxLines, "border": &s.Border, "mini": &s.Mini, "metrics": &s.Metrics,
		"coords": &s.Coords, "voice": &s.Voice, "wake": &s.Wake, "search": &s.Search, "confirmSearch": &s.ConfirmSearch,
	}
}

// Known is whether key is a setting this package reads and writes.
func Known(key string) bool {
	var s Settings
	_, ok := s.fields()[key]
	return ok
}

// Load is the settings the widget runs with: the defaults, then the environment seed, then the file.
func Load() Settings {
	s := Defaults()
	seedFromEnv(&s)
	raw, err := readRaw(Path())
	if err != nil || raw == nil {
		return s
	}
	for k, dst := range s.fields() {
		if v, ok := raw[k]; ok {
			// A key of the wrong type keeps the value before it (decodeInto writes only a
			// value that decoded whole).
			_ = decodeInto(v, dst)
		}
	}
	return s
}

// decodeInto decodes v into dst only if the whole value decodes.
func decodeInto(v json.RawMessage, dst any) error {
	switch p := dst.(type) {
	case *string:
		var x string
		if err := json.Unmarshal(v, &x); err != nil {
			return err
		}
		*p = x
	case *float64:
		var x float64
		if err := json.Unmarshal(v, &x); err != nil {
			return err
		}
		*p = x
	case *int:
		var x int
		if err := json.Unmarshal(v, &x); err != nil {
			return err
		}
		*p = x
	case *bool:
		var x bool
		if err := json.Unmarshal(v, &x); err != nil {
			return err
		}
		*p = x
	default:
		return fmt.Errorf("widgetconfig: no decoder for %T", dst)
	}
	return nil
}

// seedFromEnv is the widget's environment seed: so a setup made with `$MARCO_OVERLAY_*` before
// the file existed carries over, and so every surface shows what the widget will run with.
func seedFromEnv(s *Settings) {
	if v := os.Getenv("MARCO_OVERLAY_THEME"); v != "" {
		s.Theme = v
	}
	if v := os.Getenv("MARCO_OVERLAY_IDLE"); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			s.Idle = f
		}
	}
	if v := os.Getenv("MARCO_OVERLAY_MONITOR"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			s.Monitor = n
		}
	}
	if parts := strings.SplitN(os.Getenv("MARCO_OVERLAY_SIZE"), "x", 2); len(parts) == 2 {
		w, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		h, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if e1 == nil && e2 == nil {
			s.Width, s.Height = w, h
		}
	}
	if v := os.Getenv("MARCO_OVERLAY_LEADER"); v != "" {
		s.Leader = v
	}
	if v := os.Getenv("MARCO_OVERLAY_FONT"); v != "" {
		s.Font = v
	}
	if v := os.Getenv("MARCO_OVERLAY_MAXLINES"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			s.MaxLines = n
		}
	}
	if v := os.Getenv("MARCO_VOICE"); v != "" {
		s.Voice = !strings.EqualFold(v, "off") // MARCO_VOICE=off starts muted
	}
}

// errUnreadable is a settings file that exists and is not a JSON object.
var errUnreadable = errors.New("is not a settings file Marco can read")

// readRaw is the file's own fields, nil when there is no file.
func readRaw(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(b)) == "" {
		return map[string]json.RawMessage{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s %w: %v", path, errUnreadable, err)
	}
	return raw, nil // nil for a file holding `null`, which Write starts from as empty
}

// writeWithin bounds the wait for another writer. A write is one read and one rename.
const writeWithin = 2 * time.Second

// Write saves the named keys of s into the settings file and changes nothing else in it.
//
// Naming the keys is the point: a surface writes what its person changed, so a save of the
// activation phrase does not also write a leader, and a field nobody here knows survives every
// save. An unknown key, or a file that exists and cannot be read, is an error and nothing is
// written.
//
// Writing the whole struct back must fail TestASaveKeepsWhatItDidNotChange.
func Write(s Settings, keys ...string) error {
	vals := s.fields()
	for _, k := range keys {
		if _, ok := vals[k]; !ok {
			return fmt.Errorf("widgetconfig: %q is not a widget setting", k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	hold, err := routes.LockFile(path+".lock", "another change to the widget's settings", writeWithin)
	if err != nil {
		return err
	}
	defer hold.Release()
	raw, err := readRaw(path)
	if err != nil {
		return fmt.Errorf("the widget's settings were not saved: %w", err)
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	for _, k := range keys {
		b, err := json.Marshal(vals[k])
		if err != nil {
			return err
		}
		raw[k] = b
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(out, '\n'))
}

// writeAtomic replaces path through a temporary file beside it, so a reader never sees half a file.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".overlay-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
