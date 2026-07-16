package conductor

import (
	"encoding/json"
	"sort"
	"strings"
)

// Skip-aware verdict for `go test -json` verifier steps.
//
// `go test` exits 0 when tests skip themselves, so an exit-code verdict would
// report green for a run that verified nothing — for example a sealed
// (network-less, Docker-less) verifier run whose integration tests all called
// t.Skip. This evaluator parses the -json event stream and classifies the run
// so a skipped test can never pass silently: it becomes a "blocked"
// (unverified) verdict, which halts the sequence exactly like a failure.
//
// The evaluator is deliberately pure — no containers, no git, no toolchain —
// so the security-critical verdict logic can be reviewed and tested in
// isolation.

// goTestVerdict classifies the outcome of one `go test -json` run.
type goTestVerdict string

const (
	// goTestVerdictPassed means every applicable test ran and passed.
	goTestVerdictPassed goTestVerdict = "passed"
	// goTestVerdictFailed means at least one test or package failed.
	goTestVerdictFailed goTestVerdict = "failed"
	// goTestVerdictBlocked means nothing failed, but at least one applicable
	// test skipped itself, so the run verified less than it was asked to.
	goTestVerdictBlocked goTestVerdict = "blocked"
)

// goTestEvent is the subset of the `go test -json` (test2json) event schema
// the verdict depends on. Test-level events carry a non-empty Test field;
// package-level events do not.
type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
}

// goTestOutcome is the classified result of a `go test -json` event stream.
type goTestOutcome struct {
	Verdict goTestVerdict
	// FailedSubjects lists "package.Test" for every failing test and the bare
	// package path for every package-level failure.
	FailedSubjects []string
	// SkippedTests lists "package.Test" for every skipped test — skip events
	// carrying a Test name. Package-level skip events (a package with no test
	// files) are benign and never recorded here.
	SkippedTests []string
}

// evaluateGoTestJSONStream classifies a `go test -json` event stream.
//
// Rules, in precedence order:
//   - any "fail" event → failed;
//   - otherwise any "skip" event WITH a non-empty Test field → blocked: a
//     real test decided not to run, so its subject is unverified;
//   - otherwise → passed.
//
// The critical distinction: a "skip" event WITHOUT a Test field is
// package-level — the toolchain reporting "no test files" — which verifies
// nothing but also promised nothing, so it is benign, not blocked.
//
// Lines that do not parse as event objects (stray tool output interleaved
// with the stream) are ignored: the caller combines this verdict with the
// process exit code, which already fails the step for any error severe
// enough to break the stream.
func evaluateGoTestJSONStream(stream string) goTestOutcome {
	outcome := goTestOutcome{Verdict: goTestVerdictPassed}
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var event goTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		switch event.Action {
		case "fail":
			outcome.FailedSubjects = append(outcome.FailedSubjects, goTestSubject(event))
		case "skip":
			if event.Test != "" {
				outcome.SkippedTests = append(outcome.SkippedTests, goTestSubject(event))
			}
		}
	}
	sort.Strings(outcome.FailedSubjects)
	sort.Strings(outcome.SkippedTests)
	switch {
	case len(outcome.FailedSubjects) > 0:
		outcome.Verdict = goTestVerdictFailed
	case len(outcome.SkippedTests) > 0:
		outcome.Verdict = goTestVerdictBlocked
	}
	return outcome
}

// goTestSubject renders an event's subject: "package.Test" for a test-level
// event, the bare package path for a package-level one.
func goTestSubject(event goTestEvent) string {
	switch {
	case event.Package != "" && event.Test != "":
		return event.Package + "." + event.Test
	case event.Test != "":
		return event.Test
	default:
		return event.Package
	}
}

// isGoTestJSONCommand reports whether a verifier command is a `go test`
// invocation streaming -json events — the only command whose exit code hides
// skipped tests and therefore needs the skip-aware verdict. Every other
// command keeps the plain exit-code verdict.
func isGoTestJSONCommand(command []string) bool {
	if len(command) < 2 || command[0] != "go" || command[1] != "test" {
		return false
	}
	for _, argument := range command[2:] {
		switch argument {
		case "-json", "--json", "-json=true", "--json=true":
			return true
		}
	}
	return false
}
