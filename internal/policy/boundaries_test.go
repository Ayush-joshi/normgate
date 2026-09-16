package policy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "normgate.dev/normgate/internal/contracts/v1"
)

func testBundle(t testing.TB) Bundle {
	t.Helper()
	b, err := LoadDir("../../policies/baseline")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func testEvent(t testing.TB) v1.EventEnvelope {
	t.Helper()
	raw, err := os.ReadFile("../../testkit/fixtures/contracts/valid/event-envelope.basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var wrapper struct {
		Value v1.EventEnvelope `json:"value"`
	}
	if json.Unmarshal(raw, &wrapper) != nil {
		t.Fatal("fixture")
	}
	return wrapper.Value
}
func modifyManifest(b Bundle, edit func(*v1.PolicyManifest)) {
	m, _ := ParseManifest(b.Files["manifest.json"])
	edit(&m)
	b.Files["manifest.json"], _ = json.Marshal(m)
}
func TestBundleValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(Bundle)
	}{
		{"traversal", func(b Bundle) { b.Files["../escape"] = []byte("x") }},
		{"absolute", func(b Bundle) { b.Files["/escape"] = []byte("x") }},
		{"oversized file", func(b Bundle) { b.Files["large"] = bytes.Repeat([]byte("x"), 1<<20+1) }},
		{"oversized total", func(b Bundle) {
			for i := range 5 {
				b.Files[fmt.Sprint(i)] = bytes.Repeat([]byte("x"), 1<<20)
			}
		}},
		{"too many files", func(b Bundle) {
			for i := range MaxFiles {
				b.Files[fmt.Sprint(i)] = nil
			}
		}},
		{"too many modules", func(b Bundle) {
			for i := range MaxModules {
				b.Files[fmt.Sprintf("%d.rego", i)] = nil
			}
		}},
		{"missing modules", func(b Bundle) { delete(b.Files, "baseline.rego") }},
		{"invalid manifest", func(b Bundle) { b.Files["manifest.json"] = []byte(`{}`) }},
		{"missing version", func(b Bundle) { modifyManifest(b, func(m *v1.PolicyManifest) { m.Version = "bad" }) }},
		{"incompatible version", func(b Bundle) { modifyManifest(b, func(m *v1.PolicyManifest) { m.MinimumCoreVersion = "0.3.0" }) }},
		{"invalid identity", func(b Bundle) { modifyManifest(b, func(m *v1.PolicyManifest) { m.SigningIdentity = "other" }) }},
		{"empty identity", func(b Bundle) { modifyManifest(b, func(m *v1.PolicyManifest) { m.SigningIdentity = "key:" }) }},
		{"unknown layer", func(b Bundle) {
			b.Files["layers.json"] = []byte(`[{"name":"unknown","namespace":"normgate.baseline","priority":0}]`)
		}},
		{"empty layers", func(b Bundle) { b.Files["layers.json"] = []byte(`[]`) }},
		{"duplicate priorities", func(b Bundle) {
			b.Files["layers.json"] = []byte(`[{"name":"baseline","namespace":"normgate.baseline","priority":0},{"name":"tenant","namespace":"normgate.baseline","priority":0}]`)
		}},
		{"missing baseline", func(b Bundle) {
			b.Files["layers.json"] = []byte(`[{"name":"tenant","namespace":"normgate.baseline","priority":0}]`)
		}},
		{"collision", func(b Bundle) {
			modifyManifest(b, func(m *v1.PolicyManifest) { m.Namespaces = append(m.Namespaces, "normgate.baseline.child") })
		}},
		{"reasons missing", func(b Bundle) { delete(b.Files, "reasons.json") }},
		{"reasons invalid", func(b Bundle) { b.Files["reasons.json"] = []byte(`{"wrong":{"description":"x","remediation":"y"}}`) }},
		{"reasons incomplete", func(b Bundle) { b.Files["reasons.json"] = []byte(`{"ng.policy.test":{"description":"x"}}`) }},
		{"data invalid", func(b Bundle) { b.Files["data.json"] = []byte(`null`) }},
		{"data shadows policy", func(b Bundle) { b.Files["data.json"] = []byte(`{"normgate":{}}`) }},
		{"dependencies invalid", func(b Bundle) { b.Files["dependencies.json"] = []byte(`true`) }},
		{"dependencies invalid manifest", func(b Bundle) { b.Files["dependencies.json"] = []byte(`[{}]`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := testBundle(t)
			tc.edit(b)
			if _, err := Inspect(b); err == nil {
				t.Fatal("accepted invalid bundle")
			}
			if _, err := Build(b); err == nil {
				t.Fatal("built invalid bundle")
			}
		})
	}
	b := testBundle(t)
	a, _ := Build(b)
	file := filepath.Join(t.TempDir(), "a.tar.gz")
	os.WriteFile(file, a, 0600)
	if _, err := Load(file); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("../../policies/baseline"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("/missing"); err == nil {
		t.Fatal("missing")
	}
	changed := testBundle(t)
	delete(changed.Files, "tests/registered_identity.yaml")
	changed.Files["new.txt"] = []byte("new")
	changed.Files["data.json"] = append(changed.Files["data.json"], ' ')
	if d := Diff(b, changed); len(d) != 3 || d[0].Before == d[0].After {
		t.Fatal(d)
	}
	linked := t.TempDir()
	os.Symlink(file, filepath.Join(linked, "bundle"))
	if _, err := LoadDir(linked); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := LoadDir("/missing"); err == nil {
		t.Fatal("missing directory")
	}
}
func TestDependencies(t *testing.T) {
	base, _ := ParseManifest(testBundle(t).Files["manifest.json"])
	dep := base
	dep.Id = "dependency"
	dep.Namespaces = []string{"example.dependency"}
	dep.Dependencies = []string{}
	base.Dependencies = []string{dep.Id + "@" + dep.Version}
	if err := ValidateDependencies([]v1.PolicyManifest{base, dep}); err != nil {
		t.Fatal(err)
	}
	for _, all := range [][]v1.PolicyManifest{{base}, {base, base}, {base, func() v1.PolicyManifest { d := dep; d.Version = "9.0.0"; return d }()}, {base, func() v1.PolicyManifest { d := dep; d.Dependencies = []string{base.Id + "@" + base.Version}; return d }()}, {func() v1.PolicyManifest { b := base; b.Dependencies = []string{"bad"}; return b }(), dep}} {
		if ValidateDependencies(all) == nil {
			t.Fatal("bad dependency accepted")
		}
	}
	for _, v := range []string{"1", "x.1.0", "01.0.0", "-1.0.0"} {
		if _, err := version(v); err == nil {
			t.Fatal(v)
		}
	}
	if compatible("0.2.1") || compatible("1.0.0") || !compatible("0.1.0") {
		t.Fatal("compatibility")
	}
	b := testBundle(t)
	modifyManifest(b, func(m *v1.PolicyManifest) { m.Dependencies = base.Dependencies })
	b.Files["dependencies.json"], _ = json.Marshal([]v1.PolicyManifest{dep})
	if _, err := Inspect(b); err != nil {
		t.Fatal(err)
	}
}
func archiveEntries(t *testing.T, names []string, kind byte, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, n := range names {
		size := len(content)
		if kind != tar.TypeReg {
			size = 0
		}
		if err := tw.WriteHeader(&tar.Header{Name: n, Typeflag: kind, Mode: 0600, Size: int64(size)}); err != nil {
			t.Fatal(err)
		}
		if size > 0 {
			tw.Write(content)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}
func TestArchiveAttacks(t *testing.T) {
	for _, raw := range [][]byte{[]byte("garbage"), archiveEntries(t, []string{"../escape"}, tar.TypeReg, nil), archiveEntries(t, []string{"link"}, tar.TypeSymlink, nil), archiveEntries(t, []string{"duplicate", "duplicate"}, tar.TypeReg, nil), archiveEntries(t, []string{"big"}, tar.TypeReg, bytes.Repeat([]byte("x"), 1<<20+1)), archiveEntries(t, nil, tar.TypeReg, nil)} {
		if _, err := ReadArchive(bytes.NewReader(raw)); err == nil {
			t.Fatal("malformed archive accepted")
		}
	}
	raw, _ := Build(testBundle(t))
	if _, err := ReadArchive(bytes.NewReader(raw[:len(raw)/2])); err == nil {
		t.Fatal("truncated")
	}
	if _, err := ReadArchive(bytes.NewReader(append(raw, 1, 2, 3))); err == nil {
		t.Fatal("trailing gzip")
	}
	if _, err := readBounded(strings.NewReader("123"), 2); err == nil {
		t.Fatal("size limit")
	}
	if _, err := readBounded(errorReader{}, 2); err == nil {
		t.Fatal("io error")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("private read error") }
func TestCanonicalAndCompositionBoundaries(t *testing.T) {
	ev := testEvent(t)
	if _, _, err := CanonicalEvent(ev); err != nil {
		t.Fatal(err)
	}
	ev.OccurredAt = "invalid"
	if FactsPresent(ev, nil) {
		t.Fatal("invalid time")
	}
	ev = testEvent(t)
	if !FactsPresent(ev, []string{"registry"}) {
		t.Fatal("present fact")
	}
	ev.ExternalFacts = append(ev.ExternalFacts, ev.ExternalFacts[0])
	if FactsPresent(ev, nil) {
		t.Fatal("duplicate fact")
	}
	ev = testEvent(t)
	ev.OccurredAt = ev.ExternalFacts[0].ExpiresAt
	if FactsPresent(ev, []string{"registry"}) {
		t.Fatal("expiry boundary")
	}
	if _, err := DigestValue(make(chan int)); err == nil {
		t.Fatal("unsupported value")
	}
	if _, err := DigestValue(1.5); err == nil {
		t.Fatal("unsafe value")
	}
	if _, err := Decision("r", testEvent(t), Result{}); err == nil {
		t.Fatal("bad decision")
	}
	extras := map[string]any{"example.test/value": make(chan int)}
	ev = testEvent(t)
	ev.Extensions = &extras
	if _, _, err := CanonicalEvent(ev); err == nil {
		t.Fatal("bad extensions")
	}
	paths := []string{"/secret"}
	parent := []string{""}
	destA, destB := "a", "b"
	mask := v1.Obligation{SchemaVersion: "1.0", Id: "mask", Kind: "mask", Paths: &paths}
	retain := v1.Obligation{SchemaVersion: "1.0", Id: "retain", Kind: "retain_paths", Paths: &parent}
	for _, parts := range [][]Contribution{{part("unknown", "allow")}, {{Layer: "baseline", Outcome: "allow"}}, {part("baseline", "transform", mask), part("tenant", "transform", func() v1.Obligation { o := mask; o.Kind = "tokenize"; return o }())}, {part("baseline", "transform", mask), part("tenant", "transform", retain)}, {part("baseline", "allow", v1.Obligation{SchemaVersion: "1.0", Id: "a", Kind: "restrict_destination", DestinationId: &destA}), part("tenant", "allow", v1.Obligation{SchemaVersion: "1.0", Id: "b", Kind: "restrict_destination", DestinationId: &destB})}} {
		if Compose(parts).Outcome != "deny" {
			t.Fatal("conflict allowed")
		}
	}
	ex, err := ExplainResult("revision", testEvent(t), []Contribution{part("baseline", "transform", mask), part("tenant", "transform", mask)})
	if err != nil || len(ex.ObligationProvenance["mask"]) != 2 {
		t.Fatal(ex, err)
	}
	if _, err = ExplainResult("r", testEvent(t), nil); err != nil {
		t.Fatal(err)
	}
	if _, err = ExplainResult("r", testEvent(t), []Contribution{part("baseline", "allow", v1.Obligation{Extensions: &extras})}); err == nil {
		t.Fatal("bad explanation")
	}
	_ = (&Error{Code: "code", File: "policy.rego", Row: 1, Column: 2}).Error()
}
func TestScenarioParsing(t *testing.T) {
	for _, raw := range []string{"", "id: x\nid: y", "id: &x one\nevent: *x", "id: x\n---\nid: y", "id: !custom x", "1: value", `{"id":"x","extra":true}`, `{"id":""}`, strings.Repeat("[", 70) + strings.Repeat("]", 70), "id: 2026-01-01"} {
		if _, err := ParseScenario([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	if _, err := ParseScenario(bytes.Repeat([]byte("x"), 1<<20+1)); err == nil {
		t.Fatal("oversized")
	}
	if _, err := ReadEvent(errorReader{}); err == nil {
		t.Fatal("event read")
	}
	if _, err := ReadEvent(strings.NewReader("{}")); err != nil {
		t.Fatal(err)
	}
	if strictJSON([]byte(`{"x":1,"x":2}`), &map[string]any{}) == nil {
		t.Fatal("duplicate json")
	}
}

func TestArchiveOrderAndMetadata(t *testing.T) {
	b, report, err := Prepare(testBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Header.ModTime = time.Now()
	tw := tar.NewWriter(gz)
	for i := len(report.Files) - 1; i >= 0; i-- {
		name := report.Files[i]
		raw := b.Files[name]
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(raw)), Typeflag: tar.TypeReg, ModTime: time.Now()}); err != nil {
			t.Fatal(err)
		}
		tw.Write(raw)
	}
	tw.Close()
	gz.Close()
	loaded, err := ReadArchive(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(loaded)
	if err != nil || inspection.Revision != report.Revision {
		t.Fatal("metadata affected content revision", err)
	}
	// A byte change inside a sealed archive must not be repaired on read.
	b.Files["data.json"] = append(b.Files["data.json"], ' ')
	buf.Reset()
	gz = gzip.NewWriter(&buf)
	tw = tar.NewWriter(gz)
	for _, name := range report.Files {
		raw := b.Files[name]
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(raw)), Typeflag: tar.TypeReg})
		tw.Write(raw)
	}
	tw.Close()
	gz.Close()
	if _, err = ReadArchive(bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatal("tampered archive accepted")
	}
}
