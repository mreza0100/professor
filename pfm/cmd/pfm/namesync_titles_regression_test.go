package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/gather"
)

func TestNameSyncTitlesReportsAnUnreadableServerAsUnverified(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := t.TempDir()
	client := gather.CommandTmux{Binary: "tmux", TmuxTmpDir: root}
	titles := pfmconfig.DefaultTmuxTitles()
	var stdout, stderr bytes.Buffer
	converged, unverified := convergeTmuxTitles(
		context.Background(), client,
		[]string{filepath.Join(root, "missing-socket")},
		titles,
		&stdout,
		&stderr,
	)
	if converged != 0 {
		t.Fatalf("converged=%d, want 0 for an unreadable server", converged)
	}
	if unverified == 0 {
		t.Fatalf("unverified=%d, want title probe failure carried into command result; stderr=%q", unverified, stderr.String())
	}
	if !strings.Contains(stderr.String(), "could not read set-titles") {
		t.Fatalf("stderr=%q, want the failed title read named", stderr.String())
	}
}

func TestTmuxTitlesDoctorReportsStringDriftAsDivergent(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	root := t.TempDir()
	socket := "titles-doctor"
	environment := append(os.Environ(), "TMUX=", "TMUX_TMPDIR="+root)
	start := exec.Command("tmux", "-L", socket, "-f", "/dev/null", "new-session", "-d", "-s", "fixture", "sleep", "120")
	start.Env = environment
	if output, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start tmux fixture: %v: %s", err, output)
	}
	t.Cleanup(func() {
		kill := exec.Command("tmux", "-L", socket, "kill-server")
		kill.Env = environment
		_ = kill.Run()
	})
	set := exec.Command("tmux", "-L", socket, "set-option", "-g", "set-titles", "on")
	set.Env = environment
	if output, err := set.CombinedOutput(); err != nil {
		t.Fatalf("set-titles: %v: %s", err, output)
	}
	set = exec.Command("tmux", "-L", socket, "set-option", "-g", "set-titles-string", "custom-host-title")
	set.Env = environment
	if output, err := set.CombinedOutput(); err != nil {
		t.Fatalf("set-titles-string: %v: %s", err, output)
	}

	state, detail := readTmuxTitlesState(
		context.Background(),
		gather.CommandTmux{Binary: "tmux", TmuxTmpDir: root},
		socket,
		true,
	)
	if state != titlesDivergent {
		t.Fatalf("state=%q detail=%q, want %q for custom string drift", state, detail, titlesDivergent)
	}
	if !strings.Contains(detail, "custom-host-title") || !strings.Contains(detail, pfmconfig.TmuxTitlesString) {
		t.Fatalf("detail=%q, want actual and expected title strings", detail)
	}
}
