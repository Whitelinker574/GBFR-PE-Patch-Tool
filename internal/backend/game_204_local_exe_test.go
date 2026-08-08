package backend

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func loadLocalGame204Sections(t *testing.T) (string, []runtimePatchLocalExecutableSection) {
	t.Helper()
	path := os.Getenv("GBFR_GAME_EXE_204_TEST")
	if path == "" {
		t.Skip("set GBFR_GAME_EXE_204_TEST to verify the locally supplied game 2.0.4 executable")
	}
	if err := verifyRuntimePatchLocalGameIdentityExact(path, runtimePatchLocalGame204Size, runtimePatchLocalGame204SHA256); err != nil {
		t.Fatalf("verify local game 2.0.4 identity: %v", err)
	}
	sections, err := readRuntimePatchLocalExecutableSections(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, sections
}

func requireLocalGame204Pattern(t *testing.T, sections []runtimePatchLocalExecutableSection, raw string, wantRVA uintptr, original []byte) {
	t.Helper()
	pattern, err := parseRuntimePatchPattern(raw)
	if err != nil {
		t.Fatal(err)
	}
	matches := findRuntimePatchLocalPatternMatches(sections, pattern)
	if len(matches) != 1 || uintptr(matches[0].rva) != wantRVA {
		t.Fatalf("matches=%s, want one at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), wantRVA)
	}
	if len(original) != 0 {
		got := runtimePatchLocalBytesAtRVA(sections, uint32(wantRVA), len(original))
		if !bytes.Equal(got, original) {
			t.Fatalf("RVA 0x%X original=% X, want % X", wantRVA, got, original)
		}
	}
}

func requireLocalGame204MaskedPattern(t *testing.T, sections []runtimePatchLocalExecutableSection, spec combatTuningSiteSpec, wantRVA uintptr) {
	t.Helper()
	pattern := runtimePatchPattern{Values: append([]byte(nil), spec.Pattern...), Mask: make([]byte, len(spec.Mask))}
	for index, exact := range spec.Mask {
		if exact {
			pattern.Mask[index] = 0xFF
		}
	}
	matches := findRuntimePatchLocalPatternMatches(sections, pattern)
	if len(matches) != 1 || uintptr(matches[0].rva) != wantRVA {
		t.Fatalf("matches=%s, want one at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), wantRVA)
	}
	got := runtimePatchLocalBytesAtRVA(sections, uint32(wantRVA), len(spec.Original))
	if !bytes.Equal(got, spec.Original) {
		t.Fatalf("RVA 0x%X original=% X, want % X", wantRVA, got, spec.Original)
	}
}

func TestGame204RuntimeLayoutPatterns(t *testing.T) {
	_, sections := loadLocalGame204Sections(t)
	layout := runtimeGameLayouts[2]
	checks := []struct {
		name     string
		aob      string
		rva      uintptr
		original []byte
	}{
		{name: "party pointer", aob: runtimePatchPartyPointerAOB, rva: layout.PartyPointerRVA},
		{name: "selected material", aob: runtimePatchSelectedMaterialAOB, rva: layout.SelectedMaterialRVA, original: runtimePatchSelectedMaterialOriginal},
		{name: "selected key item", aob: runtimePatchSelectedKeyItemAOB, rva: layout.SelectedKeyItemRVA, original: runtimePatchSelectedKeyItemOriginal},
		{name: "inventory material", aob: runtimeInventoryMaterialAOB, rva: layout.InventoryMaterialRVA},
		{name: "sigil hook", aob: runtimeSigilHookAOB, rva: layout.SigilHookRVA, original: sigilMemoryOriginalBytes},
		{name: "wrightstone hook", aob: runtimeWrightstoneHookAOB, rva: layout.WrightstoneHookRVA, original: wrightstoneMemoryOriginalBytes},
		{name: "save function", aob: runtimeItemSaveFunctionAOB, rva: layout.SaveFunctionRVA},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			requireLocalGame204Pattern(t, sections, check.aob, check.rva, check.original)
		})
	}
	if got := runtimePatchLocalBytesAtRVA(sections, uint32(layout.WeaponHookRVA), len(weaponMemoryGuardBytes)); !bytes.Equal(got, weaponMemoryGuardBytes) {
		t.Fatalf("weapon hook RVA 0x%X guard=% X, want % X", layout.WeaponHookRVA, got, weaponMemoryGuardBytes)
	}
	if got := runtimePatchLocalBytesAtRVA(sections, uint32(layout.SaveFunctionRVA), len(gameSaveFunctionPrologue)); !bytes.Equal(got, gameSaveFunctionPrologue) {
		t.Fatalf("save function RVA 0x%X prologue=% X, want % X", layout.SaveFunctionRVA, got, gameSaveFunctionPrologue)
	}
	partyEntry := runtimePatchLocalBytesAtRVA(sections, uint32(layout.PartyPointerRVA), 7)
	if len(partyEntry) != 7 {
		t.Fatal("2.0.4 party entry is unavailable")
	}
	resolvedPartySlot := int64(layout.PartyPointerRVA) + 7 + int64(int32(binary.LittleEndian.Uint32(partyEntry[3:7])))
	if resolvedPartySlot != int64(layout.PartySlotTableRVA) {
		t.Fatalf("2.0.4 party slot RVA=0x%X, want 0x%X", resolvedPartySlot, layout.PartySlotTableRVA)
	}
}

func TestGame204StandaloneRuntimePatterns(t *testing.T) {
	_, sections := loadLocalGame204Sections(t)
	free204RVAs := []uintptr{0x41D21E7, 0x3E9971A, freeConsumptionSites[2].RVA, freeConsumptionSites[3].RVA, 0x41D339C, 0x41CAC7E, freeConsumptionSites[6].RVA, freeConsumptionSites[7].RVA, 0x1BC6861, freeConsumptionSites[9].RVA, 0x41CA899}
	for index, spec := range freeConsumptionSites {
		t.Run("free-consumption-"+string(rune('A'+index)), func(t *testing.T) {
			requireLocalGame204Pattern(t, sections, spec.AOB, free204RVAs[index], spec.Original)
		})
	}
	t.Run("task-score", func(t *testing.T) {
		requireLocalGame204Pattern(t, sections, taskScoreAOB, taskScoreMultiplierRVA+0xFA0, taskScoreOriginal)
	})
	for index, spec := range taskSideQuestSpecs {
		t.Run("task-side-"+string(rune('A'+index)), func(t *testing.T) {
			requireLocalGame204Pattern(t, sections, spec.AOB, spec.RVA+0xFA0, spec.Original)
		})
	}
	t.Run("summon-duration", func(t *testing.T) {
		requireLocalGame204Pattern(t, sections, summonDurationAOB, summonDurationRVA, summonDuration204Original)
	})
	t.Run("action-speed", func(t *testing.T) {
		requireLocalGame204MaskedPattern(t, sections, combatActionSpeedSpec, 0xBB0918+0xFA0)
	})
	for index, spec := range combatCooldownSpecs {
		t.Run("cooldown-"+string(rune('A'+index)), func(t *testing.T) {
			requireLocalGame204MaskedPattern(t, sections, spec, []uintptr{0x21EA56D, 0x27FEDBE, 0x33D218A}[index])
		})
	}
	t.Run("charge", func(t *testing.T) {
		requireLocalGame204MaskedPattern(t, sections, combatChargeSpec, 0x279EFE0)
	})
	t.Run("conflux timer manager", func(t *testing.T) {
		const aob = "41 B8 D0 07 00 00 31 D2 E8 ?? ?? ?? ?? 48 89 35 ?? ?? ?? ?? 41 B8 D0 2C 00 00 48 89 F1 31 D2 E8 ?? ?? ?? ??"
		const matchRVA = uintptr(0x6093AB)
		requireLocalGame204Pattern(t, sections, aob, matchRVA, nil)
		store := runtimePatchLocalBytesAtRVA(sections, uint32(matchRVA+13), 7)
		if len(store) != 7 || !bytes.Equal(store[:3], []byte{0x48, 0x89, 0x35}) {
			t.Fatalf("2.0.4 Conflux manager store=% X", store)
		}
		target := int64(matchRVA+20) + int64(int32(binary.LittleEndian.Uint32(store[3:])))
		if target != int64(confluxTimerManagerPointerRVA204) {
			t.Fatalf("2.0.4 Conflux manager pointer RVA=0x%X, want 0x%X", target, confluxTimerManagerPointerRVA204)
		}
	})
	t.Run("task reward multiplier boundary", func(t *testing.T) {
		pattern := runtimePatchPattern{Values: append([]byte(nil), taskRewardMultiplierPattern...), Mask: make([]byte, len(taskRewardMultiplierMask))}
		for index, exact := range taskRewardMultiplierMask {
			if exact {
				pattern.Mask[index] = 0xFF
			}
		}
		matches := findRuntimePatchLocalPatternMatches(sections, pattern)
		const entryRVA = uintptr(0x1FDB960)
		if len(matches) != 1 || uintptr(matches[0].rva) != entryRVA {
			t.Fatalf("matches=%s, want task reward boundary at RVA 0x%X", formatRuntimePatchLocalMatchLocations(matches), entryRVA)
		}
		original := runtimePatchLocalBytesAtRVA(sections, uint32(entryRVA), taskRewardMultiplierHookSize)
		managerSlot, err := taskRewardMultiplierManagerSlot(entryRVA, original)
		if err != nil {
			t.Fatal(err)
		}
		if managerSlot != 0x7034CE0 {
			t.Fatalf("2.0.4 task reward manager slot RVA=0x%X, want 0x7034CE0", managerSlot)
		}
	})
}

func TestGame204SpatialAndVirtualSigilSites(t *testing.T) {
	_, sections := loadLocalGame204Sections(t)
	checks := []struct {
		name string
		rva  uintptr
		want []byte
	}{
		{name: "gravity context", rva: runtimeGameLayouts[2].SpatialGravityRVA - runtimeSpatialGravityContextBack, want: runtimeSpatialGravityContext},
		{name: "gravity", rva: runtimeGameLayouts[2].SpatialGravityRVA, want: runtimeSpatialGravityOriginal},
		{name: "flight floor query", rva: runtimeSpatialFlightHookRVA + 0xFA0, want: runtimeSpatialFlightHookOriginal},
		{name: "flight recovery", rva: runtimeSpatialVirtualGroundHookRVA + 0xFA0, want: runtimeSpatialVirtualGroundHookOriginal},
		{name: "jump gate context", rva: runtimeSpatialJumpGateRVA + 0xFA0 - 7, want: runtimeSpatialJumpGateContext},
		{name: "jump check context", rva: runtimeSpatialJumpCheckRVA + 0xFA0 - 22, want: runtimeSpatialJumpCheckContext},
		{name: "virtual sigil apply", rva: 0x00A1EBE4 + 0xFA0 - 4, want: []byte{0xFF, 0xC7, 0x83, 0xFF, 0x0D, 0x0F, 0x84, 0xB7, 0x00, 0x00, 0x00, 0xC5, 0xF8, 0x11, 0x75, 0xF0}},
		{name: "virtual sigil category", rva: 0x00A1F7F6 + 0xFA0 - 6, want: []byte{0x49, 0xFF, 0xC5, 0x49, 0x83, 0xFD, 0x0D, 0x0F, 0x84, 0xE4, 0x00, 0x00, 0x00}},
		{name: "virtual sigil fetch", rva: 0x00A1F80E + 0xFA0, want: []byte{0x84, 0xDB, 0x74, 0x3E, 0x49, 0x8B, 0x87, 0x80, 0x5E, 0x00, 0x00}},
		{name: "virtual sigil getter", rva: 0x00A25D70 + 0xFA0, want: []byte{0x55, 0x41, 0x57, 0x41, 0x56, 0x56, 0x57, 0x53, 0x48, 0x83, 0xEC, 0x28}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			got := runtimePatchLocalBytesAtRVA(sections, uint32(check.rva), len(check.want))
			if !bytes.Equal(got, check.want) {
				t.Fatalf("RVA 0x%X=% X, want % X", check.rva, got, check.want)
			}
		})
	}
}

func TestGame204CharacterPanelGuards(t *testing.T) {
	_, sections := loadLocalGame204Sections(t)
	for index, guard := range runtimeCharacterPanel204VersionGuards {
		rva := guard.RVA
		got := runtimePatchLocalBytesAtRVA(sections, uint32(rva), len(guard.Bytes))
		if !bytes.Equal(got, guard.Bytes) {
			t.Errorf("guard[%d] RVA 0x%X=% X, want 2.0.4 bytes=% X", index, rva, got, guard.Bytes)
		}
	}
}

func TestGame204MonsterEnhanceSites(t *testing.T) {
	_, sections := loadLocalGame204Sections(t)
	checks := []struct {
		name     string
		aob      string
		matchRVA uintptr
		offset   uintptr
		original []byte
	}{
		{name: "monster hp", aob: "48 8B 41 10 45 31 C9 48 29 D0 4C 0F 43 C8 B8 01 00 00 00 49 0F 47 C1 45 85 C0 49 0F 44 C1 48 89 41 10 C3", matchRVA: 0x1F756B0, original: []byte{0x48, 0x8B, 0x41, 0x10, 0x45, 0x31, 0xC9}},
		{name: "party monster damage", aob: "48 89 51 18 48 89 51 10 C3 CC CC CC CC CC CC CC 48 89 51 18 C3 CC CC CC CC CC CC CC CC CC CC CC 48 89 51 10 C3", matchRVA: 0x1F75680, offset: 0x20, original: []byte{0x48, 0x89, 0x51, 0x10, 0xC3, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC}},
		{name: "monster stun", aob: "C5 FA 58 86 60 ?? ?? ?? C5 FA 5D 86 64 ?? ?? ?? C5 FA 11 86 60 ?? ?? ??", matchRVA: 0xB23848, original: []byte{0xC5, 0xFA, 0x58, 0x86, 0x60, 0x08, 0x00, 0x00}},
		{name: "overdrive state", aob: "8B 46 10 83 F8 03 0F 84 ?? ?? ?? ?? 83 F8 01 0F 84 ?? ?? ?? ??", matchRVA: 0x22C6926, original: []byte{0x8B, 0x46, 0x10, 0x83, 0xF8, 0x03}},
		{name: "OD gauge rate", aob: "80 79 50 00 74 13 48 03 51 18 48 C7 C0 FF FF FF FF 48 0F 43 C2 48 89 41 18 C3", matchRVA: 0x22C6DF0, original: []byte{0x80, 0x79, 0x50, 0x00, 0x74, 0x13, 0x48, 0x03, 0x51, 0x18, 0x48, 0xC7, 0xC0, 0xFF, 0xFF, 0xFF, 0xFF, 0x48, 0x0F, 0x43, 0xC2, 0x48, 0x89, 0x41, 0x18, 0xC3}},
		{name: "OD gauge rate inline boss path", aob: "48 03 7E 18 48 C7 C0 FF FF FF FF 48 0F 43 C7 48 89 46 18", matchRVA: 0x2B3F77E, original: append([]byte(nil), odRateInlineOriginal...)},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			requireLocalGame204Pattern(t, sections, check.aob, check.matchRVA, nil)
			got := runtimePatchLocalBytesAtRVA(sections, uint32(check.matchRVA+check.offset), len(check.original))
			if !bytes.Equal(got, check.original) {
				t.Fatalf("target RVA 0x%X=% X, want % X", check.matchRVA+check.offset, got, check.original)
			}
		})
	}
}

func TestGame204ExtremeVoidCPStoreSite(t *testing.T) {
	_, sections := loadLocalGame204Sections(t)
	requireLocalGame204Pattern(
		t,
		sections,
		"81 F9 F8 3B 63 CC 75 21 48 8B 05 ?? ?? ?? ?? 8B 48 24",
		0x41AAF89,
		nil,
	)
}
