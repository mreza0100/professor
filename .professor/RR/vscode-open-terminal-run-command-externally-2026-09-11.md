# RR — Mechanical ways for an external macOS CLI to make running VS Code 1.128 open a new terminal tab and run a command

Question: from an external process on macOS (a Go CLI called from a shell, NOT a VS Code extension already installed), what are the mechanical ways to make a running VS Code 1.128 open a NEW integrated terminal tab in a given window/workspace and run a specific command in it? Evaluate and rank with sources: (1) any `code` CLI flag that executes a workbench command (does `code --command <id>` exist in any current or Insiders build? what about `code chat`?); (2) `vscode://` URI handlers — is there a BUILT-IN URI that opens a terminal, or does it require an extension registering `window.registerUriHandler` and calling `window.createTerminal({shellPath, shellArgs})`; (3) `.vscode/tasks.json` with `"runOptions": {"runOn": "folderOpen"}` triggered by `code -r <folder>` — does re-opening an already-open folder re-trigger it, and can a task open a NEW terminal panel running an arbitrary command; (4) macOS `osascript`/System Events keystroke (ctrl+shift+`) — permission requirements (Accessibility for the calling app) and reliability; (5) VS Code's `terminal.integrated.profiles.osx` + `workbench.action.terminal.newWithProfile` via keybinding; (6) the VS Code Remote/Server "code serve-web" or the Chat/Agent `code chat` CLI; (7) anything else documented (e.g. the `vscode.terminal` API via a Claude Code IDE extension bridge, or the `remote-cli` `code` shim inside an integrated terminal that has `--command`?). For each: does it exist today, what it needs installed/permitted, one-line reliability verdict. Return ranked options, then the single recommendation for a CLI that must open a terminal tab and run `pfm chat attach <name>` in it. Under 40 lines plus sources. Tier: sonnet.

**Answer:** No zero-setup mechanism exists; the two viable no-extension paths are `.vscode/tasks.json` (`runOn: folderOpen`, new-panel task) reopened via `code -r`, or macOS UI-scripting via `osascript`/System Events — recommendation is the tasks.json route with osascript keystroke-simulation as fallback.

**Ranked options:**

1. **tasks.json `runOn: folderOpen` + `code -r`** — Exists; task itself can force `"presentation": {"panel": "new"}` to open a fresh terminal running any command ([VS Code tasks docs](https://code.visualstudio.com/docs/debugtest/tasks)). Needs the target workspace's `.vscode/tasks.json` (and once-per-workspace "Allow Automatic Tasks" trust) pre-authored. **Gap, not fabricated:** whether re-focusing an *already-open* folder via `code -r` re-fires `folderOpen` is undocumented — no maintainer confirms it either way, and the folderOpen trigger itself is reported flaky across many issues ([#160013](https://github.com/microsoft/vscode/issues/160013), [#160174](https://github.com/microsoft/vscode/issues/160174), [#72448](https://github.com/Microsoft/vscode/issues/72448), [#174740](https://github.com/microsoft/vscode/issues/174740)). Verdict: scriptable and no permission dialogs, but reliability on reopen is unverified/inconsistent — test empirically per workspace.

2. **`osascript`/System Events keystroke (⌃⇧\`)** — Exists, no VS Code config needed. Requires the calling process (or its parent, e.g. Terminal.app) to hold Accessibility permission for `System Events`/`osascript`; sends keys to the *frontmost* app only, so the target window must be `activate`d first, with delays for UI to catch up ([Wikibooks AppleScript/System Events](https://en.wikibooks.org/wiki/AppleScript_Programming/System_Events), [Apple Developer Forums](https://developer.apple.com/forums/thread/70284)). Verdict: universally works once permission is granted, but brittle to focus/timing and a one-time permission prompt.

3. **`vscode://` URI** — No built-in URI opens a terminal; only extension-registered handlers (e.g. built-in `vscode.git/clone`) exist, each via `window.registerUriHandler` + `createTerminal({shellPath, shellArgs})` ([vscode-discussions #1065](https://github.com/microsoft/vscode-discussions/discussions/1065), [uri-handler-sample](https://github.com/microsoft/vscode-extension-samples/blob/main/uri-handler-sample/src/extension.ts)). The generic `command:` URI scheme exists but is documented only for trusted internal contexts (webviews with `enableCommandUris`, trusted markdown) — external invocation is undocumented, gated behind an "Allow extension to open URI" dialog, and was an RCE vector fixed as CVE-2022-41034 ([VS Code security advisory](https://github.com/microsoft/vscode/security/advisories/GHSA-q6rv-h25q-6pj6)). Verdict: not usable without shipping/installing an extension — excluded by the constraint.

4. **`code --command <id>`** — Does not exist in the official CLI (stable or documented Insiders) ([VS Code CLI docs](https://code.visualstudio.com/docs/configure/command-line)). Verdict: dead end.

5. **`code chat`** — Exists in 1.128 as `code chat [prompt]`, opens VS Code's chat panel, not a terminal ([VS Code CLI docs](https://code.visualstudio.com/docs/configure/command-line), [v1.128 release notes](https://code.visualstudio.com/updates/v1_128)). Verdict: wrong surface entirely.

6. **`terminal.integrated.profiles.osx` + `workbench.action.terminal.newWithProfile`** — Real command/setting, but only invocable via a keybinding, Command Palette, or `vscode.commands.executeCommand` from inside an extension — none of which an external CLI can fire directly. Falls back to option 2 (keystroke) to trigger it. Verdict: not independently useful externally.

7. **`code serve-web`** — Exists, but launches a browser-accessible remote VS Code *server*, unrelated to controlling an already-running desktop window ([VS Code Server docs](https://code.visualstudio.com/docs/remote/vscode-server)). Not applicable.

8. **Claude Code IDE MCP bridge** — Real (`ide` MCP server, WebSocket on localhost, `mcp__ide__*` tools) but requires the Claude Code VS Code extension installed and active ([Claude Code IDE integration docs](https://code.claude.com/docs/en/ide-integrations)) — explicitly excluded by the "not an installed extension" constraint, and its exposed tools are diagnostics/diff, not terminal-opening.

**Open questions:** whether `code -r` on an already-open folder re-triggers `runOn: folderOpen` (undocumented, would need empirical testing); whether the one-time "Allow Automatic Tasks" trust prompt can be pre-seeded via `task.allowAutomaticTasks` in workspace settings to make the tasks.json path fully silent.

**Recommendation:** Pre-author a `.vscode/tasks.json` task (`runOn: folderOpen`, `presentation.panel: "new"`, `command: "pfm chat attach <name>"`) with `task.allowAutomaticTasks: "on"` in the target workspace's settings, then call `code -r <folder>`; verify empirically whether reopen re-fires it. If it does not, fall back to `osascript` System Events keystroke-simulation (activate the window by title, send ⌃⇧\`, type the command) as the universal but more brittle path — never the URI or `--command` routes, which don't exist without an installed extension.

Sources:
- [VS Code CLI docs](https://code.visualstudio.com/docs/configure/command-line)
- [VS Code v1.128 release notes](https://code.visualstudio.com/updates/v1_128)
- [VS Code tasks docs](https://code.visualstudio.com/docs/debugtest/tasks)
- [microsoft/vscode #160013](https://github.com/microsoft/vscode/issues/160013)
- [microsoft/vscode #160174](https://github.com/microsoft/vscode/issues/160174)
- [microsoft/vscode #72448](https://github.com/Microsoft/vscode/issues/72448)
- [microsoft/vscode #174740](https://github.com/microsoft/vscode/issues/174740)
- [vscode-discussions #1065 (terminal URI)](https://github.com/microsoft/vscode-discussions/discussions/1065)
- [uri-handler-sample](https://github.com/microsoft/vscode-extension-samples/blob/main/uri-handler-sample/src/extension.ts)
- [VS Code security advisory GHSA-q6rv-h25q-6pj6 (CVE-2022-41034)](https://github.com/microsoft/vscode/security/advisories/GHSA-q6rv-h25q-6pj6)
- [Wikibooks AppleScript/System Events](https://en.wikibooks.org/wiki/AppleScript_Programming/System_Events)
- [Apple Developer Forums — System Events keystroke](https://developer.apple.com/forums/thread/70284)
- [VS Code Server docs](https://code.visualstudio.com/docs/remote/vscode-server)
- [Claude Code IDE integration docs](https://code.claude.com/docs/en/ide-integrations)
