package rhizome

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeBatchFixture(t *testing.T, root, name, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func supportsScriptExtensions(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".py" || extension == ".ts" || extension == ".js"
}

func TestChangedPathsColdIndexReturnsAllEligibleFiles(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeBatchFixture(t, root, "src/service.py", "def run():\n    pass\n")
	writeBatchFixture(t, root, "widget.ts", "export function widget() {}\n")
	writeBatchFixture(t, root, "main.go", "package main\n")

	store := openTestStore(t, ctx, filepath.Join(t.TempDir(), "rhizome.db"))
	defer store.Close()

	changed, warmIndex, err := ChangedPaths(ctx, root, "repo", store, supportsScriptExtensions)
	if err != nil {
		t.Fatalf("ChangedPaths returned error: %v", err)
	}
	if warmIndex {
		t.Fatal("an empty store must report a cold index")
	}
	if !reflect.DeepEqual(changed, []string{"src/service.py", "widget.ts"}) {
		t.Fatalf("changed paths mismatch: %v", changed)
	}
}

func TestChangedPathsWarmIndexReturnsOnlyDelta(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeBatchFixture(t, root, "service.py", "def run():\n    pass\n")
	writeBatchFixture(t, root, "widget.ts", "export function widget() {}\n")

	store := openTestStore(t, ctx, filepath.Join(t.TempDir(), "rhizome.db"))
	defer store.Close()

	if _, err := ScanRepository(ctx, root, "repo", store, DefaultParsers()); err != nil {
		t.Fatalf("initial warm scan: %v", err)
	}

	changed, warmIndex, err := ChangedPaths(ctx, root, "repo", store, supportsScriptExtensions)
	if err != nil {
		t.Fatalf("ChangedPaths returned error: %v", err)
	}
	if !warmIndex {
		t.Fatal("expected a warm index after a full scan")
	}
	if len(changed) != 0 {
		t.Fatalf("expected no delta for an unchanged workspace, got %v", changed)
	}

	// A modified file and a brand-new file form the next delta.
	writeBatchFixture(t, root, "widget.ts", "export function widget() { return 1 }\n")
	writeBatchFixture(t, root, "fresh.js", "function fresh() {}\n")

	changed, warmIndex, err = ChangedPaths(ctx, root, "repo", store, supportsScriptExtensions)
	if err != nil {
		t.Fatalf("ChangedPaths returned error: %v", err)
	}
	if !warmIndex {
		t.Fatal("expected a warm index on the re-scan")
	}
	if !reflect.DeepEqual(changed, []string{"fresh.js", "widget.ts"}) {
		t.Fatalf("delta mismatch: %v", changed)
	}
}

func TestChangedPathsHonorsSkipRulesAndSupportsFilter(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeBatchFixture(t, root, "node_modules/dep.js", "function hidden() {}\n")
	writeBatchFixture(t, root, "vendor/lib.py", "def hidden():\n    pass\n")
	writeBatchFixture(t, root, "main.go", "package main\n")
	writeBatchFixture(t, root, "kept.py", "def kept():\n    pass\n")

	store := openTestStore(t, ctx, filepath.Join(t.TempDir(), "rhizome.db"))
	defer store.Close()

	changed, _, err := ChangedPaths(ctx, root, "repo", store, supportsScriptExtensions)
	if err != nil {
		t.Fatalf("ChangedPaths returned error: %v", err)
	}
	if !reflect.DeepEqual(changed, []string{"kept.py"}) {
		t.Fatalf("expected skip rules and supports filter to leave only kept.py, got %v", changed)
	}
}

func TestChangedPathsValidatesArguments(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, ctx, filepath.Join(t.TempDir(), "rhizome.db"))
	defer store.Close()

	if _, _, err := ChangedPaths(ctx, t.TempDir(), "repo", nil, nil); err == nil {
		t.Fatal("expected an error for a nil store")
	}
	if _, _, err := ChangedPaths(ctx, t.TempDir(), "  ", store, nil); err == nil {
		t.Fatal("expected an error for a blank repository name")
	}
}
