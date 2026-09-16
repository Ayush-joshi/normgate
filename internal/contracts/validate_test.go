package contracts

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"testing/fstest"

	v1 "normgate.dev/normgate/internal/contracts/v1"
)

func TestNGF002SafeDecode(t *testing.T) {
	data, err := os.ReadFile("../../testkit/fixtures/contracts/valid/principal.basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct{ Value json.RawMessage }
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	value, err := Decode[v1.Principal]("principal", fixture.Value)
	if err != nil || value.Id != "principal-1" {
		t.Fatalf("decode: %v", err)
	}
	if _, err := Decode[int]("principal", fixture.Value); err == nil {
		t.Fatal("type mismatch accepted")
	}
	if v, err := Decode[v1.Principal]("principal", []byte(`{}`)); err == nil || v.Id != "" {
		t.Fatal("partial invalid object")
	}
	for _, tc := range []struct{ name, input, code string }{
		{"unknown", `{}`, "unknown_schema"}, {"principal", `{`, "invalid_json"},
		{"principal", `{"schema_version":"2.0"}`, "unsupported_version"},
		{"principal", `{}`, "invalid_contract"},
	} {
		err := Validate(tc.name, []byte(tc.input))
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != tc.code || err.Error() == "" {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestNGF002InvalidSchemaBundleFailsClosed(t *testing.T) {
	for _, fsys := range []fstest.MapFS{
		{},
		{"v1/bad.schema.json": &fstest.MapFile{Data: []byte(`{`)}},
		{"v1/bad.schema.json": &fstest.MapFile{Data: []byte(`{"type":"nonsense"}`)}},
		{"v1/bad.schema.json": &fstest.MapFile{Data: []byte(`{"$ref":"https://untrusted.example.test/schema.json"}`)}},
	} {
		if _, err := compileBundle(fsys); err == nil {
			t.Fatal("NG-F002: invalid or remote schema accepted")
		}
	}
}

func FuzzNGF002Validate(f *testing.F) {
	f.Add(`{}`)
	f.Add(`{"schema_version":"2.0"}`)
	f.Fuzz(func(t *testing.T, input string) { _ = Validate("event-envelope", []byte(input)) })
}
