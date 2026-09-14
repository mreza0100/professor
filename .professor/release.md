# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- B: pfm — a terminal opened by an app that was itself launched from inside a chat opens the pfm picker again. Such an app (a launcher or window manager a chat relaunched) hands every process it starts the chat's CLAUDECODE / session markers, so each new VS Code terminal looked like a shell inside a chat and auto-open silently stepped aside. The Professor extension's terminal and the fallback PFM profile now drop those markers, and a shell that still inherits them says so on stderr instead of skipping in silence.
#### → For: an owned VS Code PFM profile upgrades automatically on the next `pfm install`; a hand-edited PFM profile does not — add the null env keys (`CLAUDECODE`, `CLAUDE_CODE_SESSION_ID`, `CLAUDE_CODE_CHILD_SESSION`, `TMUX`, `TMUX_PANE`) yourself.
