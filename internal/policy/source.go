package policy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"normgate.dev/normgate/internal/normalization"
)

type FetchResult struct {
	Bytes       []byte
	ETag        string
	NotModified bool
}
type Source interface {
	ID() string
	Fetch(context.Context, string) (FetchResult, error)
}
type FileSource struct {
	Path     string
	Checksum string
}

func (s FileSource) ID() string { return "file:" + s.Path + "#" + s.Checksum }
func (s FileSource) Fetch(ctx context.Context, etag string) (FetchResult, error) {
	if ctx.Err() != nil {
		return FetchResult{}, ctx.Err()
	}
	if !filepath.IsAbs(s.Path) {
		return FetchResult{}, Failure("invalid_source")
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return FetchResult{}, Failure("source_unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return FetchResult{}, Failure("invalid_source")
	}
	raw, err := readBounded(f, MaxBundleBytes+65536)
	if err != nil {
		return FetchResult{}, err
	}
	digest := normalization.Digest(raw)
	if s.Checksum != "" && s.Checksum != digest {
		return FetchResult{}, Failure("checksum_mismatch")
	}
	if ctx.Err() != nil {
		return FetchResult{}, ctx.Err()
	}
	return FetchResult{Bytes: raw, ETag: digest, NotModified: etag == digest}, nil
}

type HTTPSource struct {
	url      string
	checksum string
	client   *http.Client
}

func NewHTTPSource(address, checksum string, client *http.Client) (*HTTPSource, error) {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !digestPattern.MatchString(checksum) {
		return nil, Failure("invalid_source")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	safe := *client
	safe.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if safe.Timeout <= 0 || safe.Timeout > 30*time.Second {
		safe.Timeout = 30 * time.Second
	}
	return &HTTPSource{url: address, checksum: checksum, client: &safe}, nil
}
func (s *HTTPSource) ID() string { return s.url + "#" + s.checksum }
func (s *HTTPSource) Fetch(ctx context.Context, etag string) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, Failure("invalid_source")
	}
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json, application/octet-stream")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	response, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return FetchResult{}, ctx.Err()
		}
		return FetchResult{}, Failure("source_unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		if etag == "" {
			return FetchResult{}, Failure("invalid_response")
		}
		return FetchResult{ETag: etag, NotModified: true}, nil
	}
	if response.StatusCode != http.StatusOK {
		return FetchResult{}, Failure("source_unavailable")
	}
	raw, err := readBounded(response.Body, MaxBundleBytes+65536)
	if err != nil {
		return FetchResult{}, err
	}
	if normalization.Digest(raw) != s.checksum {
		return FetchResult{}, Failure("checksum_mismatch")
	}
	if len(response.Header.Get("ETag")) > 1024 {
		return FetchResult{}, Failure("invalid_response")
	}
	return FetchResult{Bytes: raw, ETag: response.Header.Get("ETag")}, nil
}

// OCISource implements the immutable OCI distribution manifest/blob read protocol.
// Trust is checksum-pinned. Signing identity is metadata, not remote signature trust.
type OCISource struct {
	manifest   *HTTPSource
	registry   string
	repository string
}

const BundleMediaType = "application/vnd.normgate.policy.v1+tar+gzip"

var repositoryPattern = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*$`)

func NewOCISource(registry, repository, digest string, client *http.Client) (*OCISource, error) {
	registry = strings.TrimSuffix(registry, "/")
	u, err := url.Parse(registry)
	if err != nil || u.Path != "" || !repositoryPattern.MatchString(repository) {
		return nil, Failure("invalid_source")
	}
	source, err := NewHTTPSource(registry+"/v2/"+repository+"/manifests/"+digest, digest, client)
	if err != nil {
		return nil, err
	}
	return &OCISource{manifest: source, registry: registry, repository: repository}, nil
}
func (s *OCISource) ID() string { return "oci:" + s.manifest.ID() }
func (s *OCISource) Fetch(ctx context.Context, etag string) (FetchResult, error) {
	result, err := s.manifest.Fetch(ctx, etag)
	if err != nil || result.NotModified {
		return result, err
	}
	var manifest struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
		ArtifactType  string `json:"artifactType"`
		Layers        []struct {
			MediaType string `json:"mediaType"`
			Digest    string `json:"digest"`
			Size      int64  `json:"size"`
		} `json:"layers"`
	}
	// OCI allows annotations and config descriptors; these do not grant authority.
	if _, err := normalization.Decode(result.Bytes); err != nil {
		return FetchResult{}, Failure("invalid_oci_manifest")
	}
	if json.Unmarshal(result.Bytes, &manifest) != nil || manifest.SchemaVersion != 2 || manifest.MediaType != "application/vnd.oci.image.manifest.v1+json" || manifest.ArtifactType != BundleMediaType || len(manifest.Layers) != 1 {
		return FetchResult{}, Failure("invalid_oci_manifest")
	}
	layer := manifest.Layers[0]
	if layer.MediaType != BundleMediaType || layer.Size <= 0 || layer.Size > MaxBundleBytes+65536 {
		return FetchResult{}, Failure("invalid_oci_manifest")
	}
	blob, err := NewHTTPSource(s.registry+"/v2/"+s.repository+"/blobs/"+layer.Digest, layer.Digest, s.manifest.client)
	if err != nil {
		return FetchResult{}, err
	}
	body, err := blob.Fetch(ctx, "")
	if err != nil {
		return FetchResult{}, err
	}
	if int64(len(body.Bytes)) != layer.Size {
		return FetchResult{}, Failure("checksum_mismatch")
	}
	body.ETag = result.ETag
	return body, nil
}

// ReadEvent is shared by the CLI and tests so malformed wire fields are never
// silently dropped when decoding into the canonical Go transport type.
func ReadEvent(reader io.Reader) (json.RawMessage, error) {
	raw, err := readBounded(reader, normalization.MaxBytes)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}
