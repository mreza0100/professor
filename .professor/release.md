# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Patch: `scripts/dev.sh` — frontend startup unsets inherited `FORCE_COLOR` before setting `NO_COLOR`, and a single severity-aware log scanner replaces broad `ERR` substring matching in both status summaries. Structured `INFO` payload fields no longer appear as failures; numeric/string error and fatal levels plus standalone failure signatures remain visible. The scanner reports unreadable logs as scan failures instead of “no errors.”
- Minor: `pfm` launch env — every Claude process pfm starts (new, resume, reload, the `claude` launcher shim, agent-open, headless chats, `pfm headless exec`, `pfm ask`) now carries `CLAUDE_CODE_MAX_WEB_SEARCHES_PER_SESSION=9007199254740991`, lifting Claude Code's 200-WebSearch-per-session cap that silently stalled long research chats. Declared once as the Claude engine descriptor's `LaunchEnv` and carried by both spawn doors (`action.ClaudeSpawn`, the generic headless runner). (cost: no WebSearch ceiling per session — each search still bills normally)
- Minor: `/rnd` — moves from a machine-global command (with its `/rnd:hammer` and `/rnd:referee` sub-commands) to a project-scope lifecycle command that opens, continues, verifies and lands research runs under `.professor/RND/<call>/<N>-<slug>/`, spawning the new root agent `rndier` to execute one run. SETUP now always writes `rndier` beside `gitter`, `mono-documenter` and `tracer`, and substitutes the AI-service placeholders into both files.
#### → For: adopters — `pfm install` drops the global `/rnd`, `/rnd:hammer` and `/rnd:referee`; run `pfm update check`, hand-apply the NEW `commands/rnd.md` and `agents/rndier.md`, then `pfm update pin`.
- Patch: `pfm` usage hook — the critical banner names the window that actually crossed the threshold and says only what is true of it: the 5-hour window keeps "finish the in-flight step, then /reload", the 7-day account cap says to /reload, and a model-scoped 7-day cap (opus / fable) says to keep working and route that tier's spawns elsewhere — instead of printing a session-wide stop order on every prompt while the 5-hour window sits nearly empty.
