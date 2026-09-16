package architecture

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func forbiddenImport(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		switch segment {
		case "extensions", "domains", "domain-packs", "sdk", "web", "cmd":
			return true
		}
	}
	return false
}

func TestNGF001ImportBoundary(t *testing.T) {
	err := filepath.WalkDir("../../internal", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			value, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(value, "github.com/open-policy-agent/opa") && !strings.Contains(filepath.ToSlash(path), "internal/policy/opa/") {
				t.Errorf("OPA dependency escapes engine boundary: %s", path)
			}
			if forbiddenImport(value) {
				t.Errorf("NG-F001: %s imports %s", path, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNGF001BoundaryDetector(t *testing.T) {
	for _, path := range []string{"normgate.dev/normgate/extensions/example", "vendor.test/domains/example", "normgate.dev/normgate/cmd/normgate"} {
		if !forbiddenImport(path) {
			t.Fatalf("missed %s", path)
		}
	}
	if forbiddenImport("normgate.dev/normgate/internal/contracts") {
		t.Fatal("core dependency rejected")
	}
}

func TestNGF007Vocabulary(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)\b(hipaa|phi|fhir|patient|diagnosis|healthcare|banking|student|education)\b`)
	files, err := filepath.Glob("../../schemas/v1/*.json")
	if err != nil || len(files) == 0 {
		t.Fatal("NG-F007: no schemas")
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if pattern.Match(data) {
			t.Errorf("NG-F007: domain vocabulary in %s", path)
		}
	}
}
