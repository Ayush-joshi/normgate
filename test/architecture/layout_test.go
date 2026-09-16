package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNGF001Layout(t *testing.T) {
	for _, path := range []string{
		"cmd/normgate", "internal/api", "internal/config", "internal/contracts",
		"internal/enforcement", "internal/identity", "internal/normalization",
		"internal/policy", "internal/receipt", "internal/storage", "internal/telemetry",
		"api/openapi", "schemas/v1", "policies/baseline", "sdk/python", "sdk/typescript",
		"web", "deployments", "test/contract", "testkit/fixtures", "docs/architecture/decisions",
	} {
		t.Run(path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join("../..", path))
			if err != nil || !info.IsDir() {
				t.Fatalf("NG-F001: missing directory %s", path)
			}
		})
	}
}
