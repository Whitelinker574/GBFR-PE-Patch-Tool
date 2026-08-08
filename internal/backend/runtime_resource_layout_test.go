package backend

import "testing"

func TestPotionDefsFollowDetectedRuntimeLayout(t *testing.T) {
	tests := []struct {
		version string
		root    uintptr
		revive  []uintptr
		group   []uintptr
	}{
		{version: "2.0.2", root: 0x071B69B8, revive: []uintptr{0x28, 0x8, 0x8, 0x18, 0x38}, group: []uintptr{0x28, 0x8, 0x8, 0x18, 0x18}},
		{version: "2.0.3", root: 0x071B69B8, revive: []uintptr{0x28, 0x8, 0x8, 0x18, 0x38}, group: []uintptr{0x28, 0x8, 0x8, 0x18, 0x18}},
		{version: "2.0.4", root: 0x07369E08, revive: []uintptr{0x9B0, 0x38, 0xD84}, group: []uintptr{0x9B0, 0x38, 0xD64}},
	}
	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			defs, err := potionDefsForRuntimeVersion(test.version)
			if err != nil {
				t.Fatal(err)
			}
			if len(defs) != 2 || defs[0].RVA != test.root || defs[1].RVA != test.root {
				t.Fatalf("unexpected potion roots for %s: %+v", test.version, defs)
			}
			assertOffsetsEqual(t, defs[0].Offsets, test.revive)
			assertOffsetsEqual(t, defs[1].Offsets, test.group)
		})
	}
	if _, err := potionDefsForRuntimeVersion("unknown"); err == nil {
		t.Fatal("unknown runtime version must not fall back to a guessed potion pointer chain")
	}
}

func TestCurrencyCPIsIndependentFromResonancePoints(t *testing.T) {
	cp, ok := lookupCurrencyDef("cp")
	if !ok || cp.ID != "cp_extreme_void" || !cp.AOB || cp.Offset != 0x24 {
		t.Fatalf("cp lookup = %+v, %v", cp, ok)
	}
	rp, ok := lookupCurrencyDef("rp")
	if !ok || rp.ID != "rp" || rp.AOB {
		t.Fatalf("rp lookup = %+v, %v", rp, ok)
	}
	if cp.ID == rp.ID {
		t.Fatal("CP and RP must not share a definition")
	}
}

func assertOffsetsEqual(t *testing.T, got, want []uintptr) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("offset count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("offset[%d] = 0x%X, want 0x%X", i, got[i], want[i])
		}
	}
}
