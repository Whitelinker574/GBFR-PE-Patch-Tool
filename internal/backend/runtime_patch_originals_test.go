package backend

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

const runtimePatchOriginalEvidenceSHA256 = "DED21FDECF8657597BEF296E637128601A17728E8733CB5E90732AF0082825C2"

type runtimePatchOriginalEvidence struct {
	SchemaVersion    int                                `json:"schemaVersion"`
	GameVersion      string                             `json:"gameVersion"`
	ExecutableSHA256 string                             `json:"executableSha256"`
	ExecutableSize   int64                              `json:"executableSize"`
	Sites            []runtimePatchOriginalEvidenceSite `json:"sites"`
}

type runtimePatchOriginalEvidenceSite struct {
	FeatureID             string `json:"featureId"`
	CatalogID             int    `json:"catalogId"`
	SiteIndex             int    `json:"siteIndex"`
	Symbol                string `json:"symbol"`
	AOB                   string `json:"aob"`
	Offset                int    `json:"offset"`
	PatchRVA              uint32 `json:"patchRva"`
	ExpectedOriginalBytes []byte `json:"expectedOriginalBytes"`
}

func readRuntimePatchOriginalEvidence(t *testing.T) runtimePatchOriginalEvidence {
	t.Helper()
	file, err := os.Open("data/runtime_patch_originals_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var evidence runtimePatchOriginalEvidence
	if err := decoder.Decode(&evidence); err != nil {
		t.Fatal(err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("evidence has trailing JSON: %v", err)
	}
	return evidence
}

func TestRuntimePatchOriginalEvidenceIsLockedAndCoversCatalog(t *testing.T) {
	raw, err := os.ReadFile("data/runtime_patch_originals_2.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%X", sha256.Sum256(raw)); got != runtimePatchOriginalEvidenceSHA256 {
		t.Fatalf("original evidence SHA256=%s, want %s", got, runtimePatchOriginalEvidenceSHA256)
	}
	evidence := readRuntimePatchOriginalEvidence(t)
	if evidence.SchemaVersion != 1 || evidence.GameVersion != "2.0.2" || evidence.ExecutableSHA256 != runtimePatchLocalGame202SHA256 || evidence.ExecutableSize != runtimePatchLocalGame202Size {
		t.Fatalf("original evidence identity=%+v", evidence)
	}
	if len(evidence.Sites) != 81 {
		t.Fatalf("original evidence sites=%d, want 81", len(evidence.Sites))
	}

	byKey := make(map[[2]int]runtimePatchOriginalEvidenceSite, len(evidence.Sites))
	for _, site := range evidence.Sites {
		key := [2]int{site.CatalogID, site.SiteIndex}
		if _, duplicate := byKey[key]; duplicate {
			t.Fatalf("duplicate original evidence for catalog entry %d site %d", site.CatalogID, site.SiteIndex)
		}
		if site.FeatureID != fmt.Sprintf("runtime-patch-%03d", site.CatalogID) || strings.TrimSpace(site.Symbol) == "" || strings.TrimSpace(site.AOB) == "" || site.PatchRVA == 0 || len(site.ExpectedOriginalBytes) == 0 {
			t.Fatalf("invalid original evidence site=%+v", site)
		}
		byKey[key] = site
	}

	catalog := readRuntimePatchCatalogFile(t)
	seen := 0
	for _, feature := range catalog.Features {
		for siteIndex, site := range feature.Sites {
			locked, exists := byKey[[2]int{feature.CatalogID, siteIndex}]
			if !exists {
				t.Fatalf("catalog %s site %d has no original evidence", feature.ID, siteIndex)
			}
			if locked.FeatureID != feature.ID || locked.Symbol != site.Symbol || locked.AOB != site.AOB || locked.Offset != site.Offset || string(locked.ExpectedOriginalBytes) != string(site.ExpectedOriginalBytes) {
				t.Fatalf("catalog %s site %d differs from original evidence", feature.ID, siteIndex)
			}
			seen++
		}
	}
	if seen != len(evidence.Sites) {
		t.Fatalf("covered sites=%d, want %d", seen, len(evidence.Sites))
	}
}
