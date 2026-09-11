package config

import (
	"reflect"
	"testing"
)

// TestAccountProjectionsFollowTheRoster pins every per-engine projection of the
// roster, and the zero answers with an engine that has no accounts.
func TestAccountProjectionsFollowTheRoster(t *testing.T) {
	machine := Config{
		Accounts:         []Account{{ID: 1, Emoji: "🥇"}, {ID: 2, Emoji: "·"}},
		CodexAccounts:    []CodexAccount{{ID: 3, Home: "/c/codex-3", Emoji: "🟢"}, {ID: 4, Home: "/c/codex-4"}},
		OpencodeAccounts: []OpenCodeAccount{{ID: 5}},
	}
	if got := machine.CodexHomes(); !reflect.DeepEqual(got, []string{"/c/codex-3", "/c/codex-4"}) {
		t.Errorf("CodexHomes() = %v", got)
	}
	if got := machine.PrimaryCodexAccount(); got != 3 {
		t.Errorf("PrimaryCodexAccount() = %d", got)
	}
	if got := machine.OpencodeAccountIDs(); !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("OpencodeAccountIDs() = %v", got)
	}
	if got := machine.PrimaryOpencodeAccount(); got != 5 {
		t.Errorf("PrimaryOpencodeAccount() = %d", got)
	}
	emojis := machine.AccountEmojis()
	if len(emojis) != 2 || emojis[1] != machine.EmojiFor(1) || emojis[2] != machine.EmojiFor(2) {
		t.Errorf("AccountEmojis() = %v", emojis)
	}
	if got := machine.CodexAccountEmojis(); len(got) != 2 || got[3] != machine.CodexEmojiFor(3) {
		t.Errorf("CodexAccountEmojis() = %v", got)
	}
	for _, emoji := range machine.LabelEmojis() {
		if emoji == "" || emoji == "·" {
			t.Errorf("LabelEmojis() carries a non-label %q", emoji)
		}
	}
	var empty Config
	if empty.PrimaryCodexAccount() != 0 || empty.PrimaryOpencodeAccount() != 0 || len(empty.CodexHomes()) != 0 {
		t.Error("an engine with no accounts must project to zero values")
	}
}
