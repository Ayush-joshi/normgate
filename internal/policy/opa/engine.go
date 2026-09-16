// Package opa is the only package permitted to import the embedded OPA evaluator.
package opa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"normgate.dev/normgate/internal/contracts"
	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/policy"
)

const MaxRuleDepth = 64
const MaxRules = 2048
const MaxRevisions = 128
const EvaluationTimeout = 50 * time.Millisecond
const CompileTimeout = 5 * time.Second

type prepared struct {
	report  policy.Report
	queries []rego.PreparedEvalQuery
}
type Engine struct {
	mu        sync.RWMutex
	revisions map[policy.Revision]*prepared
	closed    bool
}

func New() *Engine { return &Engine{revisions: map[policy.Revision]*prepared{}} }

var _ policy.Engine = (*Engine)(nil)

func capabilities() *ast.Capabilities {
	c := ast.CapabilitiesForThisVersion()
	c.AllowNet = []string{}
	builtins := c.Builtins
	c.Builtins = nil
	allowed := strings.Fields("eq equal neq lt lte gt gte assign plus minus mul div rem and or union intersection count sum max min sort concat contains startswith endswith split lower upper trim trim_space replace substring sprintf is_array is_object is_string is_number is_boolean is_null is_set object.get object.keys object.union array.concat array.slice internal.member_2 internal.member_3 to_number")
	for _, b := range builtins {
		if slices.Contains(allowed, b.Name) {
			c.Builtins = append(c.Builtins, b)
		}
	}
	return c
}
func sanitized(err error) error {
	var es ast.Errors
	if errors.As(err, &es) && len(es) > 0 {
		e := &policy.Error{Code: "ng.policy.compile_error"}
		if es[0].Location != nil {
			e.File = es[0].Location.File
			e.Row = es[0].Location.Row
			e.Column = es[0].Location.Col
		}
		return e
	}
	return policy.Failure("compile_error")
}
func checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
func (e *Engine) Compile(ctx context.Context, bundle policy.Bundle) (policy.Revision, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, CompileTimeout)
	defer cancel()
	bundle, report, err := policy.Prepare(bundle)
	if err != nil {
		return "", err
	}
	e.mu.RLock()
	closed := e.closed
	_, cached := e.revisions[report.Revision]
	e.mu.RUnlock()
	if closed {
		return "", policy.Failure("closed")
	}
	if cached {
		if err := checkContext(ctx); err != nil {
			return "", err
		}
		return report.Revision, nil
	}
	modules := map[string]*ast.Module{}
	defaults := map[string]bool{}
	rules := 0
	for _, name := range report.Files {
		if !strings.HasSuffix(name, ".rego") {
			continue
		}
		raw := bundle.Files[name]
		if !boundedSyntax(raw) {
			return "", policy.Failure("rule_depth")
		}
		module, err := ast.ParseModuleWithOpts(name, string(raw), ast.ParserOptions{RegoVersion: ast.RegoV1, Capabilities: capabilities()})
		if err != nil {
			return "", sanitized(err)
		}
		ns := strings.TrimPrefix(module.Package.Path.String(), "data.")
		owned := false
		for _, l := range report.Layers {
			owned = owned || ns == l.Namespace
		}
		if !owned {
			return "", policy.Failure("namespace_collision")
		}
		var lint error
		ast.WalkTerms(module, func(term *ast.Term) bool {
			if obj, ok := term.Value.(ast.Object); ok {
				if obligations := obj.Get(ast.StringTerm("obligations")); obligations != nil && obligations.IsGround() {
					value, err := ast.JSON(obligations.Value)
					if err != nil {
						lint = policy.Failure("invalid_obligation")
					} else if list, ok := value.([]any); ok {
						for _, item := range list {
							raw, _ := json.Marshal(item)
							if contracts.Validate("obligation", raw) != nil {
								lint = policy.Failure("invalid_obligation")
							}
						}
					} else {
						lint = policy.Failure("invalid_obligation")
					}
				}
			}
			if s, ok := term.Value.(ast.String); ok && strings.HasPrefix(string(s), "ng.") {
				if _, ok := report.Reasons[string(s)]; !ok {
					lint = policy.Failure("unknown_reason")
				}
			}
			return false
		})
		ast.WalkRefs(module, func(ref ast.Ref) bool {
			if strings.HasPrefix(ref.String(), "input.external_facts") && len(report.Manifest.ExternalFactSources) == 0 {
				lint = policy.Failure("undeclared_fact")
			}
			return false
		})
		ast.WalkExprs(module, func(expr *ast.Expr) bool {
			if expr.IsCall() {
				op := expr.Operator().String()
				if op == "print" || op == "internal.print" {
					lint = policy.Failure("forbidden_builtin")
				}
			}
			return false
		})
		if lint != nil {
			return "", lint
		}
		for _, rule := range module.Rules {
			rules++
			if rule.Default && rule.Head.Name == "decisions" {
				defaults[ns] = true
			}
		}
		modules[name] = module
	}
	if rules > MaxRules {
		return "", policy.Failure("rule_depth")
	}
	for _, layer := range report.Layers {
		if !defaults[layer.Namespace] {
			return "", policy.Failure("missing_default")
		}
	}
	compiler := ast.NewCompiler().WithCapabilities(capabilities()).WithStrict(true).WithEnablePrintStatements(false)
	compiler.Compile(modules)
	if compiler.Failed() {
		return "", sanitized(compiler.Errors)
	}
	depths := map[*ast.Rule]int{}
	var depth func(*ast.Rule) int
	depth = func(rule *ast.Rule) int {
		if n := depths[rule]; n > 0 {
			return n
		}
		n := 1
		for dep := range compiler.Graph.Dependencies(rule) {
			if child, ok := dep.(*ast.Rule); ok {
				n = max(n, 1+depth(child))
			}
		}
		depths[rule] = n
		return n
	}
	for _, module := range compiler.Modules {
		for _, rule := range module.Rules {
			if depth(rule) > MaxRuleDepth {
				return "", policy.Failure("rule_depth")
			}
		}
	}
	// Every helper must be reachable from a published entry point. This also
	// catches accidentally orphaned security rules during policy refactoring.
	reachable := map[*ast.Rule]bool{}
	var visit func(*ast.Rule)
	visit = func(rule *ast.Rule) {
		if reachable[rule] {
			return
		}
		reachable[rule] = true
		for dep := range compiler.Graph.Dependencies(rule) {
			if child, ok := dep.(*ast.Rule); ok {
				visit(child)
			}
		}
	}
	for _, module := range compiler.Modules {
		for _, rule := range module.Rules {
			if rule.Head.Name == "decisions" {
				visit(rule)
			}
		}
	}
	for _, module := range compiler.Modules {
		for _, rule := range module.Rules {
			if !reachable[rule] {
				return "", policy.Failure("unreachable_rule")
			}
		}
	}
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	var data map[string]any
	decoder := json.NewDecoder(bytes.NewReader(bundle.Files["data.json"]))
	decoder.UseNumber()
	if err = decoder.Decode(&data); err != nil {
		return "", policy.Failure("invalid_data")
	}
	store := inmem.NewFromObject(data)
	p := &prepared{report: report}
	for _, layer := range report.Layers {
		query, err := rego.New(rego.Query("data."+layer.Namespace+".decisions"), rego.Compiler(compiler), rego.Store(store), rego.Capabilities(capabilities()), rego.StrictBuiltinErrors(true)).PrepareForEval(ctx)
		if err != nil {
			return "", sanitized(err)
		}
		p.queries = append(p.queries, query)
	}
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return "", policy.Failure("closed")
	}
	if len(e.revisions) >= MaxRevisions {
		return "", policy.Failure("revision_limit")
	}
	e.revisions[report.Revision] = p
	return report.Revision, nil
}

// Bound lexical nesting before OPA's recursive parser; strings and comments do not count.
func boundedSyntax(raw []byte) bool {
	depth := 0
	var quote byte
	comment, escaped := false, false
	for _, c := range raw {
		if comment {
			if c == '\n' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' && quote == '"' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '#':
			comment = true
		case '"', '`':
			quote = c
		case '{', '[', '(':
			depth++
			if depth > MaxRuleDepth {
				return false
			}
		case '}', ']', ')':
			depth--
		}
	}
	return true
}
func (e *Engine) Explain(ctx context.Context, revision policy.Revision, event v1.EventEnvelope) (policy.Explanation, error) {
	if err := checkContext(ctx); err != nil {
		return policy.Explanation{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, EvaluationTimeout)
	defer cancel()
	canonical, raw, err := policy.CanonicalEvent(event)
	if err != nil {
		return policy.Explanation{}, err
	}
	e.mu.RLock()
	p := e.revisions[revision]
	closed := e.closed
	e.mu.RUnlock()
	if closed {
		return policy.Explanation{}, policy.Failure("closed")
	}
	if p == nil {
		return policy.Explanation{}, policy.Failure("unknown_revision")
	}
	if !policy.FactsPresent(canonical, p.report.Manifest.ExternalFactSources) {
		d, err := policy.Decision(revision, canonical, policy.Deny("missing_fact"))
		return policy.Explanation{Decision: d, Rules: []policy.Contribution{}, ObligationProvenance: map[string][]string{}}, err
	}
	var input any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err = dec.Decode(&input); err != nil {
		return policy.Explanation{}, policy.Failure("invalid_event")
	}
	// Undeclared facts remain in the replay digest but are never visible to Rego,
	// including through dynamic references or aliases of input.
	object := input.(map[string]any)
	visible := []any{}
	for _, value := range object["external_facts"].([]any) {
		fact := value.(map[string]any)
		if slices.Contains(p.report.Manifest.ExternalFactSources, fact["source"].(string)) {
			visible = append(visible, value)
		}
	}
	object["external_facts"] = visible
	parts := []policy.Contribution{}
	for i, query := range p.queries {
		result, err := query.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			if ctx.Err() != nil {
				return policy.Explanation{}, ctx.Err()
			}
			return policy.Explanation{}, policy.Failure("evaluation_failed")
		}
		if len(result) != 1 || len(result[0].Expressions) != 1 {
			return policy.Explanation{}, policy.Failure("invalid_decision")
		}
		output, err := json.Marshal(result[0].Expressions[0].Value)
		if err != nil || len(output) > 1<<20 {
			return policy.Explanation{}, policy.Failure("invalid_decision")
		}
		var layerParts []policy.Contribution
		d := json.NewDecoder(bytes.NewReader(output))
		d.DisallowUnknownFields()
		if d.Decode(&layerParts) != nil || len(layerParts) > 1024 {
			return policy.Explanation{}, policy.Failure("invalid_decision")
		}
		if len(layerParts) == 0 {
			parts = append(parts, policy.Contribution{Layer: p.report.Layers[i].Name})
			continue
		}
		seen := map[string]bool{}
		for j := range layerParts {
			item := &layerParts[j]
			if item.Layer != "" || seen[item.RuleID] {
				return policy.Explanation{}, policy.Failure("invalid_decision")
			}
			seen[item.RuleID] = true
			item.Layer = p.report.Layers[i].Name
			for _, reason := range item.ReasonCodes {
				if _, ok := p.report.Reasons[reason]; !ok {
					return policy.Explanation{}, policy.Failure("unknown_reason")
				}
			}
		}
		parts = append(parts, layerParts...)
	}
	if err := checkContext(ctx); err != nil {
		return policy.Explanation{}, err
	}
	explanation, err := policy.ExplainResult(revision, canonical, parts)
	if err != nil {
		return policy.Explanation{}, err
	}
	if err = checkContext(ctx); err != nil {
		return policy.Explanation{}, err
	}
	return explanation, nil
}
func (e *Engine) Evaluate(ctx context.Context, r policy.Revision, event v1.EventEnvelope) (v1.Decision, error) {
	ex, err := e.Explain(ctx, r, event)
	return ex.Decision, err
}
func (e *Engine) Health(ctx context.Context) policy.Health {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return policy.Health{Ready: !e.closed && ctx.Err() == nil, Revisions: len(e.revisions)}
}
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	e.revisions = map[policy.Revision]*prepared{}
	return nil
}
