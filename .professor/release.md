# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- pfm: the transcript index no longer mistakes a detached turn for a background session — `sessionKind:"bg"` is stamped per record on every turn Claude Code produces while nobody is attached to the pane, which is how the fleet drives its own named chats through `pfm chat inject`, so reading that marker retroactively reclassified deep interactive chats as machine work and dropped them from the picker with no visible signal. Provenance (`sdkSpawned`) is now the only background signal, and `claude_parser_version` bumps to 3 so the next index pass re-derives every stored row.
