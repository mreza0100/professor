package harvestpy

import (
	"slices"
	"testing"
)

// The converter's HARVESTER_PDF_* protocol variables come only from the
// configured Runtime; a stray value in the pfm process environment is
// stripped so harvester.config.json stays the single source.
func TestWorkerEnvCarriesOnlyConfiguredConverterFlags(t *testing.T) {
	parent := []string{"PATH=/usr/bin", "HARVESTER_PDF_OCR=1", "HARVESTER_PDF_LAYOUT=true", "HOME=/home/fixture"}
	off := workerEnv(parent, Runtime{})
	if slices.Contains(off, "HARVESTER_PDF_OCR=1") || slices.Contains(off, "HARVESTER_PDF_LAYOUT=true") {
		t.Fatalf("inherited converter flags leaked into the worker: %q", off)
	}
	if !slices.Contains(off, "PATH=/usr/bin") || !slices.Contains(off, "HOME=/home/fixture") {
		t.Fatalf("unrelated environment dropped: %q", off)
	}
	on := workerEnv([]string{"PATH=/usr/bin"}, Runtime{PDFOCR: true, PDFLayout: true})
	if !slices.Contains(on, "HARVESTER_PDF_OCR=1") || !slices.Contains(on, "HARVESTER_PDF_LAYOUT=1") {
		t.Fatalf("configured converter flags missing: %q", on)
	}
}
