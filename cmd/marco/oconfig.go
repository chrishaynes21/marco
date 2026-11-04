package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// THE CONTROL CENTRE'S CONFIG VIEW READS AND WRITES THE WIDGET'S SETTINGS THROUGH THE ONE OWNER
// (internal/widgetconfig, S5).
//
// It used to keep a copy of the widget's fields and save by writing that whole struct back, read
// from the defaults and the file only. So it showed the default leader to somebody whose widget
// listened for the one in `$MARCO_OVERLAY_LEADER`, and saving the activation phrase wrote that
// default leader into the file, where it then won. Now it shows what the widget runs with
// (widgetconfig.Load) and a save writes only the settings the page sent, which are the ones the
// person changed (saveConfigView sends nothing else).

// handleOConfig (GET) returns the widget's settings as the widget runs with them; (POST) writes
// the settings sent, and nothing else.
func (e *editor) handleOConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		keys, err := saveSentSettings(r.Body)
		var notSaved errNotSaved
		switch {
		case errors.As(err, &notSaved):
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "saved": keys})
		return
	}
	writeJSON(w, map[string]any{"config": widgetconfig.Load(), "path": widgetconfig.Path()})
}

// errNotSaved marks a request that was understood and could not be written, as against one that
// was not understood.
type errNotSaved struct{ err error }

func (e errNotSaved) Error() string { return e.err.Error() }

// saveSentSettings writes the settings in body over the current ones, and returns their keys.
//
// Writing every setting must fail TestTheControlCentreSavesOnlyWhatWasSent.
func saveSentSettings(body io.Reader) ([]string, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(body); err != nil {
		return nil, err
	}
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &sent); err != nil {
		return nil, fmt.Errorf("the settings sent could not be read: %v", err)
	}
	keys := make([]string, 0, len(sent))
	for k := range sent {
		if !widgetconfig.Known(k) {
			return nil, fmt.Errorf("%q is not a widget setting", k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	s := widgetconfig.Load()
	if err := json.Unmarshal(buf.Bytes(), &s); err != nil {
		return nil, fmt.Errorf("the settings sent could not be read: %v", err)
	}
	s.Wake = strings.TrimSpace(s.Wake)
	if err := widgetconfig.Write(s, keys...); err != nil {
		return nil, errNotSaved{err}
	}
	return keys, nil
}

// runWake prints the configured voice activation phrase (overlay.json "wake", default "marco").
// The launcher reads it into $MARCO_VOICE_WAKE so editing it in the UI takes effect on relaunch.
func runWake() {
	w := strings.TrimSpace(widgetconfig.Load().Wake)
	if w == "" {
		w = "marco"
	}
	fmt.Println(w)
}
