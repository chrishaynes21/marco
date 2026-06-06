package bridgehost

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// fakeBridge wires the host to an in-process responder via io.Pipe, so the JSON
// request/response round-trip is tested without launching a subprocess. respond
// receives each decoded request and returns the wire response to send back.
func fakeBridge(t *testing.T, respond func(req map[string]any) string) *Host {
	t.Helper()
	reqR, reqW := io.Pipe()  // host writes requests here; responder reads
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
