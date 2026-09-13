# RR — Is Codex's app-server thread/MCP-process leak tracked upstream, fixed, or worked around?

Question: Is the Codex (openai/codex) bug where the long-lived `codex app-server` daemon keeps every thread it ever loaded — and each thread's full set of stdio MCP server child processes — alive after the thread ends or its TUI detaches, tracked upstream; what is its status; was it fixed in any release after Codex CLI 0.153.4; and what workaround do users/maintainers recommend?

## Answer

Yes, it's tracked — primarily as [openai/codex#30408](https://github.com/openai/codex/issues/30408) ("per-thread processes never cleaned up"), corroborated by [#37971](https://github.com/openai/codex/issues/37971) which documents the identical daemon-mode failure at 8-day uptime and explicitly names #30408 as the root cause — but as of Codex CLI 0.153.4 and the newest checked pre-releases (0.155.0-alpha.3.10), it remains **open, unfixed, with no maintainer comment on any of the 9 issues surveyed, and no maintainer-stated workaround** beyond user-posted manual-kill scripts.

## 1. Which issues match OUR exact mechanism vs. adjacent ones

**PRIMARY MATCH — general per-thread retention, not subagent-only** (matches the host's observation that several leaked sets came from single thread loads):
- [#30408](https://github.com/openai/codex/issues/30408) "MCP server processes leak: per-thread processes never cleaned up (9+ GB RSS)" — OPEN. Quote: *"Codex app-server spawns a full set of global MCP server processes for each new thread/conversation, but never kills them when threads are archived or closed. Over time, orphaned MCP processes accumulate unboundedly."*
- [#37971](https://github.com/openai/codex/issues/37971) "app-server: fd exhaustion..." — OPEN. This is the closest documented match to our exact host scenario: a standalone `codex ... app-server --listen unix://` daemon, uptime-driven, held-pipe-ends. Quote: *"The actual cause was the long-lived `codex ... app-server --listen unix://` daemon sitting at its file-descriptor ceiling... Descriptor accounting closed exactly, at 4 per child (3 stdio pipes + 1 pidfd)... None were ever reaped."* And explicitly: *"The leak itself is already reported in #26984 (fd/pipe leak, orphan children) and #30408 (per-thread MCP processes never cleaned up)."*

**ROOT-CAUSE PLUMBING BUG** (underlies #30408/#37971, not thread-specific by itself):
- [#26984](https://github.com/openai/codex/issues/26984) "MCP stdio servers leak pipe fds + orphan child processes → cumulative EMFILE" — OPEN. Quote: *"Each refresh/rebuild cycle spawns a fresh server set and tears down the old one without awaiting death and without reliably reaping the npm grandchild."*

**ADJACENT — subagent-only, narrower than our mechanism** (our leaked sets were NOT all subagent-derived, so these don't fully explain the host's finding):
- [#25015](https://github.com/openai/codex/issues/25015) "Codex app-server leaks MCP process stacks for subagents..." — OPEN. Quote: *"After the subagents complete and close_agent is called, some MCP child process trees remain alive under the long-lived codex app-server."* Filed deliberately separate from #30408: *"I am opening this separately because this is a Linux app-server reproduction with a reduced MCP set and controlled before/after measurements."*
- [#38247](https://github.com/openai/codex/issues/38247) "[Linux Desktop 0.147.0] Completed v2 subagents retain full stdio MCP runtimes" — OPEN, Desktop. Quote: *"completed v2 subagents retaining live runtimes while their logical agent identity remains resumable."*
- [#37870](https://github.com/openai/codex/issues/37870) "[Linux CLI 0.147.0] Completed subagents retain bundled plugin MCP processes" — OPEN. A non-maintainer comment (jdcodes1, authorAssociation "NONE") explains this is by design: *"each subagent is a full thread with its own connection set, so it spawns its own copies of every bundled plugin MCP server. On completion the thread is deliberately kept loaded — followup_task must be able to resume it."* This is the best explanation found anywhere for WHY threads are kept loaded (resumability), even though it's filed as subagent-scoped.

**OFF-TARGET — different process model entirely (Desktop/GUI, not the standalone CLI daemon)**:
- [#12491](https://github.com/openai/codex/issues/12491) "Codex.app GUI: MCP child processes not reaped after task completion — 1300+ zombies" — OPEN. Quote: *"Codex.app spawns `codex exec --full-auto` worker processes for each worktree but never reaps them."* PR #19753 is referenced in the issue's navigation; its merge status could not be confirmed from the fetched page (not stated in source).
- [#43971](https://github.com/openai/codex/issues/43971) "[macOS] Codex Desktop creates hidden threads and leaks one MCP process pool approximately every 5 minutes while idle" — OPEN. Its own environment block distinguishes "Codex Desktop: 26.901.51231" from "Bundled Codex CLI / app-server: 0.153.4" as a separate component — confirms this is the GUI shell, not our daemon.

**CLOSED, but does not represent a shipped fix**:
- [#19469](https://github.com/openai/codex/issues/19469) "fix: stdio MCP child process leak causing unbounded memory growth" — CLOSED 2026-04-28. It is an ISSUE, not a PR (`pull_request` field is null). It was closed on the strength of a commit (`498aac674cc2bf29f12185b1344e89b006f4a64d`, "fix: terminate MCP child processes on session shutdown", authored 2026-04-19) that is credited with fixing a *different* issue, [#17832](https://github.com/openai/codex/issues/17832) — see §2 for why that fix never actually shipped.

## 2. Status, maintainer input, and the #17832/#19469 fix that didn't ship

No openai-org-member (maintainer) comment was found on any of the 9 issues checked (#30408, #37971, #26984, #25015, #38247, #37870, #12491, #43971, #19469). All except #19469 remain OPEN.

The one seemingly-merged fix traces back further than it first appears:
- [#17832](https://github.com/openai/codex/issues/17832), "Regression: Playwright MCP stdio processes still leak after #16895 fix — 213 orphaned pairs, 13.6 GB RSS," is itself **OPEN** (`closedAt: null`) — not closed, despite #19469 crediting a commit as its fix. Its own description matches the app-server pattern closely: *"When the subagent session closes, the MCP processes are not terminated"* against a `codex app-server` process tree.
- The credited commit `498aac674cc2bf29f12185b1344e89b006f4a64d` is dated 2026-04-19T19:40:32Z. Codex CLI **0.153.4 was released 2026-09-04T23:25:48Z** — 168 days later. A GitHub compare check found the commit's branch has **diverged from `main`, with main 5113 commits ahead**, i.e., the commit does not appear to be on the branch that ships releases. Combined with #17832 still being open, this indicates **the credited fix never actually shipped**, in 0.153.4 or otherwise.
- Note: an earlier check of #19469 read its closing text as saying the commit "was merged to main," which conflicts with the compare-based finding above. This discrepancy between the two findings was not resolved within budget — flagged as an open question below.

## 3. Workarounds actually stated in the threads (quoted, none invented)

- **#30408**: *"Manually kill orphaned processes: `pkill -P <app-server-pid> -f "playwright-mcp|davinci-resolve"`"* and *"Remove rarely-used MCP servers from global config and add them per-project instead."* (User-posted; no maintainer endorsement found.)
- **#26984**: *"Raise the per-process limit before launching (`ulimit -n 65536`, and/or `sudo launchctl limit maxfiles 65536 524288` for GUI-launched servers) and periodically kill orphaned `*-mcp-server` / `npx` processes. This only delays the leak."* — explicitly framed as a mitigation, not a fix.
- **#37971**: no persistent workaround stated. Only note found: *"SIGTERM to the pid works and the daemon respawns on next use"* — a manual restart, explicitly not offered as a fix.
- **#25015**: a third-party diagnostic/cleanup tool is linked (`https://github.com/LostFrxks/codex-mcp-clean`) — a community tool, not a maintainer-recommended workaround.
- **#43971** (Desktop-only, adjacent — not the CLI daemon): *"Fully quit and reopen Codex Desktop to reclaim the leaked process pools"* and *"Disabling heavy MCP servers or plugins reduces the growth rate, but it does not fix the lifecycle issue."*

**No source anywhere stated**: a config key, an app-server idle-eviction setting, disabling `code_mode_host`, or switching MCP transport from stdio to HTTP as a workaround. None of these should be treated as confirmed remedies. (A move to HTTP-based MCP transport would, by the plain mechanism described in #26984/#30408/#37971 — leaked stdio pipes and orphaned stdio child processes — structurally sidestep this specific class of leak, but this is this report's own inference from the bug mechanism, not a workaround stated by any source, and it is untested.)

## 4. Whether upgrading past 0.153.4 is expected to fix it

**No evidence that it does.** Releases checked after 0.153.4 were all pre-release builds: 0.154.0-alpha.6.2, 0.155.0-alpha.2 / -alpha.2.3 / -alpha.3 / -alpha.3.7 / -alpha.3.8 / -alpha.3.9 / -alpha.3.10 (all dated 2026-09-10/11). None of their changelog text mentions MCP process cleanup, stdio MCP leak fixes, thread unloading/eviction, or app-server idle eviction. Combined with §2's finding that the only candidate fix commit is not on `main` and its source issue (#17832) is still open, the answer as of the newest checked pre-release (0.155.0-alpha.3.10) is: **no fix has shipped past 0.153.4; no maintainer has stated one is coming.**

## Open questions (unresolved after 2 rounds)

- **Direct contradiction, unresolved**: round-1 research read #19469's closure as crediting a commit "merged to main" for #17832; round-2 research found via GitHub compare that the same commit's branch has diverged from `main` (5113 commits behind) and that #17832 itself is still open. Both cannot be fully true simultaneously — next step: `gh pr list --search 498aac6` or `git branch --contains 498aac674cc2bf29f12185b1344e89b006f4a64d` directly against openai/codex to settle it.
- Whether PR [#19753](https://github.com/openai/codex/pull/19753) (referenced under #12491) was merged, and whether its fix has any bearing on the CLI daemon case — not checked, Desktop-scoped so lower priority.
- No issue found anywhere mentions the `features.code_mode_host=true` flag the host's own daemon was launched with — whether it interacts with thread retention is entirely unverified; genuinely unknown, not merely unstated.
- Whether HTTP/SSE-transport MCP servers are immune to this leak class (as opposed to stdio) — plausible by mechanism per #26984, but not tested or stated by any source.

## Coverage note

5 diggers dispatched across 2 rounds (4 in round 1, 1 in round 2); 5/5 returned, none failed. Fetches covered: #30408, #19469, #25015, #38247, #37870, #26984, #37971, #43971, #12491, #17832, the openai/codex releases listing, and the 0.153.4 release/tag date — 12 sources, matching the ~12-fetch budget.
