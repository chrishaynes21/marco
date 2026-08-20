// nameprobe reports what naming evidence an accessibility snapshot actually contains.
//
// A measurement tool for Roadmap 34 semantic Place naming, written so the naming rule is derived
// from trees rather than from memory. It reads the bridge's own one-shot dump
// (`uia snapshot --window <hwnd> out.json`) and prints the candidate signals, nothing more: it
// ranks nothing and decides nothing.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type element struct {
	ID           string
	ParentID     string `json:"ParentId"`
	Role         string
	ControlType  string
	Label        string
	Value        string
	Description  string
	X, Y, W, H   int
	Selected     bool
	Focused      bool
	Offscreen    bool
	AutomationID string `json:"AutomationId"`
	ClassName    string
	Depth        int
}

type snapshot struct {
	WindowTitle string
	App         string
	Elements    []element
}

func main() {
	for _, path := range os.Args[1:] {
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("read:", err)
			continue
		}
		var s snapshot
		if err := json.Unmarshal(raw, &s); err != nil {
			fmt.Println("parse:", err)
			continue
		}
		report(path, s)
	}
}

func report(path string, s snapshot) {
	fmt.Printf("\n================ %s\n", path)
	fmt.Printf("WindowTitle  %q\n", s.WindowTitle)
	fmt.Printf("App          %q\n", s.App)
	fmt.Printf("Elements     %d\n", len(s.Elements))

	byID := map[string]element{}
	for _, e := range s.Elements {
		byID[e.ID] = e
	}
	trail := func(e element) string {
		var parts []string
		for cur, n := e, 0; n < 6; n++ {
			p, ok := byID[cur.ParentID]
			if !ok {
				break
			}
			if p.Label != "" {
				parts = append(parts, p.Role+":"+p.Label)
			} else {
				parts = append(parts, p.Role)
			}
			cur = p
		}
		return strings.Join(parts, " < ")
	}

	fmt.Println("\n-- SELECTED (the navigation destination, if the tree says so)")
	any := false
	for _, e := range s.Elements {
		if e.Selected {
			any = true
			fmt.Printf("   %-11s %-28q off=%-5v auto=%-22q  %s\n",
				e.Role, e.Label, e.Offscreen, e.AutomationID, trail(e))
		}
	}
	if !any {
		fmt.Println("   (none)")
	}

	fmt.Println("\n-- FOCUSED")
	any = false
	for _, e := range s.Elements {
		if e.Focused {
			any = true
			fmt.Printf("   %-11s %-28q auto=%q\n", e.Role, e.Label, e.AutomationID)
		}
	}
	if !any {
		fmt.Println("   (none)")
	}

	fmt.Println("\n-- NAMED CONTAINERS (window/pane/group/list with a label), shallowest first")
	var containers []element
	for _, e := range s.Elements {
		switch e.Role {
		case "window", "pane", "group", "list", "document", "tab":
			if strings.TrimSpace(e.Label) != "" {
				containers = append(containers, e)
			}
		}
	}
	sort.SliceStable(containers, func(i, j int) bool { return containers[i].Depth < containers[j].Depth })
	for i, e := range containers {
		if i >= 14 {
			fmt.Printf("   … %d more\n", len(containers)-i)
			break
		}
		fmt.Printf("   d%-2d %-11s %-30q auto=%-20q class=%q\n",
			e.Depth, e.Role, e.Label, e.AutomationID, e.ClassName)
	}

	fmt.Println("\n-- ON-SCREEN TEXT, shallowest first (a heading is usually the shallowest big one)")
	var texts []element
	for _, e := range s.Elements {
		if e.Role == "text" && !e.Offscreen && strings.TrimSpace(e.Label) != "" {
			texts = append(texts, e)
		}
	}
	sort.SliceStable(texts, func(i, j int) bool {
		if texts[i].Depth != texts[j].Depth {
			return texts[i].Depth < texts[j].Depth
		}
		return texts[i].Y < texts[j].Y
	})
	for i, e := range texts {
		if i >= 12 {
			fmt.Printf("   … %d more\n", len(texts)-i)
			break
		}
		fmt.Printf("   d%-2d y=%-5d h=%-4d %-32q auto=%-18q  %s\n",
			e.Depth, e.Y, e.H, e.Label, e.AutomationID, trail(e))
	}

	fmt.Println("\n-- AUTOMATION IDS worth knowing about")
	seen := map[string]bool{}
	for _, e := range s.Elements {
		id := e.AutomationID
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		fmt.Printf("   %-32q %-11s %q\n", id, e.Role, e.Label)
	}
}
