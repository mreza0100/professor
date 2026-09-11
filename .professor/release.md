# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending
- Patch: `pfm install` — a launch agent whose plist did not change is no longer torn down and re-registered. `launchctl bootout` STOPS the running job, and the installer ran it unconditionally on every install, so an ordinary `pfm install` restarted the user's `pfm mcp serve` as a side effect — and when the immediately-following `bootstrap` lost the race against the still-in-flight teardown (EIO / exit 5), it left the daemon DOWN, reporting only "agent file is installed but service is not loaded". A loaded job with an unchanged plist is now left running untouched; when the plist did move, the bootstrap is retried while launchd finishes the teardown (no wait at all when the first attempt succeeds); and a bootstrap that fails after its job was stopped now says the service is DOWN and names the `launchctl bootstrap` that restores it, distinct from the case where nothing was running to begin with.
