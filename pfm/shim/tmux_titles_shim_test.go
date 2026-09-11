package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	pfmconfig "hostops/pfm/internal/config"
)

// The shim is the third place the title options are applied, and it must obey
// the same tmux.titles policy the two Go spawn paths do. The fixture runs the
// real `_cx_server` against a RECORDING tmux — no server is created, so no
// socket is touched — and reads back the argv the shim would have applied.
func runCxServerWithFakePfm(t *testing.T, pfmScript string) (calls string, stderr string) {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not installed")
	}
	shimPath := embeddedShimPath(t)
	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "pfm"), []byte(pfmScript), 0o700); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(home, "tmux-calls")
	fakeTmux := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + log + "\nexit 0\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "tmux"), []byte(fakeTmux), 0o700); err != nil {
		t.Fatal(err)
	}

	script := "source " + quoteZsh(shimPath) + "\n_cx_server probe-shim-sock /tmp 'sleep 1'\n"
	command := jailedZshCommand(
		zsh, script, home,
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	var errorOutput strings.Builder
	command.Stderr = &errorOutput
	if output, err := command.Output(); err != nil {
		t.Fatalf("run _cx_server: %v: %s %s", err, output, errorOutput.String())
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read tmux calls: %v", err)
	}
	return string(body), errorOutput.String()
}

// titlesLineProtocol builds the fake `pfm internal tmux-titles` script from
// the SAME constant the real binary prints (pfmconfig.TmuxTitlesString),
// never a copied literal — a hand-copied spelling here is exactly how this
// test would stop noticing a future drift between the shim and the constant.
// The value is embedded inside a single-quoted printf argument, so a
// constant that ever grew a single quote would break the fixture's own
// quoting rather than the shim under test; guard that assumption outright
// instead of producing a confusing shell syntax error.
func titlesLineProtocol(t *testing.T) string {
	t.Helper()
	if strings.ContainsRune(pfmconfig.TmuxTitlesString, '\'') {
		t.Fatalf(
			"pfmconfig.TmuxTitlesString contains a single quote, which breaks "+
				"this fixture's single-quoted embedding: %q",
			pfmconfig.TmuxTitlesString,
		)
	}
	return "#!/bin/sh\n" +
		"case \"$1:$2\" in\n" +
		"  internal:tmux-titles) printf '%s\\n' 'set-titles on' 'set-titles-string " +
		pfmconfig.TmuxTitlesString + "' ;;\n" +
		"  *) exit 0 ;;\n" +
		"esac\n"
}

func TestShimAppliesTheTitleOptionsPfmPrints(t *testing.T) {
	calls, _ := runCxServerWithFakePfm(t, titlesLineProtocol(t))
	for _, want := range []string{
		"-L probe-shim-sock set -g set-titles on",
		"-L probe-shim-sock set -g set-titles-string " + pfmconfig.TmuxTitlesString,
		"-L probe-shim-sock setw -g automatic-rename off",
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("tmux calls missing %q:\n%s", want, calls)
		}
	}
}

// tmux.titles disabled: pfm prints nothing, so the shim applies no title
// option and the host's own OSC title survives the spawn. automatic-rename is
// never gated — the window name is the fleet's DNS record.
func TestShimAppliesNoTitleOptionWhenPfmPrintsNone(t *testing.T) {
	calls, _ := runCxServerWithFakePfm(t, "#!/bin/sh\nexit 0\n")
	if strings.Contains(calls, "set-titles") {
		t.Fatalf("the shim applied a title option nobody asked for:\n%s", calls)
	}
	if !strings.Contains(calls, "setw -g automatic-rename off") {
		t.Fatalf("automatic-rename was gated by the title policy:\n%s", calls)
	}
}

// Fail-CLOSED: a policy the shim could not read leaves the host's title alone
// rather than seizing it, and says why on stderr.
func TestShimLeavesTheTitleAloneWhenPfmRefuses(t *testing.T) {
	calls, errorOutput := runCxServerWithFakePfm(
		t, "#!/bin/sh\necho 'pfm internal tmux-titles: config unreadable' >&2\nexit 1\n",
	)
	if strings.Contains(calls, "set-titles") {
		t.Fatalf("the shim applied a title option from a failed policy read:\n%s", calls)
	}
	if !strings.Contains(errorOutput, "config unreadable") {
		t.Fatalf("the shim swallowed the reason: %q", errorOutput)
	}
}
