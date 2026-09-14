package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeLauncherInstallDisplacementAndRepair(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(home, ".local", "bin", "claude")
	nativeOne := filepath.Join(home, ".local", "share", "claude", "versions", "1.0.0")
	nativeTwo := filepath.Join(home, ".local", "share", "claude", "versions", "2.0.0")
	for _, binary := range []string{nativeOne, nativeTwo} {
		if err := os.MkdirAll(filepath.Dir(binary), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nativeOne, canonical); err != nil {
		t.Fatal(err)
	}

	apply := func() {
		t.Helper()
		if _, err := Run(context.Background(), Options{
			Mode: ModeApply, Home: home, Runner: &fakeRunner{},
		}); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	managed := filepath.Join(home, ".local", "share", "pfm", "install", "bin", "claude")
	assertLink(t, canonical, managed)
	content, err := os.ReadFile(filepath.Join(home, ".local", "share", "pfm", "install", "launcher.state"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), nativeOne) {
		t.Fatalf("launcher.state=%q, want displaced target %q", content, nativeOne)
	}

	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nativeTwo, canonical); err != nil {
		t.Fatal(err)
	}
	status, err := InspectClaudeLauncher(home)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != LauncherDisplaced || status.Target != nativeTwo {
		t.Fatalf("displaced status=%#v, want target %s", status, nativeTwo)
	}

	apply()
	assertLink(t, canonical, managed)
	if changed, err := RepairClaudeLauncher(home); err != nil || changed {
		t.Fatalf("idempotent repair changed=%t err=%v", changed, err)
	}
	if err := os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(nativeTwo, canonical); err != nil {
		t.Fatal(err)
	}
	if changed, err := RepairClaudeLauncher(home); err != nil || !changed {
		t.Fatalf("displaced repair changed=%t err=%v", changed, err)
	}
	assertLink(t, canonical, managed)
}

func TestRenderedClaudeLauncherUsesConfiguredAbsoluteBinaryThenSkipsItself(t *testing.T) {
	raw, err := readAsset("bin/claude")
	if err != nil {
		t.Fatal(err)
	}
	configured := filepath.Join(t.TempDir(), "configured claude")
	renderedBytes, err := renderClaudeLauncherAsset(raw, Options{ClaudeBinary: configured})
	if err != nil {
		t.Fatal(err)
	}
	rendered := string(renderedBytes)
	if !strings.Contains(rendered, configured) {
		t.Fatalf("rendered launcher omitted configured binary %q:\n%s", configured, rendered)
	}
	for _, want := range []string{"internal launch --real", `"$@"`, ".local/share/claude/versions", "command -v"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered launcher omitted %q:\n%s", want, rendered)
		}
	}
}

func TestAssetRenderersRefuseMissingTemplateMarkers(t *testing.T) {
	if _, err := renderShimAsset([]byte("marker drift\n"), Options{}); err == nil {
		t.Fatal("shim renderer silently accepted missing markers")
	}
	if _, err := renderClaudeLauncherAsset([]byte("marker drift\n"), Options{}); err == nil {
		t.Fatal("launcher renderer silently accepted missing marker")
	}
}

func TestRenderedClaudeLauncherChoosesNewestVersionByFreshness(t *testing.T) {
	home := t.TempDir()
	raw, err := readAsset("bin/claude")
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(home, ".local", "share", "pfm", "install", "bin", "claude")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o700); err != nil {
		t.Fatal(err)
	}
	rendered, err := renderClaudeLauncherAsset(raw, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, rendered, 0o700); err != nil {
		t.Fatal(err)
	}
	versions := filepath.Join(home, ".local", "share", "claude", "versions")
	if err := os.MkdirAll(versions, 0o700); err != nil {
		t.Fatal(err)
	}
	lexicallyLast := filepath.Join(versions, "9.9.9")
	newest := filepath.Join(versions, "10.0.0")
	for _, path := range []string{lexicallyLast, newest} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lexicallyLast, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newest, stamp.Add(time.Minute), stamp.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	pfm := filepath.Join(home, ".local", "bin", "pfm")
	if err := os.MkdirAll(filepath.Dir(pfm), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pfm, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(launcher, "--resume", "fixture")
	command.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(home, ".local", "bin") + ":/usr/bin:/bin"}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed: %v: %s", err, output)
	}
	want := "internal launch --real " + newest + " -- --resume fixture\n"
	if string(output) != want {
		t.Fatalf("launcher output=%q, want newest-by-mtime %q", output, want)
	}
}

// TestRenderedClaudeLauncherExits127WithNoRealBinary pins the shim's exit
// contract: no real Claude binary anywhere (empty PATH, empty HOME) is exit
// 127 — POSIX "command not found" — never a bare 1, so doctor can tell
// absence from a real binary's own failure without reading stderr text.
func TestRenderedClaudeLauncherExits127WithNoRealBinary(t *testing.T) {
	home := t.TempDir()
	raw, err := readAsset("bin/claude")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderClaudeLauncherAsset(raw, Options{})
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(home, "claude")
	if err := os.WriteFile(launcher, rendered, 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(launcher, "--version")
	command.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(home, "empty-bin")}
	_, err = command.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 127 {
		t.Fatalf("launcher with no real binary err=%v, want exit 127", err)
	}
}

func TestInspectClaudeLauncherRejectsBrokenManagedTarget(t *testing.T) {
	home := t.TempDir()
	canonical := canonicalClaudeLauncher(home)
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedClaudeLauncher(home), canonical); err != nil {
		t.Fatal(err)
	}
	if _, err := InspectClaudeLauncher(home); err == nil || !strings.Contains(err.Error(), "inspect managed Claude launcher") {
		t.Fatalf("InspectClaudeLauncher broken target error=%v", err)
	}
}

func TestClaudeAbsentIdentifiesOnlyPfmsLauncherAtExit127(t *testing.T) {
	home := t.TempDir()
	canonical := canonicalClaudeLauncher(home)
	if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(managedClaudeLauncher(home)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedClaudeLauncher(home), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedClaudeLauncher(home), canonical); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(home, "elsewhere", "claude")
	if err := os.MkdirAll(filepath.Dir(elsewhere), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(elsewhere, []byte("#!/bin/sh\nexit 127\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	if !ClaudeAbsent(home, canonical, 127) {
		t.Fatal("pfm's own launcher at exit 127 was not recognised as absent")
	}
	if ClaudeAbsent(home, canonical, 1) {
		t.Fatal("pfm's own launcher at exit 1 was wrongly treated as absent")
	}
	if ClaudeAbsent(home, elsewhere, 127) {
		t.Fatal("a non-pfm claude at exit 127 was wrongly treated as absent")
	}
}
