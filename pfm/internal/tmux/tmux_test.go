package tmux

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestCommandAddressesTheSocketAndClearsTMUX pins the runner's contract: the
// explicit socket first, the caller's arguments after it, the named binary,
// and $TMUX defined but empty whatever the caller's environment holds.
func TestCommandAddressesTheSocketAndClearsTMUX(t *testing.T) {
	t.Setenv("TMUX", "/tmp/caller-server,1,0")
	binary := filepath.Join(t.TempDir(), "tmux-fake")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := Command(context.Background(), binary, "/sockets/cc-1", "list-panes", "-a")
	if command.Path != binary {
		t.Fatalf("binary = %q, want %q", command.Path, binary)
	}
	if want := []string{binary, "-S", "/sockets/cc-1", "list-panes", "-a"}; !slices.Equal(command.Args, want) {
		t.Fatalf("args = %q, want %q", command.Args, want)
	}
	last := ""
	for _, entry := range command.Env {
		if len(entry) >= 5 && entry[:5] == "TMUX=" {
			last = entry
		}
	}
	if last != "TMUX=" {
		t.Fatalf("effective TMUX entry = %q, want it defined and empty", last)
	}
	if defaulted := Command(context.Background(), "", "/sockets/cc-1"); filepath.Base(defaulted.Args[0]) != "tmux" {
		t.Fatalf("default binary = %q, want tmux", defaulted.Args[0])
	}
}
