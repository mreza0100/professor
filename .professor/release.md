# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Minor: `templates/themes/` + `pfm install` — three bundled Claude Code overlays, one per fleet account medal (`professor-gold` 🥇, `professor-silver` 🥈, `professor-bronze` 🥉): Tokyo Night with a per-account input bar (`promptBorder` + `promptBorderShimmer`), merged onto the fetched base at install so the base is never vendored; `sources.json` gains a `bundled` section (optional `base` naming a `source_fetched` theme) the installer reads from the source clone or downloads from beside the release manifest, under the same ownership ledger; a missing or non-JSON bundled file, an unreachable base, or a `base` that is not `source_fetched` is a named skip or refusal, never an absence.
- Fixed: `templates/themes/README.md` claimed pfm embeds palettes at build time (nothing did) and a vendored `tokyo-night.json` contradicted the manifest's never-vendor rule — the copy is gone, the README describes the two manifest kinds as the installer implements them.
#### → For: run `pfm install --yes` to place the palettes; set `"theme": "custom:professor-{gold|silver|bronze}"` per account `settings.json` (or `/theme`) to pick one.
- Fixed: `templates/project/codex/README.md` + `codex/skills/wave-builder/SKILL.md` — the two rows and the launch line that forbid Codex's full-access sandbox and never-ask approval as repo defaults no longer spell the literal config values; plugin scanners regex-match those tokens in any markdown and dock the adopter for prose that bans them.
- Minor: `templates/global/agents/` — `rr` moves to the light tier (Opus, low effort) with Sonnet diggers, and a new `rr-super` original (Opus, medium effort) answers only when the user says "super rr"; both link into `~/.claude/agents/` on `pfm install`.
- Removed: pfm — the statusline's `🚀 ultracode` effort segment, which read a `~/.claude/ultracode/<session>` marker nothing ever wrote.
- Fixed: pfm — `pfm install` arms the source clone's pre-push gate by resolving `core.hooksPath` against the repo, so an absolute `.githooks` path is recognised as armed instead of rewritten; `pfm doctor` uses the same comparison.
- Fixed: pfm — `pfm install` run outside the source checkout reads the theme manifest from the recorded source clone before trying the release URL, an unpublished `-alpha` release manifest is a named refusal instead of a bare 404, and the preview labels bundled palettes as read, not fetched.
