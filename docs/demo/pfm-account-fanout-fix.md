# pfm account-fanout fix — status (uncommitted, on develop)

Root cause: global agents/commands/skills linked into the primary Claude account's registry only (`codexgen/globalagents.go:168`, `installer.go:459`, `:617`), while retire paths already fanned out over every configured account. Found live tonight when `rr` was missing from account 2's `~/.claude2/agents/`.

## What the opus dev agent did (verified, not just trusted)

Files touched (git diff, verified clean of guarded paths):

```
 M pfm/.arch/cmd-budget.txt
 M pfm/cmd/pfm/doctor.go
 M pfm/internal/codexgen/globalagents.go
 M pfm/internal/codexgen/globalagents_test.go
 M pfm/internal/installer/installer.go
 M pfm/internal/installer/reload_test.go
?? pfm/cmd/pfm/doctor_global_agents.go
?? pfm/cmd/pfm/doctor_global_agents_test.go
?? pfm/internal/installer/global_fanout.go
?? pfm/internal/installer/global_fanout_test.go
```

No `.claude/**`, `CLAUDE.md`, or `templates/**` touched — confirmed by `git diff --name-only` grep.

- `GlobalAgentsOptions.ClaudeConfigDirs` added; one `.md` link per config dir; apply-time re-classify handles two accounts aliasing one physical registry (this host's `~/.claude2/agents -> ~/.claude/agents` shape).
- `wireGlobalCommands`, `wireGlobalSkills`/`wireGlobalSkill` (handoff included) loop `claudeConfigDirs()`.
- New `pfm doctor` line: `doctor: global-agents account=N dir=<dir> state=linked|MISSING|CONFLICT|UNREADABLE`.
- Tests written FAILING-first against unfixed code, then fixed (watched both ways, per the dev agent's own report).

## Independently re-verified by me (not taking the report on faith)

- `internal/harvest`'s two reported test failures (`TestRewritePublicImagesCopiesLocalImageIntoPublicNamespace`, `TestHarvesterWritePublicFileRefusesPathOutsidePublicNamespace`) are **pre-existing and unrelated**: confirmed `go list -deps ./internal/harvest/...` carries zero dependency on `internal/installer` or `internal/codexgen`, and re-ran both tests myself — the failure is a literal `/var` vs `/private/var` symlink-resolution mismatch on macOS, visible in the test's own error text. Not caused by this diff.
- `pfm/.arch/cmd-budget.txt` really did move `16104 → 16132` (git diff confirmed) — a real architecture-ceiling raise, not a claim. The new `pfm doctor` surface required it; the dev agent named this in its report as the "genuinely new exception, visible in review" CLAUDE.md allows, rather than hiding it.

## Open decisions (not mine to make)

1. **Commit.** Nothing has been committed — this is live uncommitted work on `develop`. Only `gitter` writes git. The one test failure is verified pre-existing/unrelated, so it doesn't block a commit under "never commit broken code," but that's your call to make, not an automatic green light.
2. **The arch-budget bump (16104→16132).** Real, disclosed, tied to genuinely new code (the doctor printer). Accept as-is, or ask for the 28 lines to be trimmed elsewhere in `cmd/pfm` first.
3. **`pfm codex agents` hand-run still wires the primary account only.** The dev agent implemented this fix too, watched it break an existing jail test (the jail's account roster isn't `$HOME/.claude`), and **reverted it** rather than silently deciding which roster a hand-run rebuild should trust. Left open, correctly.

## Harvester scholarly config — done, live, smoke-tested

`~/.config/pfm/harvester.config.json` now carries the `scholarly` block (doiMirrorURL, doiViewerURL, ipfsCatalogURL, md5CatalogURL, googleScholarURL) per your values. `pfm config validate` clean; `pfm config show` reflects all five from `(file)`. The `com.professor.pfm.mcp` launchd daemon (backs this session's `mcp__harvester__*` tools) was restarted (`launchctl kickstart -k`) so the new config is live for MCP calls too, not just fresh CLI invocations — confirmed via `pfm doctor`: `mcp daemon=running pid=84267`. Re-ran Bloomberg through the daemon post-restart: still the same `jina`-served footer (50,069 chars) — expected, since Bloomberg's homepage never touches the scholarly/DOI path; the scholarly config is for DOI/paper fetches, not general news sites. Not yet live-tested against an actual paywalled DOI to confirm the DOI mirror/IPFS catalog/MD5 catalog chain resolves end-to-end — worth one before claiming it on stage.
