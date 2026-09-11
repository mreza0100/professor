package action

import (
	"context"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	pfmengine "hostops/pfm/internal/engine"
)

// pfm stages its own system prompt (--system-prompt-file); Claude Code's own
// output style (a project or user "outputStyle" setting) would otherwise
// double-apply a persona on top of it. Every purpose the door recognizes —
// interactive, resume, probe, query — must carry --settings
// {"outputStyle":"default"} on both renderers, since a probe or a query still
// starts a real Claude process even though it carries no prompt material of
// its own.
func TestClaudeSpawnCarriesOutputStyleDefaultSettings(t *testing.T) {
	home := t.TempDir()
	machine := configuredMachinePolicy(home)
	want := "'--settings' " + Quote(pfmengine.OutputStyleDefaultSettings)
	for _, purpose := range []Purpose{PurposeInteractive, PurposeResume, PurposeProbe, PurposeQuery} {
		spawn := ClaudeSpawn{Purpose: purpose, Account: 42, Home: home, Machine: machine}

		shell, err := spawn.ShellCommand()
		if err != nil {
			t.Fatalf("%s shell spawn: %v", purpose, err)
		}
		if !strings.Contains(shell, want) {
			t.Fatalf("%s shell spawn %q lacks %q", purpose, shell, want)
		}

		command, err := spawn.Command(context.Background())
		if err != nil {
			t.Fatalf("%s command spawn: %v", purpose, err)
		}
		if !containsFlagPair(command.Args, "--settings", pfmengine.OutputStyleDefaultSettings) {
			t.Fatalf("%s command argv %#v lacks --settings %s", purpose, command.Args, pfmengine.OutputStyleDefaultSettings)
		}
	}
}

// A launch that already staged the professor prompt but somehow lost the
// settings flag would double-apply a persona; the launcher-run door
// (action.LauncherRun, the argv-preserving shim spawn) must carry the flag
// too, since it is a distinct constructor from ClaudeSpawn's exported fields.
func TestLauncherRunCarriesOutputStyleDefaultSettings(t *testing.T) {
	home := t.TempDir()
	shell, err := LauncherRun("/opt/claude/real", nil, "/home/tester/.cc/1", home, pfmconfig.ClaudePrefs{})
	if err != nil {
		t.Fatalf("LauncherRun() error = %v", err)
	}
	want := "'--settings' " + Quote(pfmengine.OutputStyleDefaultSettings)
	if !strings.Contains(shell, want) {
		t.Fatalf("launcher run %q lacks %q", shell, want)
	}
}

func containsFlagPair(args []string, key, value string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key && args[index+1] == value {
			return true
		}
	}
	return false
}
