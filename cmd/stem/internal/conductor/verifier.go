package conductor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opentendril/core/cmd/stem/internal/terrarium"
)

// ErrVerifierBlocked marks a verifier step whose run could not verify
// everything it was asked to: applicable tests skipped themselves (for
// example because the sealed terrarium offers no Docker daemon or network),
// so the outcome is neither green nor a code failure — it is unverified.
// Callers distinguish it from a plain failure with errors.Is; the sequence
// engine wraps step errors with %w, so the sentinel survives to the surface
// that renders the final verdict.
var ErrVerifierBlocked = errors.New("blocked")

// verifierImage is the toolchain-bearing image used for deterministic verifier
// (CI) command steps. Unlike opentendril-go:latest it keeps the full Go
// toolchain at runtime so `go build` / `go test` can execute. See
// sprouts/go-verifier/Dockerfile.
const verifierImage = "opentendril-go-verifier:latest"

// verifierContainerTimeout bounds a single verifier command exec.
const verifierContainerTimeout = 5 * time.Minute

// runVerifierCommand executes a fixed command deterministically in the verifier
// terrarium and reports its outcome. No LLM is involved: the command's exit
// code is the verdict. The workspace is mounted read-only and no commit or
// merge-back occurs, so a verifier step can never mutate the host — it only
// reads and reports.
func runVerifierCommand(ctx context.Context, providerName, workspacePath string, command []string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(command) == 0 {
		return "", fmt.Errorf("verifier step has an empty command")
	}

	if err := ensureSproutImageFn(ctx, verifierImage); err != nil {
		return "", fmt.Errorf("build verifier image: %w", err)
	}

	provider, err := terrarium.NewProvider(ctx, providerName)
	if err != nil {
		return "", fmt.Errorf("resolve terrarium provider for verifier: %w", err)
	}

	spec := terrarium.TerrariumSpec{
		Image:         verifierImage,
		WorkingDir:    "/app",
		NetworkMode:   terrarium.NetworkModeNone,
		CPUQuota:      "1.0",
		MemoryLimitMB: 2048,
		PidsLimit:     512,
		Timeout:       verifierContainerTimeout,
		Mounts: []terrarium.MountSpec{
			// Read-only: a verifier reads and reports, it never writes the
			// workspace. `go build`/`go test` write only to GOCACHE/tmp below.
			{Source: workspacePath, Target: "/app", ReadOnly: true},
		},
		// Deliberately no RunAsUser: a build-tool container (like a CI runner),
		// not a code-execution one — running as root avoids UID-mismatch
		// permission failures on the read-only GOMODCACHE bind mount. Isolation
		// still comes from --network none, --cap-drop=ALL,
		// --security-opt=no-new-privileges, and the CPU/memory/pids caps above.
	}
	if modCache, ok := hostGoModCache(); ok {
		spec.Mounts = append(spec.Mounts, terrarium.MountSpec{
			Source: modCache, Target: "/go/pkg/mod", ReadOnly: true,
		})
	}

	instance, err := provider.Create(ctx, spec)
	if err != nil {
		return "", fmt.Errorf("start verifier terrarium: %w", err)
	}
	defer func() { _ = instance.Stop(context.Background()) }()

	result, runErr := instance.Run(ctx, terrarium.CommandSpec{
		Command:    command,
		WorkingDir: "/app",
		Environment: map[string]string{
			"GOPATH":     "/go",
			"GOMODCACHE": "/go/pkg/mod",
			"GOCACHE":    "/tmp/gocache",
			// Fully offline and deterministic: the module cache is warm and
			// mounted read-only, so no fetches and no go.mod/go.sum mutation.
			// -buildvcs=false because VCS stamping shells out to git, which
			// fails on the read-only bind mount owned by a different uid than
			// the container root — irrelevant to a build/test verdict anyway.
			"GOFLAGS": "-mod=readonly -buildvcs=false",
			"GOPROXY": "off",
		},
		Timeout: verifierContainerTimeout,
	})
	if runErr != nil {
		return "", fmt.Errorf("run verifier command %q: %w", strings.Join(command, " "), runErr)
	}

	// A `go test -json` step gets the skip-aware verdict: `go test` exits 0
	// when tests skip themselves, so the exit code alone could report green
	// for a run that verified nothing. Every other command (build, vet,
	// format checks) keeps the plain exit-code verdict unchanged.
	if isGoTestJSONCommand(command) {
		return reportGoTestVerifier(command, result)
	}

	report := formatVerifierReport(command, result)
	if result.ExitCode != 0 {
		// The sequence runner prints a step's output only on success, but for a
		// verifier the failing command's output is the point — surface it here.
		fmt.Fprintln(os.Stderr, report)
		return report, fmt.Errorf("verifier command %q failed (exit %d)", strings.Join(command, " "), result.ExitCode)
	}
	return report, nil
}

// reportGoTestVerifier derives the verdict of a `go test -json` verifier step
// from the parsed event stream instead of the exit code alone:
//
//   - a fail event → failed (an error, as today);
//   - a non-zero exit with no fail event (for example a compile error) →
//     failed on the exit code, as today;
//   - a skip event carrying a test name → blocked: the step ran but did not
//     verify everything it was asked to, so it returns an error wrapping
//     ErrVerifierBlocked with a message that is unmistakably not a pass and
//     not a code failure — the sequence halts either way;
//   - otherwise → passed.
func reportGoTestVerifier(command []string, result terrarium.CommandResult) (string, error) {
	outcome := evaluateGoTestJSONStream(result.Stdout)
	report := formatGoTestVerifierReport(command, result, outcome)
	switch {
	case outcome.Verdict == goTestVerdictFailed:
		fmt.Fprintln(os.Stderr, report)
		return report, fmt.Errorf("verifier command %q failed: %d test(s) or package(s) failed", strings.Join(command, " "), len(outcome.FailedSubjects))
	case result.ExitCode != 0:
		fmt.Fprintln(os.Stderr, report)
		return report, fmt.Errorf("verifier command %q failed (exit %d)", strings.Join(command, " "), result.ExitCode)
	case outcome.Verdict == goTestVerdictBlocked:
		fmt.Fprintln(os.Stderr, report)
		return report, fmt.Errorf("verifier command %q is %w: %d applicable test(s) skipped and were not verified", strings.Join(command, " "), ErrVerifierBlocked, len(outcome.SkippedTests))
	}
	return report, nil
}

// formatVerifierReport renders a compact pass/fail report with the command's
// combined output for the sequence log.
func formatVerifierReport(command []string, result terrarium.CommandResult) string {
	status := "PASSED"
	if result.ExitCode != 0 {
		status = "FAILED"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🔬 %s — %s (exit %d)", strings.Join(command, " "), status, result.ExitCode)
	if out := strings.TrimSpace(result.Stdout); out != "" {
		fmt.Fprintf(&b, "\n%s", out)
	}
	if errOut := strings.TrimSpace(result.Stderr); errOut != "" {
		fmt.Fprintf(&b, "\n%s", errOut)
	}
	return b.String()
}

// formatGoTestVerifierReport renders the skip-aware verdict of a
// `go test -json` step: PASSED, FAILED, or BLOCKED. Instead of echoing the
// raw event stream — one JSON object per output line — it names the failing
// and the skipped (unverified) subjects, and includes the full stream only
// when the step failed, where the output is the point.
func formatGoTestVerifierReport(command []string, result terrarium.CommandResult, outcome goTestOutcome) string {
	status := "PASSED"
	switch {
	case outcome.Verdict == goTestVerdictFailed || result.ExitCode != 0:
		status = "FAILED"
	case outcome.Verdict == goTestVerdictBlocked:
		status = "BLOCKED"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "🔬 %s — %s (exit %d)", strings.Join(command, " "), status, result.ExitCode)
	if len(outcome.FailedSubjects) > 0 {
		fmt.Fprintf(&b, "\nfailed: %s", strings.Join(outcome.FailedSubjects, ", "))
	}
	if len(outcome.SkippedTests) > 0 {
		fmt.Fprintf(&b, "\nskipped and NOT verified: %s", strings.Join(outcome.SkippedTests, ", "))
	}
	if status == "FAILED" {
		if out := strings.TrimSpace(result.Stdout); out != "" {
			fmt.Fprintf(&b, "\n%s", out)
		}
	}
	if errOut := strings.TrimSpace(result.Stderr); errOut != "" {
		fmt.Fprintf(&b, "\n%s", errOut)
	}
	return b.String()
}
