package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opentendril/opentendril/cmd/stem/internal/conductor"
)

// TestParseGitSetupArgsDefaultsAndValidation covers the secure defaults and the
// per-posture required-flag enforcement.
func TestParseGitSetupArgsDefaultsAndValidation(t *testing.T) {
	// App posture is the default; managed checkout is the default.
	opts, err := parseGitSetupArgs([]string{"--substrate", "r", "--repo", "o/r", "--app-id", "1", "--key", "/k.pem"})
	if err != nil {
		t.Fatalf("valid app args rejected: %v", err)
	}
	if opts.posture != "app" || opts.checkout != "managed" {
		t.Fatalf("defaults = posture %q checkout %q, want app/managed", opts.posture, opts.checkout)
	}

	for name, args := range map[string][]string{
		"missing substrate":    {"--repo", "o/r", "--app-id", "1", "--key", "/k"},
		"missing repo":         {"--substrate", "r", "--app-id", "1", "--key", "/k"},
		"bad posture":          {"--posture", "nope", "--substrate", "r", "--repo", "o/r"},
		"bad checkout":         {"--substrate", "r", "--repo", "o/r", "--checkout", "nope", "--app-id", "1", "--key", "/k"},
		"app missing key":      {"--substrate", "r", "--repo", "o/r", "--app-id", "1"},
		"pat missing sign-key": {"--posture", "pat", "--substrate", "r", "--repo", "o/r", "--identity-name", "n", "--identity-email", "e@x"},
		"pat missing identity": {"--posture", "pat", "--substrate", "r", "--repo", "o/r", "--sign-key", "k"},
		"repo without slash":   {"--substrate", "r", "--repo", "noslash", "--app-id", "1", "--key", "/k"},
		"unknown flag":         {"--substrate", "r", "--repo", "o/r", "--bogus"},
	} {
		if _, err := parseGitSetupArgs(args); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// TestParseGitSetupArgsPatDefaultsTokenEnv verifies the pat posture defaults the
// token env when the caller omits it (a low-cognitive-load default).
func TestParseGitSetupArgsPatDefaultsTokenEnv(t *testing.T) {
	opts, err := parseGitSetupArgs([]string{
		"--posture", "pat", "--substrate", "r", "--repo", "o/r",
		"--sign-key", "KEY", "--identity-name", "N", "--identity-email", "e@x",
	})
	if err != nil {
		t.Fatalf("valid pat args rejected: %v", err)
	}
	if opts.tokenEnv != "GITHUB_TOKEN" {
		t.Fatalf("tokenEnv = %q, want the GITHUB_TOKEN default", opts.tokenEnv)
	}
}

// TestGeneratedAppConfigResolves proves the generated app-posture YAML is valid
// and resolves to a GitHub App credential in commit: api mode — the whole point
// of the command is that its output is directly usable.
func TestGeneratedAppConfigResolves(t *testing.T) {
	opts := gitSetupOptions{posture: "app", substrate: "r", repo: "o/r", appID: "4276558", keyPath: "/tmp/k.pem", checkout: "managed"}
	cred := resolveGenerated(t, opts)
	if cred.Method != conductor.CredentialApp {
		t.Fatalf("method = %q, want app", cred.Method)
	}
	if cred.CommitMode != conductor.CommitModeAPI {
		t.Fatalf("commit mode = %q, want api", cred.CommitMode)
	}
	if cred.App.AppID != "4276558" {
		t.Fatalf("app id = %q, want 4276558", cred.App.AppID)
	}
}

// TestGeneratedPatConfigResolves proves the generated pat-posture YAML resolves
// to a PAT credential carrying the dedicated signing key and identity.
func TestGeneratedPatConfigResolves(t *testing.T) {
	opts := gitSetupOptions{
		posture: "pat", substrate: "r", repo: "o/r", tokenEnv: "TENDRIL_GITHUB_PAT",
		signKey: "ABC123", identityName: "Tendril Bot", identityEmail: "bot@example.com", checkout: "managed",
	}
	cred := resolveGenerated(t, opts)
	if cred.Method != conductor.CredentialPAT {
		t.Fatalf("method = %q, want pat", cred.Method)
	}
	if cred.Sign.Method != "gpg" || cred.Sign.Key != "ABC123" {
		t.Fatalf("sign = %+v, want gpg/ABC123", cred.Sign)
	}
	if cred.Identity.Name != "Tendril Bot" || cred.Identity.Email != "bot@example.com" {
		t.Fatalf("identity = %+v, want the configured name/email", cred.Identity)
	}
}

// resolveGenerated writes the generated substrates.yaml to a temp dir and
// resolves the substrate's credential through the real conductor loader.
func resolveGenerated(t *testing.T, opts gitSetupOptions) conductor.ResolvedCredential {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "substrates.yaml"), []byte(renderSubstratesYAML(opts)), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := conductor.LoadSubstratesConfig(dir)
	if err != nil {
		t.Fatalf("load generated config: %v", err)
	}
	spec, isName := conductor.ResolveSubstrate(opts.substrate, cfg)
	if !isName || spec == nil {
		t.Fatalf("generated config did not resolve substrate %q", opts.substrate)
	}
	cred, err := conductor.ResolveSubstrateCredential(*spec, cfg)
	if err != nil {
		t.Fatalf("resolve generated credential: %v", err)
	}
	return cred
}

// TestRenderGrantsYAMLParses proves the generated grant is valid control-plane
// YAML for the named subject and substrate.
func TestRenderGrantsYAMLParses(t *testing.T) {
	opts := gitSetupOptions{substrate: "r", grantSubject: "claude"}
	out := renderGrantsYAML(opts)
	for _, want := range []string{"grants:", "claude:", "operationClasses: [git.commit, git.push]", "substrates: [r]"} {
		if !strings.Contains(out, want) {
			t.Errorf("generated grants missing %q:\n%s", want, out)
		}
	}
}

// TestWriteConfigFileRefusesClobber proves setup never silently overwrites
// hand-edited config without --force.
func TestWriteConfigFileRefusesClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "substrates.yaml")
	if err := writeConfigFile(path, "first", false); err != nil {
		t.Fatalf("first write failed: %v", err)
	}
	if err := writeConfigFile(path, "second", false); err == nil {
		t.Fatal("second write without --force overwrote an existing file")
	}
	if err := writeConfigFile(path, "second", true); err != nil {
		t.Fatalf("forced overwrite failed: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Fatalf("content = %q, want the forced overwrite", got)
	}
}
