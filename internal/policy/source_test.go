package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"normgate.dev/normgate/internal/normalization"
)

func TestHTTPFailureModes(t *testing.T) {
	raw := []byte("bundle")
	ctx := context.Background()
	mode := "ok"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode {
		case "redirect":
			http.Redirect(w, r, "https://forbidden.invalid", http.StatusFound)
		case "error":
			w.WriteHeader(503)
		case "not_modified":
			w.WriteHeader(304)
		case "interrupted":
			w.Header().Set("Content-Length", "999")
			w.Write(raw)
		case "oversized":
			w.Write(bytes.Repeat([]byte("x"), MaxBundleBytes+65537))
		case "long_etag":
			w.Header().Set("ETag", strings.Repeat("x", 1025))
			w.Write(raw)
		default:
			w.Header().Set("ETag", "one")
			w.Write(raw)
		}
	}))
	defer server.Close()
	source, err := NewHTTPSource(server.URL, normalization.Digest(raw), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if source.ID() == "" {
		t.Fatal("identity")
	}
	for _, value := range []string{"redirect", "error", "not_modified", "interrupted", "oversized", "long_etag"} {
		mode = value
		if _, err := source.Fetch(ctx, ""); err == nil {
			t.Fatal(value)
		}
	}
	mode = "ok"
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := source.Fetch(c, ""); err == nil {
		t.Fatal("canceled")
	}
	for _, address := range []string{"http://example.test", "https://user:pass@example.test", "https://example.test?secret=x", "https://example.test#fragment", ":invalid", "https:///missing"} {
		if _, err := NewHTTPSource(address, normalization.Digest(raw), nil); err == nil {
			t.Fatal(address)
		}
	}
	if _, err := NewHTTPSource(server.URL, "invalid", nil); err == nil {
		t.Fatal("bad pin")
	}
	if _, err := NewHTTPSource(server.URL, normalization.Digest(raw), &http.Client{Timeout: time.Hour}); err != nil {
		t.Fatal(err)
	}
	broken, _ := NewHTTPSource(server.URL, normalization.Digest(raw), &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) { return nil, errors.New("private network error") })})
	if _, err := broken.Fetch(ctx, ""); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal(err)
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestFileFailureModes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	name := filepath.Join(dir, "bundle")
	os.WriteFile(name, []byte("bundle"), 0600)
	source := FileSource{Path: name}
	r, err := source.Fetch(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := source.Fetch(ctx, r.ETag); err != nil || !got.NotModified {
		t.Fatal(err)
	}
	if source.ID() == "" {
		t.Fatal("id")
	}
	for _, s := range []FileSource{{Path: "relative"}, {Path: filepath.Join(dir, "missing")}, {Path: dir}, {Path: name, Checksum: "wrong"}} {
		if _, err := s.Fetch(ctx, ""); err == nil {
			t.Fatal(s)
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := source.Fetch(c, ""); err == nil {
		t.Fatal("cancel")
	}
	os.WriteFile(name, bytes.Repeat([]byte("x"), MaxBundleBytes+65537), 0600)
	if _, err := source.Fetch(ctx, ""); err == nil {
		t.Fatal("size")
	}
}
func TestOCIProtocol(t *testing.T) {
	blob := []byte("policy archive")
	digest := normalization.Digest(blob)
	manifest := func(media string, size int, dig string) []byte {
		raw, _ := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json", "artifactType": BundleMediaType, "layers": []any{map[string]any{"mediaType": media, "digest": dig, "size": size}}})
		return raw
	}
	body := manifest(BundleMediaType, len(blob), digest)
	etag := "one"
	blobFail := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/manifests/") {
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(304)
				return
			}
			w.Header().Set("ETag", etag)
			w.Write(body)
			return
		}
		if blobFail {
			w.WriteHeader(500)
			return
		}
		w.Write(blob)
	}))
	defer server.Close()
	makeSource := func() *OCISource {
		s, err := NewOCISource(server.URL, "example/policy", normalization.Digest(body), server.Client())
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	source := makeSource()
	got, err := source.Fetch(context.Background(), "")
	if err != nil || !bytes.Equal(got.Bytes, blob) || got.ETag != etag || source.ID() == "" {
		t.Fatal(got, err)
	}
	if got, err = source.Fetch(context.Background(), etag); err != nil || !got.NotModified {
		t.Fatal("etag", err)
	}
	for _, bad := range [][]byte{[]byte(`{}`), []byte(`bad`), manifest("wrong", len(blob), digest), manifest(BundleMediaType, 0, digest), manifest(BundleMediaType, len(blob)+1, digest), manifest(BundleMediaType, len(blob), "bad")} {
		body = bad
		if _, err := makeSource().Fetch(context.Background(), ""); err == nil {
			t.Fatal("bad manifest", string(body))
		}
	}
	body = manifest(BundleMediaType, len(blob), digest)
	blobFail = true
	if _, err := makeSource().Fetch(context.Background(), ""); err == nil {
		t.Fatal("blob unavailable")
	}
	blobFail = false
	source = makeSource()
	body = []byte("tampered")
	if _, err := source.Fetch(context.Background(), ""); err == nil {
		t.Fatal("manifest pin")
	}
	for _, args := range [][3]string{{server.URL + "/path", "example", "sha256:" + repeatZero(64)}, {server.URL, "../path", digest}, {"http://example.test", "example", digest}, {"%", "example", digest}} {
		if _, err := NewOCISource(args[0], args[1], args[2], nil); err == nil {
			t.Fatal(args)
		}
	}
}
