package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Service discovery.
//
// A client needs to answer three questions before it can do anything: is a service
// running, where is it, and may I talk to it. All three are answered by one small
// file that the service writes on startup and removes on exit.
//
// The file is the ONLY source of truth about the port and the token, and it is
// validated rather than trusted: a service that was killed leaves the file behind,
// and a stale file must not stop the next one starting. So a client that finds one
// checks it by connecting, not by believing it.

// Endpoint is what the service publishes so clients can find it.
type Endpoint struct {
	ProtocolVersion int       `json:"protocol_version"`
	Address         string    `json:"address"`
	Token           string    `json:"token"`
	PID             int       `json:"pid"`
	StartedAt       time.Time `json:"started_at"`
}

// EndpointPath is where the endpoint file lives, alongside the action graph.
func EndpointPath(dir string) string { return filepath.Join(dir, "director-service.json") }

// lockPath is the startup mutex.
func lockPath(dir string) string { return filepath.Join(dir, "director-service.lock") }

// NewToken mints a fresh connection token.
//
// 256 bits from crypto/rand, per service start. It is not a secret from a process
// already running as this user — that process could read the file — but it does stop
// anything that merely stumbles onto the port from being able to drive the desktop.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("service: generating a token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// WriteEndpoint publishes the endpoint file with restrictive permissions.
func WriteEndpoint(dir string, ep Endpoint) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return err
	}
	path := EndpointPath(dir)
	tmp := path + ".tmp"
	// 0600: the token is in here.
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadEndpoint loads the endpoint file, if there is one.
func ReadEndpoint(dir string) (Endpoint, bool) {
	data, err := os.ReadFile(EndpointPath(dir))
	if err != nil {
		return Endpoint{}, false
	}
	var ep Endpoint
	if json.Unmarshal(data, &ep) != nil {
		return Endpoint{}, false
	}
	if ep.Address == "" || ep.Token == "" {
		return Endpoint{}, false
	}
	return ep, true
}

// RemoveEndpoint deletes the endpoint file.
func RemoveEndpoint(dir string) { _ = os.Remove(EndpointPath(dir)) }

// Reachable reports whether a service is actually answering at this endpoint.
//
// It connects and pings rather than checking whether the PID exists. A PID can be
// recycled, and a process can be alive but wedged; the only question that matters is
// whether it will answer, so that is the question asked.
func Reachable(ep Endpoint, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", ep.Address, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	c := &Client{conn: conn, token: ep.Token}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if err := c.handshake(); err != nil {
		return false
	}
	resp, err := c.roundTrip(RequestPing, nil)
	return err == nil && resp.Type == ResponsePong
}

// ── startup locking ───────────────────────────────────────────────────────────

// startupLock is held while a client is starting the service, so two clients
// arriving at once produce one service rather than two.
type startupLock struct{ path string }

// acquireStartupLock takes the lock, breaking a stale one.
//
// Staleness is by age rather than by PID, deliberately: a lock is held for the few
// seconds a service takes to come up, so anything older than that was abandoned by a
// process that died mid-start. Checking a PID would be more precise and would also
// reintroduce the recycled-PID problem this design is trying to avoid.
func acquireStartupLock(dir string, staleAfter time.Duration) (*startupLock, error) {
	path := lockPath(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().Format(time.RFC3339))
			_ = f.Close()
			return &startupLock{path: path}, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		info, statErr := os.Stat(path)
		if statErr != nil || time.Since(info.ModTime()) > staleAfter {
			// Abandoned. Breaking it is safe: the worst case is two starters, and
			// the endpoint re-check below means only one service survives.
			_ = os.Remove(path)
			continue
		}
		return nil, errLockHeld
	}
	return nil, errLockHeld
}

func (l *startupLock) release() {
	if l != nil {
		_ = os.Remove(l.path)
	}
}

// errLockHeld means another process is starting the service right now.
var errLockHeld = fmt.Errorf("service: another process is starting the Director")

// ── stop-file compatibility ───────────────────────────────────────────────────

// DeprecatedStopPath is the pre-service cancellation mechanism: a timestamp file
// polled between replay iterations.
//
// It is retained only so a build of the CLI from before this milestone can still
// stop a replay. The service does not depend on it — cancellation is now a request
// on the wire, delivered to the running command's context — and it is checked as a
// fallback rather than as the mechanism.
//
// Deprecated: use RequestCancelActive. This will be removed once no shipped client
// relies on it.
func DeprecatedStopPath(dir string) string { return filepath.Join(dir, "director-stop") }

// DeprecatedStopCheck returns a check for the legacy stop file, honouring only
// requests made after the given moment so a stale file cannot cancel a new command.
//
// Deprecated: kept behind this adapter so exactly one place has to be deleted later.
func DeprecatedStopCheck(dir string, since time.Time) func() bool {
	path := DeprecatedStopPath(dir)
	return func() bool {
		info, err := os.Stat(path)
		if err != nil {
			return false
		}
		return info.ModTime().After(since)
	}
}
