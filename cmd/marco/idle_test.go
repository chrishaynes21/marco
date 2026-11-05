package main

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// shortPresence makes the control centre give up in milliseconds for one test.
func shortPresence(t *testing.T) {
	t.Helper()
	gone, check := pageGone, pageCheck
	pageGone, pageCheck = 300*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { pageGone, pageCheck = gone, check })
}

// startEditor runs the control centre the way `marco ui` does and reports when it stops.
func startEditor(t *testing.T, ed *editor) (url string, stopped <-chan error) {
	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- serveEditor(ln, ed, ed.serveMux()) }()
	return "http://" + ln.Addr().String(), done
}

func alivePing(t *testing.T, url string) {
	t.Helper()
	resp, err := http.Post(url+"/api/alive", "text/plain", nil)
	if err != nil {
		t.Fatalf("the page could not reach its control centre: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("/api/alive answered %d", resp.StatusCode)
	}
}

// The owner found one running a day after its page was closed, holding marco.exe open.
func TestTheControlCentreStopsOnceItsPageHasGone(t *testing.T) {
	shortPresence(t)
	_, stopped := startEditor(t, &editor{})
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("stopped with an error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("nothing had been heard from the page and the control centre was still running")
	}
}

func TestTheControlCentreStaysWhileItsPageIsOpen(t *testing.T) {
	shortPresence(t)
	url, stopped := startEditor(t, &editor{})
	until := time.Now().Add(1200 * time.Millisecond) // four times the allowance
	for time.Now().Before(until) {
		select {
		case <-stopped:
			t.Fatal("the control centre stopped while its page was still saying it was there")
		default:
		}
		alivePing(t, url)
		time.Sleep(80 * time.Millisecond)
	}
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("the page stopped and the control centre never did")
	}
}

func TestTheControlCentreWaitsForARunItStarted(t *testing.T) {
	shortPresence(t)
	ed := &editor{}
	id := ed.runs.begin("save the file") // a run still going, as a clicked Run leaves one
	_, stopped := startEditor(t, ed)
	select {
	case <-stopped:
		t.Fatal("the control centre stopped with a run it started still going")
	case <-time.After(1200 * time.Millisecond):
	}
	ed.runs.finish(id, "done", "", "")
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("the run finished and the control centre never stopped")
	}
}

// The page is what keeps the server up, so it must keep saying so, and must say when nobody
// answers rather than failing quietly on the next click.
func TestThePageKeepsItsControlCentreAliveAndSaysWhenItHasClosed(t *testing.T) {
	for _, want := range []string{
		"setInterval(alive, 20000)",
		"fetch('/api/alive',{method:'POST'})",
		"This control centre has closed.",
	} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the page no longer has %q", want)
		}
	}
	// The heartbeat must come well inside the allowance even when a hidden tab's timers are
	// throttled to about once a minute.
	if pageGone < 2*time.Minute {
		t.Errorf("pageGone is %v; a throttled hidden tab can be a minute between heartbeats", pageGone)
	}
}
