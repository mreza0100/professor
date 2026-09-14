// Package update parses release tags and derives the release-notes an update
// moved past; cmd/pfm keeps the git plumbing and calls in here for the pure logic.
package update

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Version is a parsed vMAJOR.MINOR.PATCH release tag.
type Version struct {
	major int
	minor int
	patch int
}

// Less reports whether version orders before other.
func (version Version) Less(other Version) bool {
	if version.major != other.major {
		return version.major < other.major
	}
	if version.minor != other.minor {
		return version.minor < other.minor
	}
	return version.patch < other.patch
}

// ParseVersion parses a vMAJOR.MINOR.PATCH tag; ok is false for anything else.
func ParseVersion(tag string) (Version, bool) {
	parts := strings.Split(strings.TrimSpace(tag), ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "v") {
		return Version{}, false
	}
	major, err := strconv.Atoi(strings.TrimPrefix(parts[0], "v"))
	if err != nil || major < 0 {
		return Version{}, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return Version{}, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil || patch < 0 {
		return Version{}, false
	}
	return Version{major: major, minor: minor, patch: patch}, true
}

// SelectHighest returns the highest vMAJOR.MINOR.PATCH tag among tags.
func SelectHighest(tags []string) (string, error) {
	ordered := append([]string(nil), tags...)
	sort.Strings(ordered)
	var selected string
	var selectedVersion Version
	for _, tag := range ordered {
		version, ok := ParseVersion(tag)
		if !ok {
			continue
		}
		if selected == "" || selectedVersion.Less(version) {
			selected, selectedVersion = tag, version
		}
	}
	if selected == "" {
		return "", errors.New("no semantic-version tags (expected vMAJOR.MINOR.PATCH)")
	}
	return selected, nil
}

// ReleaseNotes filters releaseFiles (the names `git ls-tree --name-only
// <target> releases/` lists at target) down to the releases/vX.Y.Z.md entries
// an update from previousTag to target moved past: version newer than
// previousTag and no newer than target, returned oldest first as paths
// relative to the repository root (e.g. "releases/v0.77.0.md"). An adopter
// several releases behind reads each one before adopting — their "→ For:"
// lines are the actions those releases ask for.
func ReleaseNotes(previousTag, target string, releaseFiles []string) ([]string, error) {
	previousVersion, ok := ParseVersion(previousTag)
	if !ok {
		return nil, fmt.Errorf("previous release tag %q does not parse as vMAJOR.MINOR.PATCH", previousTag)
	}
	targetVersion, ok := ParseVersion(target)
	if !ok {
		return nil, fmt.Errorf("target tag %q does not parse as vMAJOR.MINOR.PATCH", target)
	}
	type versionedNote struct {
		version Version
		base    string
	}
	var notes []versionedNote
	for _, line := range releaseFiles {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		base := filepath.Base(line)
		if !strings.HasSuffix(base, ".md") {
			continue
		}
		stem := strings.TrimSuffix(base, ".md")
		version, ok := ParseVersion(stem)
		if !ok {
			continue
		}
		if !previousVersion.Less(version) || targetVersion.Less(version) {
			continue
		}
		notes = append(notes, versionedNote{version: version, base: base})
	}
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].version.Less(notes[j].version)
	})
	paths := make([]string, 0, len(notes))
	for _, note := range notes {
		paths = append(paths, filepath.Join("releases", note.base))
	}
	return paths, nil
}
