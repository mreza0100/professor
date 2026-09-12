#!/usr/bin/env bash
# pfm architecture ratchet — `make arch` (part of `make gate`). Design and the
# meaning of every check: docs/dev/pfm-architecture.md § Checks.
#
# Every check prints exactly one line: CHECK <id> PASS|FAIL|ERROR <detail>.
#   PASS  = the enumerator ran and found nothing beyond the committed baseline
#   FAIL  = a violation the baseline does not list, or a count above its baseline
#   ERROR = the enumerator could not run (baseline missing, grep could not read,
#           nothing parsed where something must parse) — never PASS
# Exit status: 0 all PASS · 1 any FAIL · 2 any ERROR.
#
# Baselines live in pfm/.arch/ and only ever shrink. `--measure` locks in what a
# wave fixed and nothing else: with a baseline present it keeps only entries
# still violating and lowers counts to today's, never adding an entry or
# raising a count. A genuinely new exception is a hand edit to .arch/, visible
# in review. With no baseline, --measure writes today's tree as the first one.
set -uo pipefail
PFM="${PFM:-$(cd "$(dirname "$0")/.." && pwd)}"
BASE="$PFM/.arch"
CEIL_SRC="${CEIL_SRC:-800}"; CEIL_TEST="${CEIL_TEST:-1000}"
# A baselined over-ceiling file may carry CEIL_SLACK lines of move churn (an
# import line gained when a symbol it calls moved packages); logic growth fails.
CEIL_SLACK="${CEIL_SLACK:-5}"
MODE="${1:-check}"
case "$MODE" in check|--measure) ;; *) echo "usage: arch-check.sh [--measure]" >&2; exit 2 ;; esac
rc=0
say() { printf 'CHECK %-22s %-7s %s\n' "$1" "$2" "$3"; case $2 in FAIL) [ "$rc" -lt 1 ] && rc=1 ;; ERROR) rc=2 ;; esac; }

cd "$PFM" || { say setup ERROR "cannot cd $PFM"; exit 2; }
T=$(mktemp -d) || { say setup ERROR "mktemp failed"; exit 2; }
trap 'rm -rf "$T"' EXIT
# repo_git reads the worktree's own index. Inside the dev fence a linked
# worktree's .git names a host path the container cannot see, so dev.sh iso
# hands over the mounted git dir and work tree instead.
repo_git() {
  if [[ -n "${PFM_DEV_REPO_GIT_DIR:-}" && -n "${PFM_DEV_REPO_WORK_TREE:-}" ]]; then
    git --git-dir="$PFM_DEV_REPO_GIT_DIR" --work-tree="$PFM_DEV_REPO_WORK_TREE" \
      -c safe.directory="$PFM_DEV_REPO_WORK_TREE" "$@"
  else
    git "$@"
  fi
}
# The file lists include untracked files (a wave's new package exists before its
# commit) and exclude deleted ones (a wave's removed file is gone before its commit).
repo_git ls-files -co --exclude-standard '*.go' | while read -r f; do [ -f "$f" ] && echo "$f"; done | sort -u > "$T/all.list"
grep -v '_test\.go$' "$T/all.list" > "$T/src.list"
grep '_test\.go$' "$T/all.list" > "$T/test.list"
[ -s "$T/src.list" ] || { say setup ERROR "no Go sources listed under $PFM — the enumerator did not run"; exit 2; }

# g <out> <list> <grep args...>: grep over a file list; returns 2 when grep could
# not read (rc ≥ 2), so an unreadable tree never passes as a clean one.
g() { local out=$1 list=$2; shift 2; grep "$@" $(cat "$list") > "$out"; [ $? -le 1 ] || return 2; }

# ratchet <id> <name> <current>: set ratchet — FAIL on any line the baseline lacks.
ratchet() {
  local id=$1 name=$2 cur=$3
  sort -u "$cur" -o "$cur"
  if [ "$MODE" = --measure ]; then
    mkdir -p "$BASE"
    if [ -f "$BASE/$name.txt" ]; then comm -12 "$BASE/$name.txt" "$cur" > "$T/measured"; mv "$T/measured" "$BASE/$name.txt"; else cp "$cur" "$BASE/$name.txt"; fi
    say "$id" MEASURE "$(wc -l < "$BASE/$name.txt" | tr -d ' ') entries -> .arch/$name.txt"; return
  fi
  [ -f "$BASE/$name.txt" ] || { say "$id" ERROR "baseline .arch/$name.txt missing — cannot tell new from old"; return; }
  local new gone
  new=$(comm -13 "$BASE/$name.txt" "$cur"); gone=$(comm -23 "$BASE/$name.txt" "$cur" | wc -l | tr -d ' ')
  if [ -n "$new" ]; then say "$id" FAIL "new: $(echo "$new" | tr '\n' ' ')"; return; fi
  local note=""; [ "$gone" -gt 0 ] && note="; $gone fixed — run --measure to lock the shrink"
  say "$id" PASS "$(wc -l < "$cur" | tr -d ' ') baselined, 0 new$note"
}

# ratchet_counts <id> <name> <current> [slack]: lines "<key> <count>" — FAIL on a
# key the baseline lacks or a count above the baseline's (+ slack). A count that
# shrank passes.
ratchet_counts() {
  local id=$1 name=$2 cur=$3 slack=${4:-0}
  sort -u "$cur" -o "$cur"
  if [ "$MODE" = --measure ]; then
    mkdir -p "$BASE"
    if [ -f "$BASE/$name.txt" ]; then
      awk 'FILENAME==ARGV[1] {base[$1]=$2; next} ($1 in base) {print $1" "($2<base[$1] ? $2 : base[$1])}' "$BASE/$name.txt" "$cur" | sort -u > "$T/measured"; mv "$T/measured" "$BASE/$name.txt"
    else cp "$cur" "$BASE/$name.txt"; fi
    say "$id" MEASURE "$(awk '{s+=$2} END {print s+0}' "$BASE/$name.txt") in $(wc -l < "$BASE/$name.txt" | tr -d ' ') keys -> .arch/$name.txt"; return
  fi
  [ -f "$BASE/$name.txt" ] || { say "$id" ERROR "baseline .arch/$name.txt missing — cannot tell new from old"; return; }
  local over
  over=$(awk -v slack="$slack" 'FILENAME==ARGV[1] {base[$1]=$2; next} !($1 in base) {print $1" (new "$2")"; next} $2>base[$1]+slack {print $1" ("base[$1]"->"$2")"}' "$BASE/$name.txt" "$cur")
  if [ -n "$over" ]; then say "$id" FAIL "$(echo "$over" | tr '\n' ' ')"; return; fi
  local now was; now=$(awk '{s+=$2} END {print s+0}' "$cur"); was=$(awk '{s+=$2} END {print s+0}' "$BASE/$name.txt")
  local note=""; [ "$now" -lt "$was" ] && note="; baseline $was — run --measure to lock the shrink"
  say "$id" PASS "$now in $(wc -l < "$cur" | tr -d ' ') keys, none above baseline$note"
}

# count_by_file <raw grep -n output> → "<file> <count>"
count_by_file() { cut -d: -f1 "$1" | sort | uniq -c | awk '{print $2" "$1}'; }

# C1/C2 size ceilings: an over-ceiling file may not appear, and a listed one may not grow.
: > "$T/c1"; while read -r f; do n=$(wc -l < "$f"); [ "$n" -gt "$CEIL_SRC" ] && echo "$f $n"; done < "$T/src.list" > "$T/c1"
ratchet_counts C1-ceiling-src ceiling-src "$T/c1" "$CEIL_SLACK"
while read -r f; do n=$(wc -l < "$f"); [ "$n" -gt "$CEIL_TEST" ] && echo "$f $n"; done < "$T/test.list" > "$T/c2"
ratchet_counts C2-ceiling-test ceiling-test "$T/c2" "$CEIL_SLACK"

# C3 cmd/pfm is dispatch: its non-test line total may not exceed .arch/cmd-budget.txt.
n=$(grep '^cmd/pfm/' "$T/src.list" | xargs cat | wc -l | tr -d ' ')
if [ "$MODE" = --measure ]; then mkdir -p "$BASE"; [ -f "$BASE/cmd-budget.txt" ] && [ "$(cat "$BASE/cmd-budget.txt")" -lt "$n" ] && n=$(cat "$BASE/cmd-budget.txt"); echo "$n" > "$BASE/cmd-budget.txt"; say C3-cmd-budget MEASURE "budget $n lines -> .arch/cmd-budget.txt"
elif [ ! -f "$BASE/cmd-budget.txt" ]; then say C3-cmd-budget ERROR "baseline .arch/cmd-budget.txt missing"
elif [ "$n" -gt "$(cat "$BASE/cmd-budget.txt")" ]; then say C3-cmd-budget FAIL "cmd/pfm = $n > budget $(cat "$BASE/cmd-budget.txt")"
else say C3-cmd-budget PASS "cmd/pfm = $n <= budget $(cat "$BASE/cmd-budget.txt")"; fi

# C4 primitives inside cmd/pfm (exec, SQL, raw fs writes) — each belongs to a package.
grep '^cmd/pfm/' "$T/src.list" > "$T/cmd.list"
if g "$T/raw" "$T/cmd.list" -nE 'exec\.Command|sql\.Open\(|os\.(WriteFile|Rename)\('; then count_by_file "$T/raw" > "$T/c4"; ratchet_counts C4-cmd-primitives cmd-primitives "$T/c4"
else say C4-cmd-primitives ERROR "grep could not read cmd/pfm sources"; fi

# C5 one tmux runner: outside internal/tmux/, a file that builds its own tmux
# invocation — resolves the tmux binary or assembles the -S socket argv itself.
grep -v '^internal/tmux/' "$T/src.list" > "$T/notmux.list"
if g "$T/raw" "$T/notmux.list" -lE 'deps\.Executable\("tmux"\)|\[\]string\{"-S", '; then cp "$T/raw" "$T/c5"; ratchet C5-tmux-runner tmux-runners "$T/c5"
else say C5-tmux-runner ERROR "grep could not read sources"; fi

# C6 one atomic writer: outside internal/atomicfile/, a file naming an atomic-write
# helper or hand-rolling the scratch-file-plus-rename pattern (os.CreateTemp + os.Rename).
grep -v '^internal/atomicfile/' "$T/src.list" > "$T/noatomic.list"
if g "$T/c6" "$T/noatomic.list" -lE '^func (writeAtomic|WriteAtomic|atomicWrite|AtomicWrite|writeFileAtomic|WriteFileAtomic)\(' &&
   g "$T/temps" "$T/noatomic.list" -l 'os\.CreateTemp('; then
  if [ ! -s "$T/temps" ] || g "$T/renames" "$T/temps" -l 'os\.Rename('; then
    [ -s "$T/temps" ] && cat "$T/renames" >> "$T/c6"; ratchet C6-atomic-write atomic-writers "$T/c6"
  else say C6-atomic-write ERROR "grep could not read the scratch-file writers"; fi
else say C6-atomic-write ERROR "grep could not read sources"; fi

# C7 one SQLite opener: sql.Open outside internal/sqlitedb/.
grep -v '^internal/sqlitedb/' "$T/src.list" > "$T/nosql.list"
if g "$T/raw" "$T/nosql.list" -nE 'sql\.Open\('; then count_by_file "$T/raw" > "$T/c7"; ratchet_counts C7-sql-open sql-openers "$T/c7"
else say C7-sql-open ERROR "grep could not read sources"; fi

# C8 negation-named directories.
if find internal cmd -type d \( -iname '*util*' -o -name helpers -o -name common -o -name misc -o -name shared \) > "$T/c8"; then ratchet C8-negation-dirs negation-dirs "$T/c8"
else say C8-negation-dirs ERROR "find failed under internal/ cmd/"; fi

# C9 every package states what it owns in a `// Package` doc comment.
: > "$T/c9"
for dir in $(xargs -n1 dirname < "$T/src.list" | sort -u); do
  grep -lq '^// Package ' $(grep "^$dir/[^/]*$" "$T/src.list") 2>/dev/null || echo "$dir" >> "$T/c9"
done
ratchet C9-package-doc no-package-doc "$T/c9"

# C10 MCP reaches chat verbs through typed calls, never argv into package main.
grep '^internal/mcpserv/' "$T/src.list" > "$T/mcp.list"
if [ ! -s "$T/mcp.list" ]; then say C10-mcp-argv ERROR "no internal/mcpserv sources listed"
elif g "$T/raw" "$T/mcp.list" -nE 'backend\.dispatch\(|cliAction\('; then count_by_file "$T/raw" > "$T/c10"; ratchet_counts C10-mcp-argv mcp-argv-calls "$T/c10"
else say C10-mcp-argv ERROR "grep could not read internal/mcpserv"; fi

# C11 one name per database file: "fleet.db" spelled in Go source.
if g "$T/raw" "$T/src.list" -n '"fleet\.db"'; then count_by_file "$T/raw" > "$T/c11"; ratchet_counts C11-db-names fleet-db-spellings "$T/c11"
else say C11-db-names ERROR "grep could not read sources"; fi

# C12 pfm/CLAUDE.md cites only what exists: packages, *.md docs, PFM_* variables something reads.
if [ ! -f CLAUDE.md ]; then say C12-claude-pointers ERROR "pfm/CLAUDE.md missing"
else
  : > "$T/c12"
  for p in $(grep -oE '^\| `[a-z/]+/`' CLAUDE.md | tr -d '|` '; grep -oE '`[a-z/]+/`' CLAUDE.md | grep -vE '^`(cmd|internal|testdata|shim|e2e|prompts)' | tr -d '`'); do
    [ -d "internal/$p" ] || [ -d "$p" ] || echo "$p" >> "$T/c12"
  done
  for f in $(grep -oE '`?[A-Z][A-Z_]+\.md`?' CLAUDE.md | tr -d '`' | sort -u); do [ -e "$f" ] || [ -e "../$f" ] || echo "$f" >> "$T/c12"; done
  for v in $(grep -oE 'PFM_[A-Z_]+' CLAUDE.md | sort -u); do grep -q "\"$v\"" $(cat "$T/src.list") || echo "$v" >> "$T/c12"; done
  ratchet C12-claude-pointers claude-dangling "$T/c12"
fi

# C13 tests mirror sources: every x.go has x_test.go.
while read -r s; do [ -e "${s%.go}_test.go" ] || echo "$s"; done < "$T/src.list" > "$T/c13"
ratchet C13-test-mirror untested-sources "$T/c13"

# C14 every dispatched top-level command appears in usage (structural once a command table lands).
cases=$(awk '/^func run\(/,/^}/' cmd/pfm/main.go | grep -oE 'case "[a-z-]+"' | grep -oE '"[a-z-]+"' | tr -d '"' | grep -vE '^(help|version|internal)$')
if [ -z "$cases" ]; then say C14-usage-parity ERROR "no case labels parsed from cmd/pfm/main.go run()"
else
  : > "$T/c14"; for c in $cases; do awk '/^func printUsage/,/^}/' cmd/pfm/main.go | grep -qE "\"  $c " || echo "$c" >> "$T/c14"; done
  ratchet C14-usage-parity usage-missing "$T/c14"
fi

# C15 every `pfm internal` entry appears in its usage line (structural once hooks.Table lands).
iv=$(awk '/^func runInternal\(/,/^}/' cmd/pfm/main.go | grep -oE 'args\[0\] (==|!=) "[a-z-]+"' | grep -oE '"[a-z-]+"' | tr -d '"' | sort -u)
line=$(grep -oE 'usage: pfm internal [a-z-]+(\|[a-z-]+)+' cmd/pfm/main.go | head -1)
if [ -z "$iv" ]; then say C15-internal-usage ERROR "no entries parsed from cmd/pfm/main.go runInternal()"
elif [ -z "$line" ]; then say C15-internal-usage ERROR "no multi-entry 'usage: pfm internal a|b' line in cmd/pfm/main.go"
else
  : > "$T/c15"; for v in $iv; do echo "$line" | grep -qE "(^|[ |])$v([|]|$)" || echo "$v" >> "$T/c15"; done
  ratchet C15-internal-usage internal-usage-missing "$T/c15"
fi

# C16 PFM_* environment reads stay inside internal/paths.
grep -v '^internal/paths/' "$T/src.list" > "$T/nopaths.list"
if g "$T/raw" "$T/nopaths.list" -nE 'Getenv\("PFM_|LookupEnv\("PFM_'; then count_by_file "$T/raw" > "$T/c16"; ratchet_counts C16-env-outside-paths env-outside-paths "$T/c16"
else say C16-env-outside-paths ERROR "grep could not read sources"; fi

exit $rc
