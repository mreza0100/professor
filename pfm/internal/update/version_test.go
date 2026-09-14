package update

import (
	"reflect"
	"testing"
)

func TestSelectHighestSemverUsesParsedComponents(t *testing.T) {
	got, err := SelectHighest([]string{"v0.9.0", "v0.10.0", "v0.10.0-rc1", "notes"})
	if err != nil {
		t.Fatalf("SelectHighest() error = %v", err)
	}
	if got != "v0.10.0" {
		t.Fatalf("SelectHighest() = %q, want v0.10.0", got)
	}
}

// TestReleaseNotesReturnsOldestFirstExcludingPreviousAndAboveTarget pins the
// filtering window: strictly newer than previousTag, no newer than target,
// oldest first — a release AT previousTag never repeats, and a release past
// target (already staged for a later update) never leaks in early.
func TestReleaseNotesReturnsOldestFirstExcludingPreviousAndAboveTarget(t *testing.T) {
	files := []string{
		"releases/v0.75.0.md", // at previous — excluded
		"releases/v0.77.0.md",
		"releases/v0.76.0.md",
		"releases/v0.78.0.md", // above target — excluded
		"CHANGELOG.md",        // not a release note — ignored
	}
	got, err := ReleaseNotes("v0.75.0", "v0.77.0", files)
	if err != nil {
		t.Fatalf("ReleaseNotes() error = %v", err)
	}
	want := []string{"releases/v0.76.0.md", "releases/v0.77.0.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReleaseNotes() = %v, want %v", got, want)
	}
}

// TestReleaseNotesNoneBetweenIsEmptyNotAnError pins the absence branch: a
// previous and target one patch apart with no releases/ file strictly
// between them returns an empty, error-free result — never conflated with
// the unparseable-tag error branch below.
func TestReleaseNotesNoneBetweenIsEmptyNotAnError(t *testing.T) {
	got, err := ReleaseNotes("v0.77.0", "v0.77.1", []string{"releases/v0.77.0.md"})
	if err != nil {
		t.Fatalf("ReleaseNotes() error = %v, want none", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReleaseNotes() = %v, want empty", got)
	}
}

// TestReleaseNotesUnparseableTagIsAnError pins the error branch for either
// side: a previous or target tag that does not parse as vMAJOR.MINOR.PATCH
// must fail loudly rather than silently report "none between" — an error is
// never absence.
func TestReleaseNotesUnparseableTagIsAnError(t *testing.T) {
	if _, err := ReleaseNotes("not-a-tag", "v0.77.0", []string{"releases/v0.76.0.md"}); err == nil {
		t.Fatal("ReleaseNotes() with an unparseable previous tag: error = nil, want an error")
	}
	if _, err := ReleaseNotes("v0.76.0", "not-a-tag", []string{"releases/v0.76.0.md"}); err == nil {
		t.Fatal("ReleaseNotes() with an unparseable target tag: error = nil, want an error")
	}
}
