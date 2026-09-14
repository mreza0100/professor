package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gatewayExemptFiles are the ONLY files allowed to perform HTTP egress without
// going through the fetch gateway, each for a reason that cannot be designed
// away:
//
//   - gateway.go is the gateway.
//   - doh.go resolves DNS. The gateway's own dial needs DNS, so routing DNS
//     through the gateway is a resolution cycle, not a layering choice.
//   - net_chrome_transport.go follows redirects INSIDE the Chrome transport,
//     a layer beneath the gateway entirely.
var gatewayExemptFiles = map[string]string{
	"gateway.go":              "is the gateway",
	"doh.go":                  "resolves DNS; routing it through the gateway would be a cycle",
	"net_chrome_transport.go": "is transport-internal, below the gateway",
}

// TestEveryEgressGoesThroughTheGateway is a closed-world guard, not a spot
// check. The gateway only delivers its guarantees — one SSRF assertion, one
// cookie-jar rule, one redirect re-validation, one byte ceiling, one challenge
// ladder — if EVERY caller enters it. A single new `client.Do(...)` elsewhere
// silently reopens the exact split this package was refactored to close: the
// scholarly provider path could not pass a wall the generic ladder passed.
//
// Such a regression is invisible to every behavioural test, because the new
// call site works fine until the day it meets a wall. So the invariant is
// enforced against the SOURCE.
//
// WHAT THIS REPORTS WHEN IT IS ITSELF BROKEN: if it cannot read the package
// directory or finds no Go files to scan, it FAILS with that fact rather than
// passing on an empty enumeration — "we could not look" must never render as
// "there is nothing there".
func TestEveryEgressGoesThroughTheGateway(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not read the package directory to enumerate egress: %v", err)
	}
	scanned := 0
	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := os.ReadFile(filepath.Clean(name))
		if readErr != nil {
			t.Fatalf("could not read %s while enumerating egress: %v", name, readErr)
		}
		scanned++
		if _, exempt := gatewayExemptFiles[name]; exempt {
			continue
		}
		for i, line := range strings.Split(string(source), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(line, ".Do(") || strings.Contains(line, "http.NewRequest") {
				offenders = append(offenders, name+":"+itoa(i+1)+": "+trimmed)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("enumerated 0 Go files — the guard did not actually run, which is not the same as finding nothing")
	}
	if len(offenders) > 0 {
		t.Fatalf("HTTP egress outside the fetch gateway (%d site(s)); route these through gatewayFetch/gatewayAttempt, or justify an entry in gatewayExemptFiles:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
	t.Logf("egress chokepoint holds: %d source file(s) scanned, %d exempt", scanned, len(gatewayExemptFiles))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
