package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationSplitsRenamesAndMovesPort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "pfm")
	legacy := filepath.Join(dir, LegacyFileName)
	writeFile(t, legacy, `{"version":2,"theme":"tokyo-night","mcp":{"http":{"port":8377},"servers":{"chat":{"enabled":true},"harvester":{"enabled":true}}}}`, 0o600)
	before, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(before)
	if err != nil {
		t.Fatal(err)
	}
	if migration.Empty() || len(migration.Steps()) != 3 {
		t.Fatalf("plan = %+v steps=%q, want rename + harvester move + port", migration, migration.Steps())
	}
	if preview := migration.Preview(before); preview.MCP.HTTP.Port != DefaultMCPPort || preview.Path != legacy {
		t.Fatalf("preview port=%d path=%q", preview.MCP.HTTP.Port, preview.Path)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("pre-split file still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyBackupName)); err != nil {
		t.Fatalf("pre-split backup missing: %v", err)
	}
	after, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.Path != filepath.Join(dir, FileName) || after.Theme != "tokyo-night" || after.MCP.HTTP.Port != DefaultMCPPort ||
		!after.MCPServers["chat"].Enabled || !after.Harvester.Enabled || after.MCPServerSource("harvester") != SourceFile {
		t.Fatalf("after migration: path=%q theme=%q port=%d chat=%t harvester=%t source=%q", after.Path, after.Theme,
			after.MCP.HTTP.Port, after.MCPServers["chat"].Enabled, after.Harvester.Enabled, after.MCPServerSource("harvester"))
	}
	content, err := os.ReadFile(after.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "harvester") {
		t.Fatalf("pfm.config.json still carries the harvester key:\n%s", content)
	}
	again, err := PlanMigration(after)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Empty() {
		t.Fatalf("second plan = %q, want empty", again.Steps())
	}
}

func TestMigrationKeepsCustomPortAndExistingHarvesterFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	writeFile(t, path, `{"version":2,"mcp":{"http":{"port":9999},"servers":{"harvester":{"enabled":true}}}}`, 0o600)
	writeFile(t, filepath.Join(dir, HarvesterFileName), `{"enabled":false,"search":{"searxngURL":"http://127.0.0.1:8888"}}`, 0o600)
	before, err := Load(path, filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(before)
	if err != nil {
		t.Fatal(err)
	}
	if migration.MovePort || migration.LegacyPath != "" || migration.HarvesterEnabled == nil {
		t.Fatalf("plan = %+v", migration)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	after, err := Load(path, filepath.Join(dir, "home"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.MCP.HTTP.Port != 9999 || after.Harvester.Enabled || after.Harvester.Search.SearXNGURL != "http://127.0.0.1:8888" {
		t.Fatalf("after: port=%d enabled=%t searxng=%q", after.MCP.HTTP.Port, after.Harvester.Enabled, after.Harvester.Search.SearXNGURL)
	}
}

// A migration interrupted between writing pfm.config.json and parking
// config.json leaves both files; the next plan must park the leftover, never
// treat the machine as already migrated.
func TestInterruptedMigrationLeftoverIsParkedNotIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "pfm")
	migrated := `{"version":2,"mcp":{"http":{"port":18377}}}` + "\n"
	writeFile(t, filepath.Join(dir, FileName), migrated, 0o600)
	stray := filepath.Join(dir, LegacyFileName)
	writeFile(t, stray, `{"version":2,"mcp":{"http":{"port":8377},"servers":{"harvester":{"enabled":true}}}}`, 0o600)
	loaded, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if migration.Empty() || migration.StrayLegacyPath != stray {
		t.Fatalf("plan = %+v, want the leftover %s parked", migration, stray)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Fatalf("leftover still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, legacyBackupName)); err != nil {
		t.Fatalf("leftover not parked: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(dir, FileName)); err != nil || string(content) != migrated {
		t.Fatalf("parking rewrote the migrated config: %q %v", content, err)
	}
}

// Moving the loopback port onto a port the external gateway already holds
// would leave a config that refuses to load; the plan keeps the old port.
func TestMigrationKeepsPortWhenExternalGatewayHoldsTheTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "pfm")
	writeFile(t, filepath.Join(dir, LegacyFileName), `{"version":2,"mcp":{"http":{"port":8377}}}`, 0o600)
	writeFile(t, filepath.Join(dir, HarvesterFileName),
		`{"enabled":true,"external":{"enabled":true,"port":18377,"publicURL":"https://h.example.test","auth":{"staticToken":"t"}}}`, 0o600)
	before, err := Load("", home, nil)
	if err != nil {
		t.Fatal(err)
	}
	migration, err := PlanMigration(before)
	if err != nil {
		t.Fatal(err)
	}
	if migration.MovePort || migration.PortKept == "" {
		t.Fatalf("plan = %+v, want the port kept with a reason", migration)
	}
	if err := ApplyMigration(migration); err != nil {
		t.Fatal(err)
	}
	after, err := Load("", home, nil)
	if err != nil {
		t.Fatalf("migrated config no longer loads: %v", err)
	}
	if after.MCP.HTTP.Port != legacyDefaultMCPPort || after.Path != filepath.Join(dir, FileName) {
		t.Fatalf("after: port=%d path=%q", after.MCP.HTTP.Port, after.Path)
	}
}
