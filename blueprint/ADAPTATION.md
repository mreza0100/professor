# ADAPTATION — Customizing the blueprint for your stack

The templates are stack-agnostic. This guide shows how to fit them to common setups.

---

## Single-project repo

If your repo has only one project (no monorepo):

1. Skip child CLAUDE.md files entirely.
2. Skip child agent directories. All agents live at `.claude/agents/`.
3. In `.claude/agents/`, you have:
   - `gitter.md`
   - `mono-documenter.md`
   - `planner.md` (no project prefix)
   - `architect.md`
   - `developer.md`
   - `qa.md`
4. Drop `mono-planner.md` and `mono-architect.md` — there's no cross-project consolidation needed. The orchestrator goes straight from `planner` → `architect` → `developer` → `qa`.
5. In `/build`, remove the parallel-fan-out steps. Keep the linear pipeline.

---

## Monorepo with N projects

If your repo has 2–6 projects:

1. Keep `mono-planner` and `mono-architect`.
2. For each project, create `{project}/CLAUDE.md` and `{project}/.claude/agents/` with the standard set.
3. In `/build`, replicate the per-project parallel blocks for each project.
4. Keep the `$PROJECTS` routing — let `mono-planner` decide which projects are affected per pipeline.

---

## Stack-specific adjustments

### Node.js / pnpm (backend)

- **Test:** `pnpm test`
- **Lint:** `pnpm lint`
- **Typecheck:** `pnpm typecheck` or `pnpm tsc --noEmit`
- **Build:** `pnpm build`
- **Dev:** `pnpm dev`
- **Worktree install:** `pnpm install --frozen-lockfile`
- **CLAUDE.md rule:** "TypeScript strict mode, ESM-only — no `any` without justification comment"

### Node.js / npm (frontend)

- **Test:** `npm test` (or `npm run test:unit` + `npm run test:e2e`)
- **Lint:** `npm run lint`
- **Typecheck:** `npm run typecheck`
- **Build:** `npm run build`
- **Dev:** `npm run dev` or `npm start`
- **Worktree install:** Symlink `node_modules` from main checkout (saves minutes)

### Python / uv

- **Test:** `uv run pytest`
- **Lint:** `uv run ruff check`
- **Typecheck:** `uv run mypy .` or `uv run pyright`
- **Build:** `uv build`
- **Dev:** `uv run python -m {package}.main`
- **Worktree install:** `uv sync`
- **CLAUDE.md rule:** "Python 3.12+ with strict type hints — no `Any` without justification comment"

### Python / poetry

- Same as uv but `poetry install` and `poetry run ...`

### Rust / cargo

- **Test:** `cargo test`
- **Lint:** `cargo clippy -- -D warnings`
- **Typecheck:** `cargo check`
- **Build:** `cargo build --release`
- **Dev:** `cargo run`
- **Worktree install:** Cargo handles incremental builds — no install step needed

### Go

- **Test:** `go test ./...`
- **Lint:** `golangci-lint run`
- **Typecheck:** `go vet ./...`
- **Build:** `go build ./...`
- **Dev:** `go run ./cmd/{name}`

### Next.js / Vercel marketing site

- Treat as a normal Node.js project
- `npm run dev` for local
- Note: deployment is automatic via Vercel — gitter doesn't deploy, just merges to main and Vercel picks it up

### Mobile (Expo / React Native)

- **Test:** `npm test` (Jest)
- **E2E:** Detox or Maestro — flag in QA agent
- **Dev:** `npm start` opens Metro bundler
- **Web variant:** Some Expo projects have a web target (`expo start --web`) — set `WEB_PORT` in `.env.ports`

---

## Domain-specific commands (optional)

The Freudche project has many domain-specific commands. You probably don't want all of them, but here are the patterns in case you do want similar ones:

| Pattern | Freudche example | When you'd want it |
|---------|------------------|-------------------|
| Compliance / privacy advisor | `/officer` (GDPR) | Regulated data domain |
| Domain knowledge curator | `/ckm` (clinical knowledge) | Specialized knowledge base in repo |
| Multi-agent debate | `/council` | High-stakes architectural decisions |
| Product / UX advisor | `/tpm` | Product-led decisions need PM perspective |
| Business / startup advisor | `/mentor` | Founder using the repo for company-building |
| SEO / marketing | `/marketer` | Marketing site lives in the repo |
| System analysis | `/professor` | Periodic deep-dive audits |
| Code health audit | `/ca` | Hygiene + security scanning |
| Wave runner | `/wave` | Many `/build`s from a backlog file |

The Freudche `.claude/commands/` directory has each of these — copy and adapt as patterns.

---

## Memory & character

The blueprint does NOT include the auto-memory directory or the character system. Those are user-specific.

If you want them:
- **Auto-memory:** mostly handled by Claude Code itself — see `~/.claude/projects/{slug}/memory/`. The repo only needs to know it exists; agents can read it.
- **Character:** define in your root `CLAUDE.md` if you want personality. Be specific about tone, when to drop the persona, and what NOT to do.

---

## What NOT to change

- **The "only gitter touches git" rule.** Loosening this is how you end up with three agents racing to commit and a corrupted index.
- **The QA-before-merge gate.** Skipping QA is how broken code reaches main.
- **Path variables.** Hardcoding `docs/dev/tasks/...` in agents means renaming the convention requires touching every agent.
- **Worktree isolation.** Running pipelines on `main` or shared branches is how you lose work.
- **Self-improvement at the source.** Don't replace `/ccm` with a "lessons learned" file — the file rots, the agents don't read it, the bugs come back.

These five are the load-bearing walls. Touch anything else.

---

## When something feels wrong

After running a few real pipelines, you'll notice rough edges:

- An agent always asks for the same clarification → add it to the agent definition.
- A step always gets skipped → either remove it or make it conditional in `/build`.
- A bug class keeps coming back → add a non-negotiable rule to the relevant CLAUDE.md.
- Pipeline name collisions are common → adjust the naming convention or add automatic versioning.

For each of these: invoke `/ccm`. Describe what you noticed. Let the meta-agent edit the source.

The pipeline is supposed to evolve. Static configurations rot — evolving ones get sharper with use.
