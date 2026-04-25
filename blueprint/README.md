# Claude Pipeline Blueprint

A portable, opinionated multi-agent development pipeline for Claude Code — extracted from the Freudche project and generalized for any codebase (single-project or monorepo).

## What this gives you

A complete `.claude/` infrastructure that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team** with:

- **Worktree isolation** — every feature gets its own git worktree branch + port allocation. No more "did I commit on the wrong branch?" Multiple parallel pipelines on the same repo.
- **A pipeline that refuses cowboy coding** — `planner → architect → developer → QA → merge`. QA gates block bad code from reaching `main`. Only one agent (`gitter`) touches git.
- **Self-improvement** — a meta-agent (`/ccm`) that edits its own pipeline rules at the source instead of accumulating lesson files nobody reads.
- **Hotfix mode** — `/jc` lets you bypass the full pipeline for surgical bug fixes, but still routes through QA + gitter.
- **Path conventions** that scale — `$DOCS`, `$WORKTREE`, `$CDOCS` variables so agents never hardcode paths.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent writes to permanent project docs.

## When to use it

✅ **Good fit:**
- Multi-project monorepo (BE + FE + AI + infra) where features cross boundaries
- Single project with complex pipelines (planning → impl → QA → merge worth modeling)
- Team or solo dev who keeps losing work to half-finished branches and forgotten state
- Project where "what was decided and why" matters as much as the code

⚠️ **Overkill for:**
- A 200-line script
- Throwaway prototypes
- Anything where you genuinely don't care if `main` breaks

## What's in the box

```
tmp/claude-blueprint/
├── README.md              ← you are here
├── BLUEPRINT.md           ← philosophy, structure, design decisions
├── SETUP.md               ← step-by-step install for your project
├── ADAPTATION.md          ← how to customize for your stack & domain
└── templates/
    ├── CLAUDE.md          ← root project rules (the contract)
    ├── agents/            ← agent definitions (planner, architect, dev, qa, gitter, documenter)
    ├── commands/          ← /build, /jc, /ccm, /dev
    └── scripts/           ← worktree.sh, alloc-ports.sh
```

## Quick start

1. Read `BLUEPRINT.md` to understand what you're agreeing to.
2. Follow `SETUP.md` to copy the scaffolding into your repo.
3. Read `ADAPTATION.md` to customize for your stack.
4. Run `/build my-first-feature` and watch the system work.

## Origin

Built and battle-tested on **Freudche** (an AI clinical-documentation assistant: TypeScript backend + React Native frontend + Python AI engine + infra). The pipeline survived hundreds of features, dozens of hotfixes, and at least one near-disaster involving silent exception swallowing. The lessons are baked in.

You don't have to keep the personality (Freudche's root CLAUDE.md mandates a sarcastic Dr. House character — that's optional). The mechanics are what matter.
