package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const ct084OriginalEvidenceSHA256 = "F26E011815E358D6DA2C894F707AD3A43E363D1274C9C1379CB39AE1F673BBED"

type ct084OriginalEvidence struct {
	SchemaVersion    int                         `json:"schemaVersion"`
	SourceVersion    string                      `json:"sourceVersion"`
	SourceSHA256     string                      `json:"sourceSha256"`
	ExecutableSHA256 string                      `json:"executableSha256"`
	ExecutableSize   int64                       `json:"executableSize"`
	Sites            []ct084OriginalEvidenceSite `json:"sites"`
}

type ct084OriginalEvidenceSite struct {
	FeatureID             string `json:"featureId"`
	CTID                  int    `json:"ctId"`
	SiteIndex             int    `json:"siteIndex"`
	Symbol                string `json:"symbol"`
	AOB                   string `json:"aob"`
	Offset                int    `json:"offset"`
	PatchRVA              uint32 `json:"patchRva"`
	ExpectedOriginalBytes []byte `json:"expectedOriginalBytes"`
}

func readCT084OriginalEvidence(t *testing.T) ct084OriginalEvidence {
	t.Helper()
	file, err := os.Open("data/ct084_originals_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var evidence ct084OriginalEvidence
	if err := decoder.Decode(&evidence); err != nil {
		t.Fatal(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("evidence has trailing JSON: %v", err)
	}
	return evidence
}

func TestCT084OriginalEvidenceIsLockedAndCoversEveryCatalogSite(t *testing.T) {
	raw, err := os.ReadFile("data/ct084_originals_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%X", sha256.Sum256(raw)); got != ct084OriginalEvidenceSHA256 {
		t.Fatalf("original evidence SHA256=%s, want %s", got, ct084OriginalEvidenceSHA256)
	}
	evidence := readCT084OriginalEvidence(t)
	if evidence.SchemaVersion != 1 || evidence.SourceVersion != ct084CatalogSourceVersion || evidence.SourceSHA256 != ct084CatalogSourceSHA256 {
		t.Fatalf("original evidence source identity=%+v", evidence)
	}
	if evidence.ExecutableSHA256 != ct084LocalGame202SHA256 || evidence.ExecutableSize != ct084LocalGame202Size {
		t.Fatalf("original evidence executable identity=%s/%d", evidence.ExecutableSHA256, evidence.ExecutableSize)
	}
	if len(evidence.Sites) != 81 {
		t.Fatalf("original evidence sites=%d, want 81", len(evidence.Sites))
	}

	byKey := make(map[[2]int]ct084OriginalEvidenceSite, len(evidence.Sites))
	for _, site := range evidence.Sites {
		key := [2]int{site.CTID, site.SiteIndex}
		if _, duplicate := byKey[key]; duplicate {
			t.Fatalf("duplicate original evidence for CT %d site %d", site.CTID, site.SiteIndex)
		}
		if site.FeatureID != fmt.Sprintf("ct084-%d", site.CTID) || strings.TrimSpace(site.Symbol) == "" || strings.TrimSpace(site.AOB) == "" || site.PatchRVA == 0 || len(site.ExpectedOriginalBytes) == 0 {
			t.Fatalf("invalid original evidence site=%+v", site)
		}
		byKey[key] = site
	}

	catalog := readCT084CatalogFile(t)
	seen := 0
	for _, feature := range catalog.Features {
		for siteIndex, site := range feature.Sites {
			key := [2]int{feature.CTID, siteIndex}
			locked, exists := byKey[key]
			if !exists {
				t.Fatalf("catalog %s site %d has no original evidence", feature.ID, siteIndex)
			}
			if locked.FeatureID != feature.ID || locked.Symbol != site.Symbol || locked.AOB != site.AOB || locked.Offset != site.Offset {
				t.Fatalf("catalog %s site %d identity differs from original evidence", feature.ID, siteIndex)
			}
			if !bytes.Equal(locked.ExpectedOriginalBytes, site.ExpectedOriginalBytes) {
				t.Fatalf("catalog %s site %d expected original=% X, evidence=% X", feature.ID, siteIndex, site.ExpectedOriginalBytes, locked.ExpectedOriginalBytes)
			}
			seen++
		}
	}
	if seen != len(byKey) {
		t.Fatalf("catalog used %d/%d original evidence records", seen, len(byKey))
	}
}

func TestCT084GeneratorReproducesLockedProductionCatalog(t *testing.T) {
	sourcePath := os.Getenv("GBFR_CT084_SOURCE_TEST")
	if sourcePath == "" {
		t.Skip("set GBFR_CT084_SOURCE_TEST to verify production catalog regeneration")
	}
	powerShell := ""
	for _, candidate := range []string{"powershell", "pwsh"} {
		if path, err := exec.LookPath(candidate); err == nil {
			powerShell = path
			break
		}
	}
	if powerShell == "" {
		t.Skip("PowerShell is not installed")
	}
	tempOutput := filepath.Join(t.TempDir(), "ct084_patches.json")
	scriptPath, err := filepath.Abs("tools/generate_ct084_patches.ps1")
	if err != nil {
		t.Fatal(err)
	}
	evidencePath, err := filepath.Abs("data/ct084_originals_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(powerShell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath, "-InputCT", sourcePath, "-OriginalEvidence", evidencePath, "-Output", tempOutput)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("regenerate production CT084 catalog: %v\n%s", err, output)
	}
	got, err := os.ReadFile(tempOutput)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("data/ct084_patches.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("locked CT source + original evidence did not reproduce data/ct084_patches.json byte-for-byte")
	}
}

func TestCT084OriginalEvidenceRegeneratesFromLockedGame202(t *testing.T) {
	executablePath := os.Getenv("GBFR_GAME_EXE_TEST")
	if executablePath == "" {
		t.Skip("set GBFR_GAME_EXE_TEST to verify original evidence regeneration")
	}
	tempOutput := filepath.Join(t.TempDir(), "ct084_originals_2.0.2.json")
	command := exec.Command("go", "run", "./tools/ct084_originals", "-input-catalog", "data/ct084_patches.json", "-input-exe", executablePath, "-output", tempOutput)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("regenerate CT084 original evidence: %v\n%s", err, output)
	}
	got, err := os.ReadFile(tempOutput)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("data/ct084_originals_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("locked catalog + game 2.0.2 executable did not reproduce original-byte evidence byte-for-byte")
	}
}
