# Opening a VS Code terminal from outside VS Code

Status: research, not built. Question: can `pfm` (an external process) make a running VS Code
window open a NEW integrated terminal and run a command in it — e.g. `pfm chat attach <name>`?
Full trail with sources: `.professor/RR/vscode-open-terminal-run-command-externally-2026-09-11.md`.

## Answer

No zero-setup mechanism exists. VS Code exposes no "open a terminal" surface to outside
processes: the `code` CLI (1.128) has no flag that runs a workbench command, there is no built-in
`vscode://` URI for terminals, and generic `command:` URIs are trust-gated after CVE-2022-41034.
Everything that works is either an extension or UI automation.

## Routes, ranked by what could ship

| # | Route | Exists | Needs | Verdict |
| --- | --- | --- | --- | --- |
| 1 | A small pfm VS Code extension: `window.registerUriHandler` + `window.createTerminal({shellPath: "pfm", shellArgs: ["chat", "attach", name]})`, opened via `open "vscode://professor.pfm/attach?name=X"` | Pattern documented (uri-handler-sample) | One-time `code --install-extension` (a `pfm install --vscode` step) | The only deterministic, cross-platform, window-targeted route. A product, not a trick. |
| 2 | macOS UI scripting: `osascript` activates VS Code, System Events sends ⌃⇧` (new terminal), then types the command + Enter | Yes — verified on this host: the keystroke reached VS Code and opened its multi-root folder picker, so a follow-up "folder name, Enter" is needed in a multi-root workspace | Accessibility permission for the calling process; VS Code frontmost; delays between steps | Works today, macOS only, brittle to focus and timing. A darwin-gated hack at best. |
| 3 | `.vscode/tasks.json` task with `"runOptions": {"runOn": "folderOpen"}`, `"presentation": {"panel": "new"}`, re-triggered by `code -r <folder>` | Yes | Pre-authored task per workspace; `task.allowAutomaticTasks: "on"` or a one-time trust prompt | Undocumented whether re-focusing an already-open folder re-fires `folderOpen`; the trigger has a trail of flake reports (vscode #160013, #160174, #174740, #72448); cannot carry a per-call argument such as a chat name. |
| 4 | `code --command <id>` | No | — | Dead end. |
| 5 | `code chat [prompt]` | Yes (1.128) | — | Opens the chat panel, not a terminal. |
| 6 | `terminal.integrated.profiles.osx` + `workbench.action.terminal.newWithProfile` | Yes | A keybinding or an extension to fire it | Only reachable through route 2 or route 1. |
| 7 | `code serve-web` | Yes | — | A browser-served server, unrelated to the running desktop window. |
| 8 | Claude Code's `ide` MCP bridge | Yes | The Claude Code VS Code extension | Exposes diagnostics and diffs, no terminal verb. |

## Recommendation

Build route 1 if opening the seat in a VS Code tab becomes a pfm feature; route 2 only as a
darwin-gated convenience on a single machine. Never routes 3–8.

## Local findings

- `code --help` (1.128) lists no command-execution flag; `--locate-shell-integration-path` is the
  only terminal-related option.
- `osascript` with `activate` + ⌃⇧` reached VS Code on this host without an Accessibility denial;
  in a multi-root workspace the keystroke lands on the "select a folder for the new terminal"
  picker, so the automation must answer it before typing the command.
