# Getting Professor into hashgraph-online/awesome-ai-plugins

Research date: 2026-09-14. Read-only (gh CLI + raw GitHub content). No PR/issue/fork/star/comment was opened.

## 1. Mechanics

- **File to edit:** `README.md` only. `plugins.json` (237KB) is a generated catalog artifact — PR #286 ("Add CommitLore to Development & Workflow") touched only `README.md`; nothing in `CONTRIBUTING.md` asks contributors to hand-edit `plugins.json`.
- **Entry format** (CONTRIBUTING.md, "Follow the format"):

  ```
  - [Extension Name](https://github.com/owner/repo) - Description (max 1 sentence).
  ```

- **Not required to be a packaged plugin.** CONTRIBUTING.md's ecosystem-specific rules (native manifest required, documented install flow) are called out explicitly only for **Grok** (`.grok-plugin/plugin.json` + `grok plugin install`), **Kimi** (`kimi.plugin.json`) and **DeepSeek Harness** (`dsh.bundle` + `apply(ctx)`). No such manifest requirement is stated for Claude Code / Codex / cross-AI entries in "Development & Workflow" or "Tools & Integrations" — those sections already list plain repos/frameworks (agent orchestration layers, audit tools, context-management layers), not exclusively `.claude-plugin/plugin.json` bundles. Quote: "Add to appropriate section - Codex plugins, Claude Code skills, Gemini extensions, Grok plugins, Kimi plugins, DeepSeek Harness plugins, MCP servers, or Cross-AI tools."
- **Scanner CI / trust score** (CONTRIBUTING.md + SCANNER_GUIDE.md):
  - Quote: "Scanner CI is optional for listing. HOL still scans listed projects independently... Projects that maintain scanner CI receive the full trust score; projects without it remain eligible and receive a 10% trust-score reduction."
  - The bot runs a centralized HOL scan (`plugin-scanner`, via `hashgraph-online/ai-plugin-scanner-action`) against the source repo regardless of whether the repo has its own workflow. **Passing criteria: normalized score ≥ 80/100, with no critical or high severity findings** (SCANNER_GUIDE.md).
  - If the repo *does* self-host the SHA-pinned scanner workflow (`contents: read` only, `min_score: 80`, `fail_on_severity: high`), it keeps full trust score instead of the 10% reduction.
  - A low centralized-scan score is used by reviewers as a real merge blocker in practice (see § 4, PR #276 — 49/100 → left unapproved/closed), even though CONTRIBUTING.md frames the source-repo's own CI as advisory ("HOL clones and scans each valid new source repository; those scan results are advisory and do not block a valid listing" — that line refers to the repo's own CI results, not the centralized gate score itself).
- **CI on PRs:** `.github/workflows/validate-contribution.yml` — "Catalog format, section, and discovery checks are required." A second workflow, `.github/workflows/sweep-open-prs.yml`, periodically re-scans open PRs (reads README diff only, no fork code execution) and posts/updates a single bot comment with the scan verdict.
- **PR template:** none exists (`.github/` contains only `FUNDING.yml` and `workflows/`). CONTRIBUTING.md just asks for: "Clear description of the extension / Link to the repository / Category section where it should be added / Brief verification that it works."
- **Ordering:** strictly alphabetical within each section (CONTRIBUTING.md: "Use alphabetical order within sections").
- **Activity bar:** "Community plugins should have some activity (recent commits or releases)."

## 2. Does Professor qualify as-is (not a packaged plugin)?

**Yes, on format grounds.** The "Development & Workflow" subsection of "Community Plugins" already lists non-packaged, non-plugin-manifest frameworks that ship agents/commands/skills/CLIs across multiple assistants — i.e. exactly Professor's shape:

- `[A Team](https://github.com/RBraga01/a-team)` — "Universal multi-agent infrastructure with 25 specialist agents, 16 enforced workflow skills, and a lead orchestrator for Claude Code, Codex CLI, Cursor, and OpenCode."
- `[Agent Context OS](https://github.com/conorbronsdon/agent-context-os)` — "Portable Git-backed context and session workflow layer with first-class Claude Code, Codex, and OpenClaw support..."
- `[Aegis](https://github.com/GanyuanRan/Aegis)` — "An agentic skills framework & software development methodology..."

None of these are demonstrably `.claude-plugin/plugin.json` marketplace bundles; they are cross-tool frameworks/methodologies described by what they ship, same as Professor's pitch. This is direct precedent that non-packaged frameworks are accepted.

**One counter-precedent worth flagging (§4, PR #276, closed/not merged):** "Coddy" was rejected not for being unpackaged, but for (a) centralized scan score 49/100 (well under the 80 threshold, with 50 high-severity findings) and (b) the submitter's own pushback that the scanner's plugin-detector had misfired on an OpenCode rules-wrapper file. The rejection reason was security-scan score, not "not a plugin."

**Net:** Professor qualifies structurally as-is for "Development & Workflow." The one real gate to clear before submitting is the **HOL centralized scanner score** — Professor's public repo would need to score ≥80/100 with no critical/high findings when scanned (repo `github.com/mreza0100/professor` has not been scanned by this research since that would require opening a PR; this is a gap, not a finding).

## 3. Section and exact insertion point

- **Section:** `## Community Plugins` → `### Development & Workflow` (README.md, alphabetical list starting at line 107).
- **Insertion point** (alphabetical: Praxis < Professor < Project Autopilot):

  Immediately before:

  ```
  - [Praxis](https://github.com/ouonet/praxis) - Intent-driven workflow skills for coding agents: describe what done looks like, not the steps. Triage-first design keeps token costs low across design, TDD, debug, review, and release.
  ```

  New Professor entry here. Immediately after:

  ```
  - [Project Autopilot](https://github.com/AlexMi64/codex-project-autopilot) - Turn an idea into a structured project workflow with planning, execution, verification, and handoff.
  ```

  (README.md, `hashgraph-online/awesome-ai-plugins`, `main` branch, lines 280-281 as of this research.)

## 4. Recent PRs — merged vs closed, patterns

| PR | Title | Result | Time to merge/close | Reason / pattern |
| --- | --- | --- | --- | --- |
| [#287](https://github.com/hashgraph-online/awesome-ai-plugins/pull/287) | Update TaskDock: current deliverables and reversible organization | Merged | ~8h | Entry-update PR, format-only |
| [#286](https://github.com/hashgraph-online/awesome-ai-plugins/pull/286) | Add CommitLore to Development & Workflow | Merged | ~3h | Scan reported findings but "advisory, does not block" |
| [#282](https://github.com/hashgraph-online/awesome-ai-plugins/pull/282) | Add opencode-skills-collection to Development & Workflow | Merged | ~1h | "All 1 source-repository scanner job(s) passed" |
| [#280](https://github.com/hashgraph-online/awesome-ai-plugins/pull/280) | Add i-hate-editing to Community Plugins | Merged | ~8h | routine |
| [#278](https://github.com/hashgraph-online/awesome-ai-plugins/pull/278) | Add Cordon to Tools & Integrations | Merged | ~4h | Reviewer: "80/100... meets required threshold... approving and squash merging" — scan score right at the line still merges |
| [#272](https://github.com/hashgraph-online/awesome-ai-plugins/pull/272) | Add LetsFG to Tools & Integrations | Merged | ~16h | routine |
| [#271](https://github.com/hashgraph-online/awesome-ai-plugins/pull/271) | Add bury-bench to Community Plugins | Merged | ~7h | routine |
| [#268](https://github.com/hashgraph-online/awesome-ai-plugins/pull/268) | Add Stvena to Development & Workflow | Merged | ~21h | "84/100... passed the catalog threshold but reported findings" — still merged |
| [#264](https://github.com/hashgraph-online/awesome-ai-plugins/pull/264) | Add TaskDock to Development & Workflow | Merged | <1h | routine, fast merge |
| [#279](https://github.com/hashgraph-online/awesome-ai-plugins/pull/279) | add @geml/geml to Development & Workflow | **Closed, not merged** | — | Duplicate: "branch is currently identical to catalog main... requested geml entry is already present" |
| [#276](https://github.com/hashgraph-online/awesome-ai-plugins/pull/276) | Add Coddy to Community Plugins | **Closed, not merged** | ~ same day | "latest centralized scan is 49/100 (0 critical, 50 high...), below the required 80... leaving this PR unapproved." Submitter disputed the plugin-classifier trigger but score never cleared 80. |
| [#270](https://github.com/hashgraph-online/awesome-ai-plugins/pull/270) | Add bury-bench to Community Plugins | Closed (superseded by #271) | — | duplicate/refiled |

**Patterns:**

- The bot posts one standing "contribution-gate" comment (pass/fail + scan summary) per PR, updated on rerun (not re-posted).
- A human reviewer (maintainer) makes the merge call referencing the numeric scan score explicitly, with 80/100 as the hard cutoff quoted verbatim in review comments twice (#278, #276).
- Missing self-hosted scanner CI never blocked a merge in this sample — only the score itself did.
- Merge times ranged under 1 hour to ~21 hours; no PR sat multiple days when the score cleared 80.
- Duplicate/already-present submissions are closed same-day with a one-line explanation, no scan discussion.

## 5. What to submit

**Verbatim entry** (fits the `- [Name](url) - one sentence.` format, under ~1 sentence per CONTRIBUTING.md; using the given pitch, trimmed to one sentence and repo URL as given):

```
- [Professor](https://github.com/mreza0100/professor) - LLM-harness fleet framework that turns Claude Code, Codex, and OpenCode into a disciplined engineering team you can see, message, and hold to the rules, shipping compiled Codex/OpenCode mirrors, a Go fleet CLI/TUI (pfm), and MCP servers.
```

**Branch name** (repo convention not documented in CONTRIBUTING.md; inferred from no stated scheme — no branch-name examples appear in the sampled PRs' titles/refs available via `gh pr view`): use a descriptive slug, e.g. `add-professor-dev-workflow`.

**Commit message / PR title** — match the observed pattern exactly, e.g. PR #286 "Add CommitLore to Development & Workflow", PR #264 "Add TaskDock to Development & Workflow":

```
Add Professor to Development & Workflow
```

**PR body** (no template file exists; CONTRIBUTING.md's four asks are the de facto checklist):

```
- Extension: Professor — LLM-harness fleet boost framework for Claude Code, Codex & OpenCode.
- Repo: https://github.com/mreza0100/professor
- Section: Community Plugins → Development & Workflow (alphabetical, between "Praxis" and "Project Autopilot")
- Verification: MIT-licensed, active daily commits, ships Claude Code agents/commands/skills/hooks plus compiled Codex and OpenCode mirrors and a Go fleet CLI/TUI (pfm) and MCP servers; installed and exercised locally.
```

**Before submitting, close this gap:** run (or have HOL's PR-time bot run) the centralized `plugin-scanner` against `mreza0100/professor` and confirm it clears **≥80/100 with no critical/high findings** — this research did not execute the scanner (out of scope: read-only, no PR opened) and cannot state Professor's actual score. If it self-hosts the SHA-pinned `hol-guard`/`ai-plugin-scanner-action` workflow per SCANNER_GUIDE.md ahead of submission, it avoids the 10% trust-score reduction and gives reviewers a reproducible source-repo result, matching the pattern reviewers rewarded in PR #278 and #282.

## Gaps

- Did not execute `plugin-scanner` / `hol-guard` against `mreza0100/professor` (would require running third-party scanning tooling against the candidate repo, orthogonal to "read GitHub read-only" and not requested) — actual trust score/pass-fail for Professor is unknown.
- No PR-template file exists in `.github/` to quote verbatim; the PR body above is synthesized from CONTRIBUTING.md's stated checklist, not copied from a template.
- Branch-naming convention is not documented anywhere in CONTRIBUTING.md/SCANNER_GUIDE.md and PR head-branch names were not retrievable via the `gh pr view --json files` calls used (only base README diffs were inspected); the suggested branch name is a reasonable inference, not an observed fact.

## Sources

- <https://github.com/hashgraph-online/awesome-ai-plugins> (README.md, CONTRIBUTING.md, SCANNER_GUIDE.md — `main` branch, fetched 2026-09-14)
- <https://github.com/hashgraph-online/awesome-ai-plugins/pull/286>, /282, /280, /278, /272, /271, /268, /264, /279, /276, /270, /287
