package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
	v1 "normgate.dev/normgate/internal/contracts/v1"
)

type Expected struct {
	Outcome     string          `json:"outcome"`
	ReasonCodes []string        `json:"reason_codes"`
	Obligations []v1.Obligation `json:"obligations"`
	RuleIDs     []string        `json:"rule_ids"`
}
type Scenario struct {
	ID            string           `json:"id"`
	Event         v1.EventEnvelope `json:"event"`
	Expected      Expected         `json:"expected"`
	ExpectedError string           `json:"expected_error,omitempty"`
}
type TestReport struct {
	Passed int      `json:"passed"`
	Failed []string `json:"failed"`
}

func ParseScenario(raw []byte) (Scenario, error) {
	if len(raw) > 1<<20 {
		return Scenario{}, Failure("invalid_scenario")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var node yaml.Node
	if decoder.Decode(&node) != nil {
		return Scenario{}, Failure("invalid_scenario")
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return Scenario{}, Failure("invalid_scenario")
	}
	if !safeYAML(&node, 0) {
		return Scenario{}, Failure("invalid_scenario")
	}
	var value any
	if node.Decode(&value) != nil {
		return Scenario{}, Failure("invalid_scenario")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return Scenario{}, Failure("invalid_scenario")
	}
	var scenario Scenario
	if strictJSON(encoded, &scenario) != nil || scenario.ID == "" {
		return Scenario{}, Failure("invalid_scenario")
	}
	return scenario, nil
}
func safeYAML(n *yaml.Node, depth int) bool {
	if depth > 64 || n.Kind == yaml.AliasNode || n.Anchor != "" {
		return false
	}
	if n.Kind == yaml.ScalarNode && !slices.Contains([]string{"!!str", "!!int", "!!bool", "!!null"}, n.Tag) {
		return false
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag != "!!str" || seen[k.Value] {
				return false
			}
			seen[k.Value] = true
		}
	}
	for _, child := range n.Content {
		if !safeYAML(child, depth+1) {
			return false
		}
	}
	return true
}
func RunTests(ctx context.Context, engine Engine, revision Revision, bundle Bundle) (TestReport, error) {
	report := TestReport{Failed: []string{}}
	names := []string{}
	for name := range bundle.Files {
		if strings.HasPrefix(name, "tests/") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".json")) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	if len(names) == 0 {
		return report, Failure("missing_tests")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		scenario, err := ParseScenario(bundle.Files[name])
		if err != nil {
			return report, err
		}
		if seen[scenario.ID] {
			return report, Failure("duplicate_scenario")
		}
		seen[scenario.ID] = true
		ex, err := engine.Explain(ctx, revision, scenario.Event)
		if scenario.ExpectedError != "" {
			if err != nil && err.Error() == scenario.ExpectedError {
				report.Passed++
				continue
			}
			report.Failed = append(report.Failed, scenario.ID)
			continue
		}
		ids := []string{}
		for _, r := range ex.Rules {
			ids = append(ids, r.RuleID)
		}
		slices.Sort(ids)
		actual := Expected{Outcome: ex.Decision.Outcome, ReasonCodes: ex.Decision.ReasonCodes, Obligations: ex.Decision.Obligations, RuleIDs: ids}
		if err != nil || !same(actual, scenario.Expected) {
			report.Failed = append(report.Failed, scenario.ID)
		} else {
			report.Passed++
		}
	}
	if len(report.Failed) > 0 {
		return report, Failure("tests_failed")
	}
	return report, nil
}
