# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Minor: PFM headless — shared Claude/Codex execution interface with system prompts, schemas, configurable timeouts, normalized results, and native streaming; prepared-source asks, harness capture, credential refresh, and Walker equivalence route through one internal process runner, with explicit errors for unsupported engine capabilities.
