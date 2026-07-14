package rhizome

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BatchParser is the repo-level sibling of the per-file Parser seam: one call
// parses many files in a single pass and returns symbols keyed by
// slash-separated root-relative path (the same normalization ScanRepository
// applies). The Conductor's tree-sitter terrarium is the canonical
// implementation — it mounts the repository once and parses every requested
// file inside the container — but rhizome itself stays Docker-free: this
// package only defines the seam and replays results through
// PrecomputedParser.
//
// Contract:
//   - paths == nil requests a full walk: the implementation discovers and
//     parses every file it supports under root (the cold-index path).
//   - paths != nil requests exactly that changed subset (root-relative slash
//     paths); implementations must not parse beyond it, so an incremental
//     re-scan only pays for the delta.
//   - A file the implementation cannot parse is omitted from the result, never
//     an error — scanner precedence lets the per-file parsers catch it. An
//     error return means the whole batch is unusable and the caller should
//     fall back to per-file parsers entirely.
type BatchParser interface {
	Supports(path string) bool
	ParseBatch(ctx context.Context, root string, paths []string) (map[string][]Symbol, error)
}

// ChangedPaths walks root with the same skip rules and path normalization as
// ScanRepository and returns the supports-eligible files whose content hash
// differs from the stored file record (or that have no record yet) — the
// exact set ScanRepository will re-parse, so a BatchParser fed this list
// pre-computes precisely the delta and nothing else.
//
// The second result reports whether any eligible file already had a store
// record: false means a cold index, where the caller should prefer a
// full-walk batch (ParseBatch with nil paths) over an explicit list.
func ChangedPaths(ctx context.Context, root string, repositoryName string, store IndexStore, supports func(path string) bool) ([]string, bool, error) {
	if store == nil {
		return nil, false, fmt.Errorf("index store is required")
	}
	if strings.TrimSpace(repositoryName) == "" {
		return nil, false, fmt.Errorf("repositoryName is required")
	}
	if supports == nil {
		supports = func(string) bool { return true }
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, false, fmt.Errorf("resolve repository root: %w", err)
	}

	var changed []string
	warmIndex := false
	err = filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absoluteRoot {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		relativePath, err := filepath.Rel(absoluteRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(filepath.Clean(relativePath))

		if shouldSkipPath(relativePath, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !supports(relativePath) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		existing, found, err := store.GetFile(ctx, repositoryName, relativePath)
		if err != nil {
			return err
		}
		if found {
			warmIndex = true
			if existing.Hash == hashContent(content) {
				return nil
			}
		}
		changed = append(changed, relativePath)
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("collect changed paths: %w", err)
	}

	return changed, warmIndex, nil
}
