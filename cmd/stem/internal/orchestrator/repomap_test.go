package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRepoMapExtractsGoSignatures(t *testing.T) {
	root := t.TempDir()

	writeRepoMapTestFile(t, root, "pkg/service/service.go", `
package service

import "context"

type User struct {
	ID   int
	Name string
}

type Repo interface {
	Save(ctx context.Context, user User) error
}

type Service struct {
	Repo Repo
}

func (s *Service) Run(ctx context.Context) error {
	return s.Repo.Save(ctx, User{})
}
`)

	writeRepoMapTestFile(t, root, "pkg/service/service_test.go", `
package service

func TestService(t *testing.T) {}
`)

	output, err := GenerateRepoMap(root)
	if err != nil {
		t.Fatalf("GenerateRepoMap failed: %v", err)
	}

	assertContains(t, output, "## Tree")
	assertContains(t, output, "## Signatures")
	assertContains(t, output, "pkg/service/service.go")
	assertContains(t, output, "package service")
	assertContains(t, output, "type User struct { ID int; Name string }")
	assertContains(t, output, "type Repo interface { Save(ctx context.Context, user User) error }")
	assertContains(t, output, "func (s *Service) Run(ctx context.Context) error")
	assertNotContains(t, output, "service_test.go")
}

func TestGenerateRepoMapExtractsTypeScriptSignatures(t *testing.T) {
	root := t.TempDir()

	writeRepoMapTestFile(t, root, "src/main.ts", `
export interface ToolCall {
  tool: string;
  arguments?: Record<string, unknown>;
}

export class Runner {
  async run(input: string): Promise<void> {
    return Promise.resolve();
  }
}

export function helper(value: number): number {
  return value + 1;
}

export type Result = {
  ok: boolean;
};
`)

	writeRepoMapTestFile(t, root, "src/main.test.ts", `
export function ignored() {
  return true;
}
`)

	output, err := GenerateRepoMap(root)
	if err != nil {
		t.Fatalf("GenerateRepoMap failed: %v", err)
	}

	assertContains(t, output, "src/main.ts")
	assertContains(t, output, "export interface ToolCall { tool: string; arguments?: Record<string, unknown> }")
	assertContains(t, output, "export class Runner { Runner.run(input: string): Promise<void> }")
	assertContains(t, output, "export function helper(value: number): number")
	assertContains(t, output, "export type Result = { ok: boolean }")
	assertNotContains(t, output, "main.test.ts")
}

func TestGenerateRepoMapExtractsPythonSignatures(t *testing.T) {
	root := t.TempDir()

	writeRepoMapTestFile(t, root, "app/main.py", `
class ToolResponse:
    pass


def main(
    value: int,
    name: str,
) -> None:
    return None


class Worker:
    def run(
        self,
        task: str,
    ) -> None:
        return None
`)

	writeRepoMapTestFile(t, root, "tests/test_main.py", `
def ignored() -> None:
    pass
`)

	output, err := GenerateRepoMap(root)
	if err != nil {
		t.Fatalf("GenerateRepoMap failed: %v", err)
	}

	assertContains(t, output, "app/main.py")
	assertContains(t, output, "class ToolResponse")
	assertContains(t, output, "def main(value: int, name: str) -> None")
	assertContains(t, output, "Worker.run(self, task: str) -> None")
	assertNotContains(t, output, "test_main.py")
}

func TestGenerateRepoMapDemotesLargeFiles(t *testing.T) {
	root := t.TempDir()

	largeBody := strings.Repeat("// filler\n", 2001)
	writeRepoMapTestFile(t, root, "pkg/huge/huge.go", "package huge\n\n"+largeBody+`

func Huge() {}
`)

	output, err := GenerateRepoMap(root)
	if err != nil {
		t.Fatalf("GenerateRepoMap failed: %v", err)
	}

	assertContains(t, output, "pkg/huge/huge.go")
	assertNotContains(t, output, "func Huge()")
}

func writeRepoMapTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	trimmed := strings.TrimLeft(content, "\n")
	if !strings.HasSuffix(trimmed, "\n") {
		trimmed += "\n"
	}
	if err := os.WriteFile(path, []byte(trimmed), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected output to contain %q\noutput:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if strings.Contains(haystack, needle) {
		t.Fatalf("expected output not to contain %q\noutput:\n%s", needle, haystack)
	}
}
