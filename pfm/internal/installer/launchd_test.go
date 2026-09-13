package installer

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

var plistEnvironmentPath = regexp.MustCompile(`<key>EnvironmentVariables</key>\s*<dict>\s*<key>PATH</key>\s*<string>([^<]*)</string>`)

// TestMCPLaunchAgentGivesTheDaemonAPathThatFindsTmux is the regression for
// the deaf shared daemon: the plist carried no EnvironmentVariables, so launchd
// ran `pfm mcp serve` on /usr/bin:/bin:/usr/sbin:/sbin. tmux (Homebrew:
// /opt/homebrew/bin, or /usr/local/bin on Intel) was off that PATH, every
// socket probe failed to start, and every chat tool a Codex chat called over
// the daemon answered as if no chat were live. The systemd unit had pinned its
// PATH all along; the launchd twin never did.
//
// It drives the real install path and reads the plist launchd will load.
func TestMCPLaunchAgentGivesTheDaemonAPathThatFindsTmux(t *testing.T) {
	home := t.TempDir()
	installer := &engine{
		options: Options{
			Home:       home,
			Stdout:     io.Discard,
			MCPEnabled: map[string]bool{"chat": true},
			Runner:     &loadedRunner{},
			Sleep:      func(time.Duration) {},
		},
		apply: true,
		stamp: "test",
	}
	if err := installer.wireMCPLaunchAgent(context.Background()); err != nil {
		t.Fatalf("wireMCPLaunchAgent() error = %v", err)
	}
	written, err := os.ReadFile(installer.mcpLaunchAgentPath())
	if err != nil {
		t.Fatalf("read installed MCP launch agent: %v", err)
	}
	match := plistEnvironmentPath.FindSubmatch(written)
	if match == nil {
		t.Fatalf("installed MCP launch agent declares no EnvironmentVariables PATH; launchd will run the daemon on its bare default PATH:\n%s", written)
	}
	daemonPath := strings.Split(string(match[1]), ":")
	for _, want := range []string{
		filepath.Join(home, ".local", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
	} {
		if !slices.Contains(daemonPath, want) {
			t.Errorf("daemon PATH %q is missing %s", match[1], want)
		}
	}
	if strings.Contains(string(written), "__PFM_HOME__") {
		t.Fatalf("installed MCP launch agent still holds an unrendered __PFM_HOME__:\n%s", written)
	}
}

// TestEveryServiceUnitTakesTheOneServicePath keeps the search path ONE
// implementation: every launchd agent and systemd unit that starts pfm takes
// its PATH from the servicePath marker — never a PATH spelled by hand, which
// is how the systemd unit had one and its launchd twins had none — and the
// rendered unit carries servicePath exactly.
func TestEveryServiceUnitTakesTheOneServicePath(t *testing.T) {
	home := t.TempDir()
	units := 0
	err := fs.WalkDir(embeddedAssets, "assets", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative := strings.TrimPrefix(name, "assets/")
		if !strings.HasPrefix(relative, "launchd/") && !(strings.HasPrefix(relative, "systemd/") && strings.HasSuffix(relative, ".service")) {
			return nil
		}
		content, err := readAsset(relative)
		if err != nil {
			return err
		}
		if !strings.Contains(string(content), ".local/bin/pfm") {
			return nil
		}
		units++
		if got := strings.Count(string(content), servicePathMarker); got != 1 {
			t.Errorf("%s holds the service PATH marker %d times, want exactly once", relative, got)
			return nil
		}
		rendered, err := renderServicePath(content, home)
		if err != nil {
			return fmt.Errorf("render %s: %w", relative, err)
		}
		if got := strings.Count(string(rendered), servicePath(home)); got != 1 {
			t.Errorf("%s renders servicePath %d times, want once:\n%s", relative, got, rendered)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// 2 launchd agents + 2 systemd services start pfm; fewer means the walk
	// did not look.
	if units < 4 {
		t.Fatalf("found %d pfm service units, want at least 4 — the asset walk did not look", units)
	}
}

// TestNameSyncLaunchAgentGivesTheJobAPathThatFindsTmux is the same regression
// on the other launchd agent: `pfm name-sync` converges every chat's tmux
// window name — the name a VS Code tab shows through set-titles — and under
// launchd's bare PATH it could not exec tmux. It ran 159 times on a live Mac,
// planned "0 windows" each time, and exited 0, so no Codex rename ever reached
// a window or a tab.
func TestNameSyncLaunchAgentGivesTheJobAPathThatFindsTmux(t *testing.T) {
	home := t.TempDir()
	installer := &engine{
		options: Options{Home: home, Stdout: io.Discard, Runner: &loadedRunner{}, Sleep: func(time.Duration) {}},
		apply:   true,
		stamp:   "test",
	}
	if err := installer.wireLaunchAgent(context.Background()); err != nil {
		t.Fatalf("wireLaunchAgent() error = %v", err)
	}
	written, err := os.ReadFile(installer.launchAgentPath())
	if err != nil {
		t.Fatalf("read installed name-sync launch agent: %v", err)
	}
	match := plistEnvironmentPath.FindSubmatch(written)
	if match == nil {
		t.Fatalf("installed name-sync launch agent declares no EnvironmentVariables PATH; launchd will run it on its bare default PATH:\n%s", written)
	}
	if want := servicePath(home); string(match[1]) != want {
		t.Fatalf("name-sync PATH = %q, want the one service path %q", match[1], want)
	}
}
