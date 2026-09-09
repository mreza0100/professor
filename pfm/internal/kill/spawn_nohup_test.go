package kill

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pfmengine "hostops/pfm/internal/engine"
)

// TestCommandSpawnerUsesNohupWhenSetsidIsAbsent pins the branch the fix
// exists for — every macOS kill, since Darwin ships no setsid. Three facts
// distinguish this from the setsid path in TestCommandSpawnerUsesSetsidSelfReexec:
// the finisher argv must carry no "-f" (nohup takes no such flag); Spawn
// must Start()+Release() rather than Run(), so it returns before the
// finisher exits; and a parent context cancelled right after Spawn returns
// must not kill the finisher — under nohup the launched process IS the
// finisher, and killing it defeats the whole detach.
func TestCommandSpawnerUsesNohupWhenSetsidIsAbsent(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is not installed")
	}
	root := t.TempDir()
	argvPath := filepath.Join(root, "argv")
	donePath := filepath.Join(root, "done")
	nohupPath := filepath.Join(root, "nohup")
	writeTestFile(
		t,
		nohupPath,
		"#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PFM_SPAWN_ARGV\"\nsleep 1\ntouch \"$PFM_SPAWN_DONE\"\n",
	)
	if err := os.Chmod(nohupPath, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PFM_SPAWN_ARGV", argvPath)
	t.Setenv("PFM_SPAWN_DONE", donePath)
	spawner := CommandSpawner{
		Executable: "/jail/pfm",
		Setsid:     filepath.Join(root, "no-such-setsid-binary"),
		Nohup:      nohupPath,
		ConfigPath: "/jail/config/pfm.json",
	}
	args := ExitArgs{
		Engine:     pfmengine.Claude,
		ID:         "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		DataPath:   "/jail/transcript.jsonl",
		SocketPath: "/jail/tmux/cc-2-1-1",
		SocketName: "cc-2-1-1",
		PaneID:     "%4",
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := time.Now()
	if err := spawner.Spawn(ctx, args); err != nil {
		cancel()
		t.Fatalf("Spawn returned an error on the nohup branch: %v", err)
	}
	elapsed := time.Since(started)
	// Cancel immediately: the nohup branch must not bind the launched
	// process to ctx, or this kills the finisher before its 1s sleep ends.
	cancel()
	if elapsed >= 900*time.Millisecond {
		t.Fatalf("Spawn took %v — it must Start()+Release() the nohup helper, not wait on it", elapsed)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(donePath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nohup finisher never completed — a cancelled parent context killed it")
		}
		time.Sleep(20 * time.Millisecond)
	}
	content, err := os.ReadFile(argvPath)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	for _, field := range argv {
		if field == "-f" {
			t.Fatalf("nohup argv = %q carries setsid's -f flag; nohup takes no such flag", argv)
		}
	}
	want := []string{
		"/jail/pfm",
		"--config",
		"/jail/config/pfm.json",
		"internal",
		"kill-exit",
		"--engine",
		string(args.Engine),
		"--id",
		args.ID,
		"--path",
		args.DataPath,
		"--socket",
		args.SocketPath,
		"--socket-name",
		args.SocketName,
		"--pane",
		args.PaneID,
	}
	if strings.Join(argv, "\n") != strings.Join(want, "\n") {
		t.Fatalf("nohup argv = %q, want %q", argv, want)
	}
}
