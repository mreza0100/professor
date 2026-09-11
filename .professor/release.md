# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending
- B: pfm — a fleet chat's terminal tab (tmux `set-titles-string`, the label VS Code shows through `${sequence}`) now carries ONE name, the chat's own: a Claude pane renders Claude Code's title with its status glyph stripped, so a `/rename` or `pfm chat name` reaches the tab at once, unclipped, and the label holds still through the spinner; every other engine renders its window name, which name-sync converges. The former `⬢ #{window_name} · #{pane_title}` showed a Claude chat's name twice — the first copy clipped at 24 runes and trailing a rename by a statusline redraw. `pfm chat name` also clips the window name through `gather.WindowNameFor`, like the statusline and name-sync writers, so a name over 24 runes no longer flips the window between its full and clipped spellings. Live servers take the new format on the next name-sync tick; `pfm name-sync` applies it immediately.
