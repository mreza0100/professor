#!/usr/bin/env bash
# fleet.sh — runs INSIDE the readme-gif fence container (record.sh starts it):
# installs pfm from the mounted checkout exactly as a user would, builds the
# fake `claude`, dresses the shell, and stands up an invented, talking fleet
# across two Claude seats with two Codex homes beside them.
#
# Every name, repo, message, and number here is invented. Run from /tmp: the
# container's working dir /worktree holds a .git file pointing at a host path
# the container cannot resolve, so any git command there fails.
#
# BROKEN STATE: any failing step exits non-zero with its own message (set -e);
# the final `pfm ls --plain` must list the fleet — an empty listing means the
# fake harness never reached pfm's statusline, so no pane became a live row.
set -euo pipefail
export PATH="$HOME/.local/bin:$PATH"
SRC=/worktree
HERE="$SRC/infra/readme-gif"

# 1. pfm, built with the release stamp (the image has no make), installed normally.
mkdir -p "$HOME/.local/bin" "$HOME/.claude" "$HOME/.cc/2" "$HOME/.codex" "$HOME/.codex2" "$HOME/.config/pfm"
(cd "$SRC/pfm" && go build -ldflags "-X main.version=$(cat ../VERSION)" -o "$HOME/.local/bin/pfm" ./cmd/pfm)
# A first Claude Code run leaves settings.json behind; pfm wires its statusline
# and hooks only into one that exists.
for d in .claude .cc/2; do [ -f "$HOME/$d/settings.json" ] || echo '{}' > "$HOME/$d/settings.json"; done
# pfm's config validation requires a Codex sign-in per home. Placeholders only —
# the seeded usage cache (seed-limits.py) means none is ever spent.
for h in .codex .codex2; do
  echo '{"tokens":{"access_token":"demo-not-a-token","account_id":"demo-account"}}' > "$HOME/$h/auth.json"
done
cat > "$HOME/.config/pfm/pfm.config.json" <<'EOF'
{
  "version": 2,
  "accounts": [
    {"id": 1, "configDir": "~/.claude", "emoji": "🥇"},
    {"id": 2, "configDir": "~/.cc/2", "emoji": "🥈"}
  ],
  "codex": {
    "homes": [
      {"id": 1, "home": "~/.codex", "emoji": "🥇"},
      {"id": 2, "home": "~/.codex2", "emoji": "🥈"}
    ]
  }
}
EOF
(cd "$SRC" && pfm install --yes --skip-harvest --skip-themes --skip-engine codex >/dev/null)

# 2. The fake harness (see fakeclaude/main.go for the contract it keeps).
(cd "$HERE/fakeclaude" && GOFLAGS=-buildvcs=false go build -o /usr/local/bin/claude .)

# 3. The shell: Starship with the Catppuccin powerline preset (Nerd Font glyphs;
#    the tape sets FiraCode Nerd Font Mono), loaded after pfm's shim.
command -v starship >/dev/null || curl -fsSL https://starship.rs/install.sh | sh -s -- -y >/dev/null
starship preset catppuccin-powerline -o "$HOME/.config/starship.toml"
grep -q 'starship init zsh' "$HOME/.zshrc" || echo 'eval "$(starship init zsh)"' >> "$HOME/.zshrc"

# 4. Invented repos, each on develop (the prompt and statusline show the branch).
git config --global user.name demo
git config --global user.email demo@example.invalid
git config --global init.defaultBranch main
for p in api webapp ops docs-site; do
  mkdir -p "/work/$p"
  if [ ! -d "/work/$p/.git" ]; then
    (cd "/work/$p" && git init -q && git commit -q --allow-empty -m init && git checkout -q -b develop)
  fi
done

# 5. The fleet. Spawns pin --engine claude: with Codex homes configured, the
#    default engine resolution looks for a codex binary the fence does not have.
sock_of() { pfm ls --tsv 2>/dev/null | awk -F'\t' -v n="$1" '$5 == n {print $11; exit}'; }
id_of() { pfm ls --tsv 2>/dev/null | awk -F'\t' -v n="$1" '$5 == n {print $2; exit}'; }
spawn() { # spawn <project> <account> <name> <first prompt>
  (cd "/work/$1" && pfm chat new --engine claude --account "$2" --name "$3" "$4" >/dev/null)
  sleep 3
}
say() { # say <from> <to> <message> — a signed turn, so the ledger can draw the edge
  CHAT_SENDER_LABEL="$1" CHAT_SENDER_SESSION="$(sock_of "$1")" CHAT_SENDER_SID="$(id_of "$1")" \
    pfm chat inject "$2" "$3" >/dev/null
  sleep 2
}
spawn webapp 1 ORCHESTRATOR "Coordinate the checkout redesign across the team"
spawn api 2 PAYMENTS_LEAD "Move the payments service onto the new ledger"
spawn ops 1 FLEET_BUILDER "Provision the staging cluster"
spawn docs-site 2 README_REWRITE "Rewrite the README around the product pitch"
spawn webapp 1 DESIGN_PASS "Apply the new design tokens to the checkout flow"
spawn webapp 2 A11Y_AUDIT "Audit the checkout flow for accessibility"
spawn webapp 1 DEPLOY_PROD "Prepare the production deploy checklist"
spawn api 2 SCHEMA_MIGRATION "Write the migration for the ledger tables"
spawn api 1 RATE_LIMITER "Add a token-bucket limiter to the public API"
spawn ops 1 INCIDENT_REVIEW "Write the postmortem for the queue outage"
say ORCHESTRATOR DESIGN_PASS "Checkout tokens are merged — take the payment step next."
say DESIGN_PASS ORCHESTRATOR "ACK — payment step in progress, screenshots within the hour."
say ORCHESTRATOR FLEET_BUILDER "Checkout ships Friday — is staging ready for the load test?"
say FLEET_BUILDER ORCHESTRATOR "ACK — staging is up, load test at 14:00."
say PAYMENTS_LEAD SCHEMA_MIGRATION "Ledger schema is frozen; generate the migration."
say README_REWRITE ORCHESTRATOR "Need one screenshot of the new checkout for the README."
pfm chat end DEPLOY_PROD >/dev/null
pfm chat end RATE_LIMITER >/dev/null

# 6. `pfm chat new` from this bare shell records UNSIGNED spawns (the spawner is
#    derived from tmux-pane ancestry, which a docker-exec shell lacks), and the
#    cosmos tab rightly warns that it cannot draw them. Drop exactly those rows;
#    the signed inject rows stay and draw the edges.
python3 - <<'EOF'
import os, sqlite3, sys
path = os.path.expanduser("~/.cc/fleet.db")
if not os.path.exists(path):
    sys.exit(f"fleet.sh: comms ledger not found at {path}")
con = sqlite3.connect(path)
removed = con.execute(
    "DELETE FROM comms WHERE kind='spawn' AND sender_session='' AND sender_uuid='' AND sender_label=''"
).rowcount
signed = con.execute("SELECT count(*) FROM comms WHERE kind='inject'").fetchone()[0]
con.commit()
if signed == 0:
    sys.exit("fleet.sh: no signed messages in the ledger — the sky would draw no edges")
print(f"ledger: removed {removed} unsigned spawn rows, kept {signed} signed messages")
EOF
pfm ls --plain
