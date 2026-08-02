package backend

import "testing"

func TestRuntimeFeatureEntrypointsRemainAvailable(t *testing.T) {
	if !stableReleaseCandidateWriteEnabled {
		t.Fatal("generic runtime candidate entrypoints are locked")
	}
	if !stableReleaseCombatTuningWriteEnabled {
		t.Fatal("combat tuning entrypoints are locked")
	}
	if !stableReleaseVirtualSigilWriteEnabled {
		t.Fatal("virtual sigil entrypoint is locked")
	}
	if runtimeSpatialGravityStableReleaseEnabled {
		t.Fatal("the ModelImpl field setter must not be exposed as gravity suppression")
	}

	catalog, err := loadRuntimePatchCatalog()
	if err != nil {
		t.Fatal(err)
	}
	candidate := findRuntimePatchCatalogFeature(catalog, "runtime-patch-059")
	if candidate == nil || !runtimePatchFeatureAvailableInStableRelease(*candidate) {
		t.Fatalf("candidate runtime patch entrypoint is locked: %+v", candidate)
	}
	verified := findRuntimePatchCatalogFeature(catalog, "runtime-patch-001")
	if verified == nil || !runtimePatchFeatureAvailableInStableRelease(*verified) {
		t.Fatalf("verified runtime patch was disabled by the candidate gate: %+v", verified)
	}
}
