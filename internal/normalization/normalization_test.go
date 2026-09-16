package normalization_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"normgate.dev/normgate/internal/normalization"
)

func TestNGF004GoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../testkit/fixtures/normalization/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name      string                `json:"name"`
		Input     string                `json:"input"`
		Profile   normalization.Profile `json:"profile"`
		Canonical string                `json:"canonical"`
		Digest    string                `json:"digest"`
		Invalid   bool                  `json:"invalid"`
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			got, err := normalization.Canonicalize([]byte(v.Input), v.Profile)
			if v.Invalid {
				var e *normalization.Error
				if !errors.As(err, &e) {
					t.Fatalf("NG-F004: expected typed error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != v.Canonical {
				t.Fatalf("canonical=%s want=%s", got, v.Canonical)
			}
			if digest := normalization.Digest(got); digest != v.Digest {
				t.Fatalf("digest=%s want=%s", digest, v.Digest)
			}
		})
	}
}

func FuzzNGF004Canonicalize(f *testing.F) {
	for _, s := range []string{`{"b":2,"a":1}`, `{"x":null}`, `[]`, `"é"`, `{"x":1,"x":2}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := normalization.Canonicalize([]byte(input), normalization.Profile{})
		if err != nil {
			return
		}
		again, err := normalization.Canonicalize(got, normalization.Profile{})
		if err != nil || string(got) != string(again) {
			t.Fatalf("NG-F004: not idempotent: %s -> %s (%v)", got, again, err)
		}
	})
}

func TestNGF004RejectInvalidUTF8BeforeNormalization(t *testing.T) {
	// Regression boundary for GO-2026-5970. Invalid bytes must never reach NFC.
	for _, input := range [][]byte{{'"', 0xff, '"'}, {'{', '"', 0xc0, 0xaf, '"', ':', '0', '}'}} {
		if _, err := normalization.Canonicalize(input, normalization.Profile{}); err == nil {
			t.Fatal("NG-F004: invalid UTF-8 accepted")
		}
	}
	if _, err := normalization.Canonicalize(make([]byte, normalization.MaxBytes+1), normalization.Profile{}); err == nil {
		t.Fatal("NG-F004: size limit not enforced")
	}
}
