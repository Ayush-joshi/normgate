package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"normgate.dev/normgate/internal/contracts"
)

func TestNGF002Fixtures(t *testing.T) {
	for _, group := range []string{"valid", "invalid", "compatibility"} {
		paths, err := filepath.Glob("../../testkit/fixtures/contracts/" + group + "/*.json")
		if err != nil || len(paths) == 0 {
			t.Fatalf("NG-F002: missing %s fixtures", group)
		}
		for _, path := range paths {
			t.Run(group+"/"+filepath.Base(path), func(t *testing.T) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var fixture struct {
					Schema string          `json:"schema"`
					Valid  bool            `json:"valid"`
					Value  json.RawMessage `json:"value"`
				}
				if err := json.Unmarshal(data, &fixture); err != nil {
					t.Fatal(err)
				}
				err = contracts.Validate(fixture.Schema, fixture.Value)
				if (err == nil) != fixture.Valid {
					t.Fatalf("NG-F002: valid=%t, error=%v", fixture.Valid, err)
				}
			})
		}
	}
}

func TestNGF002CorpusCompleteness(t *testing.T) {
	paths, err := filepath.Glob("../../schemas/v1/*.schema.json")
	if err != nil || len(paths) < 12 {
		t.Fatal("missing public schemas")
	}
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".schema.json")
		for _, group := range []string{"valid", "invalid", "compatibility"} {
			matches, _ := filepath.Glob("../../testkit/fixtures/contracts/" + group + "/" + name + ".*.json")
			if len(matches) == 0 {
				t.Errorf("NG-F002: %s has no %s fixture", name, group)
			}
		}
	}
}
