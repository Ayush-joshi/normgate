package policy_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/normalization"
	"normgate.dev/normgate/internal/policy"
	"normgate.dev/normgate/internal/policy/opa"
)

func baseline(t *testing.T) policy.Bundle {
	t.Helper()
	b, e := policy.LoadDir("../../policies/baseline")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func input(t *testing.T) v1.EventEnvelope {
	t.Helper()
	raw, e := os.ReadFile("../../testkit/fixtures/contracts/valid/event-envelope.basic.json")
	if e != nil {
		t.Fatal(e)
	}
	var x struct {
		Value v1.EventEnvelope `json:"value"`
	}
	if json.Unmarshal(raw, &x) != nil {
		t.Fatal("fixture")
	}
	return x.Value
}
func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	e := opa.New()
	defer e.Close()
	dir := t.TempDir()
	m, err := policy.NewManager(e, dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	b := baseline(t)
	raw, err := policy.Build(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Activate(ctx, raw, "local", "v1"); err != nil {
		t.Fatal(err)
	}
	first := m.Status().Revision
	var wg sync.WaitGroup
	var successes atomic.Int64
	for range 8 {
		wg.Go(func() {
			for range 20 {
				d, err := m.Evaluate(ctx, input(t))
				if err != nil {
					if !errors.Is(err, context.DeadlineExceeded) || d.Outcome != "" || d.Id != "" {
						t.Errorf("reader observed unsafe failure: %+v %v", d, err)
					}
					continue
				}
				if d.Outcome != "allow" {
					t.Errorf("reader observed invalid candidate: %+v", d)
				}
				successes.Add(1)
			}
		})
	}
	// An authoritative change with identical semantics is still a distinct revision.
	b.Files["data.json"] = append(b.Files["data.json"], ' ')
	next, err := policy.Build(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Activate(ctx, next, "local", "v2"); err != nil {
		t.Fatal(err)
	}
	second := m.Status().Revision
	if first == second {
		t.Fatal("revision unchanged")
	}
	wg.Wait()
	if successes.Load() == 0 {
		t.Fatal("no concurrent evaluations completed")
	}
	if err = m.Activate(ctx, []byte("interrupted"), "local", ""); err == nil || m.Status().Revision != second {
		t.Fatal("bad candidate replaced active")
	}
	e2 := opa.New()
	defer e2.Close()
	restarted, err := policy.NewManager(e2, dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Status().Revision != second {
		t.Fatal("restart lost active")
	}
	if err = restarted.Rollback(ctx, first); err != nil {
		t.Fatal(err)
	}
	a, err := restarted.Evaluate(ctx, input(t))
	if err != nil || a.PolicyRevision != string(first) {
		t.Fatal("rollback not exact", err)
	}
	if err = restarted.Rollback(ctx, "bad/path"); err == nil {
		t.Fatal("invalid rollback")
	}
}
func TestSources(t *testing.T) {
	ctx := context.Background()
	raw, err := policy.Build(baseline(t))
	if err != nil {
		t.Fatal(err)
	}
	etag := `"revision"`
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Write(raw)
	}))
	defer server.Close()
	source, err := policy.NewHTTPSource(server.URL, normalization.Digest(raw), server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := source.Fetch(ctx, "")
	if err != nil || !bytes.Equal(got.Bytes, raw) {
		t.Fatal(err)
	}
	got, err = source.Fetch(ctx, etag)
	if err != nil || !got.NotModified || calls != 2 {
		t.Fatal("etag", err)
	}
	bad, _ := policy.NewHTTPSource(server.URL, "sha256:"+string(bytes.Repeat([]byte("0"), 64)), server.Client())
	if _, err = bad.Fetch(ctx, ""); err == nil {
		t.Fatal("checksum accepted")
	}
	if _, err = policy.NewHTTPSource("http://insecure.test", normalization.Digest(raw), nil); err == nil {
		t.Fatal("insecure source")
	}
	name := filepath.Join(t.TempDir(), "bundle.tar.gz")
	if os.WriteFile(name, raw, 0600) != nil {
		t.Fatal("write")
	}
	local := policy.FileSource{Path: name, Checksum: normalization.Digest(raw)}
	if _, err = local.Fetch(ctx, ""); err != nil {
		t.Fatal(err)
	}
}
