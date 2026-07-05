package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// defaultLabels is the class list assumed when the model ships no labels — the four UI
// kinds a route cares about. The REAL class names and ORDER come from the model: set
// $MARCO_VISION_LABELS (comma-separated) or drop a labels.txt (one per line) beside the
// model file, in the model's class-index order. A wrong list only mislabels, never
// misplaces — boxes are still correct.
var defaultLabels = []string{"button", "icon", "menu item", "prompt"}

// loadLabels reads the model's class names from $MARCO_VISION_LABELS (comma-separated),
// else a labels.txt next to $MARCO_VISION_MODEL, else defaultLabels.
func loadLabels() []string {
	if v := strings.TrimSpace(os.Getenv("MARCO_VISION_LABELS")); v != "" {
		return splitTrim(v, ",")
	}
	if model := strings.TrimSpace(os.Getenv("MARCO_VISION_MODEL")); model != "" {
		if data, err := os.ReadFile(filepath.Join(filepath.Dir(model), "labels.txt")); err == nil {
			if ls := splitTrim(string(data), "\n"); len(ls) > 0 {
				return ls
			}
		}
	}
	return defaultLabels
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
