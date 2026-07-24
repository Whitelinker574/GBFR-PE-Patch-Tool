package backend

import "testing"

func TestLegacyRuntimeEntrypointsAcquireStableProcessLease(t *testing.T) {
	bodies := currencyAppFunctionBodies(t)
	for _, name := range []string{
		"CharaGetAll", "CharaSetOne", "CharaSetAll",
		"FaceAccessoryScan", "FaceAccessoryGetStatus", "FaceAccessorySetHidden",
		"OtherSkinPurpleRuneGetStatus", "OtherSkinPurpleRuneSetEnabled",
		"UnlockAllTrophyScan", "UnlockAllTrophyGetStatus", "UnlockAllTrophySetEnabled",
		"CountdownScan", "CountdownGetStatus", "CountdownSet",
	} {
		body := bodies[name]
		if body == nil {
			t.Fatalf("missing %s", name)
		}
		if !blockCallsSelector(body, "a", "acquireGameProcessLease") {
			t.Errorf("%s must pin the process handle for its full operation", name)
		}
	}
}

func TestLegacyRuntimeMutationsJoinGlobalLiveMemoryTransaction(t *testing.T) {
	bodies := currencyAppFunctionBodies(t)
	for _, name := range []string{
		"CharaSetOne", "CharaSetAll", "FaceAccessorySetHidden",
		"OtherSkinPurpleRuneSetEnabled", "UnlockAllTrophySetEnabled", "CountdownSet",
	} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
			t.Errorf("%s must serialize with detach and other live-memory writes", name)
		}
	}
}
