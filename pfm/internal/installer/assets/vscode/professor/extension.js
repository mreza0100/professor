// Professor — the framework's assistant inside VS Code. pfm installs it (`pfm install --vscode`)
// and links it into every VS Code extensions directory on the host.
//
// Its first surface is the Professor terminal profile: a login shell that opens the pfm chat
// fleet, whose tab carries the attached chat's live name, and which puts you in the terminal
// you just made. Each new terminal also takes the next icon+colour pair — icons and colours
// advance on independent counters persisted in globalState — so tabs read apart at a glance.
const vscode = require('vscode');

// Terminals this activation handed to VS Code and has not yet seen open, keyed by the marker
// each one carries in its env. In-memory on purpose: a terminal revived by a window reload was
// not asked for by this activation, so it never steals focus.
const pending = new Set();

function nextTerminal(context) {
  const cfg = vscode.workspace.getConfiguration('professor.terminal');
  const icons = cfg.get('icons'), colors = cfg.get('colors');
  const n = context.globalState.get('terminal.n', 0);
  context.globalState.update('terminal.n', n + 1);
  const marker = `${n}-${Date.now()}`;
  pending.add(marker);
  // No `name`: a named terminal gets a static title, and VS Code's label then ignores the
  // ${sequence} title tmux sends — every chat tab would read the same fixed word forever.
  return {
    shellPath: cfg.get('shellPath'),
    shellArgs: cfg.get('shellArgs'),
    env: { ...cfg.get('env'), PROFESSOR_TERMINAL: marker },
    iconPath: new vscode.ThemeIcon(icons[n % icons.length]),
    color: new vscode.ThemeColor(colors[n % colors.length]),
  };
}

function activate(context) {
  context.subscriptions.push(
    // A contributed profile's terminal reaches the workbench with no id to focus, so it
    // activates the newest instance it already knows — the PREVIOUS terminal — and focus lands
    // one behind. The ext host fires onDidOpenTerminal once the workbench registers ours;
    // showing it then puts the user in the terminal they just made.
    vscode.window.onDidOpenTerminal((terminal) => {
      const marker = terminal.creationOptions?.env?.PROFESSOR_TERMINAL;
      if (marker && pending.delete(marker)) terminal.show(false);
    }),
    vscode.window.registerTerminalProfileProvider('professor.terminal', {
      provideTerminalProfile: () => new vscode.TerminalProfile(nextTerminal(context)),
    }),
    vscode.commands.registerCommand('professor.newChatTerminal', () => {
      vscode.window.createTerminal(nextTerminal(context)).show();
    }),
  );
}
function deactivate() {}
module.exports = { activate, deactivate };
