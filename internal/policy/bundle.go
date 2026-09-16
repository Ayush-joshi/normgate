package policy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"normgate.dev/normgate/internal/contracts"
	v1 "normgate.dev/normgate/internal/contracts/v1"
	"normgate.dev/normgate/internal/normalization"
)

const MaxBundleBytes = 4 << 20
const MaxFiles = 256
const MaxModules = 64
const zeroDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

type Bundle struct{ Files map[string][]byte }
type Layer struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Priority  int    `json:"priority"`
}
type Reason struct {
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}
type Report struct {
	Manifest          v1.PolicyManifest `json:"manifest"`
	Revision          Revision          `json:"revision"`
	Files             []string          `json:"files"`
	Layers            []Layer           `json:"layers"`
	Reasons           map[string]Reason `json:"reasons"`
	SignatureVerified bool              `json:"signature_verified"`
}

var namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
var reasonPattern = regexp.MustCompile(`^ng\.(validation|authentication|policy|enforcement|storage|upstream|internal)\.[a-z][a-z0-9_]*$`)
var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func safePath(name string) bool {
	return name != "" && len(name) <= 256 && fs.ValidPath(name) && !strings.ContainsAny(name, "\\:\x00") && path.Clean(name) == name && name != "."
}
func readBounded(r io.Reader, limit int) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil || len(raw) > limit {
		return nil, Failure("bundle_limit")
	}
	return raw, nil
}
func checkFiles(b Bundle) error {
	if len(b.Files) == 0 || len(b.Files) > MaxFiles {
		return Failure("bundle_limit")
	}
	size, modules := 0, 0
	for name, data := range b.Files {
		size += len(data)
		if !safePath(name) {
			return Failure("invalid_path")
		}
		if len(data) > normalization.MaxBytes {
			return Failure("bundle_limit")
		}
		if strings.HasSuffix(name, ".rego") {
			modules++
		}
	}
	if size > MaxBundleBytes || modules > MaxModules || modules == 0 {
		return Failure("bundle_limit")
	}
	return nil
}
func LoadDir(dir string) (Bundle, error) {
	b := Bundle{Files: map[string][]byte{}}
	total := 0
	err := filepath.WalkDir(dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return Failure("bundle_read")
		}
		if d.Type()&os.ModeSymlink != 0 {
			return Failure("invalid_path")
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return Failure("invalid_path")
		}
		rel, err := filepath.Rel(dir, name)
		if err != nil {
			return Failure("invalid_path")
		}
		rel = filepath.ToSlash(rel)
		if rel == ".gitkeep" {
			return nil
		}
		if len(b.Files) >= MaxFiles {
			return Failure("bundle_limit")
		}
		f, err := os.Open(name)
		if err != nil {
			return Failure("bundle_read")
		}
		defer f.Close()
		data, err := readBounded(f, normalization.MaxBytes)
		if err != nil {
			return err
		}
		total += len(data)
		if total > MaxBundleBytes {
			return Failure("bundle_limit")
		}
		b.Files[rel] = data
		return nil
	})
	if err != nil {
		return Bundle{}, err
	}
	if err = checkFiles(b); err != nil {
		return Bundle{}, err
	}
	return b, nil
}
func ParseManifest(raw []byte) (v1.PolicyManifest, error) {
	m, err := contracts.Decode[v1.PolicyManifest]("policy-manifest", raw)
	if err != nil {
		return m, Failure("invalid_manifest")
	}
	return m, nil
}
func strictJSON(raw []byte, out any) error {
	if _, err := normalization.Decode(raw); err != nil {
		return Failure("invalid_bundle")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return Failure("invalid_bundle")
	}
	return nil
}
func version(v string) ([3]int, error) {
	var out [3]int
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, Failure("incompatible_version")
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || strconv.Itoa(n) != p {
			return out, Failure("incompatible_version")
		}
		out[i] = n
	}
	return out, nil
}
func compatible(min string) bool {
	a, e := version(min)
	b, _ := version(CoreVersion)
	if e != nil || a[0] != b[0] {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return true
}
func ValidateDependencies(manifests []v1.PolicyManifest) error {
	byID := map[string]v1.PolicyManifest{}
	owners := map[string]string{}
	for _, m := range manifests {
		if !compatible(m.MinimumCoreVersion) {
			return Failure("incompatible_version")
		}
		if _, ok := byID[m.Id]; ok {
			return Failure("dependency_conflict")
		}
		byID[m.Id] = m
		for _, n := range m.Namespaces {
			for previous := range owners {
				if n == previous || strings.HasPrefix(n, previous+".") || strings.HasPrefix(previous, n+".") {
					return Failure("namespace_collision")
				}
			}
			owners[n] = m.Id
		}
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return Failure("dependency_cycle")
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, dep := range byID[id].Dependencies {
			parts := strings.Split(dep, "@")
			if len(parts) != 2 {
				return Failure("dependency_constraint")
			}
			m, ok := byID[parts[0]]
			if !ok || m.Version != parts[1] {
				return Failure("dependency_constraint")
			}
			if err := visit(m.Id); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
func Inspect(b Bundle) (Report, error) {
	if err := checkFiles(b); err != nil {
		return Report{}, err
	}
	m, err := ParseManifest(b.Files["manifest.json"])
	if err != nil {
		return Report{}, err
	}
	manifests := []v1.PolicyManifest{m}
	if raw, ok := b.Files["dependencies.json"]; ok {
		var deps []json.RawMessage
		if strictJSON(raw, &deps) != nil {
			return Report{}, Failure("invalid_manifest")
		}
		for _, raw := range deps {
			dep, err := ParseManifest(raw)
			if err != nil {
				return Report{}, err
			}
			manifests = append(manifests, dep)
		}
	}
	if err = ValidateDependencies(manifests); err != nil {
		return Report{}, err
	}
	if m.SigningIdentity != "unsigned" && !strings.HasPrefix(m.SigningIdentity, "key:") {
		return Report{}, Failure("invalid_signing_identity")
	}
	if strings.TrimSpace(m.SigningIdentity) == "key:" {
		return Report{}, Failure("invalid_signing_identity")
	}
	var layers []Layer
	if strictJSON(b.Files["layers.json"], &layers) != nil || len(layers) == 0 || len(layers) > 32 {
		return Report{}, Failure("invalid_layers")
	}
	namespaces, priorities, names := map[string]bool{}, map[int]bool{}, map[string]bool{}
	owned := []string{}
	for _, manifest := range manifests {
		owned = append(owned, manifest.Namespaces...)
	}
	for _, l := range layers {
		if !slices.Contains(LayerNames, l.Name) || !namespacePattern.MatchString(l.Namespace) || !slices.Contains(owned, l.Namespace) || names[l.Name] || priorities[l.Priority] {
			return Report{}, Failure("invalid_layers")
		}
		for n := range namespaces {
			if n == l.Namespace || strings.HasPrefix(n, l.Namespace+".") || strings.HasPrefix(l.Namespace, n+".") {
				return Report{}, Failure("namespace_collision")
			}
		}
		names[l.Name] = true
		namespaces[l.Namespace] = true
		priorities[l.Priority] = true
	}
	if !names["baseline"] {
		return Report{}, Failure("missing_baseline")
	}
	slices.SortFunc(layers, func(a, b Layer) int { return slices.Index(LayerNames, a.Name) - slices.Index(LayerNames, b.Name) })
	for i := 1; i < len(layers); i++ {
		if layers[i].Priority <= layers[i-1].Priority {
			return Report{}, Failure("invalid_layers")
		}
	}
	var reasons map[string]Reason
	if strictJSON(b.Files["reasons.json"], &reasons) != nil || len(reasons) == 0 {
		return Report{}, Failure("invalid_reasons")
	}
	for code, r := range reasons {
		if !reasonPattern.MatchString(code) || r.Description == "" || r.Remediation == "" {
			return Report{}, Failure("invalid_reasons")
		}
	}
	var data map[string]any
	if strictJSON(b.Files["data.json"], &data) != nil || data == nil {
		return Report{}, Failure("invalid_data")
	}
	// Keep static data in a dedicated tree; it cannot shadow executable namespaces.
	for k := range data {
		if k != "config" {
			return Report{}, Failure("invalid_data")
		}
	}
	files := make([]string, 0, len(b.Files))
	for name := range b.Files {
		files = append(files, name)
	}
	slices.Sort(files)
	return Report{Manifest: m, Revision: Revision(m.BuildDigest), Files: files, Layers: layers, Reasons: reasons}, nil
}
func seal(b Bundle) (Bundle, Report, error) {
	report, err := Inspect(b)
	if err != nil {
		return Bundle{}, Report{}, err
	}
	out := Bundle{Files: map[string][]byte{}}
	for name, raw := range b.Files {
		out.Files[name] = bytes.Clone(raw)
	}
	m := report.Manifest
	m.SourceDigest = zeroDigest
	m.BuildDigest = zeroDigest
	// Both hashes are over sorted path/content digests; the two self fields are
	// blanked for source, and only build_digest is blanked for the build hash.
	out.Files["manifest.json"], _ = json.Marshal(m)
	m.SourceDigest = fileDigest(out)
	out.Files["manifest.json"], _ = json.Marshal(m)
	m.BuildDigest = fileDigest(out)
	out.Files["manifest.json"], _ = json.Marshal(m)
	report.Manifest = m
	report.Revision = Revision(m.BuildDigest)
	return out, report, nil
}
func fileDigest(b Bundle) string {
	names := make([]string, 0, len(b.Files))
	for name := range b.Files {
		names = append(names, name)
	}
	slices.Sort(names)
	var text strings.Builder
	for _, name := range names {
		fmt.Fprintf(&text, "%s\x00%s\n", name, normalization.Digest(b.Files[name]))
	}
	return normalization.Digest([]byte(text.String()))
}

// Prepare validates and detaches a source bundle and assigns its content revision.
func Prepare(b Bundle) (Bundle, Report, error) { return seal(b) }
func Build(b Bundle) ([]byte, error) {
	b, report, err := seal(b)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	for _, name := range report.Files {
		raw := b.Files[name]
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(raw)), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}); err != nil {
			return nil, Failure("archive_write")
		}
		if _, err = tw.Write(raw); err != nil {
			return nil, Failure("archive_write")
		}
	}
	if err = tw.Close(); err != nil {
		return nil, Failure("archive_write")
	}
	if err = gz.Close(); err != nil {
		return nil, Failure("archive_write")
	}
	return buffer.Bytes(), nil
}
func ReadArchive(r io.Reader) (Bundle, error) {
	compressed, err := readBounded(r, MaxBundleBytes+65536)
	if err != nil {
		return Bundle{}, err
	}
	reader := bytes.NewReader(compressed)
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return Bundle{}, Failure("invalid_archive")
	}
	defer gz.Close()
	gz.Multistream(false)
	expanded, err := readBounded(gz, MaxBundleBytes+MaxFiles*1024+1024)
	if err != nil {
		return Bundle{}, err
	}
	if reader.Len() != 0 {
		return Bundle{}, Failure("invalid_archive")
	}
	tr := tar.NewReader(bytes.NewReader(expanded))
	b := Bundle{Files: map[string][]byte{}}
	size := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Bundle{}, Failure("invalid_archive")
		}
		if !safePath(h.Name) || h.Typeflag != tar.TypeReg || h.Size < 0 || h.Size > normalization.MaxBytes {
			return Bundle{}, Failure("invalid_path")
		}
		if _, ok := b.Files[h.Name]; ok {
			return Bundle{}, Failure("duplicate_file")
		}
		if len(b.Files) >= MaxFiles || size+int(h.Size) > MaxBundleBytes {
			return Bundle{}, Failure("bundle_limit")
		}
		raw, err := readBounded(tr, normalization.MaxBytes)
		if err != nil {
			return Bundle{}, err
		}
		size += len(raw)
		b.Files[h.Name] = raw
	}
	_, report, err := seal(b)
	if err != nil {
		return Bundle{}, err
	}
	m, _ := ParseManifest(b.Files["manifest.json"])
	if m.SourceDigest != report.Manifest.SourceDigest || m.BuildDigest != report.Manifest.BuildDigest {
		return Bundle{}, Failure("checksum_mismatch")
	}
	return b, nil
}
func Load(name string) (Bundle, error) {
	info, err := os.Stat(name)
	if err != nil {
		return Bundle{}, Failure("bundle_read")
	}
	if info.IsDir() {
		return LoadDir(name)
	}
	f, err := os.Open(name)
	if err != nil {
		return Bundle{}, Failure("bundle_read")
	}
	defer f.Close()
	return ReadArchive(f)
}

type Change struct {
	Path      string `json:"path"`
	OldDigest string `json:"old_digest,omitempty"`
	NewDigest string `json:"new_digest,omitempty"`
	Before    string `json:"before,omitempty"`
	After     string `json:"after,omitempty"`
}

func Diff(a, b Bundle) []Change {
	names := map[string]bool{}
	for n := range a.Files {
		names[n] = true
	}
	for n := range b.Files {
		names[n] = true
	}
	out := []Change{}
	for n := range names {
		av, aok := a.Files[n]
		bv, bok := b.Files[n]
		if aok && bok && bytes.Equal(av, bv) {
			continue
		}
		c := Change{Path: n}
		if aok {
			c.OldDigest = normalization.Digest(av)
			c.Before = string(av)
		}
		if bok {
			c.NewDigest = normalization.Digest(bv)
			c.After = string(bv)
		}
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	return out
}
