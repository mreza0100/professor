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
