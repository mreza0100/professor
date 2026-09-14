# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- repo: security hygiene — `SECURITY.md` names the private reporting channel and scope; `.github/dependabot.yml` updates workflow actions and Go modules weekly, PRs targeting `develop`; every workflow action is pinned to a full commit SHA with its version as a comment. `scripts/check-token-pricing.mjs` parses the PRICING table as JSON instead of `eval`-ing it, and Go test fixtures that read as live credentials carry an `example-` prefix. The HOL AI Plugin Scanner that gates the awesome-ai-plugins listing moves from 56/F with 11 high findings to 88/B with none, and `.github/workflows/plugin-scan.yml` now runs that scanner (pinned action SHA, min score 80, fail on high) on every push and PR. No adopter action.
