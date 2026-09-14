package installer

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallRetiresAPFMHookThisBinaryDoesNotImplement pins issue #24
// finding 2's still-open half: a rollback to an older pfm (or a hook a
// newer-then-reverted pfm wrote) leaves an entry of pfm's own shape naming a
// subcommand THIS binary neither implements nor lists as retired. Unlike the
// table-retired shapes (already covered), nothing recognized this one at
// all before unknownPFMHookCommand — so the very next `pfm install` apply
// must strip it while every real template hook converges untouched.
func TestInstallRetiresAPFMHookThisBinaryDoesNotImplement(t *testing.T) {
	home := filepath.Join("neutral", "home")
	pfm := home + "/.local/bin/pfm"
	unknown := pfm + " internal hook-from-a-newer-pfm"
	raw := []byte(`{
  "hooks": {
    "UserPromptSubmit": [
      {"matcher":"","hooks":[
        {"type":"command","command":"` + unknown + `"}
      ]}
    ]
  }
}`)

	updated, changed, _, err := updateSettings(raw, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("apply did not change a document containing an unknown pfm-shaped hook")
	}
	if strings.Contains(string(updated), "hook-from-a-newer-pfm") {
		t.Fatalf("unknown pfm-shaped hook survived an install apply:\n%s", updated)
	}
	for _, template := range claudeHookTemplates(home) {
		if got := hookCommandCount(t, string(updated), template.Event, template.Command); got != 1 {
			t.Fatalf("apply dropped installer template %q: count=%d\n%s", template.Name, got, updated)
		}
	}
	// The removal above is powered by exactly this naming rule — the
	// "unknown:<name>" identity ProbeExpectedHooks reports the same entry
	// under (TestProbeExpectedHooksReportsAnUnknownPFMHookAsStale) is the
	// name this call returns.
	if name, ok := unknownPFMHookCommand(unknown, pfm); !ok || name != "hook-from-a-newer-pfm" {
		t.Fatalf("unknownPFMHookCommand(%q, %q) = (%q, %v), want (%q, true)", unknown, pfm, name, ok, "hook-from-a-newer-pfm")
	}
}

// TestInstallLeavesAForeignHookThatMerelyMentionsPFM is the boundary pin for
// unknownPFMHookCommand's false-positive guard: a command whose first token
// is not one of pfm's own binary forms is never matched, even though its
// argument text contains "pfm". This assertion holds on unfixed code too —
// it pins the guard, not the new behavior.
func TestInstallLeavesAForeignHookThatMerelyMentionsPFM(t *testing.T) {
	home := filepath.Join("neutral", "home")
	foreign := "/usr/local/bin/notify --tag pfm"
	raw := []byte(`{
  "hooks": {
    "PostToolUse": [
      {"matcher":"","hooks":[
        {"type":"command","command":"` + foreign + `"}
      ]}
    ]
  }
}`)

	updated, _, _, err := updateSettings(raw, home, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := hookCommandCount(t, string(updated), "PostToolUse", foreign); got != 1 {
		t.Fatalf("foreign hook that merely mentions pfm was removed: count=%d\n%s", got, updated)
	}
}
