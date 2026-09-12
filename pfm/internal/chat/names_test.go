package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hostops/pfm/internal/compose"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/inject"
	"hostops/pfm/internal/paths"
	"hostops/pfm/internal/resolve"
	"hostops/pfm/internal/testjail"
)

// TestSeatTargetKeepsACodexScopedLookupOnCodex pins the requiredEngine
// narrowing (chat_resolve cxwin, inject --engine): two live seats share a name
// across engines, and a Codex-scoped lookup lands on the Codex seat, addressed
// under the jailed tmux directory.
func TestSeatTargetKeepsACodexScopedLookupOnCodex(t *testing.T) {
	testjail.Fleet(t)
	const name = "same name"
	rows := []compose.Row{
		{ID: "claude-id", Name: name, Kind: compose.LiveClaude, Socket: "cc-1-2-3", PaneID: "%1"},
		{ID: "codex-id", Name: name, Kind: compose.LiveCodex, Socket: "cx-1-2-3", PaneID: "%2"},
	}
	target, code, detail, err := seatTarget(liveSeats(rows, string(pfmengine.Codex)), name)
	resolved, resolveErr := paths.Resolve()
	if resolveErr != nil {
		t.Fatal(resolveErr)
	}
	if err != nil || code != 0 || detail != "" || target.ID != "codex-id" ||
		target.SocketPath != filepath.Join(resolved.TmuxDir, "cx-1-2-3") || target.Pane != "%2" {
		t.Fatalf("seatTarget(codex)=(%+v,%d,%q,%v), want the Codex seat", target, code, detail, err)
	}
	if _, code, detail, err := seatTarget(liveSeats(rows, ""), name); err != nil || code != inject.CodeAmbiguous || detail == "" {
		t.Fatalf("seatTarget(any engine)=(%d,%q,%v), want the collision refused as ambiguous", code, detail, err)
	}
}

// TestLiveSeatsNeverAnswerForAKilledOrResumableRow is the reverse lookup the
// delivery footer is built from: the caller's live seat answers with its name,
// a killed seat never does — its tombstone must not collide with the chat that
// took its name — and a resumable row has no seat to answer from.
func TestLiveSeatsNeverAnswerForAKilledOrResumableRow(t *testing.T) {
	rows := []compose.Row{
		{ID: "5a3bb7cb-258d", Name: "LUNA:ORCHESTRATOR", Kind: compose.LiveClaude, Socket: "cc-1788256324-1866070-42739", PaneID: "%0"},
		{ID: "5a3bb7cb-258d", Name: "LUNA:ORCHESTRATOR (old)", Kind: compose.LiveClaude, Socket: "cc-dead", PaneID: "%0", Killed: true},
		{ID: "resume-only", Name: "Resume", Kind: compose.ResumeClaude, Socket: "cc-2"},
	}
	seats := RosterCandidates(liveSeats(rows, ""))
	if name, found := resolve.ResolveRosterSeat(seats, resolve.Identity{ID: "5a3bb7cb-258d"}); !found || name != "LUNA:ORCHESTRATOR" {
		t.Fatalf("sender name = (%q,%t), want the live seat's name", name, found)
	}
	if name, found := resolve.ResolveRosterSeat(seats, resolve.Identity{ID: "resume-only"}); found || name != "" {
		t.Fatalf("sender name (resumable) = (%q,%t), want not found", name, found)
	}
	if _, code, _, err := seatTarget(liveSeats(rows, ""), "LUNA:ORCHESTRATOR (old)"); err != nil || code != inject.CodeUnknown {
		t.Fatalf("killed seat by name = code %d err %v, want a roster miss", code, err)
	}
}

// TestNameResolverReportsAScanFailureNotAMiss pins the root law at the roster
// rung: a fleet that could not be read is an error with CodeUndelivered —
// never CodeUnknown, which would send inject on to guess from raw panes.
func TestNameResolverReportsAScanFailureNotAMiss(t *testing.T) {
	root := testjail.Fleet(t)
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(paths.EnvDB, filepath.Join(blocker, "index.db"))
	resolver := NameResolver{}
	if _, code, _, err := resolver.ResolveName(context.Background(), "anyone", ""); err == nil || code != inject.CodeUndelivered {
		t.Fatalf("ResolveName over an unreadable fleet = code %d err %v, want CodeUndelivered and the error", code, err)
	}
	if _, found, err := resolver.SenderName(context.Background(), resolve.Identity{ID: "x", Session: "cc-x"}); err == nil || found {
		t.Fatalf("SenderName over an unreadable fleet = found %t err %v, want the error", found, err)
	}
}
