package usagehook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ErrSignedOut marks an account whose credential EXISTS but carries no usable
// token — the shape Claude Code leaves behind after a refresh token expires.
// It is deliberately distinct from os.ErrNotExist: "there is no credential
// here" sends a reader hunting for a missing file, while this condition has
// exactly one repair, an interactive `claude /login`, and no automatic probe
// can perform it. Callers that must tell the two apart at the visible surface
// use IsCredentialUnavailable to cover both and errors.Is for the difference.
var ErrSignedOut = errors.New("account is signed out; run `claude /login` for it")

// KeychainService derives the macOS Keychain generic-password service name
// Claude Code stores an account's OAuth credential under. Claude Code keys the
// entry by the account's config directory — the first four bytes of the
// SHA-256 of the directory path, hex — so the mapping is per-account and this
// derivation is the only thing that ties a config dir to its secret. Deriving
// it (rather than scanning the keychain for any "Claude Code-credentials-*"
// entry) is what keeps one account's quota from ever being attributed to
// another: a host accumulates one entry per config dir it has ever used.
func KeychainService(configDir string) string {
	sum := sha256.Sum256([]byte(configDir))
	return "Claude Code-credentials-" + hex.EncodeToString(sum[:])[:8]
}

// keychainReader is the seam tests replace; the real implementation is
// per-GOOS in keychain_darwin.go / keychain_other.go.
var keychainReader = readKeychain

// loadCredential resolves one account's OAuth credential, file first and
// keychain second. The file is authoritative where it exists because a jail or
// a test can plant one; the keychain is where a real macOS Claude Code install
// actually keeps it, which is why an account can be fully logged in and still
// have no .credentials.json anywhere on disk.
//
// The returned error distinguishes the three outcomes a caller must render
// differently: neither source holds anything (wraps os.ErrNotExist, so the
// statusline-snapshot fallback and the credential probe still engage), a
// source holds a signed-out credential (ErrSignedOut), or a source holds
// something unreadable (a decode error, which is a real failure to look and
// must never be reported as absence).
func loadCredential(configDir string) (credentials, error) {
	var credential credentials
	body, fileErr := os.ReadFile(CredentialPath(configDir))
	if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) {
		return credential, fmt.Errorf("read usage credentials: %w", fileErr)
	}
	source := CredentialPath(configDir)
	if fileErr != nil {
		service := KeychainService(configDir)
		keychainBody, keychainErr := keychainReader(service)
		if keychainErr != nil {
			if errors.Is(keychainErr, os.ErrNotExist) {
				// Name BOTH doors. A message naming only the file sends a
				// keychain host hunting for a file it is never supposed to
				// have, which is the exact confusion this path exists to end.
				return credential, fmt.Errorf(
					"read usage credentials: no %s and no keychain item %q: %w",
					source, service, os.ErrNotExist,
				)
			}
			return credential, fmt.Errorf("read keychain item %q: %w", service, keychainErr)
		}
		body = keychainBody
		source = "keychain item " + service
	}
	if err := json.Unmarshal(body, &credential); err != nil {
		return credential, fmt.Errorf("decode usage credentials from %s: %w", source, err)
	}
	if credential.OAuth.AccessToken == "" {
		return credential, fmt.Errorf("%s holds no access token: %w", source, ErrSignedOut)
	}
	return credential, nil
}

// IsCredentialUnavailable reports whether err means "this account has no
// usable credential for us to spend", covering both the absent and the
// signed-out shape. It is the gate for every fallback that exists because an
// account cannot be queried directly — the statusline snapshot above all —
// since a signed-out account is exactly as unqueryable as a missing one.
func IsCredentialUnavailable(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrSignedOut)
}

// CredentialAvailable reports whether an account has a usable credential in
// either source, returning the same distinguishable error loadCredential does.
func CredentialAvailable(configDir string) error {
	_, err := loadCredential(configDir)
	return err
}
