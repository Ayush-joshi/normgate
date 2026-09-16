package config_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"normgate.dev/normgate/internal/config"
)

const valid = `{"schema_version":"1.0","listen_address":"127.0.0.1:8080","policy_source":"file:///etc/normgate/policy","request_timeout_ms":5000,"fail_closed":true,"api_key":"${ENV:NORMGATE_API_KEY}"}`

func TestNGF005LoadAndRedact(t *testing.T) {
	t.Setenv("NORMGATE_API_KEY", "secret-value-never-log")
	for _, input := range []string{valid, "schema_version: '1.0'\nlisten_address: '127.0.0.1:8080'\npolicy_source: 'https://policies.example.test/bundle'\nrequest_timeout_ms: 5000\nfail_closed: true\napi_key: '${ENV:NORMGATE_API_KEY}'\n"} {
		cfg, err := config.Load([]byte(input), nil)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.APIKey() != "secret-value-never-log" || cfg.RequestTimeoutMS != 5000 {
			t.Fatal("NG-F005: references not resolved")
		}
		data, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, output := range []string{string(data), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg)} {
			if strings.Contains(output, "secret-value") {
				t.Fatal("NG-F005: diagnostic secret leak")
			}
		}
	}
}

func TestNGF005PrecedenceAndFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("file-secret-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input := strings.Replace(valid, "${ENV:NORMGATE_API_KEY}", "${FILE:"+path+"}", 1)
	cfg, err := config.Load([]byte(input), map[string]any{"request_timeout_ms": 1000})
	if err != nil || cfg.APIKey() != "file-secret-value" || cfg.RequestTimeoutMS != 1000 {
		t.Fatalf("NG-F005: file reference: %v", err)
	}
	cfg, err = config.Load([]byte(valid), map[string]any{"api_key": "flag-secret-value"})
	if err != nil || cfg.APIKey() != "flag-secret-value" {
		t.Fatalf("flag must override unresolved reference: %v", err)
	}
}

func TestNGF005Invalid(t *testing.T) {
	t.Setenv("NORMGATE_API_KEY", "secret-value-never-log")
	cases := []string{
		strings.Replace(valid, `"fail_closed":true`, `"fail_closed":false`, 1),
		strings.Replace(valid, `5000`, `0`, 1), strings.Replace(valid, `5000`, `60001`, 1),
		strings.Replace(valid, `5000`, `1.5`, 1),
		strings.Replace(valid, `file:///etc/normgate/policy`, `http://remote.test/policy`, 1),
		strings.Replace(valid, `file:///etc/normgate/policy`, `file://remote/etc/policy`, 1),
		strings.Replace(valid, `file:///etc/normgate/policy`, `https://user:password@remote.test/policy`, 1),
		strings.Replace(valid, `127.0.0.1:8080`, `nonsense`, 1),
		strings.Replace(valid, `127.0.0.1:8080`, `127.0.0.1:99999`, 1),
		strings.Replace(valid, `127.0.0.1:8080`, `127.0.0.1:0`, 1),
		strings.Replace(valid, `${ENV:NORMGATE_API_KEY}`, `${ENV:MISSING_NORMGATE_TEST_KEY}`, 1),
		strings.Replace(valid, `${ENV:NORMGATE_API_KEY}`, `${FILE:/nonexistent/normgate-secret}`, 1),
		strings.Replace(valid, `${ENV:NORMGATE_API_KEY}`, `${FILE:relative}`, 1),
		strings.Replace(valid, `${ENV:NORMGATE_API_KEY}`, `prefix-${ENV:NORMGATE_API_KEY}`, 1),
		strings.Replace(valid, `"schema_version"`, `"unexpected"`, 1),
		valid + ` {}`, `{"api_key":"a","api_key":"b"}`, "api_key: x\napi_key: y", "[]", "", "api_key: &key secret\nother: *key", "a: [", "api_key: !!binary YWJjZA==", "api_key: a\n---\napi_key: b",
	}
	for i, input := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, err := config.Load([]byte(input), nil)
			if err == nil {
				t.Fatal("NG-F005: unsafe config accepted")
			}
			if strings.Contains(err.Error(), "secret-value") || strings.Contains(err.Error(), "password") {
				t.Fatal("error leaked a secret")
			}
		})
	}
	if _, err := config.Load(make([]byte, (1<<20)+1), nil); err == nil {
		t.Fatal("unbounded config")
	}
}

func FuzzNGF005Load(f *testing.F) {
	f.Add(valid)
	f.Add("{}")
	f.Add("x: &x [*x]")
	f.Fuzz(func(t *testing.T, input string) { _, _ = config.Load([]byte(input), nil) })
}

func TestNGF005StandaloneValidation(t *testing.T) {
	t.Setenv("NORMGATE_API_KEY", "secret-value-never-log")
	cfg, err := config.Load([]byte(valid), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.SchemaVersion = "2.0"
	if err := config.Validate(cfg); err == nil {
		t.Fatal("NG-F005: standalone validation accepted unsupported version")
	}
}
