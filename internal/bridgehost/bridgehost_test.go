package bridgehost

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// fakeBridge wires the host to an in-process responder via io.Pipe, so the JSON
// request/response round-trip is tested without launching a subprocess. respond
// receives each decoded request and returns the wire response to send back.
func fakeBridge(t *testing.T, respond func(req map[string]any) string) *Host {
	t.Helper()
	reqR, reqW := io.Pipe()   // host writes requests here; responder reads
	respR, respW := io.Pipe() // responder writes responses here; host reads
	h := &Host{}
	h.startFn = func() (io.Writer, io.Reader, func() error, error) {
		return reqW, respR, func() error { return nil }, nil
	}
	go func() {
		sc := bufio.NewScanner(reqR)
		for sc.Scan() {
			var req map[string]any
			_ = json.Unmarshal(sc.Bytes(), &req)
			io.WriteString(respW, respond(req)+"\n")
		}
	}()
	return h
}

func TestBridgeEchoOK(t *testing.T) {
	h := fakeBridge(t, func(req map[string]any) string {
		// Echo the input back as data.
		b, _ := json.Marshal(map[string]any{"status": "ok", "data": req["input"]})
		return string(b)
	})
	status, data, err := h.Invoke(runtime.HostCall{
		Act: "OS", Action: "Key", Input: runtime.Text("e"), Ctx: context.Background(),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status != "ok" || data.AsText() != "e" {
		t.Fatalf("got status=%q data=%q", status, data.AsText())
	}
}

func TestBridgeForwardsActAndAction(t *testing.T) {
	var seenAct, seenAction string
	h := fakeBridge(t, func(req map[string]any) string {
		seenAct, _ = req["act"].(string)
		seenAction, _ = req["action"].(string)
		return `{"status":"ok","data":null}`
	})
	_, _, _ = h.Invoke(runtime.HostCall{Act: "OS", Action: "Click", Ctx: context.Background()})
	if seenAct != "OS" || seenAction != "Click" {
		t.Fatalf("bridge saw act=%q action=%q", seenAct, seenAction)
	}
}

func TestBridgeFailed(t *testing.T) {
	h := fakeBridge(t, func(map[string]any) string {
		return `{"status":"failed","error":"unknown key"}`
	})
	status, data, _ := h.Invoke(runtime.HostCall{Act: "OS", Action: "Key", Ctx: context.Background()})
	if status != "failed" || !data.IsError() || data.AsError().Message != "unknown key" {
		t.Fatalf("got status=%q data=%#v", status, data)
	}
}

// TestBridgePushesEvent proves the demux: a bridge may interleave a {"feed":...}
// event line with its response — the event surfaces on Events() while Invoke
// still pairs with the response line.
func TestBridgePushesEvent(t *testing.T) {
	h := fakeBridge(t, func(map[string]any) string {
		// One event line, then the response line (fakeBridge appends a newline).
		return `{"feed":"Hotkeys","event":"Stop"}` + "\n" + `{"status":"ok","data":null}`
	})
	evs := h.Events() // bring the source online
	status, _, _ := h.Invoke(runtime.HostCall{Act: "OS", Action: "Key", Ctx: context.Background()})
	if status != "ok" {
		t.Fatalf("status=%q", status)
	}
	select {
	case ev := <-evs:
		if ev.Feed != "Hotkeys" || ev.Message != "Stop" {
			t.Fatalf("got feed=%q msg=%q", ev.Feed, ev.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("no event surfaced")
	}
}

// TestBridgeStartFailureDegrades proves a bridge that can't launch (e.g. a
// missing overlay/macros binary) degrades to failed and closes its event source,
// instead of panicking the engine on a nil encoder.
func TestBridgeStartFailureDegrades(t *testing.T) {
	h := &Host{}
	h.startFn = func() (io.Writer, io.Reader, func() error, error) {
		return nil, nil, nil, errors.New("boom")
	}
	evs := h.Events() // triggers the (failed) start
	status, _, err := h.Invoke(runtime.HostCall{Act: "X", Action: "Y", Ctx: context.Background()})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if status != "failed" {
		t.Fatalf("status=%q, want failed", status)
	}
	select {
	case _, ok := <-evs:
		if ok {
			t.Fatal("expected a closed event channel after start failure")
		}
	case <-time.After(time.Second):
		t.Fatal("event channel not closed after start failure")
	}
}

func TestBridgeReturnsSet(t *testing.T) {
	h := fakeBridge(t, func(map[string]any) string {
		return `{"status":"ok","data":{"Exe":"notepad.exe"}}`
	})
	status, data, _ := h.Invoke(runtime.HostCall{Act: "OS", Action: "Active", Ctx: context.Background()})
	if status != "ok" || !data.IsSet() {
		t.Fatalf("got status=%q set=%v", status, data.IsSet())
	}
	exe, _ := data.AsSet().Get("Exe")
	if exe.AsText() != "notepad.exe" {
		t.Fatalf("Exe = %q", exe.AsText())
	}
}
