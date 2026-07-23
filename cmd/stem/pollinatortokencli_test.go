package main

import (
	"testing"
	"time"

	"github.com/opentendril/opentendril/cmd/stem/internal/core"
)

// TestMintPollinatorAccessTokenHappyPath: a named Pollen yields a token that
// verifies to that identity under the Stem signer.
func TestMintPollinatorAccessTokenHappyPath(t *testing.T) {
	dir := t.TempDir()
	token, expiresAt, err := mintPollinatorAccessToken(dir, "claude", 5*time.Minute)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if !core.LooksLikeAccessToken(token) {
		t.Fatalf("token %q is not access-token shaped", token)
	}
	signer, err := core.LoadOrCreateStemSigner(dir)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	claims, ok := signer.VerifyAccessToken(token)
	if !ok || claims.Pollen != "claude" {
		t.Fatalf("verify: ok=%v pollen=%q, want claude", ok, claims.Pollen)
	}
	if expiresAt.IsZero() || expiresAt.Before(time.Now().UTC()) {
		t.Fatalf("expiresAt = %s, want a future time", expiresAt)
	}
}

// TestMintPollinatorAccessTokenRejectsOverCap: a TTL above the hard max is
// refused, not clamped — the mint error must surface.
func TestMintPollinatorAccessTokenRejectsOverCap(t *testing.T) {
	dir := t.TempDir()
	_, _, err := mintPollinatorAccessToken(dir, "claude", core.MaxAccessTokenTTL+time.Minute)
	if err == nil {
		t.Fatal("expected over-cap mint to fail")
	}
}

// TestMintPollinatorAccessTokenRejectsEmptyPollen: a token must name who it
// authenticates as.
func TestMintPollinatorAccessTokenRejectsEmptyPollen(t *testing.T) {
	dir := t.TempDir()
	_, _, err := mintPollinatorAccessToken(dir, "  ", 0)
	if err == nil {
		t.Fatal("expected empty-pollen mint to fail")
	}
}
