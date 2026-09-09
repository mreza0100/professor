package usagehook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"hostops/pfm/internal/deps"
)

// Only errSecItemNotFound means absence; cancellation and ACL failures do not.
const keychainNotFoundStatus = 44

// runKeychain bounds both the process and inherited output pipes so a locked
// keychain cannot stall a prompt hook or keep a cancelled sampler running.
func runKeychain(ctx context.Context, binary, service string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, deps.ProbeTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "find-generic-password", "-s", service, "-w")
	command.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("security find-generic-password: %w", ctx.Err())
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == keychainNotFoundStatus {
			return nil, os.ErrNotExist
		}
		detail := strings.Join(strings.Fields(stderr.String()), " ")
		return nil, fmt.Errorf("security find-generic-password: %w: %s", err, detail)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}
