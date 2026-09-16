package policy

import (
	"encoding/json"
	"slices"
	"strings"

	"normgate.dev/normgate/internal/contracts"
	v1 "normgate.dev/normgate/internal/contracts/v1"
)

var LayerNames = []string{"baseline", "organization", "tenant", "application", "local_override"}

func Deny(code string) Result {
	return Result{Outcome: "deny", ReasonCodes: []string{"ng.policy." + code}, Obligations: []v1.Obligation{}}
}

// Compose is commutative and fail closed. It never permits a higher layer to erase
// a lower layer's constraints. Missing configured layers are explicit empty contributions.
func Compose(parts []Contribution) Result {
	parts = slices.Clone(parts)
	slices.SortFunc(parts, func(a, b Contribution) int {
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		return strings.Compare(string(x), string(y))
	})
	result := Result{Outcome: "allow", ReasonCodes: []string{}, Obligations: []v1.Obligation{}}
	if len(parts) == 0 {
		return Deny("no_decision")
	}
	deny, approval, transform := false, false, false
	byID := map[string]v1.Obligation{}
	for _, p := range parts {
		if p.Outcome == "" {
			return Deny("no_decision")
		}
		if !slices.Contains(LayerNames, p.Layer) || p.RuleID == "" {
			return Deny("invalid_decision")
		}
		// Validate a complete decision to check outcome/obligation relationships too.
		d := v1.Decision{SchemaVersion: "1.0", Id: "validation", Outcome: p.Outcome, PolicyRevision: "validation", OperationDigest: zeroDigest, ExternalFactsDigest: zeroDigest, EvaluatedAt: "2026-01-01T00:00:00Z", ReasonCodes: p.ReasonCodes, Obligations: p.Obligations}
		raw, err := json.Marshal(d)
		if err != nil || contracts.Validate("decision", raw) != nil {
			return Deny("invalid_decision")
		}
		deny = deny || p.Outcome == "deny"
		approval = approval || p.Outcome == "require_approval"
		transform = transform || p.Outcome == "transform"
		result.ReasonCodes = append(result.ReasonCodes, p.ReasonCodes...)
		for _, o := range p.Obligations {
			// Copy and sort path sets before comparing or publishing them.
			if o.Paths != nil {
				paths := slices.Clone(*o.Paths)
				slices.Sort(paths)
				o.Paths = &paths
			}
			if previous, ok := byID[o.Id]; ok {
				if !same(previous, o) {
					return Deny("obligation_conflict")
				}
			} else {
				byID[o.Id] = o
			}
		}
	}
	for _, o := range byID {
		result.Obligations = append(result.Obligations, o)
	}
	slices.SortFunc(result.Obligations, func(a, b v1.Obligation) int { return strings.Compare(a.Id, b.Id) })
	for i, a := range result.Obligations {
		for _, b := range result.Obligations[i+1:] {
			if conflict(a, b) {
				return Deny("obligation_conflict")
			}
		}
	}
	slices.Sort(result.ReasonCodes)
	result.ReasonCodes = slices.Compact(result.ReasonCodes)
	switch {
	case deny:
		result.Outcome = "deny"
		result.Obligations = []v1.Obligation{}
	case approval:
		result.Outcome = "require_approval"
	case transform:
		result.Outcome = "transform"
	}
	return result
}
func same(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func overlaps(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
func conflict(a, b v1.Obligation) bool {
	if a.Kind == "restrict_destination" && b.Kind == a.Kind {
		return *a.DestinationId != *b.DestinationId
	}
	// Multiple retain sets would be order-dependent; identical sets are idempotent.
	if a.Kind == "retain_paths" || b.Kind == "retain_paths" {
		if a.Paths != nil && b.Paths != nil {
			return a.Kind != b.Kind || !same(a.Paths, b.Paths)
		}
	}
	if a.Paths != nil && b.Paths != nil {
		for _, x := range *a.Paths {
			for _, y := range *b.Paths {
				if overlaps(x, y) && (a.Kind != b.Kind || a.Kind == "tokenize" || a.Kind == "mask") {
					return true
				}
			}
		}
	}
	return false
}
func ExplainResult(rev Revision, event v1.EventEnvelope, parts []Contribution) (Explanation, error) {
	// Detach explanation data from engine/caller-owned storage.
	raw, err := json.Marshal(parts)
	if err != nil {
		return Explanation{}, Failure("invalid_decision")
	}
	var rules []Contribution
	if err = json.Unmarshal(raw, &rules); err != nil {
		return Explanation{}, err
	}
	if rules == nil {
		rules = []Contribution{}
	}
	slices.SortFunc(rules, func(a, b Contribution) int {
		if x := slices.Index(LayerNames, a.Layer) - slices.Index(LayerNames, b.Layer); x != 0 {
			return x
		}
		return strings.Compare(a.RuleID, b.RuleID)
	})
	decision, err := Decision(rev, event, Compose(rules))
	if err != nil {
		return Explanation{}, err
	}
	provenance := map[string][]string{}
	for _, p := range rules {
		for _, o := range p.Obligations {
			provenance[o.Id] = append(provenance[o.Id], p.Layer+":"+p.RuleID)
		}
	}
	for id, refs := range provenance {
		slices.Sort(refs)
		provenance[id] = slices.Compact(refs)
	}
	return Explanation{Decision: decision, Rules: rules, ObligationProvenance: provenance}, nil
}
