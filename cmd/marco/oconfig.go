package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// overlayConfig mirrors the overlay's persisted settings (plugins/overlay/config.go) plus the
// voice Wake word (activation phrase), so the control center can edit the SAME overlay.json the
// overlay reads. Unknown-to-here keys aren't preserved, so this struct lists every overlay field.
type overlayConfig struct {
	Theme    string  `json:"theme"`
	Idle     float64 `json:"idle"`
	Monitor  int     `json:"monitor"`
	Corner   string  `json:"corner"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Leader   string  `json:"leader"`
	Font     string  `json:"font"`
	MaxLines int     `json:"maxLines"`
	Border   bool    `json:"border"`
	Mini     bool    `json:"mini"`
	Metrics  bool    `json:"metrics"`
	Coords   bool    `json:"coords"`
	Voice    bool    `json:"voice"`
	Wake     string  `json:"wake"` // voice activation phrase — read at launch as $MARCO_VOICE_WAKE
}

func defaultOverlayConfig() overlayConfig {
	return overlayConfig{Theme: "default", Idle: 0.72, Corner: "top-right", Width: 340, Height: 270,
		Leader: "`", MaxLines: 5, Voice: true, Wake: "marco"}
}

// overlayConfigPath matches plugins/overlay/config.go: $MARCO_OVERLAY_CONFIG, else
// <user-config-dir>/marco/overlay.json.
func overlayConfigPath() string {
	if p := os.Getenv("MARCO_OVERLAY_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "marco", "overlay.json")
}

// loadOverlayConfig starts from defaults and overlays the saved file (absent keys keep defaults).
func loadOverlayConfig() overlayConfig {
	cfg := defaultOverlayConfig()
	if b, err := os.ReadFile(overlayConfigPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	return cfg
}

func saveOverlayConfig(cfg overlayConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	p := overlayConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

// handleOConfig (GET) returns the overlay settings; (POST) merges the submitted fields into the
// saved file. Decoding the POST over the loaded config updates only the keys the client sent.
func (e *editor) handleOConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		cfg := loadOverlayConfig()
		if json.NewDecoder(r.Body).Decode(&cfg) != nil {
			http.Error(w, "bad request", 400)
			return
		}
		cfg.Wake = strings.TrimSpace(cfg.Wake)
		if err := saveOverlayConfig(cfg); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	writeJSON(w, map[string]any{"config": loadOverlayConfig(), "path": overlayConfigPath()})
}

// runWake prints the configured voice activation phrase (overlay.json "wake", default "marco").
// The launcher reads it into $MARCO_VOICE_WAKE so editing it in the UI takes effect on relaunch.
func runWake() {
	w := strings.TrimSpace(loadOverlayConfig().Wake)
	if w == "" {
		w = "marco"
	}
	fmt.Println(w)
}
