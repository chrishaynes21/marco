package driver

import (
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// voicingOverlay is the Overlay bridge as the widget now is: it fulfils the Overlay act and
// PUSHES the Voice events itself, because the overlay starts the recogniser
// (plugins/overlay/voice.go). Before, they arrived only on serve's standard input, through a pipe
// overlay.cmd alone set up.
type voicingOverlay struct {
	mu    sync.Mutex
	calls []string
	ev    chan runtime.Event
	ran   chan struct{}
	once  sync.Once
}

func (o *voicingOverlay) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	o.mu.Lock()
	o.calls = append(o.calls, c.Action+" "+c.Input.AsText())
	o.mu.Unlock()
	if c.Action == "RunVoice" {
		o.once.Do(func() { close(o.ran) })
	}
	return "ok", runtime.Value{}, nil
}

func (o *voicingOverlay) Events() <-chan runtime.Event { return o.ev }

// The real overlay.marco, served by the real driver, hears a Partial and a Final pushed by the
// Overlay bridge, with nothing on standard input: the partial is shown, and the finished phrase is
// handed back as RunVoice, the widget's one intake.
//
// The widget's voice depends on this leg; dropping overlay.marco's `when Voice reads Final` must
// fail it.
func TestTheWidgetsOwnVoiceEventsReachRunVoice(t *testing.T) {
	o := &voicingOverlay{ev: make(chan runtime.Event), ran: make(chan struct{})}
	go func() {
		o.ev <- runtime.Event{Feed: "Voice", Message: "Partial", Payload: runtime.Text("cloud pull")}
		o.ev <- runtime.Event{Feed: "Voice", Message: "Final", Payload: runtime.Text("cloud pull stone")}
		select {
		case <-o.ran:
		case <-time.After(10 * time.Second):
		}
		close(o.ev) // the overlay closing ends the run
	}()
	script := filepath.Join("..", "..", "plugins", "overlay", "overlay.marco")
	err := ServeFile(script, strings.NewReader(""), io.Discard, map[string]runtime.Host{"Overlay": o})
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	got := strings.Join(o.calls, "\n")
	for _, want := range []string{"Heard cloud pull", "RunVoice cloud pull stone"} {
		if !strings.Contains(got, want) {
			t.Errorf("the overlay was not asked %q; it was asked:\n%s", want, got)
		}
	}
}
