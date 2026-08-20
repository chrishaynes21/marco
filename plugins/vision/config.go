package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// defaultLabels is the class list assumed when nothing better is available — the four UI
// kinds a route cares about. It is a LAST RESORT and a dangerous one, because a wrong
// label is not a cosmetic error: the Director's vision provider maps "button" to a control
// role and "icon" to a weaker one, so a single-class icon detector labelled with this list
// announces every icon it finds as a button.
//
// That is not hypothetical — it is what `models/icon_detect.onnx` (Ultralytics YOLO11m,
// one class, `names={0:'icon'}`) did on the first live run: 56 desktop icons, every one
// reported as a button. Prefer, in order: $MARCO_VISION_LABELS, a labels.txt beside the
// model, or the model's own embedded class names.
var defaultLabels = []string{"button", "icon", "menu item", "prompt"}

// labelSource says where a class list came from, so the backend knows whether it may be
// overridden by the model's own metadata and a reader knows what to trust.
type labelSource int

const (
	labelsFromDefault labelSource = iota // a guess; the model's own names beat it
	labelsFromModel                      // the model's embedded `names` metadata
	labelsFromFile                       // labels.txt beside the model
	labelsFromEnv                        // $MARCO_VISION_LABELS
)

func (s labelSource) String() string {
	switch s {
	case labelsFromEnv:
		return "$MARCO_VISION_LABELS"
	case labelsFromFile:
		return "labels.txt"
	case labelsFromModel:
		return "the model's own names"
	default:
		return "built-in defaults"
	}
}

// loadLabels reads the class names from $MARCO_VISION_LABELS (comma-separated), else a
// labels.txt next to $MARCO_VISION_MODEL, else defaultLabels — reporting which, because
// only the last of those may be overridden by what the model says about itself.
func loadLabels() ([]string, labelSource) {
	if v := strings.TrimSpace(os.Getenv("MARCO_VISION_LABELS")); v != "" {
		return splitTrim(v, ","), labelsFromEnv
	}
	if model := strings.TrimSpace(os.Getenv("MARCO_VISION_MODEL")); model != "" {
		if data, err := os.ReadFile(filepath.Join(filepath.Dir(model), "labels.txt")); err == nil {
			if ls := splitTrim(string(data), "\n"); len(ls) > 0 {
				return ls, labelsFromFile
			}
		}
	}
	return defaultLabels, labelsFromDefault
}

// parseNames reads an Ultralytics `names` metadata value into a class list, in class-index
// order. The value is a Python dict repr, which is what the exporter writes:
//
//	{0: 'icon'}
//	{0: 'button', 1: 'icon', 2: 'text'}
//
// Indices are honoured rather than assumed contiguous: a sparse or out-of-order map yields
// a list long enough to index safely, with gaps left empty rather than shifting every label
// onto the wrong class. An unparseable value returns nil, and the caller keeps what it had.
func parseNames(meta string) []string {
	meta = strings.TrimSpace(meta)
	meta = strings.TrimPrefix(meta, "{")
	meta = strings.TrimSuffix(meta, "}")
	if meta == "" {
		return nil
	}
	byIndex := map[int]string{}
	max := -1
	for _, entry := range strings.Split(meta, ",") {
		key, value, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil || idx < 0 {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `'"`)
		if value == "" {
			continue
		}
		byIndex[idx] = value
		if idx > max {
			max = idx
		}
	}
	if max < 0 {
		return nil
	}
	out := make([]string, max+1)
	for i, name := range byIndex {
		out[i] = name
	}
	return out
}

// splitTrim splits s on sep and trims each field, dropping blanks.
func splitTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// envStr reads a string tunable with a default.
func envStr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

// envInt / envFloat read a numeric tunable with a default.
func envInt(name string, def int) int {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envFloat(name string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}
