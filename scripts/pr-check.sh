#!/usr/bin/env bash
# pr-check.sh — dry-run pull-request review Sprout.
#
# Given a PR number, this script fetches the PR's metadata and unified diff
# with `gh`, then delegates the review to an autonomous Tendril Sprout inside
# a read-only terrarium (`tendril sprout run`), driven by a local model via
# Ollama and shaped by the embedded `pr-reviewer` genotype. The Sprout's
# review findings are printed to stdout and written to a file.
#
# DRY RUN BY DESIGN: this script never posts to GitHub. The pr-reviewer
# genotype denies every mutating plasmid (runCommand, writeFile,
# networkFetch, injectPlasmid) and the substrate is mounted readonly, so the
# terrarium is ephemeral and all its edits are discarded. When a human is
# happy with the review, the human posts it:
#
#   gh pr comment <PR_NUMBER> --body-file <review-file>
#
# Usage:
#   scripts/pr-check.sh <pr-number> [model]
#
# Environment:
#   TENDRIL_BIN                  tendril binary to use        (default: tendril)
#   LOCAL_INFERENCE_URL          Ollama OpenAI-compatible URL (default: http://localhost:11434/v1)
#   TENDRIL_PR_CHECK_DIFF_CAP    max diff bytes fed to the model (default: 24000)
#   TENDRIL_PR_CHECK_GENOTYPE    reviewer genotype            (default: pr-reviewer)
set -euo pipefail

PR_NUMBER="${1:?usage: scripts/pr-check.sh <pr-number> [model]}"
MODEL="${2:-qwen2.5-coder:7b}"
GENOTYPE="${TENDRIL_PR_CHECK_GENOTYPE:-pr-reviewer}"
TENDRIL_BIN="${TENDRIL_BIN:-tendril}"
DIFF_CAP="${TENDRIL_PR_CHECK_DIFF_CAP:-24000}"

export LOCAL_INFERENCE_URL="${LOCAL_INFERENCE_URL:-http://localhost:11434/v1}"
export DEFAULT_LLM_PROVIDER="${DEFAULT_LLM_PROVIDER:-local}"

REPO_ROOT="$(git rev-parse --show-toplevel)"

command -v gh >/dev/null || { echo "❌ gh CLI is required" >&2; exit 1; }
command -v "$TENDRIL_BIN" >/dev/null || { echo "❌ tendril binary not found (set TENDRIL_BIN)" >&2; exit 1; }

# The Sprout run is anchored in its own scratch workspace so the session
# store (history.db) and the substrates.yaml it resolves are isolated from
# whatever repository the human happens to be standing in.
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/tendril-pr-check-XXXXXX")"
trap 'rm -rf "$WORKDIR"' EXIT

echo "🔎 Fetching PR #${PR_NUMBER} ..." >&2
gh pr view "$PR_NUMBER" --json number,title,body \
  --template $'PR #{{.number}}: {{.title}}\n\n{{.body}}\n' > "$WORKDIR/pr-meta.txt"
gh pr diff "$PR_NUMBER" > "$WORKDIR/pr.diff"

DIFF_BYTES="$(wc -c < "$WORKDIR/pr.diff")"
TRUNCATED_NOTE=""
if [ "$DIFF_BYTES" -gt "$DIFF_CAP" ]; then
  head -c "$DIFF_CAP" "$WORKDIR/pr.diff" > "$WORKDIR/pr.diff.capped"
  mv "$WORKDIR/pr.diff.capped" "$WORKDIR/pr.diff"
  TRUNCATED_NOTE="(NOTE: the diff was truncated from ${DIFF_BYTES} to ${DIFF_CAP} bytes to fit the local model's context window; say so in your SUMMARY.)"
  echo "⚠️ Diff truncated from ${DIFF_BYTES} to ${DIFF_CAP} bytes for the local model." >&2
fi

# Read-only named substrate: the terrarium mounts the repository but every
# modification is discarded — the review is the only artifact.
cat > "$WORKDIR/substrates.yaml" <<EOF
substrates:
  pr-check-workspace:
    path: ${REPO_ROOT}
    readonly: true
EOF

{
  echo "Review this GitHub pull request. Produce your structured review findings as your final answer. ${TRUNCATED_NOTE}"
  echo
  cat "$WORKDIR/pr-meta.txt"
  echo
  echo "--- UNIFIED DIFF ---"
  cat "$WORKDIR/pr.diff"
} > "$WORKDIR/transcript.txt"

cd "$WORKDIR"

echo "🧬 Sprouting review session (provider=local model=${MODEL} genotype=${GENOTYPE}) ..." >&2
SESSION_ID="$("$TENDRIL_BIN" session create --provider local --model "$MODEL" --genotype "$GENOTYPE" \
  | sed -n 's/.*"sessionId": "\([^"]*\)".*/\1/p' | head -n 1)"
[ -n "$SESSION_ID" ] || { echo "❌ Failed to create Tendril session" >&2; exit 1; }

REVIEW_FILE="${REPO_ROOT}/.tendril/pr-check-${PR_NUMBER}.md"
mkdir -p "$(dirname "$REVIEW_FILE")"

"$TENDRIL_BIN" sprout run \
  --substrate pr-check-workspace \
  --session "$SESSION_ID" \
  "$(cat "$WORKDIR/transcript.txt")" > "$REVIEW_FILE"

echo
echo "===================== PR #${PR_NUMBER} REVIEW (DRY RUN) ====================="
cat "$REVIEW_FILE"
echo "=============================================================================="
echo
echo "🌿 Review written to ${REVIEW_FILE} — NOT posted to GitHub."
echo "   A human can post it with:"
echo "   gh pr comment ${PR_NUMBER} --body-file ${REVIEW_FILE}"
