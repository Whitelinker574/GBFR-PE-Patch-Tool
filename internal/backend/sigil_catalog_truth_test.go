package backend

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func sigilInfoJSONByID(t *testing.T, id string) map[string]any {
	t.Helper()
	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.InternalID != id {
			continue
		}
		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Fatalf("因子目录中找不到 %s", id)
	return nil
}

func intSliceField(t *testing.T, value map[string]any, key string) []int {
	t.Helper()
	raw, ok := value[key].([]any)
	if !ok {
		t.Fatalf("%s is not a JSON array: %#v", key, value[key])
	}
	result := make([]int, len(raw))
	for index, item := range raw {
		number, ok := item.(float64)
		if !ok {
			t.Fatalf("%s[%d] is not a number: %#v", key, index, item)
		}
		result[index] = int(number)
	}
	return result
}

func boolField(t *testing.T, item map[string]any, field string) bool {
	t.Helper()
	value, ok := item[field].(bool)
	if !ok {
		t.Fatalf("目录字段 %q 缺失或不是 bool: %#v", field, item[field])
	}
	return value
}

func TestSigilCatalogMarksOnlyVerifiedNaturalEntriesConstructible(t *testing.T) {
	safe := sigilInfoJSONByID(t, "GEEN_045_24")
	if !boolField(t, safe, "verified") || !boolField(t, safe, "constructible") {
		t.Fatalf("已验证且有显式副词条白名单的因子应可构造: %#v", safe)
	}

	tableBackedPool := sigilInfoJSONByID(t, "GEEN_000_24")
	if !boolField(t, tableBackedPool, "verified") || !boolField(t, tableBackedPool, "constructible") {
		t.Fatalf("gem/lot 三表验证的记录应可构造: %#v", tableBackedPool)
	}

	tableBackedDodge := sigilInfoJSONByID(t, "GEEN_063_24")
	if !boolField(t, tableBackedDodge, "verified") || !boolField(t, tableBackedDodge, "constructible") {
		t.Fatalf("Improved Dodge+ 的本地 lot 7 已闭环，应可构造: %#v", tableBackedDodge)
	}

	sevenNet := sigilInfoJSONByID(t, "GEEN_142_02")
	if !boolField(t, sevenNet, "verified") || boolField(t, sevenNet, "constructible") {
		t.Fatalf("GEEN_142_02 应标记为真实 DLC 记录，但普通 flags=2 构造必须禁用: %#v", sevenNet)
	}
	if levels := intSliceField(t, sevenNet, "allowedSigilLevels"); !reflect.DeepEqual(levels, []int{6}) {
		t.Fatalf("GEEN_142_02 物品等级=%v，真实 DLC 表/存档应为 [6]", levels)
	}
	if levels := intSliceField(t, sevenNet, "allowedFirstTraitLevels"); !reflect.DeepEqual(levels, []int{6}) {
		t.Fatalf("GEEN_142_02 主词条等级=%v，真实 DLC 表/存档应为 [6]", levels)
	}
}

func TestPotentGreensPlusUsesLocal202GemPrimary(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := catalog.RequireSigil("GEEN_156_24")
	if err != nil {
		t.Fatal(err)
	}
	if sigil.PrimaryTraitID != "SKILL_023_00" {
		t.Fatalf("GEEN_156_24 primary = %s; local 2.0.2 gem.tbl SkillId1 is SKILL_023_00", sigil.PrimaryTraitID)
	}
	trait, err := catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(trait.Hash, "0xCAC6AFF2") {
		t.Fatalf("GEEN_156_24 primary hash = %s; want 0xCAC6AFF2", trait.Hash)
	}
}

func TestFearlessDriveFixedSecondaryRejectsAnythingElse(t *testing.T) {
	gen := NewSigilGen()
	compatible, err := gen.GetCompatibleSecondaryTraits("GEEN_114_90")
	if err != nil {
		t.Fatal(err)
	}
	if len(compatible) != 1 || compatible[0].InternalID != "SKILL_114_01" {
		t.Fatalf("GEEN_114_90 compatible secondaries = %+v; local 2.0.2 gem.tbl fixes SkillId2 to SKILL_114_01", compatible)
	}

	fixed := QueueItem{
		SigilID: "GEEN_114_90", Level: 15, PrimaryLevel: 15,
		SecondaryTraitID: "SKILL_114_01", SecondaryLevel: 15, Quantity: 1,
	}
	report, err := gen.CheckLegality(fixed)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityLegal || !report.Writable {
		t.Fatalf("gem.tbl fixed GEEN_114_90 pairing must be legal+writable, got %+v", report)
	}

	nonFixed := fixed
	nonFixed.SecondaryTraitID = "SKILL_000_00"
	report, err = gen.CheckLegality(nonFixed)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("GEEN_114_90 non-fixed secondary must warn but remain writable, got %+v", report)
	}
	if err := gen.AddToQueue(nonFixed); err != nil {
		t.Fatalf("GEEN_114_90 non-fixed secondary should enter the writable queue: %v", err)
	}

	missing := fixed
	missing.SecondaryTraitID = ""
	missing.SecondaryLevel = 0
	report, err = gen.CheckLegality(missing)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != LegalityForced || !report.Writable {
		t.Fatalf("GEEN_114_90 missing fixed SkillId2 must warn but remain writable, got %+v", report)
	}
	if err := gen.AddToQueue(missing); err != nil {
		t.Fatalf("GEEN_114_90 without fixed SkillId2 should enter the writable queue: %v", err)
	}
}

func TestLoadoutDraftAllowsMissingFixedCharacterSecondaryWithWarning(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	_, err = prepareLoadoutSigil(catalog, LoadoutConstructedSigil{
		Index: 0,
		Item: QueueItem{
			SigilID: "GEEN_114_90", Level: 15, PrimaryLevel: 15, Quantity: 1,
		},
	})
	if err != nil {
		t.Fatalf("loadout draft should preserve a writable non-natural character factor: %v", err)
	}
}

func TestSigilMemoryRejectsMissingFixedCharacterSecondary(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := catalog.RequireSigil("GEEN_114_90")
	if err != nil {
		t.Fatal(err)
	}
	primary, err := catalog.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		t.Fatal(err)
	}
	sigilHash, err := ParseHashHex(sigil.Hash)
	if err != nil {
		t.Fatal(err)
	}
	primaryHash, err := ParseHashHex(primary.Hash)
	if err != nil {
		t.Fatal(err)
	}
	err = validateSigilMemoryUpdate(catalog, SigilMemoryUpdate{
		SigilHash: sigilHash, SigilLevel: 15,
		PrimaryTraitHash: primaryHash, PrimaryTraitLevel: 15,
		SecondaryTraitHash: EmptyHash, SecondaryTraitLevel: 0,
	})
	if err == nil {
		t.Fatal("sigil memory editor accepted GEEN_114_90 without its fixed SKILL_114_01 secondary")
	}
}

func TestCharacterSigilSecondaryPoolsMatchLocal202GemLotTables(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}

	fixed := map[string]string{
		"GEEN_114_90": "SKILL_114_01", "GEEN_115_90": "SKILL_115_01",
		"GEEN_116_90": "SKILL_116_01", "GEEN_117_90": "SKILL_117_01",
		"GEEN_118_90": "SKILL_118_01", "GEEN_119_90": "SKILL_119_01",
		"GEEN_120_90": "SKILL_120_01", "GEEN_121_90": "SKILL_121_01",
		"GEEN_122_90": "SKILL_122_01", "GEEN_123_90": "SKILL_123_01",
		"GEEN_124_90": "SKILL_124_01", "GEEN_125_90": "SKILL_125_01",
		"GEEN_126_90": "SKILL_126_01", "GEEN_127_90": "SKILL_127_01",
		"GEEN_128_90": "SKILL_128_01", "GEEN_129_90": "SKILL_129_01",
		"GEEN_130_90": "SKILL_130_01", "GEEN_131_90": "SKILL_131_01",
		"GEEN_132_90": "SKILL_132_01", "GEEN_170_90": "SKILL_170_01",
		"GEEN_171_90": "SKILL_171_01", "GEEN_172_90": "SKILL_172_01",
		"GEEN_170_74": "SKILL_066_00", "GEEN_171_74": "SKILL_066_00",
		"GEEN_172_74": "SKILL_066_00",
	}
	randomPool := []string{
		"SKILL_000_00", "SKILL_001_00", "SKILL_003_00", "SKILL_004_00",
		"SKILL_005_00", "SKILL_006_00", "SKILL_008_00", "SKILL_009_00",
		"SKILL_012_00", "SKILL_013_00", "SKILL_014_00", "SKILL_017_00",
		"SKILL_018_00", "SKILL_020_00", "SKILL_024_00", "SKILL_027_00",
		"SKILL_028_00", "SKILL_029_00", "SKILL_030_00", "SKILL_031_00",
		"SKILL_036_00", "SKILL_045_00", "SKILL_046_00", "SKILL_047_00",
		"SKILL_051_00", "SKILL_052_00", "SKILL_054_00", "SKILL_055_00",
		"SKILL_057_00", "SKILL_058_00", "SKILL_060_00", "SKILL_061_00",
		"SKILL_063_00", "SKILL_064_00", "SKILL_065_00", "SKILL_066_00",
		"SKILL_067_00", "SKILL_068_00", "SKILL_069_00", "SKILL_070_00",
		"SKILL_072_00", "SKILL_073_00", "SKILL_077_00", "SKILL_078_00",
		"SKILL_079_00", "SKILL_080_00", "SKILL_083_00", "SKILL_085_00",
		"SKILL_086_00", "SKILL_087_00", "SKILL_088_00", "SKILL_094_00",
		"SKILL_096_00", "SKILL_104_00", "SKILL_106_00", "SKILL_107_00",
		"SKILL_109_00", "SKILL_110_00", "SKILL_111_00", "SKILL_136_00",
		"SKILL_137_00", "SKILL_138_00", "SKILL_139_00",
	}
	randomSet := make(map[string]bool, len(randomPool))
	for _, id := range randomPool {
		randomSet[id] = true
	}

	characterCount := 0
	fixedCount := 0
	randomCount := 0
	for index := range catalog.Sigils {
		sigil := &catalog.Sigils[index]
		if !strings.EqualFold(derefStr(sigil.Category), "character_sigil") {
			continue
		}
		characterCount++
		if !strings.Contains(sigil.Source, "local 2.0.2 gem.tbl") {
			t.Errorf("%s source does not identify local 2.0.2 gem.tbl: %q", sigil.InternalID, sigil.Source)
		}
		if want, ok := fixed[sigil.InternalID]; ok {
			fixedCount++
			if len(sigil.AllowedSecondaryTraitIDs) != 1 || sigil.AllowedSecondaryTraitIDs[0] != want {
				t.Errorf("%s fixed SkillId2 pool = %v; want [%s]", sigil.InternalID, sigil.AllowedSecondaryTraitIDs, want)
			}
			if sigil.DefaultSecondaryTraitID == nil || *sigil.DefaultSecondaryTraitID != want {
				t.Errorf("%s default secondary = %v; want fixed SkillId2 %s", sigil.InternalID, sigil.DefaultSecondaryTraitID, want)
			}
			continue
		}

		randomCount++
		got := make(map[string]bool, len(sigil.AllowedSecondaryTraitIDs))
		for _, id := range sigil.AllowedSecondaryTraitIDs {
			got[id] = true
		}
		if len(got) != len(randomSet) {
			t.Errorf("%s random pool has %d distinct traits; skill_type_lot 0x0F has %d", sigil.InternalID, len(got), len(randomSet))
			continue
		}
		for id := range randomSet {
			if !got[id] {
				t.Errorf("%s random pool is missing %s from skill_type_lot 0x0F", sigil.InternalID, id)
			}
		}
	}
	if characterCount != 91 || fixedCount != 25 || randomCount != 66 {
		t.Fatalf("character sigil table coverage = %d total, %d fixed, %d random; want 91/25/66", characterCount, fixedCount, randomCount)
	}
}

func TestConstructibleSigilMetadataUsesOnlyNaturalLevels(t *testing.T) {
	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	constructibleCount := 0
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		constructible, _ := fields["constructible"].(bool)
		if !constructible {
			continue
		}
		constructibleCount++
		if len(item.AllowedSigilLevels) == 0 || len(item.AllowedFirstTraitLevels) == 0 {
			t.Fatalf("可构造因子 %s 必须公开明确的自然等级: %+v", item.InternalID, item)
		}
		for _, levels := range [][]int{item.AllowedSigilLevels, item.AllowedFirstTraitLevels} {
			for _, level := range levels {
				if level < 1 || level > 15 {
					t.Fatalf("可构造因子 %s 公开了非自然等级 %d", item.InternalID, level)
				}
			}
		}
		if item.FirstTraitMaxLevel < 1 || item.FirstTraitMaxLevel > 15 {
			t.Fatalf("可构造因子 %s 的主词条自然上限=%d，期望 1..15", item.InternalID, item.FirstTraitMaxLevel)
		}
	}
	if constructibleCount == 0 {
		t.Fatal("目录没有任何可构造因子，真实性过滤过度")
	}
}

func TestCompatibleSecondaryTraitsMatchFreshLocalLots(t *testing.T) {
	gen := NewSigilGen()
	for id, wantCount := range map[string]int{"GEEN_063_24": 16, "GEEN_146_24": 38, "GEEN_000_24": 59} {
		traits, err := gen.GetCompatibleSecondaryTraits(id)
		if err != nil {
			t.Fatalf("%s 读取兼容副词条失败: %v", id, err)
		}
		if len(traits) != wantCount {
			t.Fatalf("%s 兼容副词条=%d，2.0.2 本地 lot 应为 %d", id, len(traits), wantCount)
		}
	}

	traits, err := gen.GetCompatibleSecondaryTraits("GEEN_045_24")
	if err != nil {
		t.Fatal(err)
	}
	if len(traits) == 0 {
		t.Fatal("已验证且有显式白名单的因子没有返回兼容副词条")
	}
	for _, trait := range traits {
		if trait.MaxLevel < 1 || trait.MaxLevel > 15 {
			t.Fatalf("兼容副词条 %s 暴露的自然上限=%d，期望 1..15", trait.InternalID, trait.MaxLevel)
		}
		for _, level := range trait.AllowedLevels {
			if level < 1 || level > 15 {
				t.Fatalf("兼容副词条 %s 暴露了非自然等级 %d", trait.InternalID, level)
			}
		}
	}
}

func TestSigilQueueAcceptsSevenNetWithSpecialFlagsWarning(t *testing.T) {
	gen := NewSigilGen()
	item := QueueItem{
		SigilID: "GEEN_142_02", Level: 6, PrimaryLevel: 6, Quantity: 1,
	}
	report, err := gen.CheckLegality(item)
	if err != nil || report.Status != LegalityForced || !report.Writable || !strings.Contains(report.Message, "flags=22") {
		t.Fatalf("Seven Net 应提示特殊 flags 并保持可写，报告=%+v 错误=%v", report, err)
	}
	if err := gen.AddToQueue(item); err != nil {
		t.Fatalf("Seven Net 应进入使用 flags=22 的写入队列: %v", err)
	}
}
