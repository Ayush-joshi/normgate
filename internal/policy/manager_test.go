package policy

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "normgate.dev/normgate/internal/contracts/v1"
)

type stubEngine struct {
	unhealthy  bool
	compileErr error
	explainErr error
	outcome    string
	onCompile  func()
}

func (s *stubEngine) Compile(ctx context.Context, b Bundle) (Revision, error) {
	if s.onCompile != nil {
		s.onCompile()
	}
	if s.compileErr != nil {
		return "", s.compileErr
	}
	_, r, err := Prepare(b)
	return r.Revision, err
}
func (s *stubEngine) Explain(ctx context.Context, r Revision, event v1.EventEnvelope) (Explanation, error) {
	if s.explainErr != nil {
		return Explanation{}, s.explainErr
	}
	outcome := s.outcome
	if outcome == "" {
		outcome = "allow"
	}
	return ExplainResult(r, event, []Contribution{{Layer: "baseline", RuleID: "baseline_allow", Outcome: outcome, ReasonCodes: []string{"ng.policy.baseline_allow"}, Obligations: []v1.Obligation{}}})
}
func (s *stubEngine) Evaluate(ctx context.Context, r Revision, e v1.EventEnvelope) (v1.Decision, error) {
	x, err := s.Explain(ctx, r, e)
	return x.Decision, err
}
func (s *stubEngine) Health(context.Context) Health { return Health{Ready: !s.unhealthy} }
func (s *stubEngine) Close() error                  { return nil }
func simpleBundle(t *testing.T) Bundle {
	b := testBundle(t)
	for name := range b.Files {
		if len(name) > 6 && name[:6] == "tests/" && name != "tests/registered_identity.yaml" {
			delete(b.Files, name)
		}
	}
	return b
}

type stubSource struct {
	result FetchResult
	err    error
	calls  int
	fn     func()
}

func (s *stubSource) ID() string { return "stub" }
func (s *stubSource) Fetch(context.Context, string) (FetchResult, error) {
	s.calls++
	if s.fn != nil {
		s.fn()
	}
	return s.result, s.err
}
func TestManagerFailuresAndRefresh(t *testing.T) {
	ctx := context.Background()
	e := &stubEngine{}
	m, err := NewManager(e, t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if m.Status().Ready {
		t.Fatal("empty ready")
	}
	if _, err = m.Evaluate(ctx, testEvent(t)); err == nil {
		t.Fatal("empty evaluation")
	}
	b := simpleBundle(t)
	raw, _ := Build(b)
	source := &stubSource{result: FetchResult{Bytes: raw, ETag: "one"}}
	if err = m.Refresh(ctx, source); err != nil {
		t.Fatal(err)
	}
	first := m.Status().Revision
	now := m.Status().RefreshedAt
	m.now = func() time.Time { return now.Add(time.Hour) }
	if m.Status().Ready {
		t.Fatal("stale ready")
	}
	if _, err = m.Evaluate(ctx, testEvent(t)); err == nil {
		t.Fatal("stale evaluation")
	}
	source.result = FetchResult{NotModified: true, ETag: "one"}
	if err = m.Refresh(ctx, source); err != nil || !m.Status().Ready {
		t.Fatal("304 refresh", err)
	}
	e.unhealthy = true
	if err = m.Activate(ctx, raw, "stub", "one"); err == nil || m.Status().Revision != first {
		t.Fatal("unhealthy candidate activated")
	}
	e.unhealthy = false
	e.compileErr = Failure("compile_error")
	if m.Activate(ctx, raw, "stub", "") == nil {
		t.Fatal("compile failure")
	}
	e.compileErr = nil
	e.outcome = "deny"
	if m.Activate(ctx, raw, "stub", "") == nil {
		t.Fatal("failing tests")
	}
	e.outcome = ""
	source.err = errors.New("private remote content")
	if m.Refresh(ctx, source) == nil || m.Status().LastError != "ng.policy.activation_failed" {
		t.Fatal("error not redacted")
	}
	source.err = nil
	if m.Refresh(ctx, nil) == nil {
		t.Fatal("nil source")
	}
	empty, _ := NewManager(e, t.TempDir(), time.Hour)
	if empty.Refresh(ctx, source) == nil {
		t.Fatal("304 without active")
	}
	source.result = FetchResult{Bytes: raw, ETag: "two"}
	if m.Refresh(ctx, source) != nil {
		t.Fatal("same revision refresh")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	e.onCompile = cancel
	if m.Activate(cancelCtx, raw, "stub", "") == nil {
		t.Fatal("cancellation during activation")
	}
	e.onCompile = nil
	source.result = FetchResult{NotModified: true}
	if m.Refresh(cancelCtx, source) == nil {
		t.Fatal("canceled 304")
	}
	if m.Rollback(ctx, Revision("sha256:"+string(make([]byte, 64)))) == nil {
		t.Fatal("unknown revision")
	}
	// A directory where the state file must be written causes persistence to fail
	// without replacing the atomic active pointer.
	os.Remove(filepath.Join(m.dir, "state.json"))
	os.Mkdir(filepath.Join(m.dir, "state.json"), 0700)
	if m.Activate(ctx, raw, "stub", "") == nil {
		t.Fatal("persistence failure")
	}
	if m.Refresh(ctx, source) == nil {
		t.Fatal("304 persistence failure")
	}
	if m.Status().Revision != first {
		t.Fatal("failed commit changed active")
	}
}
func TestManagerRecovery(t *testing.T) {
	e := &stubEngine{}
	ctx := context.Background()
	dir := t.TempDir()
	m, _ := NewManager(e, dir, time.Hour)
	b := simpleBundle(t)
	raw, _ := Build(b)
	if m.Activate(ctx, raw, "stub", "") != nil {
		t.Fatal("activate")
	}
	first := m.Status().Revision
	b.Files["data.json"] = append(b.Files["data.json"], ' ')
	next, _ := Build(b)
	if m.Activate(ctx, next, "stub", "") != nil {
		t.Fatal("second")
	}
	second := m.Status().Revision
	os.WriteFile(m.artifactPath(second), []byte("corruption"), 0600)
	recovered, err := NewManager(e, dir, time.Hour)
	if err != nil || recovered.Status().Revision != first || recovered.Status().LastError != "ng.policy.recovered_previous" {
		t.Fatal("fallback", err)
	}
	if err = recovered.Rollback(ctx, second); err == nil {
		t.Fatal("corrupt rollback")
	}
	if err = recovered.Rollback(ctx, Revision("sha256:"+repeatZero(64))); err == nil {
		t.Fatal("missing rollback")
	}
	os.Remove(m.artifactPath(first))
	if _, err = NewManager(e, dir, time.Hour); err == nil {
		t.Fatal("missing recovery")
	}
	for _, state := range []string{`{}`, `invalid`, `{"revision":"bad"}`} {
		os.WriteFile(filepath.Join(dir, "state.json"), []byte(state), 0600)
		if _, err = NewManager(e, dir, time.Hour); err == nil {
			t.Fatal("bad state")
		}
	}
	if _, err = NewManager(nil, dir, time.Hour); err == nil {
		t.Fatal("nil engine")
	}
	if _, err = NewManager(e, dir, 0); err == nil {
		t.Fatal("bad staleness")
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0600)
	if _, err = NewManager(e, filepath.Join(file, "child"), time.Hour); err == nil {
		t.Fatal("unwritable directory")
	}
	stateDir := t.TempDir()
	os.Mkdir(filepath.Join(stateDir, "state.json"), 0700)
	if _, err = NewManager(e, stateDir, time.Hour); err == nil {
		t.Fatal("state directory")
	}
	if atomicWrite(filepath.Join(file, "child"), nil) == nil {
		t.Fatal("atomic write invalid parent")
	}
	if atomicWrite(t.TempDir(), nil) == nil {
		t.Fatal("rename over directory")
	}
}
func repeatZero(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}
func TestRefreshLoop(t *testing.T) {
	m, _ := NewManager(&stubEngine{}, t.TempDir(), time.Hour)
	if m.Run(context.Background(), nil, 0, time.Second) == nil {
		t.Fatal("invalid interval")
	}
	ctx, cancel := context.WithCancel(context.Background())
	source := &stubSource{err: Failure("source_unavailable")}
	source.fn = func() {
		if source.calls == 4 {
			cancel()
		}
	}
	if !errors.Is(m.Run(ctx, source, time.Millisecond, 2*time.Millisecond), context.Canceled) {
		t.Fatal("loop cancellation")
	}
	if source.calls != 4 {
		t.Fatal(source.calls)
	}
	ctx, cancel = context.WithCancel(context.Background())
	raw, _ := Build(simpleBundle(t))
	source = &stubSource{result: FetchResult{Bytes: raw, ETag: "one"}}
	timer := time.AfterFunc(20*time.Millisecond, cancel)
	defer timer.Stop()
	if !errors.Is(m.Run(ctx, source, time.Hour, time.Hour), context.Canceled) {
		t.Fatal("timer cancellation")
	}
	for range 10 {
		if n := jitter(time.Second); n < 750*time.Millisecond || n > time.Second {
			t.Fatal(n)
		}
	}
}
func TestScenarioExecutionFailures(t *testing.T) {
	ctx := context.Background()
	e := &stubEngine{}
	b := simpleBundle(t)
	_, r, _ := Prepare(b)
	if report, err := RunTests(ctx, e, r.Revision, b); err != nil || report.Passed != 1 {
		t.Fatal(report, err)
	}
	b.Files["tests/duplicate.yaml"] = b.Files["tests/registered_identity.yaml"]
	if _, err := RunTests(ctx, e, r.Revision, b); err == nil {
		t.Fatal("duplicate id")
	}
	delete(b.Files, "tests/duplicate.yaml")
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := RunTests(canceled, e, r.Revision, b); err == nil {
		t.Fatal("canceled tests")
	}
	scenario, _ := ParseScenario(b.Files["tests/registered_identity.yaml"])
	scenario.ExpectedError = "ng.policy.invalid_event"
	b.Files["tests/registered_identity.yaml"], _ = json.Marshal(scenario)
	if _, err := RunTests(ctx, e, r.Revision, b); err == nil {
		t.Fatal("expected error missing")
	}
	e.explainErr = Failure("invalid_event")
	if _, err := RunTests(ctx, e, r.Revision, b); err != nil {
		t.Fatal(err)
	}
	b.Files["tests/registered_identity.yaml"] = []byte("bad")
	if _, err := RunTests(ctx, e, r.Revision, b); err == nil {
		t.Fatal("malformed scenario")
	}
	delete(b.Files, "tests/registered_identity.yaml")
	if _, err := RunTests(ctx, e, r.Revision, b); err == nil {
		t.Fatal("no tests")
	}
}
