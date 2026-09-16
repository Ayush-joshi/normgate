package policy

import (
	"encoding/json"
	v1 "normgate.dev/normgate/internal/contracts/v1"
	"reflect"
	"testing"
)

func part(layer, outcome string, obligations ...v1.Obligation) Contribution {
	if obligations == nil {
		obligations = []v1.Obligation{}
	}
	return Contribution{Layer: layer, RuleID: layer + ".rule", Outcome: outcome, ReasonCodes: []string{"ng.policy.test"}, Obligations: obligations}
}
func TestComposition(t *testing.T) {
	paths := []string{"/secret"}
	other := []string{"/other"}
	group := "reviewers"
	mask := v1.Obligation{SchemaVersion: "1.0", Id: "mask", Kind: "mask", Paths: &paths}
	remove := v1.Obligation{SchemaVersion: "1.0", Id: "remove", Kind: "remove_paths", Paths: &paths}
	second := v1.Obligation{SchemaVersion: "1.0", Id: "second", Kind: "mask", Paths: &other}
	approval := v1.Obligation{SchemaVersion: "1.0", Id: "approve", Kind: "require_approval", ReviewerGroup: &group}
	for _, tc := range []struct {
		name  string
		parts []Contribution
		want  string
		count int
	}{
		{"allow", []Contribution{part("baseline", "allow")}, "allow", 0},
		{"deny wins", []Contribution{part("baseline", "allow"), part("tenant", "deny")}, "deny", 0},
		{"approval wins", []Contribution{part("baseline", "allow"), part("tenant", "require_approval", approval)}, "require_approval", 1},
		{"merge", []Contribution{part("baseline", "transform", mask), part("tenant", "transform", second)}, "transform", 2},
		{"conflict", []Contribution{part("baseline", "transform", mask), part("tenant", "transform", remove)}, "deny", 0},
		{"empty", nil, "deny", 0},
		{"missing layer", []Contribution{part("baseline", "allow"), {Layer: "tenant"}}, "deny", 0},
		{"unknown obligation", []Contribution{part("baseline", "transform", v1.Obligation{SchemaVersion: "1.0", Id: "x", Kind: "future"})}, "deny", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Compose(tc.parts)
			if got.Outcome != tc.want || len(got.Obligations) != tc.count {
				t.Fatalf("%+v", got)
			}
			for i, j := 0, len(tc.parts)-1; i < j; i, j = i+1, j-1 {
				tc.parts[i], tc.parts[j] = tc.parts[j], tc.parts[i]
			}
			if again := Compose(tc.parts); !reflect.DeepEqual(got, again) {
				t.Fatalf("order changed result: %+v %+v", got, again)
			}
		})
	}
}
func FuzzComposition(f *testing.F) {
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, b []byte) {
		var parts []Contribution
		if json.Unmarshal(b, &parts) != nil {
			return
		}
		a := Compose(parts)
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		if !reflect.DeepEqual(a, Compose(parts)) {
			t.Fatal("order changed composition")
		}
		parts = append(parts, part("local_override", "deny"))
		if Compose(parts).Outcome != "deny" {
			t.Fatal("deny weakened")
		}
		if _, err := json.Marshal(a); err != nil {
			t.Fatal(err)
		}
	})
}
func FuzzExplanation(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		var x Explanation
		if json.Unmarshal(b, &x) == nil {
			raw, err := json.Marshal(x)
			if err != nil {
				t.Fatal(err)
			}
			var y Explanation
			if json.Unmarshal(raw, &y) != nil {
				t.Fatal("roundtrip")
			}
		}
	})
}
