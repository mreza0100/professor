// Command claude is a stand-in for Claude Code inside the demo fence: it keeps
// the harness contract pfm reads — argv[0] "claude", a transcript under
// $CLAUDE_CONFIG_DIR/projects, and the configured statusLine command run with
// Claude Code's JSON payload on every redraw — and draws a Claude-like screen.
// It never calls a model. Every argv it receives is logged to ~/fake-claude.log.
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type turn struct{ who, text string }

var (
	mu    sync.Mutex
	turns []turn
)

func main() {
	home, _ := os.UserHomeDir()
	logf, err := os.OpenFile(filepath.Join(home, "fake-claude.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		fmt.Fprintf(logf, "%s argv=%q CLAUDE_CONFIG_DIR=%q\n", time.Now().Format(time.RFC3339), os.Args, os.Getenv("CLAUDE_CONFIG_DIR"))
		logf.Close()
	}

	args := os.Args[1:]
	for _, a := range args {
		if a == "-p" || a == "--print" {
			// Print mode (pfm's Limits credential refresh sends "ACK"): answer
			// once and exit, like the real harness — never a live chat.
			fmt.Println("ACK")
			return
		}
	}
	sid, name, prompt := "", os.Getenv("FAKE_NAME"), ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "--session-id" || a == "--resume" || a == "-r":
			sid = next()
		case a == "--name" || a == "-n":
			name = next()
		case strings.HasPrefix(a, "--"):
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && a != "--dangerously-skip-permissions" && a != "--continue" {
				i++
			}
		default:
			prompt = a
		}
	}
	if sid == "" {
		sid = uuid()
	}
	cfg := os.Getenv("CLAUDE_CONFIG_DIR")
	if cfg == "" {
		cfg = filepath.Join(home, ".claude")
	}
	cwd, _ := os.Getwd()
	transcript := filepath.Join(cfg, "projects", strings.ReplaceAll(cwd, "/", "-"), sid+".jsonl")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "fake claude: transcript dir: %v\n", err)
	}
	if name != "" {
		appendLine(transcript, map[string]any{"type": "custom-title", "customTitle": name, "sessionId": sid})
	}
	if prompt != "" {
		userTurn(transcript, sid, cwd, prompt)
	}

	statusCmd := readStatusLine(filepath.Join(cfg, "settings.json"))
	go func() {
		in := bufio.NewScanner(os.Stdin)
		for in.Scan() {
			line := strings.TrimSpace(in.Text())
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "/rename ") {
				appendLine(transcript, map[string]any{"type": "custom-title", "customTitle": strings.TrimPrefix(line, "/rename "), "sessionId": sid})
				continue
			}
			userTurn(transcript, sid, cwd, line)
		}
	}()
	for {
		draw(statusCmd, sid, transcript, cwd)
		time.Sleep(3 * time.Second)
	}
}

func userTurn(transcript, sid, cwd, text string) {
	appendLine(transcript, map[string]any{"type": "user", "cwd": cwd, "sessionId": sid, "message": map[string]any{"role": "user", "content": text}})
	reply := "On it."
	appendLine(transcript, map[string]any{"type": "assistant", "cwd": cwd, "sessionId": sid, "message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": reply}}}})
	mu.Lock()
	turns = append(turns, turn{"❯", text}, turn{"⏺", reply})
	mu.Unlock()
}

func draw(statusCmd, sid, transcript, cwd string) {
	var b strings.Builder
	b.WriteString("\x1b[H\x1b[2J")
	b.WriteString(" \x1b[38;5;173m▐▛███▜▌\x1b[0m   Claude Code\r\n")
	b.WriteString("\x1b[38;5;173m▝▜█████▛▘\x1b[0m  Opus 5 · Claude Max\r\n")
	b.WriteString("  \x1b[38;5;173m▘▘ ▝▝\x1b[0m    " + strings.Replace(cwd, "/root", "~", 1) + "\r\n\r\n")
	mu.Lock()
	for _, t := range turns {
		b.WriteString(t.who + " " + t.text + "\r\n")
	}
	mu.Unlock()
	b.WriteString("\r\n\x1b[2m" + strings.Repeat("─", 70) + "\x1b[0m\r\n❯ \r\n\x1b[2m" + strings.Repeat("─", 70) + "\x1b[0m\r\n")
	if statusCmd != "" {
		payload, _ := json.Marshal(map[string]any{
			"session_id": sid, "transcript_path": transcript, "cwd": cwd,
			"model":     map[string]any{"id": "claude-opus-5", "display_name": "Opus 5"},
			"workspace": map[string]any{"current_dir": cwd},
			"effort":    map[string]any{"level": "high"},
			"context_window": map[string]any{"used_percentage": 12.0, "total_input_tokens": 48000},
		})
		cmd := exec.Command("sh", "-c", statusCmd)
		cmd.Stdin = bytes.NewReader(payload)
		cmd.Env = os.Environ()
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(&b, "statusline error: %v\r\n", err)
		}
		b.WriteString(strings.ReplaceAll(strings.TrimRight(string(out), "\n"), "\n", "\r\n"))
	}
	os.Stdout.WriteString(b.String())
}

func readStatusLine(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var s struct {
		StatusLine struct {
			Command string `json:"command"`
		} `json:"statusLine"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s.StatusLine.Command
}

func appendLine(path string, v any) {
	raw, _ := json.Marshal(v)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake claude: transcript: %v\n", err)
		return
	}
	defer f.Close()
	f.Write(append(raw, '\n'))
}

func uuid() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
