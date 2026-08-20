package uiaclient

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var _ directorapi.ValueProvider = (*Provider)(nil)

// SetValue sets a control's value through its native value API.
//
// The strongest way to change text, and the reason is not speed. Typing sends
// characters through the keyboard layout, the IME, and whatever the application does
// with each keystroke — autocomplete that rewrites the field on the third letter,
// validation that rejects an intermediate state. Setting the value has none of those
// failure modes because there is no intermediate state for anything to react to.
//
// It returns the value the control actually HOLDS afterwards, which is not always the
// value it was given: a field with a length cap or an input mask accepts the call and
// keeps something else. Returning the asked-for value would make verification a
// tautology.
func (p *Provider) SetValue(ctx context.Context, window directorapi.WindowID, nativeID, value string) (string, error) {
	if nativeID == "" {
		return "", fmt.Errorf("uiaclient: setting a value needs the provider's own element id")
	}
	in := runtime.NewSet()
	in.Put("Element", runtime.Text(nativeID))
	in.Put("Value", runtime.Text(value))
	if window != "" {
		in.Put("Window", runtime.Text(string(window)))
	}
	in.Put("MaxNodes", runtime.Number(float64(p.maxNodes())))

	status, data, err := p.host.Invoke(runtime.HostCall{
		Act:    accessibilityAct,
		Action: "SetValue",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    ctx,
	})
	if err != nil {
		return "", fmt.Errorf("uiaclient: set value: %w", err)
	}
	if status == "failed" {
		msg := errText(data)
		// "unsupported" is the bridge saying the control has no writable value API.
		// That is an ANSWER, not a fault: it tells the caller to fall back. A real
		// error tells it to stop. Collapsing the two would turn every read-only field
		// into an outage and every outage into a burst of typing.
		if strings.HasPrefix(msg, "unsupported:") {
			return "", &directorapi.ValueUnsupportedError{Reason: strings.TrimSpace(strings.TrimPrefix(msg, "unsupported:"))}
		}
		return "", fmt.Errorf("uiaclient: set value failed: %s", msg)
	}
	var wire struct {
		Value string `json:"Value"`
	}
	if err := decode(data, &wire); err != nil {
		return "", fmt.Errorf("uiaclient: set value reply: %w", err)
	}
	return wire.Value, nil
}

// GetValue reads a control's current value.
//
// The bool distinguishes "this control has no value" from "its value is empty". A
// label has no value; an emptied search box does, and it is "". Verification of a
// Clear depends entirely on being able to tell those apart.
func (p *Provider) GetValue(ctx context.Context, window directorapi.WindowID, nativeID string) (string, bool, error) {
	if nativeID == "" {
		return "", false, fmt.Errorf("uiaclient: reading a value needs the provider's own element id")
	}
	in := runtime.NewSet()
	in.Put("Element", runtime.Text(nativeID))
	if window != "" {
		in.Put("Window", runtime.Text(string(window)))
	}
	in.Put("MaxNodes", runtime.Number(float64(p.maxNodes())))

	status, data, err := p.host.Invoke(runtime.HostCall{
		Act:    accessibilityAct,
		Action: "GetValue",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    ctx,
	})
	if err != nil {
		return "", false, fmt.Errorf("uiaclient: get value: %w", err)
	}
	if status == "failed" {
		msg := errText(data)
		if strings.HasPrefix(msg, "unsupported:") {
			return "", false, nil // no value API — not an error, just nothing to read
		}
		return "", false, fmt.Errorf("uiaclient: get value failed: %s", msg)
	}
	var wire struct {
		Value string `json:"Value"`
	}
	if err := decode(data, &wire); err != nil {
		return "", false, fmt.Errorf("uiaclient: get value reply: %w", err)
	}
	return wire.Value, true, nil
}
