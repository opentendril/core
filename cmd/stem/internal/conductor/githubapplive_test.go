//go:build livegithub

// This live check is excluded from the default build, and that exclusion is
// what lets the merge gate stay skip-aware. Under the skip-aware verdict a
// skipped test reads as blocked, never green — correct for code that could
// have run and did not, but this check mints a real token against real GitHub
// and can never run where those credentials are absent, which is everywhere
// except a maintainer's machine. Left as a t.Skip it would block every gate
// forever.
//
// A build tag rather than a list of tolerated skips: a list is fail-open the
// moment it drifts, whereas exclusion here is compile-time and reviewable. Add
// a credential-gated test with t.Skip and the gate blocks until someone
// decides deliberately — which is the point.
//
// Nothing is left unverified by this: the token-minting logic is covered by
// TestGithubAppInstallationToken and friends against a fake API. Only the live
// third-party round trip lives here.
//
// Run it with the credentials and the tag:
//
//	OPENTENDRIL_LIVE_APP_ID=... OPENTENDRIL_LIVE_APP_KEY=... OPENTENDRIL_LIVE_APP_REPO=... \
//	  go test -tags livegithub ./cmd/stem/internal/conductor/ -run TestGithubAppLive

package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGithubAppLive is an opt-in end-to-end check against the real GitHub API,
// run only when OPENTENDRIL_LIVE_APP_ID / OPENTENDRIL_LIVE_APP_KEY / OPENTENDRIL_LIVE_APP_REPO are set.
// It never prints the token.
func TestGithubAppLive(t *testing.T) {
	appID := os.Getenv("OPENTENDRIL_LIVE_APP_ID")
	keyPath := os.Getenv("OPENTENDRIL_LIVE_APP_KEY")
	repo := os.Getenv("OPENTENDRIL_LIVE_APP_REPO")
	if appID == "" || keyPath == "" || repo == "" {
		t.Skip("set OPENTENDRIL_LIVE_APP_ID / OPENTENDRIL_LIVE_APP_KEY / OPENTENDRIL_LIVE_APP_REPO to run the live GitHub App check")
	}
	appTokenMu.Lock()
	appTokenCache = map[string]cachedAppToken{}
	appTokenMu.Unlock()

	token, err := githubAppInstallationToken(context.Background(), AppCredential{AppID: appID, PrivateKeyPath: keyPath}, repo)
	if err != nil {
		t.Fatalf("live token mint failed: %v", err)
	}
	if !strings.HasPrefix(token, "ghs_") {
		t.Fatalf("unexpected token shape")
	}
	// Prove the token actually authenticates: list the installation's repos.
	req, _ := http.NewRequest(http.MethodGet, "https://api.github.com/installation/repositories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("live api call failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("installation token did not authenticate: status %d", resp.StatusCode)
	}
	var out struct {
		TotalCount int `json:"total_count"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	fmt.Printf("live check OK: token authenticated, installation can see %d repo(s)\n", out.TotalCount)

	cred := ResolvedCredential{Method: CredentialApp, App: AppCredential{AppID: appID, PrivateKeyPath: keyPath}}
	authEnv, err := materializeGitAuth(context.Background(), cred, repo)
	if err != nil {
		t.Fatalf("materializeGitAuth failed: %v", err)
	}

	// The credential helper serves the real token to git via stdin.
	fill := exec.Command("git", "credential", "fill")
	fill.Env = append(os.Environ(), authEnv...)
	fill.Stdin = strings.NewReader("protocol=https\nhost=github.com\n\n")
	fillOut, err := fill.Output()
	if err != nil {
		t.Fatalf("git credential fill failed: %v", err)
	}
	if !strings.Contains(string(fillOut), "username=x-access-token") || !strings.Contains(string(fillOut), "password="+token) {
		t.Fatalf("credential helper did not serve the expected credentials")
	}
	fmt.Println("live check OK: credential helper serves x-access-token + the installation token")

	// End-to-end: clone with auth in the environment only, and prove the token
	// never lands in argv or the persisted .git/config.
	dest := filepath.Join(t.TempDir(), "clone")
	clone := exec.Command("git", "clone", "--depth", "1", "--", repo, dest)
	clone.Env = append(os.Environ(), authEnv...)
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("env-auth clone failed: %v (%s)", err, out)
	}
	cfg, _ := os.ReadFile(filepath.Join(dest, ".git", "config"))
	if strings.Contains(string(cfg), token) || strings.Contains(string(cfg), "x-access-token") {
		t.Fatalf(".git/config leaked the token — hardening regressed")
	}
	fmt.Println("live check OK: env-auth clone succeeded and .git/config is token-free")
}
