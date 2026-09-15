package config

import (
	"path/filepath"
	"testing"
)

// TestAmbientClaudeConfigDirCleansOrReportsUnset pins the one rule the
// launcher shim and the Claude registry resolver both read through: unset or
// blank is "", and a set value comes back Cleaned so a trailing slash or a
// "./" segment never makes an otherwise-identical directory compare unequal
// to the physical path a registry write resolves.
func TestAmbientClaudeConfigDirCleansOrReportsUnset(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if got := AmbientClaudeConfigDir(); got != "" {
		t.Fatalf("AmbientClaudeConfigDir()=%q, want empty when unset", got)
	}

	t.Setenv("CLAUDE_CONFIG_DIR", "  ")
	if got := AmbientClaudeConfigDir(); got != "" {
		t.Fatalf("AmbientClaudeConfigDir()=%q, want empty when blank", got)
	}

	want := filepath.Join(t.TempDir(), ".cc", "2")
	t.Setenv("CLAUDE_CONFIG_DIR", want+string(filepath.Separator))
	if got := AmbientClaudeConfigDir(); got != want {
		t.Fatalf("AmbientClaudeConfigDir()=%q, want cleaned %q", got, want)
	}
}
