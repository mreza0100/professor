# BLUEPRINT — The System

The philosophy and architecture of the pipeline. Read this before installing it.

---

## Core principles

### 1. The pipeline is mandatory, no exceptions

Every code change goes through one of two paths:

- **`/build {feature}`** — full pipeline (planner → architect → developer → QA → gitter merge). For features.
- **`/jc {bug}`** — hotfix mode (locate → fix → test → gitter commit on main). For bugs.

**Never edit code directly on `main`.** The only commits allowed on `main` are merge commits from `gitter` (after QA passes) or single-purpose commits from `/jc` (after tests pass). This rule exists because:

- It forces you to articulate WHAT you're doing before writing code.
- It creates a paper trail (pipeline docs) for every change.
- It catches bugs at QA, not in production.

### 2. Only ONE agent touches git

The `gitter` agent is the **single git operator**. No other agent runs `git add`, `git commit`, `git merge`, `git push`. This isn't bureaucracy — it's safety:

- Centralizes destructive operations behind one well-tested code path.
- Prevents agents from racing each other for the merge.
- Makes "what got committed" auditable.

If an agent needs to commit, it asks gitter. Gitter has phases: SETUP, MERGE, DOCS-COMMIT, JC-COMMIT, LOCK, UNLOCK, PUSH, PULL.

### 3. Worktree isolation per pipeline

Every `/build` invocation creates:
- A git branch: `pipeline/{name}`
- A worktree checkout: `.worktrees/{name}/` (full repo)
- A unique port allocation (BE_PORT, FE_PORT, test ports, etc.)
- Pipeline docs: `docs/dev/tasks/{name}/`

This means you can run **multiple pipelines in parallel on the same machine** without port collisions or git state corruption. When the pipeline completes, gitter merges to main, the worktree is removed, and the docs are archived.

### 4. QA gates everything

The pipeline runs QA on the worktree branch BEFORE merging to main. Test failures block the merge. Then it runs **post-merge QA on main** to verify the merge didn't break anything. Zero tolerance for "pre-existing failures" — if a test was broken before your pipeline, your pipeline fixes it. Every pipeline leaves main cleaner than it found it.

### 5. Path variables, not hardcoded paths

Agents receive paths as variables:

| Variable | Purpose | Example |
|----------|---------|---------|
| `$PIPELINE` | Pipeline name (kebab-case, unique) | `session-notes` |
| `$DOCS` | Pipeline docs from repo root | `docs/dev/tasks/session-notes` |
| `$DOCS_REL` | Pipeline docs from worktree | `../../../docs/dev/tasks/session-notes` |
| `$WORKTREE` | Worktree directory | `.worktrees/session-notes` |
| `$ARCHIVE` | Archive parent | `docs/dev/tasks/archive` |
| `$CDOCS` | Command-owned docs root | `docs/commands` |

Agents NEVER hardcode `docs/dev/tasks/...` — they use what `/build` passes them. Path conventions can change without rewriting every agent.

### 6. Self-improvement at the source

When something goes wrong in the pipeline, you don't write a "lesson" file. You invoke `/ccm` (the meta-agent that owns the pipeline itself). CCM edits the actual agent definition or command instructions to prevent the bug class going forward. **Surgery at the source.** Pipeline files are meant to evolve.

### 7. Documentation discipline

Two kinds of docs:

- **Pipeline docs** (`docs/dev/tasks/{name}/`) — temporary, archived after pipeline completes. Plans, architecture decisions, QA reports, runbooks.
- **Permanent docs** (`docs/agents/`, `freudche-be/docs/`, etc.) — long-lived. Only the `documenter` agent writes here, and only after a pipeline merges.

This prevents docs from rotting under speculative "we might do this someday" content.

---

## Architecture

```
                              ┌──────────────┐
                              │  /build req  │
                              └──────┬───────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child planners     │ (parallel — one per affected project)
                          │  analyze codebase   │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  mono-planner       │ → docs/dev/tasks/{name}/1-plan.md
                          │  consolidates plan  │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  gitter SETUP       │ → worktree, branch, ports
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  mono-architect     │ → 3-architecture.md
                          │  cross-project      │   (API contracts, types)
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child architects   │ (parallel — per project)
                          │  + library research │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child developers   │ (parallel — implements code)
                          │  + happy-path tests │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child QAs          │ (parallel — adversarial tests)
                          │  + bug reports      │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  fix loop           │ (developer fixes QA bugs;
                          │                     │   loops until green or budget)
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  gitter MERGE       │ → squash to main
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  POST-MERGE QA      │ (run on main, catches merge bugs)
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  documenter         │ → updates permanent docs
                          │                     │   archives pipeline dir
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  gitter DOCS-COMMIT │
                          └─────────────────────┘
```

### Agent roles

**Root agents** (orchestration-level, project-agnostic):

| Agent | Role |
|-------|------|
| `mono-planner` | Reads parallel codebase analysis from child planners, decides routing (which projects are affected), writes the cross-project plan |
| `mono-architect` | Designs cross-project architecture (API contracts, shared types) and runs library research inline |
| `gitter` | The ONLY agent that runs git. Phases: SETUP, MERGE, DOCS-COMMIT, JC-COMMIT, LOCK, UNLOCK, PUSH, PULL |
| `mono-documenter` | Merges pipeline decisions into permanent docs after merge passes; archives pipeline directory |

**Child agents** (per-project, in `{project}/.claude/agents/`):

| Agent | Role |
|-------|------|
| `planner` | Analyzes its project's codebase + translates the root plan into project-scoped tasks |
| `architect` | Designs the project's architecture for this feature; researches libraries inline |
| `developer` | Implements the code, writes happy-path tests, debugs QA-reported bugs |
| `qa` | Writes adversarial tests (edge cases, unhappy paths), runs the full test suite, reports bugs |
| `ui-ux` *(frontend only)* | Designs colors, typography, spacing, component classes |
| `db-admin` *(infra/data layer only)* | Owns schemas, migrations, seeding — single source of truth for the data layer |
| `devops` *(infra only)* | Implements infra configs (Docker, AWS, CI/CD) |

### Commands

| Command | Purpose |
|---------|---------|
| `/build {feature}` | Full pipeline for new features |
| `/jc {bug}` | Hotfix mode — locate, fix, test, commit on main |
| `/ccm {request}` | Update pipeline infrastructure (agents, commands, conventions) |
| `/dev` | Start/stop/restart/status the local dev environment |
| `/wave {file}` | Run multiple `/build` waves in parallel from a task list |
| `/git push|pull|...` | Direct gateway to gitter for explicit git ops |

### Scripts

| Script | Purpose |
|--------|---------|
| `worktree.sh create|remove|list` | Manages git worktrees and bootstraps env (deps, env files, ports) |
| `alloc-ports.sh alloc|free|list` | Reserves unique port slots per worktree (concurrency-safe via flock) |
| `dev.sh` | Starts dev servers across all projects with the right env |

---

## File layout

```
your-project/
├── CLAUDE.md                          ← root rules, project structure, character
├── .claude/
│   ├── agents/                        ← root agents (mono-planner, mono-architect, gitter, mono-documenter)
│   ├── commands/                      ← /build, /jc, /ccm, /dev (and any extras you add)
│   ├── scripts/                       ← worktree.sh, alloc-ports.sh, dev.sh
│   └── settings.json                  ← permissions, env vars, hooks
├── {project-a}/                       ← e.g., backend
│   ├── CLAUDE.md                      ← project-specific rules
│   └── .claude/agents/                ← project agents (planner, architect, developer, qa)
├── {project-b}/                       ← e.g., frontend
│   ├── CLAUDE.md
│   └── .claude/agents/
├── docs/
│   ├── agents/                        ← cross-project permanent docs (architecture, API, map)
│   ├── commands/{cmd}/                ← command-owned docs ($CDOCS root)
│   │   ├── references/                ← must-know
│   │   ├── research/                  ← looked-up material
│   │   └── resources/                 ← static assets
│   └── dev/
│       ├── tasks/{pipeline}/          ← temp pipeline docs
│       ├── tasks/archive/             ← completed pipelines
│       └── waves/                     ← wave runner artifacts
└── .worktrees/                        ← git worktree checkouts (gitignored)
    ├── {pipeline}/                    ← per-pipeline checkout
    ├── .ports                         ← port allocation registry
    └── .merge-lock/                   ← project-scoped merge locks
```

For a single-project repo, drop the `{project-a}/`, `{project-b}/` layer — agents live in `.claude/agents/` only, no child CLAUDE.md files.

---

## Non-negotiable rules baked into the templates

These rules appear in `CLAUDE.md` and are referenced by every agent. They are the contract:

1. **No code on main except gitter merges and `/jc` commits.**
2. **Only gitter runs git commands.**
3. **Never commit broken code** — QA must pass first.
4. **Never merge before QA passes** — both pre-merge and post-merge.
5. **Never reuse pipeline names** — check `docs/dev/tasks/`, `docs/dev/tasks/archive/`, `.worktrees/` first.
6. **Never run destructive git commands** — no `--force`, no `reset --hard`, no `clean -fdx` without explicit user approval.
7. **Never swallow exceptions silently** — every catch logs the full traceback. Silent failures hide bugs.
8. **No mocking internal dependencies within 1 hop** — mock only external services (LLM APIs, third-party SaaS). Real DB, real queue, real internal services.
9. **All failing tests are blocking** — no "pre-existing failure" excuse.
10. **All infrastructure ops go through a single Makefile/script** — no direct `docker exec`, `psql`, `aws sqs` from agent code.

---

## What you adapt vs. what you keep

**Keep verbatim:**
- The `gitter` agent (with project list adjusted)
- The `worktree.sh` and `alloc-ports.sh` scripts (with port ranges adjusted)
- The pipeline flow in `/build`
- The path variable conventions
- The non-negotiable rules

**Adapt:**
- Project list (your subprojects, not Freudche's)
- Tech stack descriptions (your languages, not TS/Python)
- Test commands (your test runners)
- Port ranges (whatever's free on your machine)
- Character/personality in CLAUDE.md (or remove entirely)
- Domain-specific commands (Freudche has `/officer` for GDPR, `/ckm` for clinical knowledge — yours will be different or none)

See `ADAPTATION.md` for specifics.
