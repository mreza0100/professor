package run

import (
	"testing"

	pfmengine "hostops/pfm/internal/engine"
)

// pfm stages its own system prompt (--system-prompt-file / --system-prompt);
// Claude Code's own output style would otherwise double-apply a persona on
// top of it. Every headless Claude run — `pfm headless exec`, `pfm ask` — is
// a door of its own (it never renders through action.ClaudeSpawn), so it must
// disable Claude Code's output style too, the same as the fleet's other
// spawn door.
func TestArgumentsCarriesOutputStyleDefaultSettings(t *testing.T) {
	args, err := arguments(Request{Engine: pfmengine.Claude})
	if err != nil {
		t.Fatalf("arguments() error = %v", err)
	}
	if !containsPair(args, "--settings", pfmengine.OutputStyleDefaultSettings) {
		t.Fatalf("Claude args %#v lack --settings %s", args, pfmengine.OutputStyleDefaultSettings)
	}

	codexArgs, err := arguments(Request{Engine: pfmengine.Codex})
	if err != nil {
		t.Fatalf("arguments() error = %v", err)
	}
	for _, argument := range codexArgs {
		if argument == "--settings" {
			t.Fatalf("Codex args carried Claude's --settings flag: %#v", codexArgs)
		}
	}
}
