package backend

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeWeaponRuntimeSkillsKeepsMoreThanFiveEntries(t *testing.T) {
	request := WeaponRuntimeSkillsDeployRequest{
		WeaponSlot: 17,
		WeaponID:   0x12345678,
		Skills: []WeaponRuntimeSkill{
			{Hash: 0x10000001, Level: 15},
			{Hash: 0x10000002, Level: 20},
			{Hash: 0x10000003, Level: 25},
			{Hash: 0x10000004, Level: 30},
			{Hash: 0x10000005, Level: 35},
			{Hash: 0x10000006, Level: 40},
			{Hash: 0x10000007, Level: 45},
		},
	}
	config, err := normalizeWeaponRuntimeSkills(request)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(config.Skills); got != 7 {
		t.Fatalf("extra runtime skills were truncated to %d entries", got)
	}
	if config.WeaponSlot != request.WeaponSlot || config.WeaponID != request.WeaponID {
		t.Fatalf("weapon binding changed: %+v", config)
	}
}

func TestEncodeWeaponRuntimeSkillsCarriesBindingAndAllEntries(t *testing.T) {
	config := WeaponRuntimeSkillsConfig{
		SchemaVersion: 1,
		WeaponSlot:    3,
		WeaponID:      0xAABBCCDD,
		Skills: []WeaponRuntimeSkill{
			{Hash: 0x11111111, Level: 15},
			{Hash: 0x22222222, Level: 25},
			{Hash: 0x33333333, Level: 35},
			{Hash: 0x44444444, Level: 45},
			{Hash: 0x55555555, Level: 55},
			{Hash: 0x66666666, Level: 65},
		},
	}
	data, err := encodeWeaponRuntimeSkills(config, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != weaponRuntimeSkillsHeaderSize+len(config.Skills)*8 {
		t.Fatalf("encoded length = %d", len(data))
	}
	if string(data[:8]) != weaponRuntimeSkillsMagic || binary.LittleEndian.Uint32(data[12:16]) != 1 {
		t.Fatalf("runtime header does not enable the feature: % X", data[:16])
	}
	if got := int32(binary.LittleEndian.Uint32(data[16:20])); got != config.WeaponSlot {
		t.Fatalf("weapon slot = %d", got)
	}
	if got := binary.LittleEndian.Uint32(data[20:24]); got != config.WeaponID {
		t.Fatalf("weapon id = %08X", got)
	}
	if got := binary.LittleEndian.Uint32(data[24:28]); got != uint32(len(config.Skills)) {
		t.Fatalf("entry count = %d", got)
	}
}

func TestWeaponRuntimeSkillsRejectsInvalidRowsWithoutFiveSlotCeiling(t *testing.T) {
	if _, err := normalizeWeaponRuntimeSkills(WeaponRuntimeSkillsDeployRequest{WeaponID: 1}); err == nil {
		t.Fatal("empty runtime skill list was accepted")
	}
	if _, err := normalizeWeaponRuntimeSkills(WeaponRuntimeSkillsDeployRequest{
		WeaponID: 1,
		Skills:   []WeaponRuntimeSkill{{Hash: 0x887AE0B0, Level: 15}},
	}); err == nil {
		t.Fatal("empty trait sentinel was accepted")
	}
	if _, err := normalizeWeaponRuntimeSkills(WeaponRuntimeSkillsDeployRequest{
		WeaponID: 1,
		Skills:   []WeaponRuntimeSkill{{Hash: 2, Level: 0}},
	}); err == nil {
		t.Fatal("zero-level runtime skill was accepted")
	}
}

func TestWeaponRuntimeSkillsAcceptsExactNativeBoundaryAndRejectsOverflow(t *testing.T) {
	skills := make([]WeaponRuntimeSkill, weaponRuntimeSkillsMaxEntries)
	for index := range skills {
		skills[index] = WeaponRuntimeSkill{Hash: uint32(index + 1), Level: uint32(index%99 + 1)}
	}
	request := WeaponRuntimeSkillsDeployRequest{WeaponSlot: 7, WeaponID: 0x12345678, Skills: skills}
	config, err := normalizeWeaponRuntimeSkills(request)
	if err != nil {
		t.Fatalf("exact %d-entry native boundary rejected: %v", weaponRuntimeSkillsMaxEntries, err)
	}
	encoded, err := encodeWeaponRuntimeSkills(config, true)
	if err != nil {
		t.Fatal(err)
	}
	decoded, enabled, err := decodeWeaponRuntimeSkills(encoded)
	if err != nil || !enabled || len(decoded.Skills) != weaponRuntimeSkillsMaxEntries {
		t.Fatalf("max-entry ABI round trip failed: enabled=%v count=%d err=%v", enabled, len(decoded.Skills), err)
	}
	overflow := append(append([]WeaponRuntimeSkill(nil), skills...), WeaponRuntimeSkill{Hash: 0xEEEEEEEE, Level: 1})
	request.Skills = overflow
	if _, err := normalizeWeaponRuntimeSkills(request); err == nil {
		t.Fatalf("%d-entry request crossed the native buffer boundary", len(overflow))
	}
}

func TestNativeWeaponRuntimeUsesGuardedLocalNativeAggregation(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "static DWORD RunWeaponSkillsRuntime()")
	if start < 0 {
		t.Fatal("native weapon runtime entry is missing")
	}
	block := body[start:]
	for _, required := range []string{
		`owner.OpenFromCommand(L"weapon-skills")`,
		`"GBFRWK01"`,
		"FindUniqueSignature(aggregationSignature",
		"FindUniqueSignature(applySignature",
		"resolvedApply != applyTarget",
		"ResolveLocalWeaponStatus(status, weapon)",
		"*reinterpret_cast<int32_t*>(weapon) != targetSlot",
		"*reinterpret_cast<uint32_t*>(weapon + 4) != targetId",
		"g_originalWeaponAggregation(status, accumulator, weapon)",
		"g_applyWeaponTrait(statusValue, accumulator, skills[index].hash, skills[index].level, 0)",
		"RestoreLibmemHookAfterDrain",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("native weapon runtime is missing %q", required)
		}
	}
	if strings.Index(body, "g_originalWeaponAggregation(status, accumulator, weapon)") > strings.Index(body, "ApplyExtraWeaponSkillsUnsafe(status") {
		t.Fatal("extra weapon skills run before the native five-slot aggregation")
	}
	if !strings.Contains(block, `WriteRuntimeStatus(L"weapon-skills", L"active"`) {
		t.Fatal("weapon runtime does not publish its persistent active state")
	}
}

func TestNativeWeaponRuntimeHotDetourDoesNotAllocateOrLeakCallbackCount(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "static void WeaponTraitAggregationDetour(")
	end := strings.Index(body[start:], "static bool StopWeaponSkillsRuntime()")
	if start < 0 || end < 0 {
		t.Fatal("weapon aggregation detour block is missing")
	}
	block := body[start : start+end]
	if strings.Contains(block, "std::vector<WeaponRuntimeSkillEntry>") {
		t.Fatal("hot weapon aggregation detour performs heap allocation")
	}
	for _, required := range []string{
		"WeaponRuntimeCallbackGuard callbackGuard",
		"std::array<WeaponRuntimeSkillEntry, kWeaponRuntimeMaxEntries>",
		"g_weaponRuntimeSkillCount",
	} {
		if !strings.Contains(block, required) {
			t.Fatalf("allocation-free callback lifecycle is missing %q", required)
		}
	}
}

func TestNativeWeaponRuntimeABIAndStopStateAreExplicit(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "src_dll", "patch_core", "dllmain.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	for _, required := range []string{
		"static_assert(sizeof(WeaponRuntimeConfigHeader) == 28)",
		"static_assert(sizeof(WeaponRuntimeSkillEntry) == 8)",
		"g_weaponRuntimeCleanRebuilds",
		`L"inactive_pending_refresh"`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("native weapon runtime ABI/recovery contract is missing %q", required)
		}
	}
}
