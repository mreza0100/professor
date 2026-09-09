//go:build !darwin

package usagehook

import "os"

// readKeychain has no non-macOS implementation: every other platform Claude
// Code runs on keeps the credential in .credentials.json, so "no keychain
// entry" is the honest and permanent answer rather than a failure to look.
func readKeychain(service string) ([]byte, error) {
	return nil, os.ErrNotExist
}
