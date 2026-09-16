package opa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/policy"
)

func bundle(t testing.TB) policy.Bundle {
	t.Helper()
	b, e := policy.LoadDir("../../../policies/baseline")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func event(t testing.TB) v1.EventEnvelope {
	t.Helper()
	raw, e := os.ReadFile("../../../testkit/fixtures/contracts/valid/event-envelope.basic.json")
	if e != nil {
		t.Fatal(e)
	}
	var wrapper struct {
		Value v1.EventEnvelope `json:"value"`
	}
	if e = json.Unmarshal(raw, &wrapper); e != nil {
		t.Fatal(e)
	}
	return wrapper.Value
}
func TestEngine(t *testing.T) {
	e := New()
	defer e.Close()
	ctx := context.Background()
	b := bundle(t)
	r, err := e.Compile(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := e.Compile(ctx, b)
	if err != nil || r != r2 {
		t.Fatal("revision cache", err)
	}
	ev := event(t)
	a, err := e.Explain(ctx, r, ev)
	if err != nil || a.Decision.Outcome != "allow" || len(a.Rules) == 0 {
		t.Fatalf("%+v %v", a, err)
	}
	again, err := e.Explain(ctx, r, ev)
	if err != nil || !equal(a, again) {
		t.Fatal("not replayable", err)
	}
	var wg sync.WaitGroup
	var successes atomic.Int64
	for range 16 {
		wg.Go(func() {
			for range 10 {
				decision, err := e.Evaluate(ctx, r, ev)
				if err != nil {
					if !errors.Is(err, context.DeadlineExceeded) || decision.Outcome != "" || decision.Id != "" {
						t.Errorf("unsafe or unexpected error: %+v %v", decision, err)
					}
					continue
				}
				if decision.Outcome != "allow" || decision.PolicyRevision != string(r) {
					t.Errorf("inconsistent concurrent decision: %+v", decision)
				}
				successes.Add(1)
			}
		})
	}
	wg.Wait()
	if successes.Load() == 0 {
		t.Fatal("no concurrent evaluations completed")
	}
	ev.Principal.Id = ""
	if _, err = e.Evaluate(ctx, r, ev); err == nil {
		t.Fatal("invalid event accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = e.Evaluate(canceled, r, event(t)); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, err = e.Compile(canceled, b); err == nil {
		t.Fatal("compile cancellation ignored")
	}
	if _, err = e.Evaluate(ctx, "missing", event(t)); err == nil {
		t.Fatal("unknown revision")
	}
	if !e.Health(ctx).Ready {
		t.Fatal("not healthy")
	}
	e.Close()
	e.Close()
	if e.Health(ctx).Ready {
		t.Fatal("closed engine healthy")
	}
	if _, err = e.Compile(ctx, b); err == nil {
		t.Fatal("compiled after close")
	}
}
func equal(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func TestForbiddenCapabilities(t *testing.T) {
	for _, expr := range []string{`time.now_ns()`, `http.send({"method":"GET","url":"http://localhost/secret"})`, `rand.intn("seed", 4)`, `opa.runtime()`, `uuid.rfc4122("seed")`, `net.lookup_ip_addr("localhost")`} {
		t.Run(expr, func(t *testing.T) {
			b := bundle(t)
			b.Files["baseline.rego"] = []byte("package normgate.baseline\ndefault decisions := []\ndecisions := [] if { x := " + expr + "; x != null }\n")
			e := New()
			defer e.Close()
			if _, err := e.Compile(context.Background(), b); err == nil {
				t.Fatal("unsafe builtin allowed")
			}
		})
	}
}
func TestCompileErrorsRedacted(t *testing.T) {
	b := bundle(t)
	b.Files["baseline.rego"] = []byte("package normgate.baseline\npassword := SECRET_PRIVATE_DATA !!!\n")
	e := New()
	defer e.Close()
	_, err := e.Compile(context.Background(), b)
	if err == nil || strings.Contains(err.Error(), "SECRET_PRIVATE_DATA") || !strings.Contains(err.Error(), "baseline.rego:") {
		t.Fatal(err)
	}
}
func TestEvaluationDeadline(t *testing.T) {
	e := New()
	defer e.Close()
	r, err := e.Compile(context.Background(), bundle(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err = e.Evaluate(ctx, r, event(t)); err == nil {
		t.Fatal("deadline ignored")
	}
}
func BenchmarkColdCompile(b *testing.B) {
	source := bundle(b)
	b.ReportAllocs()
	for b.Loop() {
		e := New()
		if _, err := e.Compile(context.Background(), source); err != nil {
			b.Fatal(err)
		}
		e.Close()
	}
}
func BenchmarkCachedEvaluation(b *testing.B) {
	e := New()
	defer e.Close()
	r, err := e.Compile(context.Background(), bundle(b))
	if err != nil {
		b.Fatal(err)
	}
	ev := event(b)
	samples := make([]time.Duration, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		if _, err = e.Evaluate(context.Background(), r, ev); err != nil {
			b.Fatal(err)
		}
		samples = append(samples, time.Since(start))
	}
	b.StopTimer()
	reportPercentiles(b, samples)
}
func reportPercentiles(b *testing.B, samples []time.Duration) {
	slices.Sort(samples)
	if len(samples) > 0 {
		for _, p := range []int{50, 95, 99} {
			b.ReportMetric(float64(samples[(len(samples)-1)*p/100].Nanoseconds()), fmt.Sprintf("p%d-ns", p))
		}
	}
}
func TestBaselineScenarios(t *testing.T) {
	e := New()
	defer e.Close()
	b := bundle(t)
	r, err := e.Compile(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	report, err := policy.RunTests(context.Background(), e, r, b)
	if err != nil {
		t.Fatalf("%+v %v", report, err)
	}
}

func BenchmarkBaselineFixtureSet(b *testing.B) {
	e := New()
	defer e.Close()
	source := bundle(b)
	r, err := e.Compile(context.Background(), source)
	if err != nil {
		b.Fatal(err)
	}
	names := []string{}
	for name := range source.Files {
		if strings.HasPrefix(name, "tests/") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	cases := []policy.Scenario{}
	for _, name := range names {
		scenario, err := policy.ParseScenario(source.Files[name])
		if err != nil {
			b.Fatal(err)
		}
		cases = append(cases, scenario)
	}
	samples := make([]time.Duration, 0, b.N)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scenario := cases[i%len(cases)]
		start := time.Now()
		d, err := e.Evaluate(context.Background(), r, scenario.Event)
		samples = append(samples, time.Since(start))
		if scenario.ExpectedError == "" && (err != nil || d.Outcome != scenario.Expected.Outcome) {
			b.Fatal(d, err)
		}
	}
	b.StopTimer()
	reportPercentiles(b, samples)
}
