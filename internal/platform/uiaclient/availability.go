package uiaclient

import (
	"context"
	"io"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Asking the accessibility provider whether it can work, and keeping the answer it gives.
//
// # Why the reason was worth rescuing
//
// The provider has always answered properly. `plugins/uia/Program.cs` replies
// `{"Available":false,"Reason":"no foreground automation element"}`, and this package decoded that
// Reason into a local variable and threw it away, because its one caller's question was a bool.
//
// That bool is the whole of what every surface above could ever say. "Marco cannot act here" sends
// a person to look for a setting; "the accessibility bridge is not at plugins\uia\uia.exe" is a
// sentence they can do something about, and the difference between them was being discarded one
// line after it arrived.
//
// # Why it is a package function and not a method
//
// Two callers, one wire question. `Provider` is the Director's perception client, configured with
// node and depth caps it does not need to ask this; the Theater's accessibility Actor holds a bare
// `runtime.Host` and has no Provider at all. Both must get the SAME answer — an Actor that decided
// its own availability differently from the perception stack would be a second opinion about one
// machine, and the two would only disagree on the machine somebody is trying to diagnose.
//
// So the decode lives here once and both ask it.

// Availability is the provider's own answer to "can you work right now".
//
// Reason is EMPTY whenever Available is true. A reason is the answer to "why not", and a ready
// provider explaining itself is a contradiction — a surface rendering "ready — the bridge is
// missing" teaches a person not to believe the word ready. The one construction site below
// enforces that rather than leaving it to whoever renders it.
type Availability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// AskAvailability puts the provider's own Available question over the host boundary.
//
// # This SPAWNS the bridge, and that is the point
//
// `bridgehost` launches its subprocess on first use, so asking costs a process start on a machine
// where none is running. That is not an accident being tolerated: the question is "can this
// provider act", and the only honest way to answer it is to reach the provider. A path check
// answers a different question — whether a file exists — and a file that exists and cannot run is
// exactly the case that used to reach a person as a play that silently did nothing.
//
// A launch failure is not an error here. Every negative answer means one thing to a caller — build
// a degraded world and say so — and the sentence that came back is kept as the reason so it can be
// repeated to somebody who has to fix it.
func AskAvailability(ctx context.Context, host runtime.Host) Availability {
	if host == nil {
		return Availability{Reason: "no accessibility provider is wired into this process"}
	}
	status, data, err := host.Invoke(runtime.HostCall{
		Act:    accessibilityAct,
		Action: "Available",
		Input:  runtime.Absent(),
		Out:    io.Discard,
		Ctx:    ctx,
	})
	if err != nil {
		return Availability{Reason: err.Error()}
	}
	if status == "failed" {
		// The bridge's own sentence — usually the operating system's refusal to start
		// the binary, which names the path it could not run.
		return Availability{Reason: errText(data)}
	}
	var reply struct {
		Available bool   `json:"Available"`
		Reason    string `json:"Reason"`
	}
	if decodeErr := decode(data, &reply); decodeErr != nil {
		return Availability{Reason: "the accessibility provider's answer could not be read: " +
			decodeErr.Error()}
	}
	if !reply.Available {
		if reply.Reason == "" {
			reply.Reason = "the accessibility provider says it cannot act, and did not say why"
		}
		return Availability{Reason: reply.Reason}
	}
	return Availability{Available: true}
}
