package policy

import (
	"bytes"
	"testing"
)

func TestBundleDeterminism(t *testing.T) {
	b, err := LoadDir("../../policies/baseline")
	if err != nil {
		t.Fatal(err)
	}
	a, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Build(b)
	if err != nil || !bytes.Equal(a, again) {
		t.Fatal("nondeterministic archive", err)
	}
	got, err := ReadArchive(bytes.NewReader(a))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Inspect(got); err != nil {
		t.Fatal(err)
	}
	b.Files["data.json"] = append(b.Files["data.json"], ' ')
	changed, err := Build(b)
	if err != nil || bytes.Equal(a, changed) {
		t.Fatal("digest omitted authoritative file", err)
	}
}
func FuzzBundle(f *testing.F) {
	f.Add([]byte("bad archive"))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ReadArchive(bytes.NewReader(b)) })
}
func FuzzManifest(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = ParseManifest(b) })
}
