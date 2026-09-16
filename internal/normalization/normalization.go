// Package normalization defines NormGate canonical JSON profile v1.
package normalization

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const MaxBytes = 1 << 20
const MaxDepth = 64
const MaxInteger int64 = 9007199254740991

var timestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

// Profile paths are RFC 6901 pointers chosen by a versioned protocol, never callers.
// Sets contain strings only. All other arrays preserve their order.
type Profile struct {
	Sets       []string `json:"sets,omitempty"`
	Timestamps []string `json:"timestamps,omitempty"`
	Omit       []string `json:"omit,omitempty"`
}

// Error reports an invalid canonical input without embedding its contents.
type Error struct{ Code string }

func (e *Error) Error() string  { return "canonical JSON: " + e.Code }
func invalid(code string) error { return &Error{Code: code} }

// Decode rejects lossy and ambiguous representations before any schema validation.
func Decode(data []byte) (any, error) {
	if len(data) > MaxBytes {
		return nil, invalid("size_limit")
	}
	if !utf8.Valid(data) || !validSurrogates(data) {
		return nil, invalid("invalid_unicode")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := readValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, invalid("trailing_data")
	}
	return v, nil
}

func readValue(dec *json.Decoder, depth int) (any, error) {
	if depth > MaxDepth {
		return nil, invalid("depth_limit")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, invalid("invalid_json")
	}
	switch value := tok.(type) {
	case json.Delim:
		switch value {
		case '{':
			m := make(map[string]any)
			seen := make(map[string]bool)
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, invalid("invalid_json")
				}
				k, ok := key.(string)
				if !ok {
					return nil, invalid("invalid_key")
				}
				if err := checkString(k); err != nil {
					return nil, err
				}
				n := norm.NFC.String(k)
				if seen[n] {
					return nil, invalid("duplicate_key")
				}
				seen[n] = true
				v, err := readValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				m[k] = v
			}
			if end, err := dec.Token(); err != nil || end != json.Delim('}') {
				return nil, invalid("invalid_json")
			}
			return m, nil
		case '[':
			a := make([]any, 0)
			for dec.More() {
				v, err := readValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			if end, err := dec.Token(); err != nil || end != json.Delim(']') {
				return nil, invalid("invalid_json")
			}
			return a, nil
		}
	case json.Number:
		n, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil || n < -MaxInteger || n > MaxInteger {
			return nil, invalid("unsafe_number")
		}
		return json.Number(strconv.FormatInt(n, 10)), nil
	case string:
		if err := checkString(value); err != nil {
			return nil, err
		}
		return value, nil
	case bool, nil:
		return value, nil
	}
	return nil, invalid("invalid_json")
}

// Bound decomposed mark runs before NFC. x/text inserts CGJ after 30 nonstarters;
// Python and JavaScript do not. A shared limit prevents divergent signed bytes.
func checkString(value string) error {
	run := 0
	for _, r := range norm.NFD.String(value) {
		if unicode.Is(unicode.M, r) {
			run++
		} else {
			run = 0
		}
		if run > 30 {
			return invalid("unicode_complexity")
		}
	}
	return nil
}

// encoding/json replaces invalid surrogate escapes, which is unsafe for signatures.
func validSurrogates(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		i++
		if i >= len(data) {
			return false
		}
		if data[i] != 'u' {
			continue
		}
		if i+4 >= len(data) {
			return false
		}
		n, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return false
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}

func Canonicalize(data []byte, profile Profile) ([]byte, error) {
	v, err := Decode(data)
	if err != nil {
		return nil, err
	}
	v, err = normalize(v, "", profile)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encode(&out, v)
	return out.Bytes(), nil
}

func normalize(v any, path string, profile Profile) (any, error) {
	if slices.Contains(profile.Timestamps, path) {
		s, ok := v.(string)
		if !ok || !timestampPattern.MatchString(s) {
			return nil, invalid("invalid_timestamp")
		}
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil || t.Year() < 1 || t.Year() > 9999 || t.UTC().Year() < 1 || t.UTC().Year() > 9999 {
			return nil, invalid("invalid_timestamp")
		}
		return t.UTC().Format(time.RFC3339Nano), nil
	}
	if slices.Contains(profile.Sets, path) {
		a, ok := v.([]any)
		if !ok {
			return nil, invalid("invalid_set")
		}
		values := make([]string, len(a))
		for i, item := range a {
			s, ok := item.(string)
			if !ok {
				return nil, invalid("invalid_set")
			}
			values[i] = norm.NFC.String(s)
		}
		slices.Sort(values)
		for i, s := range values {
			if i > 0 && s == values[i-1] {
				return nil, invalid("duplicate_set_value")
			}
			a[i] = s
		}
		return a, nil
	}
	switch value := v.(type) {
	case string:
		return norm.NFC.String(value), nil
	case map[string]any:
		m := make(map[string]any)
		for key, item := range value {
			key = norm.NFC.String(key)
			next := path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
			if slices.Contains(profile.Omit, next) {
				continue
			}
			n, err := normalize(item, next, profile)
			if err != nil {
				return nil, err
			}
			m[key] = n
		}
		return m, nil
	case []any:
		for i, item := range value {
			n, err := normalize(item, path+"/"+strconv.Itoa(i), profile)
			if err != nil {
				return nil, err
			}
			value[i] = n
		}
		return value, nil
	default:
		return value, nil
	}
}

func encode(out *bytes.Buffer, value any) {
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(v))
	case json.Number:
		out.WriteString(string(v))
	case string:
		quote(out, v)
	case []any:
		out.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				out.WriteByte(',')
			}
			encode(out, item)
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			quote(out, key)
			out.WriteByte(':')
			encode(out, v[key])
		}
		out.WriteByte('}')
	}
}

func quote(out *bytes.Buffer, value string) {
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
}

// Digest hashes canonical bytes; callers must canonicalize successfully first.
func Digest(canonical []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(canonical)) }
