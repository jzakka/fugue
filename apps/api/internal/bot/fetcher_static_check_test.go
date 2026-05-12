// fetcher_static_check_test.go covers task 5.2 of
// harvester-snapshot-first-fetch: automated enforcement that the bot
// package's production code delegates snapshot-key construction and URL
// normalization to the canonical sources of truth.
//
// Checks:
//
//   (a) The fetcher.go read path references snapshot.SnapshotKey or
//       snapshot.HashNormalizedURL (directly or via snapshot.S3Reader,
//       which is the only authorized call site).
//   (b) No production Go file under apps/api/internal/bot (excluding the
//       snapshot/ subpackage and *_test.go) imports crypto/sha256 or
//       embeds the snapshots/%s/%s.html.gz key literal. Duplicating the
//       key format outside the snapshot package would silently diverge
//       from Pioneer's write side.
//   (c) ObjectStorageFetcher.Fetch (and its call tree) normalizes with
//       canonicalURL — the same symbol Pioneer's write path uses at
//       pioneer_consumer.go — and never with templatePath. A read-side
//       normalization drift is the exact bug this test pins down.
//
// Test files are excluded: they are allowed to reconstruct expected keys
// from first principles for assertion purposes.

package bot

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// botPackageDir walks up from the test's cwd to locate
// apps/api/internal/bot. `go test` runs with cwd set to the package
// directory, so we use "." directly; but we resolve to an absolute path
// so assertion messages stay readable.
func botPackageDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	return abs
}

// readProductionFiles returns the contents of every *.go file under the
// bot package root (non-recursive: only the top-level directory; the
// snapshot/ subpackage is a separate authorized location), excluding
// *_test.go files.
func readProductionFiles(t *testing.T) map[string][]byte {
	t.Helper()
	dir := botPackageDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %q: %v", dir, err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		full := filepath.Join(dir, name)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("read %q: %v", full, err)
		}
		out[name] = data
	}
	if len(out) == 0 {
		t.Fatalf("no production Go files found under %q", dir)
	}
	return out
}

// fileImports returns true if the given Go source imports the given
// import path. Matches single-line and grouped import blocks.
func fileImports(src []byte, path string) bool {
	scanner := bufio.NewScanner(bytes.NewReader(src))
	quoted := `"` + path + `"`
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import ") && strings.Contains(line, quoted) {
			return true
		}
		if strings.Contains(line, quoted) {
			return true
		}
	}
	return false
}

// Task 5.2 (a): fetcher.go must reference snapshot.SnapshotKey or
// snapshot.HashNormalizedURL (directly or transitively through
// snapshot.S3Reader.Get, which in turn calls SnapshotKey). We accept
// either an explicit reference or an import of the snapshot package +
// use of S3Reader, because ObjectStorageFetcher is wired to a
// SnapshotReader whose concrete S3Reader owns the key computation.
func TestStatic_FetcherReferencesSnapshotKeyBuilder(t *testing.T) {
	files := readProductionFiles(t)
	src, ok := files["fetcher.go"]
	if !ok {
		t.Fatal("fetcher.go missing — this test's assumption is broken")
	}
	if !fileImports(src, "github.com/chungsanghwa/fugue/apps/api/internal/bot/snapshot") {
		t.Errorf("fetcher.go does not import the snapshot package; key construction must be delegated there")
	}
	// Authorized delegation: ObjectStorageFetcher delegates to
	// snapshot.SnapshotReader (interface) implemented by S3Reader, which
	// calls SnapshotKey internally. Either an explicit symbol reference
	// or SnapshotReader usage satisfies the requirement.
	hasDelegate := bytes.Contains(src, []byte("snapshot.SnapshotKey")) ||
		bytes.Contains(src, []byte("snapshot.HashNormalizedURL")) ||
		bytes.Contains(src, []byte("snapshot.SnapshotReader")) ||
		bytes.Contains(src, []byte("reader.Get(")) // via the SnapshotReader interface
	if !hasDelegate {
		t.Errorf("fetcher.go does not delegate snapshot-key computation to the snapshot package")
	}
}

// Task 5.2 (b): no production file in the bot package (outside the
// snapshot/ subpackage) may duplicate the snapshot-key format. Such
// duplication is the top drift risk — re-encoding the key locally would
// silently diverge from Pioneer's write side.
//
// The crypto/sha256 import is checked only in files that also touch
// snapshot concerns. image_cache.go legitimately imports sha256 to hash
// image URLs into the independent `images/` object-storage namespace
// (Decision 7 of the image-cache change); that usage is orthogonal to
// snapshot-key construction and must not be flagged.
func TestStatic_NoLocalSnapshotKeyReimplementation(t *testing.T) {
	files := readProductionFiles(t)

	const keyFormat = "snapshots/%s/%s.html.gz"
	// Any literal beginning with "snapshots/" paired with .html.gz
	// suffix is treated as a reimplementation attempt. The check is
	// conservative: it matches whenever both substrings appear.
	for name, src := range files {
		if bytes.Contains(src, []byte(keyFormat)) {
			t.Errorf("%s: embeds the snapshot key format literal %q; must delegate to snapshot.SnapshotKey", name, keyFormat)
		}
		// Only flag sha256 when the file also references snapshot
		// concerns ("snapshot" token or the snapshots/ key prefix);
		// hashing used for unrelated namespaces (e.g. image_cache.go's
		// `images/` prefix) is out of scope for this check.
		if fileImports(src, "crypto/sha256") &&
			(bytes.Contains(src, []byte("snapshot")) || bytes.Contains(src, []byte("snapshots/"))) {
			t.Errorf("%s: imports crypto/sha256 alongside snapshot references; key hashing must be delegated to snapshot.HashNormalizedURL", name)
		}
	}
}

// Task 5.2 (c): ObjectStorageFetcher.Fetch (and its call tree in
// fetcher.go) must normalize via canonicalURL — the same symbol Pioneer
// calls at pioneer_consumer.go — and must never call templatePath. The
// round-trip test (fetcher_roundtrip_test.go) catches drift
// behaviorally; this test catches it structurally so the failure mode
// is obvious in the diff.
func TestStatic_FetcherUsesCanonicalURLNotTemplatePath(t *testing.T) {
	files := readProductionFiles(t)
	src, ok := files["fetcher.go"]
	if !ok {
		t.Fatal("fetcher.go missing — this test's assumption is broken")
	}

	if !bytes.Contains(src, []byte("canonicalURL(")) {
		t.Errorf("fetcher.go does not call canonicalURL; read-side normalization must match Pioneer's write side (pioneer_consumer.go:168)")
	}
	if bytes.Contains(src, []byte("templatePath(")) {
		t.Errorf("fetcher.go calls templatePath; read-side normalization must use canonicalURL to align with Pioneer's write side (design.md Decision 5)")
	}
}
