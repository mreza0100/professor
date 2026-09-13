package config

import pfmengine "hostops/pfm/internal/engine"

// Account projections: the per-engine views of the roster that the fleet scan,
// the picker and the runtime loader each need. They live here, beside the
// roster itself, so no caller re-derives them.

// CodexHomes lists each Codex account's home directory, in roster order.
func (config Config) CodexHomes() []string {
	result := make([]string, 0, len(config.CodexAccounts))
	for _, account := range config.CodexAccounts {
		result = append(result, account.Home)
	}
	return result
}

// AccountEmojis maps every Claude account id to its emoji.
func (config Config) AccountEmojis() map[int]string {
	result := make(map[int]string, len(config.Accounts))
	for _, account := range config.Accounts {
		result[account.ID] = config.EmojiFor(account.ID)
	}
	return result
}

// CodexAccountEmojis maps every Codex account id to its emoji.
func (config Config) CodexAccountEmojis() map[int]string {
	result := make(map[int]string, len(config.CodexAccounts))
	for _, account := range config.CodexAccounts {
		result[account.ID] = config.CodexEmojiFor(account.ID)
	}
	return result
}

// LabelEmojis lists the Claude account emojis that can label a chat's tmux
// window — every configured emoji except the empty one and the "·" filler.
func (config Config) LabelEmojis() []string {
	result := make([]string, 0, len(config.Accounts))
	for _, account := range config.Accounts {
		if emoji := config.EmojiFor(account.ID); emoji != "" && emoji != "·" {
			result = append(result, emoji)
		}
	}
	return result
}

// PrimaryCodexAccount is the first Codex account's id, or 0 with none.
func (config Config) PrimaryCodexAccount() int {
	if len(config.CodexAccounts) == 0 {
		return 0
	}
	return config.CodexAccounts[0].ID
}

// PrimaryAccountFor is the account a row of engine opens on: Codex and
// OpenCode rows take their own roster's primary, and only a Claude row takes
// claudePrimary — the account pfm's primary-set picked, which names a Claude
// account and means nothing to another engine's roster.
func (config Config) PrimaryAccountFor(engine pfmengine.ID, claudePrimary int) int {
	switch engine {
	case pfmengine.Codex:
		return config.PrimaryCodexAccount()
	case pfmengine.Opencode:
		return config.PrimaryOpencodeAccount()
	default:
		return claudePrimary
	}
}

// OpencodeAccountIDs lists every OpenCode account id, in roster order.
func (config Config) OpencodeAccountIDs() []int {
	result := make([]int, 0, len(config.OpencodeAccounts))
	for _, account := range config.OpencodeAccounts {
		result = append(result, account.ID)
	}
	return result
}

// PrimaryOpencodeAccount is the first OpenCode account's id, or 0 with none.
func (config Config) PrimaryOpencodeAccount() int {
	if len(config.OpencodeAccounts) == 0 {
		return 0
	}
	return config.OpencodeAccounts[0].ID
}
