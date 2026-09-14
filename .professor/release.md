# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Minor: `templates/themes/` + `pfm install` — three bundled Claude Code palettes, one per fleet account medal (`professor-gold` 🥇, `professor-silver` 🥈, `professor-bronze` 🥉), each a full 47-key palette with the echoed-prompt background as the metal over the ground; `sources.json` gains a `bundled` section the installer reads from the source clone or downloads from beside the release manifest, under the same ownership ledger as source-fetched themes; a missing or non-JSON bundled file is a named `read failed` skip, never an absence.
- Fixed: `templates/themes/README.md` claimed pfm embeds palettes at build time (nothing did) and a vendored `tokyo-night.json` contradicted the manifest's never-vendor rule — the copy is gone, the README describes the two manifest kinds as the installer implements them.
#### → For: run `pfm install --yes` to place the palettes; set `"theme": "custom:professor-{gold|silver|bronze}"` per account `settings.json` (or `/theme`) to pick one.
- Fixed: pfm — `pfm install` no longer aborts when the pre-split `config.json.pre-split` it wants to park already exists with identical bytes; a differing backup is still refused, and the refusal now says the two files differ.
- Minor: pfm — `pfm doctor` exits 3 when a state the installer owns is missing or broken (required dependency, launcher, hooks, host overlays, global agents, config or database health) and 1 when only advisory rows warn; `pfm update` now gates on failures alone, runs a baseline doctor before the install, and after it prints `doctor after update: warnings=N (before update: M)` plus every warning row the update introduced. A host with standing warnings the update cannot influence is no longer un-updatable; a rollback's doctor warnings are reported, never claimed as residue.
#### → For: hosts still on v0.77.x — the installed `pfm update` still rolls back on any doctor warning; if yours does, update by hand once: `git -C "$HOME/.professor" fetch --tags && git -C "$HOME/.professor" merge --ff-only vX.Y.Z && make -C "$HOME/.professor/pfm" host-install && pfm install --yes`.
- Fixed: pfm — `pfm update` hands the post-install doctor the config file the update just migrated (`config.json` → `pfm.config.json`) instead of the old name, so the gate judges the host's real accounts, MCP servers and spawn policy rather than defaults; a rollback restores the config files the migration renamed before it reruns the previous installer, so the restored release reads its own config and no longer disables the MCP launch agent it finds "unconfigured".
- Fixed: pfm — `pfm install --yes --config <path>` refuses to run when that path does not exist (converging on defaults would boot out and delete every MCP service the missing file enabled); the preview names the skip. A launch-agent or unit removal now says which config it read the disabled state from.
#### → For: anyone whose rollback lost `com.professor.pfm.mcp.plist` — `pfm install --yes` on the current release re-creates and bootstraps it; the timestamped `.pre-professor-<ts>` copies beside it can be deleted afterwards.
