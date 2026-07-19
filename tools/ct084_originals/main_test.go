package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLockCT084OriginalsExtractsExactWildcardSlice(t *testing.T) {
	catalog := inputCatalog{
		SourceVersion: "0.8.4",
		SourceSHA256:  strings.Repeat("A", 64),
		Features: []inputFeature{{
			ID:   "ct084-900001",
			CTID: 900001,
			Sites: []inputSite{{
				Symbol:      "SITE",
				AOB:         "AA BB ?? DD EE",
				Offset:      2,
				EnableBytes: []byte{0x90},
			}},
		}},
	}
	sections := []executableSection{{
		Name: ".text",
		RVA:  0x1000,
		Data: []byte{0x00, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x00},
	}}

	manifest, err := lockOriginals(catalog, sections, executableIdentity{
		SHA256: strings.Repeat("B", 64),
		Size:   123,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Sites) != 1 {
		t.Fatalf("sites=%d, want 1", len(manifest.Sites))
	}
	site := manifest.Sites[0]
	if site.CTID != 900001 || site.SiteIndex != 0 || site.Symbol != "SITE" || site.PatchRVA != 0x1003 {
		t.Fatalf("locked site identity=%+v", site)
	}
	if !bytes.Equal(site.ExpectedOriginalBytes, []byte{0xCC}) {
		t.Fatalf("expected original=% X, want CC", site.ExpectedOriginalBytes)
	}
}

func TestLockCT084OriginalsRejectsAmbiguousSignature(t *testing.T) {
	catalog := inputCatalog{
		SourceVersion: "0.8.4",
		SourceSHA256:  strings.Repeat("A", 64),
		Features: []inputFeature{{
			ID:    "ct084-900002",
			CTID:  900002,
			Sites: []inputSite{{Symbol: "DUP", AOB: "AA ?? CC", Offset: 1, EnableBytes: []byte{0x90}}},
		}},
	}
	sections := []executableSection{{Name: ".text", RVA: 0x2000, Data: []byte{0xAA, 1, 0xCC, 0xAA, 2, 0xCC}}}

	if _, err := lockOriginals(catalog, sections, executableIdentity{}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "matches") {
		t.Fatalf("lockOriginals() error=%v, want ambiguous-match rejection", err)
	}
}

func TestLockCT084OriginalsRejectsAlreadyEnabledImageBytes(t *testing.T) {
	catalog := inputCatalog{
		SourceVersion: "0.8.4",
		SourceSHA256:  strings.Repeat("A", 64),
		Features: []inputFeature{{
			ID:    "ct084-900003",
			CTID:  900003,
			Sites: []inputSite{{Symbol: "PATCHED", AOB: "AA ?? CC", Offset: 1, EnableBytes: []byte{0x90}}},
		}},
	}
	sections := []executableSection{{Name: ".text", RVA: 0x3000, Data: []byte{0xAA, 0x90, 0xCC}}}

	if _, err := lockOriginals(catalog, sections, executableIdentity{}); err == nil || !strings.Contains(strings.ToLower(err.Error()), "enable") {
		t.Fatalf("lockOriginals() error=%v, want already-enabled rejection", err)
	}
}

func TestOriginalEvidenceEncodesBytesAsAuditableJSONArray(t *testing.T) {
	raw, err := json.Marshal(originalSiteLock{ExpectedOriginalBytes: byteArray{0xCC, 0x8B}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"expectedOriginalBytes":[204,139]`)) {
		t.Fatalf("evidence JSON=%s, want numeric byte array", raw)
	}
}
