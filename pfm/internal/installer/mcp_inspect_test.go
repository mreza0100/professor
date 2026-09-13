package installer

import (
	"path/filepath"
	"testing"
)

// A standalone harvester entry in the Claude user registry (~/.claude.json)
// is the legacy state the cutover exists to find; reading only the Codex side
// or a project .mcp.json would report this machine migrated when it is not.
func TestInspectHarvesterClientCutoverFlagsAStandaloneEntryInTheClaudeUserRegistry(t *testing.T) {
	home := t.TempDir()
	writeFixture(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"harvester":{"type":"stdio","command":"uv","args":["run","harvester"]}}}`)
	for _, report := range InspectHarvesterClientCutover(home, 8377, nil, nil) {
		if report.State == MCPClientLegacyStandalone {
			return
		}
	}
	t.Fatal("the Claude user registry's standalone harvester was reported migrated")
}
