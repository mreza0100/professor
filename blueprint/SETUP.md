# SETUP — Installing the pipeline in your project

Step-by-step. Assumes you have a git repository and Claude Code installed.

---

## Prerequisites

- Git repository (at least one commit on `main` or `master`)
- Claude Code CLI installed and configured
- A clear list of your "subprojects" (or one if it's a single-project repo)
- A list of dev/test ports you currently use (so worktree allocations don't collide)

---

## Step 1 — Decide your structure

**Single-project repo** (one language, one codebase): all agents live at `.claude/agents/`. Skip child CLAUDE.md files.

**Monorepo** (multiple projects): root agents at `.claude/agents/`, project agents at `{project}/.claude/agents/`.

For each subproject, write down:
- Directory name (e.g., `api/`, `web/`, `worker/`)
- Tech stack (one line)
- Package manager (npm, pnpm, uv, cargo, etc.)
- Test command
- Build/dev command
- Default dev port (if it has one)

Keep this list — you'll plug it into multiple files.

---

## Step 2 — Copy the scaffolding

From the blueprint directory, copy:

```bash
# from your project root
cp -r tmp/claude-blueprint/templates/CLAUDE.md ./CLAUDE.md
cp -r tmp/claude-blueprint/templates/agents .claude/
cp -r tmp/claude-blueprint/templates/commands .claude/
cp -r tmp/claude-blueprint/templates/scripts .claude/
chmod +x .claude/scripts/*.sh
mkdir -p docs/agents docs/commands docs/dev/tasks/archive docs/dev/waves
echo ".worktrees/" >> .gitignore
echo "tmp/" >> .gitignore
```

---

## Step 3 — Edit `CLAUDE.md` (root)

Open the copied `CLAUDE.md` and replace placeholders:

| Placeholder | Replace with |
|-------------|--------------|
| `{PROJECT_NAME}` | Your project name |
| `{PROJECT_PITCH}` | One-line description |
| `{SUBPROJECT_LIST}` | Bulleted list of subprojects with tech stacks |
| `{REPO_TREE}` | Your actual repo structure (paste from `tree -L 2` output) |
| `{CHARACTER_OR_DELETE}` | Either define a character (Freudche-style) or delete the section |

Keep all the **Non-Negotiable Rules** sections verbatim unless you have a specific reason to change them.

---

## Step 4 — Edit the scripts

### `worktree.sh`

Open `.claude/scripts/worktree.sh` and adjust the dependency-installation block to match your projects. The template has a section labeled `# === Per-project setup — EDIT FOR YOUR STACK ===`. Replace each block with your actual setup:

```bash
# Example for a TS + Python monorepo
if [ -f "${worktree_dir}/api/package.json" ]; then
  (cd "${worktree_dir}/api" && pnpm install --frozen-lockfile) || true
fi
if [ -f "${worktree_dir}/worker/pyproject.toml" ]; then
  (cd "${worktree_dir}/worker" && uv sync) || true
fi
```

Also adjust the `.env.local` / `.env.test` rewriting blocks to match the env vars your projects use for ports.

### `alloc-ports.sh`

Open `.claude/scripts/alloc-ports.sh` and adjust the port ranges at the top:

```bash
API_BASE=3001        # your backend dev port range
WEB_BASE=5174        # your frontend dev port range
TEST_DB_BASE=5434    # your test database port range
# ... etc
```

Each range should be 99 slots wide and not overlap with anything you currently run.

---

## Step 5 — Edit child agent files (if monorepo)

For each subproject, create `{project}/.claude/agents/` with these files (copied and adapted):

- `planner.md`
- `architect.md`
- `developer.md` *(or `ai-engineer.md`, `devops.md`, etc.)*
- `qa.md`

Adapt:
- Tech stack references in the agent's "Context" section
- Test commands in the QA and developer agents
- Lint/typecheck/build commands in QA

The templates use generic placeholders like `{PROJECT_DIR}`, `{TEST_CMD}`, `{LINT_CMD}` — replace them.

---

## Step 6 — Edit `/build` command

Open `.claude/commands/build.md` and:

1. Replace the project list in **Step 1a** (parallel codebase analysis) with your actual subprojects.
2. Replace the architect/developer/QA invocation steps with one block per subproject.
3. Replace `freudche-be`, `freudche-fe`, etc. with your actual directory names.

Read the entire file once. The pipeline structure is the same; only the project names and paths change.

---

## Step 7 — Edit `gitter` agent

Open `.claude/agents/gitter.md` and update:

- The **Pipeline context** section's project list
- The **MERGE phase** section's per-project test commands (if they differ)
- The **POST-MERGE QA** section's project loop

Gitter is the most complex agent; touch it last and read the full file before editing.

---

## Step 8 — Test the install

Run a smoke test. From the project root:

```bash
.claude/scripts/alloc-ports.sh alloc test-pipeline
.claude/scripts/alloc-ports.sh list
.claude/scripts/alloc-ports.sh free test-pipeline
```

If those work, try a tiny `/build`:

```
/build add-readme-section
```

When Claude Code prompts you, walk through the steps. The first run will reveal anything you missed in adaptation.

---

## Step 9 — Tune and iterate

After your first real `/build`:

- If the pipeline asked for something it didn't need, invoke `/ccm` to remove it.
- If a step was missing, invoke `/ccm` to add it.
- If an agent was confused about your stack, edit its "Context" block.

The pipeline is designed to evolve. `/ccm` is the meta-tool for that — not "add more docs," but **edit the source**.

---

## Common gotchas

1. **Worktree script can't find `pnpm`/`uv`/etc.** — Make sure your shell environment is loaded. Add `source ~/.zshrc` or use absolute paths in the script.
2. **Port collisions on macOS** — `lsof -i :PORT` checks aren't always accurate for IPv6 — adjust if you see false positives.
3. **`gitter` tries to merge with conflicts unresolved** — that's a bug in your gitter setup; the template handles it, but if you simplified it, restore the conflict-detection block.
4. **Agents writing to permanent docs** — only `mono-documenter` should write to `docs/agents/` or `{project}/docs/`. If another agent tries, that's a `/ccm` fix at the source agent.
5. **`.worktrees/.ports` corrupted** — manually edit; format is one whitespace-separated line per pipeline.

---

## Optional: domain-specific commands

The Freudche pipeline includes specialized commands you may NOT need:

- `/officer` — GDPR/privacy compliance (clinical data domain)
- `/ckm` — clinical knowledge curation
- `/tpm` — therapist-product-manager
- `/mentor` — startup advisor
- `/marketer` — visibility & growth
- `/professor` — system analysis
- `/council` — multi-agent debate
- `/ca` — code auditor

These are NOT in the blueprint templates because they're domain-specific. If you want similar boutique commands for your domain, study Freudche's `.claude/commands/` and use them as patterns.
