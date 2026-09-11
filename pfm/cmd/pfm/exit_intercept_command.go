package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// exitInterceptRun runs the kill FRONT in-process — the same runKill a human's
// own `pfm chat kill self --exit` call would reach. A package var, not a direct
// call, so a test can assert the exact argument slice the hook derived from the
// typed prompt without closing a live chat.
var exitInterceptRun = runKill

// exitWords are the prompts that mean "close this chat". Both spellings are
// here on purpose: `e` is the muscle memory from the shell alias this hook
// replaces, and `/e` is the slash form a human reaches for inside a composer.
var exitWords = map[string]bool{"e": true, "/e": true}

// runExitIntercept is the UserPromptSubmit hook body for
// `pfm internal exit-intercept`. A prompt of exactly "e" or "/e" is the human
// asking to close the chat, so it runs the close here — before the model ever
// sees the prompt — instead of spending a whole turn having the model type the
// identical call into Bash.
//
// Claude Code's UserPromptSubmit contract: exit 0 lets the prompt through
// (stdout is added as context); exit 2 BLOCKS the prompt, erases it, and shows
// stderr to the human. A matched prompt always exits 2 — it must never reach
// the model whether the close succeeded or failed; the human reads the front's
// own result on stderr instead.
//
// The close itself is `chat kill self --exit`, which sends /exit to this pane.
// That means the terminal is closed by the SessionEnd hook (exit-close), the
// same path a hand-typed /exit takes — one closer, two entry points, so the two
// spellings can never drift apart.
//
// Codex has no UserPromptSubmit hook, so a Codex seat's "e" still goes through
// the model. This is a Claude-only shortcut, not the only path to a close.
func runExitIntercept(stdin io.Reader, stderr io.Writer, runtime commandRuntime) int {
	var payload struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(stdin).Decode(&payload); err != nil {
		fmt.Fprintf(stderr, "pfm internal exit-intercept: decode hook payload: %v\n", err)
		return 0
	}
	if !exitWords[strings.TrimSpace(payload.Prompt)] {
		return 0
	}
	// One buffer for both streams so the front's success line and any error it
	// writes land in the order the front itself produced them.
	var captured bytes.Buffer
	exitInterceptRun([]string{"--self", "--exit"}, &captured, &captured, runtime)
	fmt.Fprintf(stderr, "exit: %s", captured.String())
	return 2
}
