// Package contracts validates untrusted objects against embedded v1 schemas.
package contracts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"normgate.dev/normgate/internal/normalization"
	"normgate.dev/normgate/schemas"
)

// Error contains only a stable code; schema diagnostics may contain sensitive values.
type Error struct{ Code string }

func (e *Error) Error() string { return "contract validation: " + e.Code }

type offlineLoader struct{}

func (offlineLoader) Load(_ string) (any, error) {
	return nil, fmt.Errorf("remote schema loading disabled")
}

var compiled = sync.OnceValues(func() (map[string]*jsonschema.Schema, error) { return compileBundle(schemas.Files) })

func compileBundle(files fs.FS) (map[string]*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	compiler.UseLoader(offlineLoader{})
	entries, err := fs.ReadDir(files, "v1")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		data, err := fs.ReadFile(files, "v1/"+entry.Name())
		if err != nil {
			return nil, err
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		if err := compiler.AddResource("https://normgate.dev/schemas/v1/"+entry.Name(), value); err != nil {
			return nil, err
		}
	}
	result := make(map[string]*jsonschema.Schema)
	for _, entry := range entries {
		schema, err := compiler.Compile("https://normgate.dev/schemas/v1/" + entry.Name())
		if err != nil {
			return nil, err
		}
		result[strings.TrimSuffix(entry.Name(), ".schema.json")] = schema
	}
	return result, nil
}

// Validate is mandatory before converting data into generated transport types.
func Validate(name string, data []byte) error {
	schemas, err := compiled()
	if err != nil {
		return &Error{Code: "schema_unavailable"}
	}
	schema, ok := schemas[name]
	if !ok {
		return &Error{Code: "unknown_schema"}
	}
	value, err := normalization.Decode(data)
	if err != nil {
		return &Error{Code: "invalid_json"}
	}
	if obj, ok := value.(map[string]any); ok {
		if version, ok := obj["schema_version"].(string); ok && !strings.HasPrefix(version, "1.") {
			return &Error{Code: "unsupported_version"}
		}
	}
	if err := schema.Validate(value); err != nil {
		return &Error{Code: "invalid_contract"}
	}
	return nil
}

// Decode validates before decoding. No partial object is returned on failure.
func Decode[T any](name string, data []byte) (T, error) {
	var value T
	if err := Validate(name, data); err != nil {
		return value, err
	}
	if err := json.Unmarshal(data, &value); err != nil {
		var zero T
		return zero, &Error{Code: "type_mismatch"}
	}
	return value, nil
}
