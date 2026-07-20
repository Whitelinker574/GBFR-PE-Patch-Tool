package main

import "testing"

func TestInfiniteChallengeUsesTheV185SignatureAndThreeByteInstruction(t *testing.T) {
	wantPattern := []byte{0x41, 0xFF, 0, 0xB2, 0, 0xE9}
	wantMask := []bool{true, true, false, true, false, true}
	wantOrig := []byte{0x41, 0xFF, 0xC0}
	if !bytesEqual(infiniteChallengePattern, wantPattern) || len(infiniteChallengeMask) != len(wantMask) {
		t.Fatalf("continuous-challenge signature is stale: % X / %v", infiniteChallengePattern, infiniteChallengeMask)
	}
	for index := range wantMask {
		if infiniteChallengeMask[index] != wantMask[index] {
			t.Fatalf("continuous-challenge mask[%d]=%v, want %v", index, infiniteChallengeMask[index], wantMask[index])
		}
	}
	if !bytesEqual(infiniteChallengeOrig, wantOrig) || len(infiniteChallengePatch) != 3 {
		t.Fatalf("continuous-challenge instruction is stale: % X -> % X", infiniteChallengeOrig, infiniteChallengePatch)
	}
}

func TestInfiniteChallengeOwnedSurfaceUsesTheRuntimeLeaseAndDetachRecovery(t *testing.T) {
	bodies := currencyAppFunctionBodies(t)
	for _, name := range []string{"InfiniteChallengeGetStatusOwned", "InfiniteChallengeSetEnabledOwned"} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
			t.Fatalf("%s must pin and verify the current page owner", name)
		}
	}
	setter := bodies["InfiniteChallengeSetEnabledOwned"]
	if !blockCallsSelector(setter, "liveMemoryWriteMu", "Lock") {
		t.Fatal("owned continuous-challenge writes must join the global live-memory transaction")
	}
	for _, name := range []string{"CharaRelease", "charaDetachLocked"} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "a", "restoreInfiniteChallengeOwnedLocked") {
			t.Fatalf("%s must restore the owned continuous-challenge patch before releasing the process", name)
		}
	}
}
