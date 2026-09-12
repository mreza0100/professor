// Package tmux is the one tmux runner: every tmux invocation pfm makes on a
// chat's server is built by Command, so the socket addressing, the binary
// lookup and the cleared $TMUX live in one place. Each consumer package keeps
// its own narrow interface over it (its fake is its test seam).
package tmux

import (
	"context"
	"os"
	"os/exec"

	"hostops/pfm/internal/deps"
)

// Command is one tmux invocation on the server at socketPath (tmux -S).
// binary "" means the registered tmux (deps.Executable); a configured binary
// runs exactly as given. TMUX is set, empty: a command run from inside a chat must
// never nest into the caller's own server, and a defined $TMUX is also what
// makes tmux return control characters in a format string verbatim rather
// than as "_".
func Command(ctx context.Context, binary, socketPath string, arguments ...string) *exec.Cmd {
	path, argv, environment := Invocation(binary, socketPath, arguments...)
	command := exec.CommandContext(ctx, path, argv...)
	command.Env = environment
	return command
}

// Invocation is Command unassembled — the resolved binary, its arguments and
// its environment — for a caller that runs tmux under another launcher
// (spawn's durable systemd scope).
func Invocation(binary, socketPath string, arguments ...string) (string, []string, []string) {
	if binary == "" {
		binary = deps.Executable("tmux")
	}
	return binary, append([]string{"-S", socketPath}, arguments...), append(os.Environ(), "TMUX=")
}
