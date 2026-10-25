//go:build windows

package main

// READING AND PRESSING STUDIO'S REAL WINDOW, THROUGH SOMETHING THAT IS NOT STUDIO.
//
// # Why the accessibility tree and not the page
//
// Studio's controls are HTML in a WebView2. The obvious way to press one from outside would be
// Chrome DevTools over a debugging port, and it is not available: `go-webview2` clears
// `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` in a function called `preventEnvAndRegistryOverrides`,
// so no environment variable can open a port, and Wails exposes no option that would. Patching
// production to leave a debug port open in everybody's build is not a thing a harness gets to ask
// for.
//
// What is available is better. Chromium publishes the page to Windows' UI Automation, Marco
// already has a bridge host that reads it — `plugins/uia` — and that bridge is a different
// process from Studio, reached through the operating system. So:
//
//   - the ELEMENT ADDRESS is the DOM id. Chromium maps `id="routinePlayBtn"` to the
//     element's AutomationId, which makes the harness's handle on a control the same
//     string the page's author wrote, rather than a coordinate or a label that changes
//     when somebody improves the wording.
//   - pressing is `Invoke` on the control's own pattern, which is what an assistive
//     technology does. It runs the page's real handler, which calls the real binding,
//     which runs the real backend. Nothing here substitutes a backend call for a press.
//   - reading is a SNAPSHOT of what is on screen, taken from outside. When this harness
//     asserts that the review card says a recording will not replace a routine, the
//     sentence comes from the operating system's account of the window — not from the
//     backend being asked what it would have rendered.
//
// # What a snapshot does not contain
//
// Anything hidden. The page shows one surface at a time and the others are `display:none`, which
// is correctly absent from the accessibility tree — so a harness that cannot find a control must
// first go to the surface it lives on. That is why `Rail` exists.
//
// And `Partial` is load-bearing here exactly as it is for the Director: a walk that hit a node,
// depth or time limit says so, and this refuses to conclude anything from a truncated tree. "The
// Play button does not exist" and "I stopped looking" are different findings.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// element is one control in a window, as UI Automation describes it.
type element struct {
	ID           string `json:"Id"`
	ParentID     string `json:"ParentId"`
	Role         string `json:"Role"`
	ControlType  string `json:"ControlType"`
	Label        string `json:"Label"`
	Value        string `json:"Value"`
	X, Y, W, H   int
	Enabled      bool
	Visible      bool
	Offscreen    bool
	AutomationID string `json:"AutomationId"`
	ClassName    string `json:"ClassName"`
	Depth        int
}

// snapshot is one window as the operating system describes it, at one instant.
type snapshot struct {
	WindowID    string `json:"WindowId"`
	WindowTitle string `json:"WindowTitle"`
	App         string `json:"App"`
	ProcessID   int    `json:"ProcessId"`
	WindowX     int
	WindowY     int
	WindowW     int
	WindowH     int
	Minimized   bool
	Maximized   bool
	Partial     bool
	Reason      string
	ElapsedMs   int
	Elements    []element
}

// byID is the element with this DOM id, and whether there was exactly one.
//
// EXACTLY ONE. Two controls answering to one id is a page defect, and a harness that took the
// first would press one of them and report about the other.
func (s snapshot) byID(id string) (element, bool) {
	var found element
	n := 0
	for _, e := range s.Elements {
		if e.AutomationID == id {
			found, n = e, n+1
		}
	}
	return found, n == 1
}

// labelled is the first element whose visible text contains this phrase, for the things a page
// says rather than the things it offers.
func (s snapshot) labelled(phrase string) (element, bool) {
	low := strings.ToLower(phrase)
	for _, e := range s.Elements {
		if strings.Contains(strings.ToLower(e.Label), low) {
			return e, true
		}
	}
	return element{}, false
}

// says reports whether any element on screen contains this phrase.
func (s snapshot) says(phrase string) bool {
	_, ok := s.labelled(phrase)
	return ok
}

// ids is every DOM id on screen, for a failure that has to explain what WAS there.
func (s snapshot) ids() []string {
	var out []string
	for _, e := range s.Elements {
		if e.AutomationID != "" {
			out = append(out, e.AutomationID)
		}
	}
	return out
}

// uia is the bridge host, spawned per request.
//
// A process per call rather than one long-lived pipe. A snapshot costs a few hundred milliseconds
// and a scenario takes a dozen of them, so the saving would be seconds — and a long-lived child
// is a thing that can wedge, half-answer, or outlive a failed scenario, which is a worse trade
// for a harness whose whole job is to be believed when it reports.
type uia struct {
	bin string
	art *artifacts
}

// request is one bridge call: the act, the action, and the input object.
func (u uia) request(action string, input map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{
		"act": "Accessibility", "action": action, "input": input,
	})
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(u.bin)
	cmd.Stdin = bytes.NewReader(append(body, '\n'))
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	started := time.Now()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("uia %s: %w (%s)", action, err,
			strings.TrimSpace(errOut.String()))
	}
	line := strings.TrimSpace(out.String())
	u.art.note("uia", "%s %v -> %d bytes in %dms", action, input, len(line),
		time.Since(started).Milliseconds())
	var reply struct {
		Status string          `json:"status"`
		Reason string          `json:"reason"`
		Error  string          `json:"error"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(line), &reply); err != nil {
		return nil, fmt.Errorf("uia %s: the reply was not JSON: %q", action, line)
	}
	if reply.Status != "ok" {
		why := reply.Reason
		if why == "" {
			why = reply.Error
		}
		return nil, fmt.Errorf("uia %s refused: %s", action, why)
	}
	return reply.Data, nil
}

// look takes a snapshot of one window.
//
// The handle is always named. `uia` would fall back to the foreground window if asked about
// nothing, and a harness that accepted that fallback would describe whatever happened to be in
// front as though it were Studio — which is the exact defect that bridge's own README records.
func (u uia) look(hwnd uintptr) (snapshot, error) {
	data, err := u.request("Snapshot", map[string]any{
		"Window": fmt.Sprintf("hwnd:%d", hwnd), "MaxNodes": 4000, "TimeoutMs": 9000,
	})
	if err != nil {
		return snapshot{}, err
	}
	var s snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return snapshot{}, fmt.Errorf("uia Snapshot: %w", err)
	}
	if s.Partial {
		return s, fmt.Errorf("the walk of %s stopped early (%s), so nothing can be "+
			"concluded from what it did not reach", s.WindowTitle, s.Reason)
	}
	return s, nil
}

// press invokes one control, by the DOM id the page gave it.
//
// It takes a FRESH snapshot and resolves the id in it, rather than accepting an element from an
// older one: a UIA runtime id goes stale on a repaint, and this page repaints on a poll. The
// element is also required to be enabled — invoking a disabled control is a call that succeeds
// and presses nothing, which is the shape of defect this harness exists to catch rather than to
// commit.
func (u uia) press(hwnd uintptr, id string) error {
	s, err := u.look(hwnd)
	if err != nil {
		return err
	}
	e, ok := s.byID(id)
	if !ok {
		return fmt.Errorf("there is no single control called %q on screen; what is "+
			"there: %s", id, strings.Join(s.ids(), " "))
	}
	if !e.Enabled {
		return fmt.Errorf("%q is on screen and disabled, so pressing it would do "+
			"nothing", id)
	}
	_, err = u.request("Invoke", map[string]any{
		"Window": fmt.Sprintf("hwnd:%d", hwnd), "Element": e.ID,
	})
	return err
}

// focusWindow asks the window to take the keyboard, which on Windows also brings it forward.
//
// Arrangement, not input: it is the same class of act as the fixture raising itself when the
// harness asks, and for the same reason — a process that does not own the foreground cannot hand
// it to anybody, so the asking has to happen inside something that does.
func (u uia) focusWindow(hwnd uintptr) error {
	_, err := u.request("Focus", map[string]any{
		"Window": fmt.Sprintf("hwnd:%d", hwnd),
	})
	return err
}
