package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"normgate.dev/normgate/internal/cli"
)

func TestNGF005CLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"1.0","listen_address":"127.0.0.1:8080","policy_source":"file:///etc/policy","request_timeout_ms":5000,"fail_closed":true,"api_key":"example-secret-value"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"version"}, 0, "0.2.0-dev"},
		{[]string{"config", "validate", "--file", path}, 0, "Configuration valid"},
		{[]string{"config", "validate", "--file", path, "--listen", "127.0.0.1:9090", "--timeout-ms", "1000"}, 0, "Configuration valid"},
		{[]string{"config", "validate", "--file", path, "--timeout-ms", "0"}, 1, "configuration"},
		{[]string{"config", "validate", "--file", path, "extra"}, 2, "Usage"},
		{[]string{"config", "validate", "--file", "/missing/config"}, 1, "Cannot read"},
		{[]string{"config", "validate", "--bad"}, 2, "Usage"},
		{[]string{"config", "validate"}, 2, "Usage"},
		{[]string{"serve"}, 2, "Usage"},
		{nil, 2, "Usage"},
	} {
		var out, stderr bytes.Buffer
		if code := cli.Run(tc.args, &out, &stderr); code != tc.code {
			t.Fatalf("%v: code=%d, %s", tc.args, code, stderr.String())
		}
		combined := out.String() + stderr.String()
		if !strings.Contains(combined, tc.want) || strings.Contains(combined, "example-secret") {
			t.Fatalf("NG-F005: unsafe or missing output: %s", combined)
		}
	}
}
