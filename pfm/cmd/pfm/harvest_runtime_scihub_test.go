package main

import (
	"testing"

	pfmconfig "hostops/pfm/internal/config"
)

func TestHarvestRuntimeCarriesConfiguredScholarlyProviders(t *testing.T) {
	home := t.TempDir()
	config := pfmconfig.Defaults(home, nil)
	config.Harvester.Cache.Dir = home + "/cache"
	config.Harvester.Scholarly.SciHubURL = "https://mirror.example/scihub"
	config.Harvester.Scholarly.AnnasURL = "https://annas.example"
	config.Harvester.Scholarly.SciDBURL = "https://scidb.example"
	config.Harvester.Scholarly.LibGenURL = "https://libgen.example"
	config.Harvester.Scholarly.GoogleScholarURL = "https://scholar.example"
	config.Harvester.Scholarly.ContactEmail = "ops@example.com"
	runtime := harvestRuntime(commandRuntime{Config: config})

	for name, values := range map[string]struct{ got, want string }{
		"SciHubURL":        {runtime.SciHubURL, "https://mirror.example/scihub"},
		"AnnasURL":         {runtime.AnnasURL, "https://annas.example"},
		"SciDBURL":         {runtime.SciDBURL, "https://scidb.example"},
		"LibGenURL":        {runtime.LibGenURL, "https://libgen.example"},
		"GoogleScholarURL": {runtime.GoogleScholarURL, "https://scholar.example"},
	} {
		if values.got != values.want {
			t.Errorf("runtime %s = %q, want %q", name, values.got, values.want)
		}
	}
	if runtime.ContactEmail != "ops@example.com" {
		t.Fatalf("runtime ContactEmail = %q, want sibling scholarly setting preserved", runtime.ContactEmail)
	}
}
