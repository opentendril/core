package receptors

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/opentendril/opentendril/cmd/stem/internal/core"
)

// PollinatorTokenHandler mints short-lived access tokens from a durable
// Pollinator credential (the refresh root). The root is presented ONLY here; the
// minted token is what every other surface then accepts per request. This route
// is the single seam where a durable secret is exchanged for a short-lived one.
type PollinatorTokenHandler struct {
	Signer      *core.StemSigner
	Credentials PollinatorCredentials
}

// NewPollinatorTokenHandler builds the mint handler over a signer and the set of
// issued credentials it authenticates roots against.
func NewPollinatorTokenHandler(signer *core.StemSigner, credentials PollinatorCredentials) *PollinatorTokenHandler {
	return &PollinatorTokenHandler{Signer: signer, Credentials: credentials}
}

// mintTokenRequest is the optional body: a shorter lifetime may be requested,
// never a longer one. Zero or omitted takes the default.
type mintTokenRequest struct {
	TTLSeconds int `json:"ttlSeconds"`
}

type mintTokenResponse struct {
	Token     string    `json:"token"`
	Pollen    string    `json:"pollen"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// HandleMint authenticates a presented root credential and returns a fresh
// signed access token for the Pollen it resolves to.
//
// It accepts ONLY a credential-shaped bearer: an access token cannot mint
// another token (there is no self-refresh without the root), and a plain bearer
// key cannot mint for an identity it merely names. An unknown or revoked root
// resolves to nothing and is refused — the mint path never issues a token for an
// identity the caller could not prove.
func (h *PollinatorTokenHandler) HandleMint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.Signer == nil {
		http.Error(w, "access-token minting is not configured", http.StatusServiceUnavailable)
		return
	}

	presented := bearerToken(r)
	if !core.LooksLikePollinatorCredential(presented) {
		http.Error(w, "a Pollinator credential is required to mint an access token", http.StatusUnauthorized)
		return
	}
	pollen := core.ResolvePollenFromCredential(h.Credentials, presented)
	if pollen == "" {
		http.Error(w, "unknown or revoked Pollinator credential", http.StatusUnauthorized)
		return
	}

	var body mintTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	token, err := h.Signer.MintAccessToken(pollen, time.Duration(body.TTLSeconds)*time.Second, core.AccessTokenScope{})
	if err != nil {
		// The only mint failures are policy ones (ttl over the cap, empty Pollen),
		// which are the caller's request, not a server fault.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Read the authoritative expiry back off the signed token rather than
	// recomputing it, so the response cannot drift from what was signed.
	claims, _ := h.Signer.VerifyAccessToken(token)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mintTokenResponse{
		Token:     token,
		Pollen:    claims.Pollen,
		ExpiresAt: claims.ExpiresAt,
	})
}

// Register mounts the mint route. It is self-authenticating (the root credential
// is the auth), so it takes no outer bearer wrapper.
func (h *PollinatorTokenHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/pollinator/token", h.HandleMint)
}
