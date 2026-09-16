// Package config loads trusted operator configuration without contacting services.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
	"normgate.dev/normgate/internal/contracts"
	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/normalization"
)

// Config exposes non-secret settings. Explicit APIKey access is required for credentials.
type Config struct {
	SchemaVersion    string `json:"schema_version"`
	ListenAddress    string `json:"listen_address"`
	PolicySource     string `json:"policy_source"`
	RequestTimeoutMS int64  `json:"request_timeout_ms"`
	FailClosed       bool   `json:"fail_closed"`
	apiKey           string
}

func (c Config) APIKey() string   { return c.apiKey }
func (c Config) String() string   { data, _ := json.Marshal(c); return string(data) }
func (c Config) GoString() string { return c.String() }

// Error avoids exposing values, secret paths, environment names, or parser excerpts.
type Error struct{ Code string }

func (e *Error) Error() string  { return "configuration: " + e.Code }
func invalid(code string) error { return &Error{Code: code} }

var environmentReference = regexp.MustCompile(`^\$\{ENV:([A-Za-z_][A-Za-z0-9_]*)\}$`)
var fileReference = regexp.MustCompile(`^\$\{FILE:([^}]+)\}$`)

// Load applies explicit flag overrides before resolving references, then validates.
// References occupy the entire scalar and are expanded exactly once.
func Load(data []byte, overrides map[string]any) (Config, error) {
	var zero Config
	if len(data) > normalization.MaxBytes {
		return zero, invalid("size_limit")
	}
	var value any
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		parsed, err := normalization.Decode(data)
		if err != nil {
			return zero, invalid("invalid_json")
		}
		value = parsed
	} else {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var document yaml.Node
		if err := decoder.Decode(&document); err != nil {
			return zero, invalid("invalid_yaml")
		}
		if err := inspectYAML(&document, 0); err != nil {
			return zero, err
		}
		if err := document.Decode(&value); err != nil {
			return zero, invalid("invalid_yaml")
		}
		var extra yaml.Node
		if err := decoder.Decode(&extra); err != io.EOF {
			return zero, invalid("multiple_documents")
		}
	}
	object, ok := value.(map[string]any)
	if !ok {
		return zero, invalid("expected_object")
	}
	for key, value := range overrides {
		object[key] = value
	}
	for key, value := range object {
		if s, ok := value.(string); ok {
			resolved, err := resolve(s)
			if err != nil {
				return zero, err
			}
			object[key] = resolved
		}
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return zero, invalid("invalid_value")
	}
	raw, err := contracts.Decode[v1.Configuration]("configuration", encoded)
	if err != nil {
		return zero, invalid("invalid_configuration")
	}
	cfg := Config{SchemaVersion: raw.SchemaVersion, ListenAddress: raw.ListenAddress, PolicySource: raw.PolicySource, RequestTimeoutMS: raw.RequestTimeoutMs, FailClosed: raw.FailClosed, apiKey: raw.ApiKey}
	if err := Validate(cfg); err != nil {
		return zero, err
	}
	return cfg, nil
}

func inspectYAML(node *yaml.Node, depth int) error {
	if depth > normalization.MaxDepth {
		return invalid("depth_limit")
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return invalid("yaml_alias")
	}
	if node.Kind == yaml.ScalarNode {
		switch node.Tag {
		case "!!str", "!!int", "!!bool", "!!null":
		default:
			return invalid("yaml_tag")
		}
	}
	for _, child := range node.Content {
		if err := inspectYAML(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func resolve(value string) (string, error) {
	if match := environmentReference.FindStringSubmatch(value); match != nil {
		resolved, ok := os.LookupEnv(match[1])
		if !ok || resolved == "" {
			return "", invalid("missing_environment")
		}
		return resolved, nil
	}
	if match := fileReference.FindStringSubmatch(value); match != nil {
		if !filepath.IsAbs(match[1]) {
			return "", invalid("relative_secret_path")
		}
		file, err := os.Open(match[1])
		if err != nil {
			return "", invalid("secret_unavailable")
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
			return "", invalid("invalid_secret_file")
		}
		data, err := io.ReadAll(io.LimitReader(file, 65537))
		if err != nil || len(data) > 65536 {
			return "", invalid("secret_unavailable")
		}
		return strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r"), nil
	}
	if strings.Contains(value, "${") {
		return "", invalid("invalid_reference")
	}
	return value, nil
}

// Validate checks the full configuration; it never resolves references or dials a URL.
func Validate(cfg Config) error {
	data, err := json.Marshal(v1.Configuration{SchemaVersion: cfg.SchemaVersion, ListenAddress: cfg.ListenAddress, PolicySource: cfg.PolicySource, RequestTimeoutMs: cfg.RequestTimeoutMS, FailClosed: cfg.FailClosed, ApiKey: cfg.apiKey})
	if err != nil || contracts.Validate("configuration", data) != nil {
		return invalid("invalid_configuration")
	}
	_, port, err := net.SplitHostPort(cfg.ListenAddress)
	if err != nil {
		return invalid("invalid_listen_address")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return invalid("invalid_listen_address")
	}
	location, err := url.Parse(cfg.PolicySource)
	if err != nil || location.User != nil || location.Fragment != "" || location.RawQuery != "" {
		return invalid("invalid_policy_source")
	}
	switch location.Scheme {
	case "file":
		if location.Host != "" || !filepath.IsAbs(location.Path) {
			return invalid("invalid_policy_source")
		}
	case "https":
		if location.Hostname() == "" || location.Opaque != "" {
			return invalid("invalid_policy_source")
		}
	default:
		return invalid("invalid_policy_source")
	}
	return nil
}

// Ensure standard fmt formatting always uses the redacted representation.
var _ fmt.Stringer = Config{}
