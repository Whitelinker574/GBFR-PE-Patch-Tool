package backend

import "testing"

func TestStableReleaseKeepsUnverifiedRuntimeCandidatesDisabled(t *testing.T) {
	if stableReleaseCandidateWriteEnabled {
		t.Fatal("generic runtime candidates are writable in the stable release")
	}
	if stableReleaseCombatTuningWriteEnabled {
		t.Fatal("combat tuning candidates are writable in the stable release")
	}
	if stableReleaseVirtualSigilWriteEnabled {
		t.Fatal("virtual sigils are writable in the stable release")
	}

	catalog, err := loadRuntimePatchCatalog()
	if err != nil {
		t.Fatal(err)
	}
	candidate := findRuntimePatchCatalogFeature(catalog, "runtime-patch-059")
	if candidate == nil || runtimePatchFeatureAvailableInStableRelease(*candidate) {
		t.Fatalf("candidate runtime patch is available in the stable release: %+v", candidate)
	}
	verified := findRuntimePatchCatalogFeature(catalog, "runtime-patch-001")
	if verified == nil || !runtimePatchFeatureAvailableInStableRelease(*verified) {
		t.Fatalf("verified runtime patch was disabled by the candidate gate: %+v", verified)
	}
}
