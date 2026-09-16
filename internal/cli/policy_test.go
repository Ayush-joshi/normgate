package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"normgate.dev/normgate/internal/cli"
)

func TestPolicyCLI(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "policy")
	archive := filepath.Join(dir, "bundle.tar.gz")
	event := filepath.Join(dir, "event.json")
	raw, err := os.ReadFile("../../testkit/fixtures/contracts/valid/event-envelope.basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Value json.RawMessage `json:"value"`
	}
	json.Unmarshal(raw, &fixture)
	os.WriteFile(event, fixture.Value, 0600)
	for _, tc := range []struct {
		args     []string
		code     int
		contains string
	}{
		{[]string{"policy", "init", source}, 0, "initialized"},
		{[]string{"policy", "init", source}, 1, "exists"},
		{[]string{"policy", "lint", source}, 0, "revision"},
		{[]string{"policy", "test", source}, 0, "passed"},
		{[]string{"policy", "build", source, "--out", archive}, 0, "revision"},
		{[]string{"policy", "inspect", archive}, 0, "normgate.baseline"},
		{[]string{"policy", "diff", archive, archive}, 0, "[]"},
		{[]string{"policy", "explain", "--bundle", archive, "--event", event}, 0, `"allow"`},
		{[]string{"policy", "activate", "--bundle", archive, "--state", filepath.Join(dir, "state")}, 0, `"ready": true`},
		{[]string{"policy", "status", "--state", filepath.Join(dir, "state")}, 0, `"ready": true`},
		{[]string{"policy"}, 2, "Usage"}, {[]string{"policy", "unknown"}, 2, "Usage"}, {[]string{"policy", "test", "/missing"}, 1, "ng.policy.bundle_read"},
	} {
		var out, stderr bytes.Buffer
		code := cli.Run(tc.args, &out, &stderr)
		if code != tc.code || !strings.Contains(out.String()+stderr.String(), tc.contains) {
			t.Fatalf("%v: %d %s %s", tc.args, code, out.String(), stderr.String())
		}
	}
}
