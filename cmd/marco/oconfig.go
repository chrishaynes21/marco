package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
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

// saveSentSettings writes the settings in body over the current ones, and returns their keys,
// through the one check every surface's save goes through (widgetconfig.Save).
//
// Writing every setting must fail TestTheControlCentreSavesOnlyWhatWasSent.
func saveSentSettings(body io.Reader) ([]string, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(body); err != nil {
		return nil, err
	}
	keys, err := widgetconfig.Save(buf.Bytes())
	if err != nil && !errors.Is(err, widgetconfig.ErrRefused) {
		return nil, errNotSaved{err}
	}
	return keys, err
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
