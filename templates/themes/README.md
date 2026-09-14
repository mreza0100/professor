# Themes

`sources.json` declares the Claude Code themes `pfm install` places into `~/.claude/themes/` and records in the install ownership ledger (`pfm uninstall` removes only owned, unmodified files; a locally edited theme is reported and left alone).

- `source_fetched` — fetched at install from the theme's canonical public repo, always latest; the blueprint never vendors a copy.
- `bundled` — palettes the blueprint ships as `{name}.json` beside the manifest: `name`, `base` (`dark`|`light`), and `overrides`, a map of Claude Code UI colour keys to hex values. Read from the source clone when the manifest is; downloaded from beside the release manifest otherwise.

Bundled today: one palette per fleet account medal — `professor-gold` (🥇 account 1), `professor-silver` (🥈 account 2), `professor-bronze` (🥉 account 3). Select per account with `"theme": "custom:professor-gold"` in that account's `settings.json`, or `/theme` in a chat.
