package opa

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/policy"
)

// The fake defines the consumer-facing contract without depending on Rego types.
type fakeEngine struct {
	closed  bool
	bundles map[policy.Revision]policy.Bundle
}

func (f *fakeEngine) Compile(ctx context.Context, b policy.Bundle) (policy.Revision, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if f.closed {
		return "", policy.Failure("closed")
	}
	_, r, err := policy.Prepare(b)
	if err != nil {
		return "", err
	}
	f.bundles[r.Revision] = b
	return r.Revision, nil
}
func (f *fakeEngine) Explain(ctx context.Context, r policy.Revision, e v1.EventEnvelope) (policy.Explanation, error) {
	if ctx.Err() != nil {
		return policy.Explanation{}, ctx.Err()
	}
	if f.closed {
		return policy.Explanation{}, policy.Failure("closed")
	}
	if _, ok := f.bundles[r]; !ok {
		return policy.Explanation{}, policy.Failure("unknown_revision")
	}
	return policy.ExplainResult(r, e, []policy.Contribution{{Layer: "baseline", RuleID: "baseline_allow", Outcome: "allow", ReasonCodes: []string{"ng.policy.baseline_allow"}, Obligations: []v1.Obligation{}}})
}
func (f *fakeEngine) Evaluate(ctx context.Context, r policy.Revision, e v1.EventEnvelope) (v1.Decision, error) {
	x, err := f.Explain(ctx, r, e)
	return x.Decision, err
}
func (f *fakeEngine) Health(ctx context.Context) policy.Health {
	return policy.Health{Ready: !f.closed && ctx.Err() == nil, Revisions: len(f.bundles)}
}
func (f *fakeEngine) Close() error { f.closed = true; return nil }
func TestConformance(t *testing.T) {
	for name, factory := range map[string]func() policy.Engine{"fake": func() policy.Engine { return &fakeEngine{bundles: map[policy.Revision]policy.Bundle{}} }, "opa": func() policy.Engine { return New() }} {
		t.Run(name, func(t *testing.T) {
			engine := factory()
			defer engine.Close()
			ctx := context.Background()
			b := bundle(t)
			r, err := engine.Compile(ctx, b)
			if err != nil || !strings.HasPrefix(string(r), "sha256:") {
				t.Fatal(r, err)
			}
			r2, err := engine.Compile(ctx, b)
			if err != nil || r != r2 {
				t.Fatal("revision instability")
			}
			ex, err := engine.Explain(ctx, r, event(t))
			if err != nil || ex.Decision.Outcome != "allow" || len(ex.Rules) != 1 {
				t.Fatal(ex, err)
			}
			ev := event(t)
			ev.SchemaVersion = "2.0"
			if _, err = engine.Evaluate(ctx, r, ev); err == nil {
				t.Fatal("noncanonical event")
			}
			c, cancel := context.WithCancel(ctx)
			cancel()
			if _, err = engine.Evaluate(c, r, event(t)); err == nil {
				t.Fatal("cancellation")
			}
			if _, err = engine.Compile(c, b); err == nil {
				t.Fatal("compile cancellation")
			}
			if !engine.Health(ctx).Ready {
				t.Fatal("health")
			}
			engine.Close()
			engine.Close()
			if engine.Health(ctx).Ready {
				t.Fatal("close not terminal")
			}
		})
	}
}
func rewrite(b policy.Bundle, text string) policy.Bundle {
	b.Files["baseline.rego"] = []byte("package normgate.baseline\ndefault decisions := []\n" + text + "\n")
	return b
}
func TestCompileBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		edit         func(policy.Bundle)
	}{
		{"unreachable", `orphan := true`, nil},
		{"invalid obligation", `decisions := [{"rule_id":"x","outcome":"transform","reason_codes":["ng.policy.baseline_allow"],"obligations":[{"schema_version":"1.0","id":"x","kind":"unknown"}]}]`, nil},
		{"unknown reason", `decisions := [{"rule_id":"x","outcome":"deny","reason_codes":["ng.policy.unknown"],"obligations":[]}]`, nil},
		{"undeclared facts", `decisions := [] if { count(input.external_facts) > 0 }`, nil},
		{"print", `decisions := [] if { print("private"); true }`, nil},
		{"wrong namespace", "", func(b policy.Bundle) { b.Files["baseline.rego"] = []byte("package wrong\ndefault decisions := []\n") }},
		{"missing default", "", func(b policy.Bundle) {
			b.Files["baseline.rego"] = []byte("package normgate.baseline\ndecisions := []\n")
		}},
		{"depth", `decisions := ` + strings.Repeat("[", 65) + strings.Repeat("]", 65), nil},
		{"recursive", `a := b\nb := a`, nil},
		{"invalid bundle", "", func(b policy.Bundle) { delete(b.Files, "manifest.json") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New()
			defer e.Close()
			b := rewrite(bundle(t), tc.source)
			if tc.edit != nil {
				tc.edit(b)
			}
			if _, err := e.Compile(context.Background(), b); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	e := New()
	defer e.Close()
	for i := 0; i < MaxRevisions; i++ {
		e.revisions[policy.Revision(string(rune(i)))] = nil
	}
	if _, err := e.Compile(context.Background(), bundle(t)); err == nil {
		t.Fatal("unbounded revisions")
	}
	if sanitized(context.Canceled) == nil {
		t.Fatal("sanitize")
	}
	if !boundedSyntax([]byte("# [\n\"escaped \\\" [\" `[`")) {
		t.Fatal("string nesting")
	}
}
func TestOutputBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		want             string
	}{
		{"wrong type", `true`, "error"},
		{"unknown field", `[{"secret":"private"}]`, "error"},
		{"null", `null`, "deny"},
		{"missing layer", `[]`, "deny"},
		{"unknown outcome", `[{"rule_id":"x","outcome":"future","reason_codes":["ng.policy.baseline_allow"],"obligations":[]}]`, "deny"},
		{"supplied layer", `[{"layer":"tenant","rule_id":"x","outcome":"allow","reason_codes":["ng.policy.baseline_allow"],"obligations":[]}]`, "error"},
		{"runtime reason", `[{"rule_id":"x","outcome":"deny","reason_codes":[concat("",["ng", ".policy.unknown"])],"obligations":[]}]`, "error"},
		{"builtin error", `[1 / 0]`, "error"},
		{"approval", `[{"rule_id":"x","outcome":"require_approval","reason_codes":["ng.policy.baseline_allow"],"obligations":[{"schema_version":"1.0","id":"approval","kind":"require_approval","reviewer_group":"owners"}]}]`, "require_approval"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := New()
			defer e.Close()
			b := rewrite(bundle(t), "decisions := "+tc.expression)
			r, err := e.Compile(context.Background(), b)
			if err != nil {
				t.Fatal(err)
			}
			ex, err := e.Explain(context.Background(), r, event(t))
			if tc.want == "error" {
				if err == nil {
					t.Fatal("accepted invalid output")
				}
			} else if err != nil || ex.Decision.Outcome != tc.want {
				t.Fatal(ex, err)
			}
		})
	}
	e := New()
	b := bundle(t)
	var m v1.PolicyManifest
	json.Unmarshal(b.Files["manifest.json"], &m)
	m.ExternalFactSources = []string{"required"}
	b.Files["manifest.json"], _ = json.Marshal(m)
	r, err := e.Compile(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate(context.Background(), r, event(t))
	if err != nil || d.Outcome != "deny" || d.ReasonCodes[0] != "ng.policy.missing_fact" {
		t.Fatal(d, err)
	}
	e.Close()
	if _, err = e.Explain(context.Background(), r, event(t)); err == nil {
		t.Fatal("closed explain")
	}
}

func TestLayerCompositionIntegration(t *testing.T) {
	for _, tc := range []struct{ name, decision, want string }{
		{"deny", `[{"rule_id":"tenant_deny","outcome":"deny","reason_codes":["ng.policy.baseline_allow"],"obligations":[]}]`, "deny"},
		{"approval", `[{"rule_id":"tenant_approval","outcome":"require_approval","reason_codes":["ng.policy.baseline_allow"],"obligations":[{"schema_version":"1.0","id":"approval","kind":"require_approval","reviewer_group":"owners"}]}]`, "require_approval"},
		{"missing", `[]`, "deny"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := bundle(t)
			var m v1.PolicyManifest
			json.Unmarshal(b.Files["manifest.json"], &m)
			m.Namespaces = append(m.Namespaces, "example.tenant")
			b.Files["manifest.json"], _ = json.Marshal(m)
			b.Files["layers.json"] = []byte(`[{"name":"tenant","namespace":"example.tenant","priority":20},{"name":"baseline","namespace":"normgate.baseline","priority":0}]`)
			b.Files["tenant.rego"] = []byte("package example.tenant\ndefault decisions := []\ndecisions := " + tc.decision + "\n")
			e := New()
			defer e.Close()
			r, err := e.Compile(context.Background(), b)
			if err != nil {
				t.Fatal(err)
			}
			ex, err := e.Explain(context.Background(), r, event(t))
			if err != nil || ex.Decision.Outcome != tc.want || len(ex.Rules) != 2 {
				t.Fatal(ex, err)
			}
			if tc.want == "require_approval" && len(ex.ObligationProvenance["approval"]) != 1 {
				t.Fatal("missing provenance")
			}
		})
	}
}
func TestCanonicalReplayAndFactIsolation(t *testing.T) {
	e := New()
	defer e.Close()
	b := bundle(t)
	r, err := e.Compile(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	a := event(t)
	a.Principal.Roles = []string{"z", "a"}
	one, err := e.Evaluate(context.Background(), r, a)
	if err != nil {
		t.Fatal(err)
	}
	a.Principal.Roles = []string{"a", "z"}
	a.OccurredAt = "2026-09-15T05:30:00+05:30"
	two, err := e.Evaluate(context.Background(), r, a)
	if err != nil || !equal(one, two) {
		t.Fatal("normalization replay changed", err)
	}
	// Mutating source bytes after compile cannot change a prepared revision.
	b.Files["data.json"] = []byte(`{}`)
	again, err := e.Evaluate(context.Background(), r, a)
	if err != nil || !equal(one, again) {
		t.Fatal("mutable compiled policy")
	}
	b = rewrite(bundle(t), `decisions := [{"rule_id":"x","outcome":"allow","reason_codes":["ng.policy.baseline_allow"],"obligations":[]}] if { x := input; count(x["external_facts"]) == 0 }`)
	// Declare a different source to permit fact access, but keep it absent below:
	// a missing declared fact denies even if this rule would otherwise allow.
	var m v1.PolicyManifest
	json.Unmarshal(b.Files["manifest.json"], &m)
	m.ExternalFactSources = []string{"other"}
	b.Files["manifest.json"], _ = json.Marshal(m)
	r, err = e.Compile(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	d, err := e.Evaluate(context.Background(), r, event(t))
	if err != nil || d.Outcome != "deny" {
		t.Fatal("missing declared fact", err)
	}
}
func TestActiveEvaluationCancellation(t *testing.T) {
	b := rewrite(bundle(t), `decisions := [ {"rule_id": concat("", [a,b,c]), "outcome":"allow", "reason_codes":["ng.policy.baseline_allow"],"obligations":[]} | some a in input.operation.data_refs; some b in input.operation.data_refs; some c in input.operation.data_refs ]`)
	e := New()
	defer e.Close()
	r, err := e.Compile(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	ev := event(t)
	ev.Operation.DataRefs = []string{}
	for i := 0; i < 200; i++ {
		ev.Operation.DataRefs = append(ev.Operation.DataRefs, fmt.Sprint(i))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err = e.Evaluate(ctx, r, ev); err == nil {
		t.Fatal("unbounded evaluation")
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation did not interrupt evaluation")
	}
}
