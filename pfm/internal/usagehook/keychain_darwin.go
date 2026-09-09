//go:build darwin

package usagehook

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"hostops/pfm/internal/deps"
)

// keychainNotFoundStatus is the exit status /usr/bin/security uses for
// errSecItemNotFound. Mapping it to os.ErrNotExist is what lets an absent
// keychain entry stay distinguishable from a keychain we FAILED to read (a
// locked keychain, a denied ACL), which must never render as absence.
const keychainNotFoundStatus = 44

// securityCommand is the registry name for the keychain tool; deps.Resolve
// pins it to the absolute system path so no $PATH entry can shadow the door to
// the user's credentials.
const securityCommand = "security"

// readKeychain returns the raw credential blob macOS holds for one service.
// It shells out to /usr/bin/security rather than linking Security.framework
// because pfm builds CGO_ENABLED=0; the subprocess is the only cgo-free door
// to the keychain.
func readKeychain(service string) ([]byte, error) {
	binary, err := deps.Resolve(securityCommand)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", securityCommand, err)
	}
	command := exec.Command(binary, "find-generic-password", "-s", service, "-w")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == keychainNotFoundStatus {
			return nil, os.ErrNotExist
		}
		detail := strings.Join(strings.Fields(stderr.String()), " ")
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("security find-generic-password: %s", detail)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}
