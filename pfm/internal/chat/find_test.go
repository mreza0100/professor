package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"hostops/pfm/internal/testjail"
)

// TestExcerptNeedlesStripsDecorationAndKeepsTheFiveLongest pins the needle
// rules: quote/list/heading decoration stripped, 20+ characters only, the five
// longest, longest first — and a short literal (a chat name) searched whole.
func TestExcerptNeedlesStripsDecorationAndKeepsTheFiveLongest(t *testing.T) {
	excerpt := strings.Join([]string{
		"# short",
		"> aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\r",
		"- bbbbbbbbbbbbbbbbbbbbbbbbb",
		"* cccccccccccccccccccccc   ",
		"ddddddddddddddddddddd",
		"eeeeeeeeeeeeeeeeeeeee",
		"fffffffffffffffffffff",
		"nineteen characters",
	}, "\n")
	want := []string{
		strings.Repeat("a", 30), strings.Repeat("b", 25), strings.Repeat("c", 22),
		strings.Repeat("d", 21), strings.Repeat("e", 21),
	}
	if needles := ExcerptNeedles(excerpt); !reflect.DeepEqual(needles, want) {
		t.Fatalf("ExcerptNeedles() = %q, want %q", needles, want)
	}
	if got := ExcerptNeedles("  LUNA:ORCHESTRATOR \n"); !reflect.DeepEqual(got, []string{"LUNA:ORCHESTRATOR"}) {
		t.Fatalf("short literal needles = %q, want the literal itself", got)
	}
	if got := ExcerptNeedles(" \n\t"); len(got) != 0 {
		t.Fatalf("blank excerpt needles = %q, want none", got)
	}
}

// TestFindRanksByHitsAndNamesEachEmptyAnswer pins the shared search: every
// match, most needles first — and each way of finding nothing is its own
// error, so no surface reads "could not search" as "not there".
func TestFindRanksByHitsAndNamesEachEmptyAnswer(t *testing.T) {
	root := testjail.Fleet(t)
	ctx := context.Background()
	if _, err := Find(ctx, nil, FindRequest{Excerpt: "the verb layer answers every surface"}); !errors.Is(err, ErrNoTranscriptRegistry) {
		t.Fatalf("Find over an empty registry = %v, want ErrNoTranscriptRegistry", err)
	}
	const first, second = "the verb layer answers every surface", "one implementation behind each tool"
	directory := filepath.Join(root, "claude", "find")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"both.jsonl": `{"type":"user","timestamp":"2026-01-01T00:00:00Z","message":{"content":"` + first + `"}}` + "\n" +
			`{"type":"user","timestamp":"2026-01-02T00:00:00Z","message":{"content":"` + second + `"}}` + "\n",
		"one.jsonl":     `{"type":"user","message":{"content":"` + second + `"}}` + "\n",
		"neither.jsonl": `{"type":"user","message":{"content":"unrelated"}}` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	matches, err := Find(ctx, nil, FindRequest{Excerpt: first + "\n" + second})
	if err != nil || len(matches) != 2 {
		t.Fatalf("Find = %+v, %v; want the two transcripts that hold a needle", matches, err)
	}
	if best := matches[0]; best.ID != "both" || best.Hits != 2 || best.Needles != 2 ||
		best.First != "2026-01-01T00:00:00Z" || best.Last != "2026-01-02T00:00:00Z" {
		t.Fatalf("best match = %+v, want both needles and its timestamp range", best)
	}
	if matches, err := Find(ctx, nil, FindRequest{Excerpt: first + "\n" + second, Self: "both"}); err != nil ||
		len(matches) != 1 || matches[0].ID != "one" {
		t.Fatalf("Find(Self: both) = %+v, %v; want the asking session's transcript left out", matches, err)
	}
	if _, err := Find(ctx, nil, FindRequest{Excerpt: "a sentence no transcript here holds"}); !errors.Is(err, ErrNoExcerptMatch) {
		t.Fatalf("Find(absent) = %v, want ErrNoExcerptMatch", err)
	}
	if _, err := Find(ctx, nil, FindRequest{Excerpt: "  "}); !errors.Is(err, ErrNoExcerpt) {
		t.Fatalf("Find(blank) = %v, want ErrNoExcerpt", err)
	}
}
