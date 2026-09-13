package gather

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pfmtmux "hostops/pfm/internal/tmux"
)

// TestProbeTmuxFailsWholeWhenTmuxCannotRun is the regression for the shared
// MCP daemon going deaf: launchd started it with /usr/bin:/bin:/usr/sbin:/sbin,
// tmux lives outside that PATH, and every socket's list-panes failed to START.
// Each failure was filed as a per-socket warning, so the probe returned zero
// panes and no error — every chat_inject from a Codex chat then read "matched
// no live chat" when the truth was "could not look". A tmux that never ran
// read no socket at all, so the pass fails whole and names the cause.
//
// The binary is a bare name absent from PATH, exactly what deps.Executable
// hands back when its lookup fails, so this is the live error shape.
func TestProbeTmuxFailsWholeWhenTmuxCannotRun(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, probe := range []struct {
		name string
		run  func(context.Context, string, TmuxClient, time.Time) (TmuxProbe, error)
	}{
		{name: "sweeping", run: ProbeTmux},
		{name: "read-only", run: ProbeTmuxReadOnly},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tmuxDir := t.TempDir()
			now := time.Now()
			createCorpseSocket(t, filepath.Join(tmuxDir, "cc-7-8-9"), now.Add(-2*time.Hour))
			client := CommandTmux{Binary: "pfm-test-missing-tmux", TmuxTmpDir: tmuxDir}

			result, err := probe.run(context.Background(), tmuxDir, client, now)
			if err == nil {
				t.Fatalf("probe with an unstartable tmux returned no error: panes=%d warnings=%q — an empty fleet that means \"could not look\"",
					len(result.Panes), result.ProbeWarnings)
			}
			if !pfmtmux.CouldNotRun(err) {
				t.Fatalf("probe error %v does not carry the could-not-run cause", err)
			}
			if !strings.Contains(err.Error(), "pfm-test-missing-tmux") {
				t.Fatalf("probe error %q does not name the binary it could not run", err)
			}
			// A probe that could not read a socket must never sweep it.
			if _, statErr := os.Stat(filepath.Join(tmuxDir, "cc-7-8-9")); statErr != nil {
				t.Fatalf("probe that could not run removed a socket it never read: %v", statErr)
			}
		})
	}
}

// TestProbeTmuxKeepsAFailingServerAsAWarning pins the other side of the
// line: a tmux that RAN and failed against one server is that server's
// problem, reported as a warning while the rest of the fleet still lists.
func TestProbeTmuxKeepsAFailingServerAsAWarning(t *testing.T) {
	tmuxDir := t.TempDir()
	now := time.Now()
	createCorpseSocket(t, filepath.Join(tmuxDir, "cc-7-8-9"), now)
	result, err := ProbeTmuxReadOnly(context.Background(), tmuxDir, alwaysFailTmux{}, now)
	if err != nil {
		t.Fatalf("one failing server failed the whole probe: %v", err)
	}
	if len(result.ProbeWarnings) != 1 {
		t.Fatalf("warnings = %q, want the one failing server named", result.ProbeWarnings)
	}
}
