// Package policy owns deterministic policy contracts and lifecycle; it has no OPA dependency.
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"normgate.dev/normgate/internal/contracts"
	v1 "normgate.dev/normgate/internal/contracts/v1"
	nerrors "normgate.dev/normgate/internal/errors"
	"normgate.dev/normgate/internal/normalization"
)

const CoreVersion = "0.2.0"

type Revision string

type Engine interface {
	Compile(context.Context, Bundle) (Revision, error)
	Evaluate(context.Context, Revision, v1.EventEnvelope) (v1.Decision, error)
	Explain(context.Context, Revision, v1.EventEnvelope) (Explanation, error)
	Health(context.Context) Health
	Close() error
}
type Health struct {
	Ready     bool   `json:"ready"`
	Revisions int    `json:"revisions"`
	Code      string `json:"code,omitempty"`
}
type Error struct {
	Code   string `json:"code"`
	File   string `json:"file,omitempty"`
	Row    int    `json:"row,omitempty"`
	Column int    `json:"column,omitempty"`
}

func (e *Error) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s (%s:%d:%d)", e.Code, e.File, e.Row, e.Column)
	}
	return e.Code
}

// Unwrap maps detailed engine diagnostics to stable, redacted transport categories.
func (e *Error) Unwrap() error {
	code := nerrors.Policy
	if e.Code == "ng.policy.invalid_event" {
		code = nerrors.Validation
	}
	return nerrors.New(code, nil)
}

func Failure(code string) error { return &Error{Code: "ng.policy." + code} }

type Contribution struct {
	Layer       string          `json:"layer"`
	RuleID      string          `json:"rule_id"`
	Outcome     string          `json:"outcome"`
	ReasonCodes []string        `json:"reason_codes"`
	Obligations []v1.Obligation `json:"obligations"`
}
type Result struct {
	Outcome     string          `json:"outcome"`
	ReasonCodes []string        `json:"reason_codes"`
	Obligations []v1.Obligation `json:"obligations"`
}
type Explanation struct {
	Decision             v1.Decision         `json:"decision"`
	Rules                []Contribution      `json:"rules"`
	ObligationProvenance map[string][]string `json:"obligation_provenance"`
}

// CanonicalEvent freezes the input profile: sets and times normalize, transport is omitted.
// It returns a detached value so neither callers nor policy evaluation can mutate it.
func CanonicalEvent(event v1.EventEnvelope) (v1.EventEnvelope, []byte, error) {
	raw, err := json.Marshal(event)
	if err != nil {
		return v1.EventEnvelope{}, nil, Failure("invalid_event")
	}
	if contracts.Validate("event-envelope", raw) != nil {
		return v1.EventEnvelope{}, nil, Failure("invalid_event")
	}
	p := normalization.Profile{Sets: []string{"/principal/roles", "/principal/claim_refs", "/operation/data_refs", "/operation/classifications"}, Timestamps: []string{"/occurred_at"}, Omit: []string{"/transport"}}
	for i := range event.Operation.Resources {
		prefix := "/operation/resources/" + strconv.Itoa(i)
		p.Sets = append(p.Sets, prefix+"/classifications", prefix+"/data_refs")
	}
	for i := range event.ExternalFacts {
		prefix := "/external_facts/" + strconv.Itoa(i)
		p.Timestamps = append(p.Timestamps, prefix+"/resolved_at", prefix+"/expires_at")
	}
	raw, err = normalization.Canonicalize(raw, p)
	if err != nil {
		return v1.EventEnvelope{}, nil, Failure("invalid_event")
	}
	var canonical v1.EventEnvelope
	if json.Unmarshal(raw, &canonical) != nil {
		return v1.EventEnvelope{}, nil, Failure("invalid_event")
	}
	return canonical, raw, nil
}
func DigestValue(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	raw, err = normalization.Canonicalize(raw, normalization.Profile{})
	if err != nil {
		return "", err
	}
	return normalization.Digest(raw), nil
}
func Decision(rev Revision, event v1.EventEnvelope, result Result) (v1.Decision, error) {
	canonical, raw, err := CanonicalEvent(event)
	if err != nil {
		return v1.Decision{}, err
	}
	op, err := DigestValue(canonical.Operation)
	if err != nil {
		return v1.Decision{}, err
	}
	facts, err := DigestValue(canonical.ExternalFacts)
	if err != nil {
		return v1.Decision{}, err
	}
	d := v1.Decision{SchemaVersion: "1.0", Id: normalization.Digest(append([]byte(string(rev)+"\n"), raw...)), Outcome: result.Outcome, PolicyRevision: string(rev), OperationDigest: op, ReasonCodes: result.ReasonCodes, Obligations: result.Obligations, EvaluatedAt: canonical.OccurredAt, ExternalFactsDigest: facts}
	encoded, err := json.Marshal(d)
	if err != nil || contracts.Validate("decision", encoded) != nil {
		return v1.Decision{}, Failure("invalid_decision")
	}
	return d, nil
}
func FactsPresent(event v1.EventEnvelope, sources []string) bool {
	at, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if err != nil {
		return false
	}
	valid := map[string]bool{}
	seen := map[string]bool{}
	for _, f := range event.ExternalFacts {
		if seen[f.Id] {
			return false
		}
		seen[f.Id] = true
		start, e1 := time.Parse(time.RFC3339Nano, f.ResolvedAt)
		end, e2 := time.Parse(time.RFC3339Nano, f.ExpiresAt)
		if e1 == nil && e2 == nil && !at.Before(start) && at.Before(end) {
			valid[f.Source] = true
		}
	}
	for _, source := range sources {
		if !valid[source] {
			return false
		}
	}
	return true
}
