#!/usr/bin/env bash
set -uo pipefail

# PostToolUse hook — formats the one Professor-owned .md file just written, under
# the repo-root `.rumdl.toml` policy (see /quality:md-forlint). Receives hook JSON
# on stdin.
#
# What this reports when IT is broken: a missing `jq` or `rumdl`, and a failing
# `rumdl fmt`, each print one stderr line naming the reason (hook stderr reaches
# the session as feedback) — never a silent skip that looks like a clean format.
# Every path exits 0: a formatter must not block a write.

INPUT=$(cat)

if ! command -v jq >/dev/null 2>&1; then
  echo "format-md: jq not found — markdown left unformatted" >&2
  exit 0
fi

FILE_PATH=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty')

[[ -z "$FILE_PATH" ]] && exit 0
[[ "$FILE_PATH" != *.md ]] && exit 0
[[ ! -f "$FILE_PATH" ]] && exit 0

REPO_ROOT=$(git -C "$(dirname "$FILE_PATH")" rev-parse --show-toplevel 2>/dev/null) || exit 0
REL_PATH="${FILE_PATH#"$REPO_ROOT"/}"

# Only format Professor-owned files — not user source code. Generated mirrors
# (AGENTS.md, .codex/) are rebuilt by their compiler, never formatted here.
case "$REL_PATH" in
  CLAUDE.md) ;;
  .claude/*.md) ;;
  docs/commands/*.md) ;;
  docs/agents/*.md) ;;
  docs/references/*.md) ;;
  docs/features/*.md) ;;
  docs/runbooks/*.md) ;;
  docs/facts/*.md) ;;
  docs/epics/*.md) ;;
  docs/dev/*.md) ;;
  docs/business/*.md) ;;
  */CLAUDE.md|*/.claude/*.md) ;;
  *) exit 0 ;;
esac

if ! command -v rumdl >/dev/null 2>&1; then
  echo "format-md: rumdl not found — ${REL_PATH} left unformatted (\`pfm install\` provisions it)" >&2
  exit 0
fi

# rumdl resolves `[per-file-ignores]` globs against the CURRENT DIRECTORY, not
# against the config's own location: run it from the repo root or the whole
# category policy silently fails to match.
if ! (cd "$REPO_ROOT" && rumdl fmt "$REL_PATH" >/dev/null 2>&1); then
  echo "format-md: rumdl fmt failed on ${REL_PATH}" >&2
fi

exit 0
