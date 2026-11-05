package main

// THE CONTROL CENTRE STOPS ITSELF ONCE ITS PAGE HAS GONE.
//
// `marco ui` used to block in http.Serve until somebody pressed Ctrl+C in the terminal it was
// started from. Started by the widget's `help`, it had no terminal anybody could see, and closing
// the browser tab closed nothing — so one was found still running the next day, with the shell
// that launched it long gone, holding `marco.exe` open and refusing `plugins\studio\build.cmd`.
// `exit` in the widget quits the widget, and is right to: the control centre is not its child in
// any sense a person can see.
//
// So it goes the way the Director goes once nothing uses it (ADR-308): the page says it is still
// there while it is open (`/api/alive`), every request counts as being there, and once nothing
// has been heard for pageGone, the server shuts down. Never while a run it started is still going,
// because the run's answer is what the page is waiting for — and once it is done nobody is left to
// ask it.
//
// pageGone is minutes, not seconds, for a reason about browsers: a hidden tab's timers are
// throttled to about once a minute, and a heartbeat sent every twenty seconds can arrive sixty
// apart. The same allowance covers a browser that is slow to open the page at all.
//
// See [[ADR-316-the-control-centre-stops-once-its-page-has-gone]].

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// pageGone is how long the control centre waits, having heard nothing from its page, before it
// stops, and pageCheck how often it looks. Variables only so a test can wait milliseconds.
var (
	pageGone  = 3 * time.Minute
	pageCheck = 15 * time.Second
)

// serveEditor is the control centre's server as `marco ui` runs it: its own doors, stopping once
// its page has gone and no run it started is still going.
func serveEditor(ln net.Listener, ed *editor, mux http.Handler) error {
	return serveUntilGone(ln, mux, ed.runs.busy, pageGone, pageCheck)
}

// presence is when the page was last heard from.
type presence struct {
	mu   sync.Mutex
	last time.Time
}

func (p *presence) seen(t time.Time) {
	p.mu.Lock()
	p.last = t
	p.mu.Unlock()
}

func (p *presence) quietSince(now time.Time) time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return now.Sub(p.last)
}

// wrap counts every request the page makes as the page being there.
func (p *presence) wrap(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.seen(time.Now())
		h.ServeHTTP(w, r)
	})
}

// busy is whether a run this control centre started has not finished yet.
func (a *runAccount) busy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.runs {
		if !r.Done {
			return true
		}
	}
	return false
}

// handleAlive is the page saying it is still open. The request itself is the message.
func handleAlive(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

// serveUntilGone serves h on ln until the page has been gone for limit and busy says nothing is
// still running, checking every tick. It returns nil when it stopped for that reason.
//
// Starting counts as being heard from, so a browser that never opens the page still lets the
// server go after limit.
//
// Removing the quiet check must fail TestTheControlCentreStopsOnceItsPageHasGone; ignoring busy
// must fail TestTheControlCentreWaitsForARunItStarted.
func serveUntilGone(ln net.Listener, h http.Handler, busy func() bool, limit, tick time.Duration) error {
	var p presence
	p.seen(time.Now())
	srv := &http.Server{Handler: p.wrap(h)}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case err := <-served:
			return err
		case now := <-t.C:
			if p.quietSince(now) < limit || busy() {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := srv.Shutdown(ctx)
			cancel()
			if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return err
		}
	}
}
