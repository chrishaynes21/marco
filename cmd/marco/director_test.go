package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The front-end contract.
//
// The overlay dispatches this as a PROCESS, so what it can act on is exactly two
// things: the lines on stdout and the exit code. Both are tested here against a real
// service, because a front-end that guessed wrong about either would either drop a
// spoken phrase or run it twice.

// stubRuntime is a Director that returns whatever the test asks for.
type stubRuntime struct {
	graph         *actiongraph.Memory
	outcome       execute.Outcome
	phrases       []string
	confirmations *service.ConfirmationBroker
}

func (s *stubRuntime) Handle(_ context.Context, phrase string, progress func(service.ProgressPayload)) execute.Outcome {
	s.phrases = append(s.phrases, phrase)
	if progress != nil {
		progress(service.ProgressPayload{Stage: "observe", Detail: "looking at the screen"})
	}
	return s.outcome
}

func (s *stubRuntime) HandleClarified(ctx context.Context, phrase string,
	_ intent.Refinement, progress func(service.ProgressPayload)) execute.Outcome {
	return s.Handle(ctx, phrase, progress)
}

func (s *stubRuntime) Perception() diagnostics.Perception { return diagnostics.Perception{} }

func (s *stubRuntime) Explanation() diagnostics.Perception { return diagnostics.Perception{} }

func (s *stubRuntime) LiveAnalysis(service.LiveAnalysisPayload) service.LiveAnalysisResponse {
	return service.LiveAnalysisResponse{}
}

func (s *stubRuntime) ObservationEvents(service.ObservationEventsPayload) service.ObservationEventsResponse {
	return service.ObservationEventsResponse{}
}

// Playbill is the Director half of the visibility account. A stub Director watches
// nothing, which is exactly what it is entitled to report — and the thin client has to
// render that as a normal condition rather than as an error.
func (s *stubRuntime) Playbill(service.PlaybillPayload) playbill.View {
	return playbill.View{
		Version: playbill.Version,
		Current: playbill.Current{Recognition: playbill.Unobservable},
		Learning: playbill.Learning{
			Stage: playbill.NotLearning,
		},
		Doing: playbill.Doing{Phase: playbill.NotDoing},
	}
}

func (s *stubRuntime) World(service.WorldPayload) service.WorldResponse {
	return service.WorldResponse{}
}

func (s *stubRuntime) Events(service.EventsPayload) service.EventsResponse {
	return service.EventsResponse{}
}

func (s *stubRuntime) ReadText(context.Context, *directorapi.Rect) ocr.Diagnostics {
	return ocr.Diagnostics{Engine: "stub"}
}

func (s *stubRuntime) ReadRegion(context.Context, *directorapi.Rect) visualstate.Diagnostics {
	return visualstate.Diagnostics{Provider: "stub"}
}

func (s *stubRuntime) ActiveWait() waitengine.Snapshot { return waitengine.Snapshot{} }

func (s *stubRuntime) Edits() []edit.Outcome { return nil }

// Confirmations gives the stub a real broker, which is the production shape: a nil one
// means "this Director cannot ask", and that must not be what a test accidentally covers.
func (s *stubRuntime) Confirmations() *service.ConfirmationBroker {
	if s.confirmations == nil {
		s.confirmations = service.NewConfirmationBroker()
	}
	return s.confirmations
}

func (s *stubRuntime) SemanticActions() []uiact.Outcome { return nil }

func (s *stubRuntime) Lowerings() []marcoexec.Result { return nil }

func (s *stubRuntime) TraceFor(string) *trace.Trace { return nil }

func (s *stubRuntime) RunOperation(context.Context, marcoexec.Operation) marcoexec.Result {
	return marcoexec.Result{}
}

func (s *stubRuntime) OCRUnavailable() string { return "" }

func (s *stubRuntime) Graph() actiongraph.Graph            { return s.graph }
func (s *stubRuntime) Providers() []service.ProviderStatus { return nil }
func (s *stubRuntime) AttachedAt() time.Time               { return time.Now() }

// serveStub runs a real service on a temp dir and points the CLI at it.
func serveStub(t *testing.T, out execute.Outcome) *stubRuntime {
	t.Helper()
	dir := t.TempDir()
	rt := &stubRuntime{graph: actiongraph.NewMemory(), outcome: out}
	srv := service.NewServer(service.Config{Dir: dir, Runtime: rt})
	if _, err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(srv.Shutdown)
	// The CLI finds the service exactly as it does in production: through the endpoint
	// file under MARCO_HOME. Nothing is injected.
	t.Setenv("MARCO_HOME", dir)
	return rt
}

// capture runs fn with stdout and stderr redirected, returning what was written.
func capture(t *testing.T, fn func()) (string, string) {
	t.Helper()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW

	done := make(chan [2]string, 1)
	go func() {
		var o, e strings.Builder
		oc := make(chan struct{})
		go func() { _, _ = copyAll(&o, outR); close(oc) }()
		_, _ = copyAll(&e, errR)
		<-oc
		done <- [2]string{o.String(), e.String()}
	}()

	fn()
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	res := <-done
	return res[0], res[1]
}

func copyAll(dst *strings.Builder, src *os.File) (int64, error) {
	buf := make([]byte, 4096)
	var n int64
	for {
		c, err := src.Read(buf)
		if c > 0 {
			dst.Write(buf[:c])
			n += int64(c)
		}
		if err != nil {
			return n, nil
		}
	}
}

func TestASubmittedPhraseRendersEveryStageItPassesThrough(t *testing.T) {
	serveStub(t, execute.Outcome{Status: directorapi.ResultDone, Message: "clicked File"})

	var code int
	stdout, _ := capture(t, func() { code = directorSubmit("click file", false) })

	if code != exitOK {
		t.Errorf("exit %d, want %d", code, exitOK)
	}
	// Heard first — a person who has just spoken needs the acknowledgement before the
	// result, not instead of it.
	for _, want := range []string{"heard: click file", "looking at the screen", "COMPLETED: clicked File"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, stdout)
		}
	}
	if i, j := strings.Index(stdout, "heard:"), strings.Index(stdout, "COMPLETED"); i > j {
		t.Errorf("the acknowledgement came after the result:\n%s", stdout)
	}
}

func TestAnUnverifiedResultIsNotReportedAsSuccess(t *testing.T) {
	// The distinction the whole verification story rests on. A front-end that read
	// exit 0 here would tell the user it worked when nobody knows that.
	serveStub(t, execute.Outcome{Status: directorapi.ResultPartial, Message: "could not confirm"})

	var code int
	stdout, _ := capture(t, func() { code = directorSubmit("click file", false) })

	if code == exitOK {
		t.Error("an unverified result exited 0")
	}
	if code == exitUnavailable {
		t.Error("an unverified result claimed the phrase was never delivered — it was")
	}
	if !strings.Contains(stdout, "UNVERIFIED") {
		t.Errorf("stdout did not name the state:\n%s", stdout)
	}
}

func TestAnUndeliveredPhraseIsReportedLoudlyAndNeverSilentlyDropped(t *testing.T) {
	// No service, and no way to start one. The phrase must come back out — a
	// recognised phrase that vanishes is the one failure a voice front-end cannot
	// recover from, because the user believes they were heard.
	t.Setenv("MARCO_HOME", t.TempDir())
	t.Setenv("DIRECTOR_BIN", "no-such-director-binary.exe")

	var code int
	start := time.Now()
	_, stderr := capture(t, func() { code = directorSubmit("click file", false) })
	elapsed := time.Since(start)

	if code != exitUnavailable {
		t.Errorf("exit %d, want %d — only this code tells a caller to fall back",
			code, exitUnavailable)
	}
	if !strings.Contains(stderr, "click file") {
		t.Errorf("the phrase was not reported back:\n%s", stderr)
	}
	if !strings.Contains(stderr, "not delivered") {
		t.Errorf("the failure did not say the phrase was undelivered:\n%s", stderr)
	}
	// Bounded. Auto-start is attempted twice at most, and a spoken phrase that hangs
	// for a minute waiting on a service that will never appear is its own failure.
	if elapsed > 90*time.Second {
		t.Errorf("took %s to give up", elapsed)
	}
}

func TestAClarificationIsRenderedAsAChoiceAndNotAsAFailure(t *testing.T) {
	res := &directorapi.Resolution{Status: directorapi.ResolutionAmbiguous}
	for i, label := range []string{"Save", "Save As"} {
		res.Candidates = append(res.Candidates, directorapi.TargetCandidate{
			ElementID: directorapi.ElementID("el"), Label: label,
			Role: directorapi.RoleButton, Score: 0.8 - float64(i)*0.1,
		})
	}
	// Contenders is what the clarification layer offers. A fixture that set only
	// Candidates would be building a Resolution the resolver cannot produce — see
	// Resolution.Contenders for why the two are no longer separate definitions.
	res.Contenders = append([]directorapi.TargetCandidate(nil), res.Candidates...)
	serveStub(t, execute.Outcome{
		Status: directorapi.ResultNeedsClarification, Message: "which Save did you mean?",
		Resolution: res,
	})

	var code int
	stdout, _ := capture(t, func() { code = directorSubmit("click save", false) })

	if !strings.Contains(stdout, "CLARIFICATION_REQUIRED: which Save did you mean?") {
		t.Errorf("the question was not rendered:\n%s", stdout)
	}
	// The numbering IS the vocabulary of the answer. Without it "the second one" has
	// nothing to refer to.
	if !strings.Contains(stdout, "1. Save") || !strings.Contains(stdout, "2. Save As") {
		t.Errorf("the candidates were not numbered:\n%s", stdout)
	}
	if code == exitUnavailable {
		t.Error("a question was reported as an undelivered phrase")
	}
}

func TestAControlPhraseCancelsInsteadOfBeingPlanned(t *testing.T) {
	rt := serveStub(t, execute.Outcome{Status: directorapi.ResultDone, Message: "done"})

	var code int
	_, _ = capture(t, func() { code = directorSubmit("stop that", false) })

	if code != exitOK {
		t.Errorf("exit %d, want %d", code, exitOK)
	}
	// The multi-word form does not hit the CLI's bare sub-command switch — it is
	// routed as a control phrase by the service-aware path, which is the same rule
	// applied in one place rather than two.
	for _, p := range rt.phrases {
		if strings.Contains(p, "stop") {
			t.Fatalf("%q reached the planner", p)
		}
	}
}

func (s *stubRuntime) ActiveValues() values.EnvironmentSnapshot {
	return values.EnvironmentSnapshot{}
}

func (s *stubRuntime) AbandonProgram(string) {}

func (s *stubRuntime) ActiveCollections() collections.Snapshot {
	return collections.Snapshot{}
}

// Windows is what the stub can see: nothing, which is what a Director with no
// observation is entitled to report.
func (s *stubRuntime) Windows() []directorapi.Window { return nil }

// The demonstration surface.
//
// This stub is the OVERLAY's view of a Director — `marco director` is a thin client — and
// nothing in the overlay records demonstrations. So these refuse rather than pretend:
// a stub that quietly returned an empty session would make a test pass against a Director
// that cannot record, which is the one thing worth knowing here.
func (s *stubRuntime) StartDemonstration() (*demo.Demonstration, error) {
	return nil, errNoDemonstrations
}
func (s *stubRuntime) StopDemonstration() (*demo.Demonstration, error) {
	return nil, errNoDemonstrations
}
func (s *stubRuntime) AbandonDemonstration(string) (*demo.Demonstration, error) {
	return nil, errNoDemonstrations
}
func (s *stubRuntime) ActiveDemonstration() *demo.Demonstration { return nil }
func (s *stubRuntime) Demonstrations() ([]*demo.Demonstration, error) {
	return nil, errNoDemonstrations
}
func (s *stubRuntime) Demonstration(demo.ID) (*demo.Demonstration, error) {
	return nil, errNoDemonstrations
}
func (s *stubRuntime) ExtractProcedure(demo.ID) (demo.Extraction, error) {
	return demo.Extraction{}, errNoDemonstrations
}
func (s *stubRuntime) ApproveProcedure(demo.ID, string) (*demo.Learned, error) {
	return nil, errNoDemonstrations
}
func (s *stubRuntime) ForgetProcedure(string) error       { return errNoDemonstrations }
func (s *stubRuntime) LearnedProcedures() []*demo.Learned { return nil }

var errNoDemonstrations = errors.New("this stub Director does not record demonstrations")

// The capability-pack surface. The overlay's view of a Director registers no packs, and
// says so rather than inventing an empty detection that looks like a real one.
func (s *stubRuntime) DetectedGame() game.Active { return game.Active{} }
func (s *stubRuntime) GameCapabilities() game.Report {
	return game.Report{}
}
func (s *stubRuntime) GameInventory(string) game.InventoryReport {
	return game.InventoryReport{Unavailable: "this stub Director registers no capability packs"}
}

// The vision surface. The overlay's view of a Director wires no detector, and says so
// rather than reporting an empty pass.
func (s *stubRuntime) ReadVision(context.Context, *directorapi.Rect, windowref.Selector) vision.Diagnostics {
	return vision.Diagnostics{
		Backend: "stub", Available: false,
		Unavailable: "this stub Director wires no vision detector",
	}
}
func (s *stubRuntime) LastVision() vision.Diagnostics {
	return s.ReadVision(nil, nil, windowref.Selector{})
}
func (s *stubRuntime) Frames() []vision.FrameRecord { return nil }
func (s *stubRuntime) VisionUnavailable() string {
	return "this stub Director wires no vision detector"
}

func (s *stubRuntime) LiveWindows(context.Context, string) []windowref.Listing { return nil }

func (s *stubRuntime) StartObservation(service.ObservePayload) (service.ObserveStarted, error) {
	return service.ObserveStarted{}, errors.New("this stub Director observes nothing")
}
func (s *stubRuntime) Observation(service.ObserveQuery) (any, error) {
	return nil, errors.New("this stub Director observes nothing")
}

func (r *stubRuntime) LearnedPlay(service.LearnedQuery) (service.LearnedView, error) {
	return service.LearnedView{}, nil
}
