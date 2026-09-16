package policy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	v1 "normgate.dev/normgate/internal/contracts/v1"
)

type Status struct {
	Ready       bool      `json:"ready"`
	Revision    Revision  `json:"revision"`
	Previous    Revision  `json:"previous,omitempty"`
	Source      string    `json:"source"`
	ETag        string    `json:"etag,omitempty"`
	RefreshedAt time.Time `json:"refreshed_at"`
	LastError   string    `json:"last_error,omitempty"`
}
type Manager struct {
	engine       Engine
	dir          string
	maxStaleness time.Duration
	mu           sync.Mutex
	active       atomic.Pointer[Status]
	now          func() time.Time
}

func NewManager(engine Engine, dir string, maxStaleness time.Duration) (*Manager, error) {
	if engine == nil || maxStaleness <= 0 || dir == "" {
		return nil, Failure("invalid_manager")
	}
	if os.MkdirAll(dir, 0700) != nil {
		return nil, Failure("persistence_failed")
	}
	m := &Manager{engine: engine, dir: dir, maxStaleness: maxStaleness, now: time.Now}
	m.active.Store(&Status{})
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, Failure("persistence_failed")
	}
	var state Status
	if strictJSON(raw, &state) != nil || !digestPattern.MatchString(string(state.Revision)) || state.RefreshedAt.IsZero() || (state.Previous != "" && !digestPattern.MatchString(string(state.Previous))) {
		return nil, Failure("invalid_state")
	}
	// The primary state is durable before readers switch. If its artifact cannot
	// recover, the exact previous healthy revision is the only permitted fallback.
	revision, err := m.recover(context.Background(), state.Revision)
	if err != nil && state.Previous != "" {
		revision, err = m.recover(context.Background(), state.Previous)
		if err == nil {
			state.Revision = revision
			state.Previous = ""
			state.ETag = ""
			state.LastError = "ng.policy.recovered_previous"
		}
	}
	if err != nil {
		return nil, err
	}
	state.Ready = true
	if err = m.persist(state); err != nil {
		return nil, err
	}
	m.active.Store(&state)
	return m, nil
}
func (m *Manager) artifactPath(revision Revision) string {
	return filepath.Join(m.dir, strings.TrimPrefix(string(revision), "sha256:")+".tar.gz")
}
func (m *Manager) recover(ctx context.Context, revision Revision) (Revision, error) {
	f, err := os.Open(m.artifactPath(revision))
	if err != nil {
		return "", Failure("recovery_failed")
	}
	defer f.Close()
	bundle, err := ReadArchive(f)
	if err != nil {
		return "", err
	}
	r, err := m.validate(ctx, bundle)
	if err != nil {
		return "", err
	}
	if r != revision {
		return "", Failure("checksum_mismatch")
	}
	return r, nil
}
func (m *Manager) validate(ctx context.Context, bundle Bundle) (Revision, error) {
	r, err := m.engine.Compile(ctx, bundle)
	if err != nil {
		return "", err
	}
	if !m.engine.Health(ctx).Ready {
		return "", Failure("unhealthy_candidate")
	}
	if _, err = RunTests(ctx, m.engine, r, bundle); err != nil {
		return "", err
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return r, nil
}
func (m *Manager) Status() Status {
	s := *m.active.Load()
	elapsed := m.now().Sub(s.RefreshedAt)
	s.Ready = s.Ready && s.Revision != "" && elapsed >= 0 && elapsed < m.maxStaleness && m.engine.Health(context.Background()).Ready
	if !s.Ready && s.Revision != "" && s.LastError == "" {
		s.LastError = "ng.policy.stale"
	}
	return s
}
func (m *Manager) Evaluate(ctx context.Context, event v1.EventEnvelope) (v1.Decision, error) {
	s := m.Status()
	if !s.Ready {
		return v1.Decision{}, Failure("stale")
	}
	return m.engine.Evaluate(ctx, s.Revision, event)
}
func (m *Manager) recordError(err error) error {
	state := *m.active.Load()
	state.LastError = "ng.policy.activation_failed"
	if typed, ok := err.(*Error); ok {
		state.LastError = typed.Code
	}
	m.active.Store(&state)
	return err
}
func (m *Manager) Activate(ctx context.Context, archive []byte, source, etag string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activate(ctx, archive, source, etag)
}
func (m *Manager) activate(ctx context.Context, archive []byte, source, etag string) error {
	bundle, err := ReadArchive(bytes.NewReader(archive))
	if err != nil {
		return m.recordError(err)
	}
	revision, err := m.validate(ctx, bundle)
	if err != nil {
		return m.recordError(err)
	}
	state := *m.active.Load()
	previous := state.Revision
	if previous == revision {
		previous = state.Previous
	}
	next := Status{Ready: true, Revision: revision, Previous: previous, Source: source, ETag: etag, RefreshedAt: m.now().UTC()}
	name := m.artifactPath(revision)
	if existing, err := os.ReadFile(name); err == nil {
		b, err := ReadArchive(bytes.NewReader(existing))
		if err != nil {
			return m.recordError(err)
		}
		report, err := Inspect(b)
		if err != nil || report.Revision != revision {
			return m.recordError(Failure("checksum_mismatch"))
		}
	} else if os.IsNotExist(err) {
		if err = atomicWrite(name, archive); err != nil {
			return m.recordError(err)
		}
	} else {
		return m.recordError(Failure("persistence_failed"))
	}
	if ctx.Err() != nil {
		return m.recordError(ctx.Err())
	}
	if err = m.persist(next); err != nil {
		return m.recordError(err)
	}
	m.active.Store(&next)
	return nil
}
func (m *Manager) persist(state Status) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return Failure("persistence_failed")
	}
	return atomicWrite(filepath.Join(m.dir, "state.json"), raw)
}
func atomicWrite(name string, raw []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".staging-*")
	if err != nil {
		return Failure("persistence_failed")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err != nil {
		return Failure("persistence_failed")
	}
	if f.Sync() != nil {
		return Failure("persistence_failed")
	}
	if f.Close() != nil {
		return Failure("persistence_failed")
	}
	if os.Rename(f.Name(), name) != nil {
		return Failure("persistence_failed")
	}
	dir, err := os.Open(filepath.Dir(name))
	if err != nil {
		return Failure("persistence_failed")
	}
	defer dir.Close()
	if dir.Sync() != nil {
		return Failure("persistence_failed")
	}
	return nil
}
func (m *Manager) Rollback(ctx context.Context, revision Revision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !digestPattern.MatchString(string(revision)) {
		return m.recordError(Failure("unknown_revision"))
	}
	f, err := os.Open(m.artifactPath(revision))
	if err != nil {
		return m.recordError(Failure("unknown_revision"))
	}
	defer f.Close()
	raw, err := readBounded(f, MaxBundleBytes+65536)
	if err != nil {
		return m.recordError(err)
	}
	// Clear the remote ETag: a subsequent refresh must re-verify the source bytes.
	return m.activate(ctx, raw, "rollback:"+string(revision), "")
}
func (m *Manager) Refresh(ctx context.Context, source Source) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if source == nil {
		return m.recordError(Failure("invalid_source"))
	}
	state := *m.active.Load()
	etag := ""
	if state.Source == source.ID() {
		etag = state.ETag
	}
	fetched, err := source.Fetch(ctx, etag)
	if err != nil {
		return m.recordError(err)
	}
	if fetched.NotModified {
		if state.Revision == "" || state.Source != source.ID() || etag == "" {
			return m.recordError(Failure("invalid_response"))
		}
		if ctx.Err() != nil {
			return m.recordError(ctx.Err())
		}
		state.RefreshedAt = m.now().UTC()
		state.LastError = ""
		if err = m.persist(state); err != nil {
			return m.recordError(err)
		}
		m.active.Store(&state)
		return nil
	}
	return m.activate(ctx, fetched.Bytes, source.ID(), fetched.ETag)
}

// Run refreshes immediately, then uses bounded exponential retry with jitter.
// Failure is visible through Status; cached decisions stop at maxStaleness.
func (m *Manager) Run(ctx context.Context, source Source, interval, maxBackoff time.Duration) error {
	if interval <= 0 || maxBackoff < interval {
		return Failure("invalid_refresh")
	}
	delay := interval
	for {
		err := m.Refresh(ctx, source)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		wait := interval
		if err != nil {
			wait = jitter(delay)
			if delay > maxBackoff/2 {
				delay = maxBackoff
			} else {
				delay *= 2
			}
		} else {
			delay = interval
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func jitter(delay time.Duration) time.Duration {
	n, err := rand.Int(rand.Reader, big.NewInt(max(int64(delay/4), 1)))
	if err != nil {
		return delay
	}
	return delay - time.Duration(n.Int64())
}
