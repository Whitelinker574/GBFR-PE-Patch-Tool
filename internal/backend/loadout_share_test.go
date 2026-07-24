package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSharedWeaponIdentityIgnoresAwakeningStorageVariant(t *testing.T) {
	var variant, base uint32
	for candidate, canonical := range awakeningWeaponAliases {
		variant, base = candidate, canonical
		break
	}
	if variant == 0 || base == 0 {
		t.Fatal("awakening weapon alias table is empty")
	}
	if !sameSharedWeaponIdentity(hashText(variant), hashText(base)) {
		t.Fatalf("awakening variant %08X did not match base weapon %08X", variant, base)
	}
	if sameSharedWeaponIdentity(hashText(variant), "DEADBEEF") {
		t.Fatal("unrelated weapon hash matched an awakening variant")
	}
}

func TestLoadoutShareExportUsesReadableCompactJSON(t *testing.T) {
	nodes := make([]LoadoutShareEnhancementNode, 400)
	for index := range nodes {
		nodes[index] = LoadoutShareEnhancementNode{Index: index, Value: index % 4}
	}
	share := &LoadoutShare{
		Format: loadoutShareFormat, Version: loadoutShareVersion, CharaHash: "4D0A60C3", CharaName: "伊欧",
		MasteryHashes: []string{"1F52146F", "317E9A83"},
		Character:     &LoadoutShareCharacterProgression{EnhancementNodes: nodes},
	}
	payload, err := marshalLoadoutShare(share)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "\n  \"character\"") {
		t.Fatalf("loadout export is not human-readable JSON: %q", payload)
	}
	if strings.Contains(string(payload), `"enhancementNodes"`) || !strings.Contains(string(payload), `"enhancementNodeValues": {`) || !strings.Contains(string(payload), `"encoding": "rle-bitpack-v1"`) {
		t.Fatalf("v11 export did not RLE/bit-pack enhancement node values: %s", payload)
	}
	if strings.Contains(string(payload), `"enhancementNodeValues": [`) {
		t.Fatalf("v11 export still contains the uncompressed node array: %s", payload)
	}
	if len(payload) >= 2000 {
		t.Fatalf("v11 packed node data is unexpectedly large: %d bytes", len(payload))
	}
	decoded, err := unmarshalLoadoutShare(payload)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Format != share.Format || decoded.Version != share.Version || !reflect.DeepEqual(decoded.Character, share.Character) {
		t.Fatalf("compact export changed the loadout payload: %+v", decoded)
	}
	apiPayload, err := json.Marshal(share)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(apiPayload), `"enhancementNodes"`) || strings.Contains(string(apiPayload), `"enhancementNodeValues"`) {
		t.Fatalf("API/Wails JSON must keep enhancementNodes for the generated frontend model: %s", apiPayload)
	}
}

func TestLoadoutShareStillReadsV10EnhancementNodeValues(t *testing.T) {
	payload := []byte(`{"format":"gbfr-loadout","version":10,"character":{"enhancementNodeValues":[0,1,1,0,255]}}`)
	share, err := unmarshalLoadoutShare(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := []LoadoutShareEnhancementNode{{Index: 0, Value: 0}, {Index: 1, Value: 1}, {Index: 2, Value: 1}, {Index: 3, Value: 0}, {Index: 4, Value: 255}}
	if share.Character == nil || !reflect.DeepEqual(share.Character.EnhancementNodes, want) {
		t.Fatalf("v10 enhancementNodeValues were not restored: %+v", share.Character)
	}
}

func TestLoadoutShareRejectsInvalidPackedEnhancementNodeValues(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{name: "未知编码", payload: `{"encoding":"unknown","count":4,"valueBits":1,"runBits":2,"data":"AA"}`},
		{name: "位流截断", payload: `{"encoding":"rle-bitpack-v1","count":4,"valueBits":8,"runBits":3,"data":"AA"}`},
		{name: "RLE越界", payload: `{"encoding":"rle-bitpack-v1","count":2,"valueBits":1,"runBits":3,"data":"DA"}`},
		{name: "多余字节", payload: `{"encoding":"rle-bitpack-v1","count":1,"valueBits":1,"runBits":1,"data":"AAA"}`},
		{name: "非零填充位", payload: `{"encoding":"rle-bitpack-v1","count":1,"valueBits":1,"runBits":1,"data":"BA"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(`{"format":"gbfr-loadout","version":11,"character":{"enhancementNodeValues":` + tc.payload + `}}`)
			if _, err := unmarshalLoadoutShare(payload); err == nil {
				t.Fatalf("invalid packed enhancementNodeValues was accepted: %s", tc.payload)
			}
		})
	}
}

func TestLoadoutShareReadsNullEnhancementNodeValues(t *testing.T) {
	share, err := unmarshalLoadoutShare([]byte(`{"format":"gbfr-loadout","version":10,"character":{"enhancementNodeValues": null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if share.Character == nil || len(share.Character.EnhancementNodes) != 0 {
		t.Fatalf("null enhancementNodeValues changed character data: %+v", share.Character)
	}
}

func TestLoadoutShareStillReadsLegacyEnhancementNodeObjects(t *testing.T) {
	payload := []byte(`{"format":"gbfr-loadout","version":9,"character":{"enhancementNodes":[{"index":1,"value":2},{"index":7,"value":3}]}}`)
	var share LoadoutShare
	if err := json.Unmarshal(payload, &share); err != nil {
		t.Fatal(err)
	}
	want := []LoadoutShareEnhancementNode{{Index: 1, Value: 2}, {Index: 7, Value: 3}}
	if share.Character == nil || !reflect.DeepEqual(share.Character.EnhancementNodes, want) {
		t.Fatalf("legacy v9 enhancement nodes were not preserved: %+v", share.Character)
	}
}

func actualLoadoutShareFixture(t *testing.T) (string, *LoadoutShare) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv("GBFR_TEST_SHARE_SAVE"))
	if path == "" {
		t.Skip("set GBFR_TEST_SHARE_SAVE to a read-only save fixture")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("真实测试存档不存在: %v", err)
	}
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		for _, loadout := range group.Loadouts {
			if loadout.IsParty || len(loadout.Sigils) < 4 || len(loadout.Mastery) != 50 {
				continue
			}
			share, err := buildLoadoutShare(path, loadout.UnitID)
			if err != nil {
				t.Fatal(err)
			}
			return path, share
		}
	}
	t.Fatal("真实存档里没有可用于分享格式测试的完整配装")
	return "", nil
}

// 用用户提供的真实存档做单套配装分享往返测试；测试只读，不会写入原档。
func TestLoadoutShareRoundTripWithActualSave(t *testing.T) {
	path := strings.TrimSpace(os.Getenv("GBFR_TEST_SHARE_SAVE"))
	if path == "" {
		t.Skip("set GBFR_TEST_SHARE_SAVE to a read-only save fixture")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("真实测试存档不存在: %v", err)
	}

	app := &App{}
	groups, err := app.LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	var source LoadoutEntry
	for _, group := range groups {
		if group.CharaName != "伊欧" {
			continue
		}
		for _, loadout := range group.Loadouts {
			if !loadout.IsParty && len(loadout.Sigils) == 12 && len(loadout.Skills) == 4 && len(loadout.Mastery) == 50 {
				source = loadout
				break
			}
		}
	}
	if source.UnitID == 0 {
		t.Fatal("真实存档里未找到伊欧的 12 因子 / 4 技能 / 50 专精完整配装")
	}

	hasSecondaryTrait := false
	for _, sigil := range source.Sigils {
		if sigil.Missing {
			continue
		}
		raw, err := json.Marshal(sigil)
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]any{}
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		primaryName, primaryOK := fields["primaryTraitName"].(string)
		primaryLevel, primaryLevelOK := fields["primaryTraitLevel"].(float64)
		if !primaryOK || primaryName == "" || !primaryLevelOK || primaryLevel <= 0 {
			t.Fatalf("配装查看接口未返回因子 %s(%d) 的主词条: %s", sigil.Name, sigil.SlotID, raw)
		}
		secondaryName, secondaryOK := fields["secondaryTraitName"].(string)
		if !secondaryOK {
			t.Fatalf("配装查看接口缺少因子 %s(%d) 的副词条字段: %s", sigil.Name, sigil.SlotID, raw)
		}
		if secondaryName != "" {
			secondaryLevel, ok := fields["secondaryTraitLevel"].(float64)
			if !ok || secondaryLevel <= 0 {
				t.Fatalf("因子 %s(%d) 的副词条等级无效: %s", sigil.Name, sigil.SlotID, raw)
			}
			hasSecondaryTrait = true
		}
		if strings.Contains(primaryName, "伶利") || strings.Contains(secondaryName, "伶利") {
			t.Fatalf("真实配装返回了错误译名，应为“魔法师的伶俐”: %s", raw)
		}
	}
	if !hasSecondaryTrait {
		t.Fatal("真实配装的 12 个因子没有返回任何副词条")
	}
	for _, skill := range source.Skills {
		if skill.Key == "" {
			t.Fatalf("真实配装技能 %s(%s) 没有返回稳定的解包键", skill.Name, skill.Hash)
		}
	}

	share, err := buildLoadoutShare(path, source.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	if share.Format != loadoutShareFormat || share.Version != loadoutShareVersion {
		t.Fatalf("分享格式标识错误: %+v", share)
	}
	encoded, err := marshalLoadoutShare(share)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"enhancementNodes"`) || !strings.Contains(string(encoded), `"enhancementNodeValues"`) {
		t.Fatalf("真实配装没有使用 v10 固定位置强化数组: %s", encoded)
	}
	t.Logf("真实配装 v10 JSON 大小=%d bytes", len(encoded))
	filePath := filepath.Join(t.TempDir(), "real-v10.gbfr-loadout.json")
	if err := os.WriteFile(filePath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	diskPayload, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	encodedShare, err := unmarshalLoadoutShare(diskPayload)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encodedShare.Character.EnhancementNodes, share.Character.EnhancementNodes) {
		t.Fatal("v10 enhancementNodeValues did not restore the original node vector")
	}
	decodedDraft, err := resolveLoadoutShare(path, source.CharaHash, encodedShare)
	if err != nil {
		t.Fatalf("重新读取 v10 JSON 后导入解析失败: %v", err)
	}
	if len(decodedDraft.ConstructedSigils) != 12 || len(decodedDraft.MasteryHashes) != 50 {
		t.Fatalf("重新读取 v10 JSON 后导入草稿不完整: factors=%d mastery=%d", len(decodedDraft.ConstructedSigils), len(decodedDraft.MasteryHashes))
	}
	if len(share.Sigils) != 12 || len(share.Skills) != 4 || len(share.MasteryHashes) != 50 {
		t.Fatalf("导出丢字段: 因子%d 技能%d 专精%d", len(share.Sigils), len(share.Skills), len(share.MasteryHashes))
	}
	if len(share.WeaponSkillHashes) != 5 || share.Character == nil || len(share.Character.EnhancementPanel) != 2 || len(share.Character.EnhancementNodes) == 0 || !share.Character.WeaponWrightstonesCaptured ||
		share.Weapon == nil || share.Weapon.Wrightstone == nil {
		t.Fatalf("v8 精确快照不完整: weaponSkills=%v character=%+v weapon=%+v", share.WeaponSkillHashes, share.Character, share.Weapon)
	}

	draft, err := resolveLoadoutShare(path, source.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Missing) != 0 {
		t.Fatalf("同一真实存档导入不应缺资源: %v", draft.Missing)
	}
	if len(draft.SigilSlotIDs) != 12 || len(draft.ConstructedSigils) != 12 || len(draft.SkillHashes) != 4 || len(draft.MasteryHashes) != 50 {
		t.Fatalf("导入解析丢字段: 因子槽%d 构造%d 技能%d 专精%d", len(draft.SigilSlotIDs), len(draft.ConstructedSigils), len(draft.SkillHashes), len(draft.MasteryHashes))
	}
	if !reflect.DeepEqual(draft.MasteryHashes, share.MasteryHashes) || !reflect.DeepEqual(draft.WeaponSkillHashes, share.WeaponSkillHashes) {
		t.Fatalf("位置敏感快照在导入解析时被重排: mastery=%v weapon=%v", draft.MasteryHashes, draft.WeaponSkillHashes)
	}
	ctx, err := app.LoadoutEditContext(path, source.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Skills) < 8 {
		t.Fatalf("伊欧技能池不完整：得到 %d，至少应有 8 个", len(ctx.Skills))
	}
	for _, slotID := range draft.SigilSlotIDs {
		if slotID != 0 {
			t.Fatalf("单套导入不应复用旧因子 SlotID: %v", draft.SigilSlotIDs)
		}
	}
	for index, constructed := range draft.ConstructedSigils {
		if constructed.Index != index || constructed.Item.SigilID == "" || constructed.Item.PrimaryTraitID == "" {
			t.Fatalf("第 %d 格没有完整构造草稿: %+v", index+1, constructed)
		}
	}
}

func TestLoadoutShareV3PreservesSparseSigilIndices(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	if share.Version != loadoutShareVersion {
		t.Fatalf("新导出格式版本=%d，期望 %d", share.Version, loadoutShareVersion)
	}
	first, fourth := share.Sigils[0], share.Sigils[3]
	zero, three := 0, 3
	first.Index, fourth.Index = &zero, &three
	share.Sigils = []LoadoutShareSigil{first, fourth}

	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.SigilSlotIDs) != loadoutMaxSigils {
		t.Fatalf("v2 导入因子向量长度=%d，期望固定 %d", len(draft.SigilSlotIDs), loadoutMaxSigils)
	}
	if len(draft.ConstructedSigils) != 2 || draft.ConstructedSigils[0].Index != 0 || draft.ConstructedSigils[1].Index != 3 {
		t.Fatalf("稀疏因子没有回到原始 0/3 槽: %+v", draft.ConstructedSigils)
	}
	for i, slotID := range draft.SigilSlotIDs {
		if slotID != 0 {
			t.Fatalf("稀疏导入复用了第 %d 格的旧 SlotID: %v", i, draft.SigilSlotIDs)
		}
	}
}

func TestLoadoutShareV2RebuildsGeneratedCombinationHashAtOriginalPosition(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	first, fourth := share.Sigils[0], share.Sigils[3]
	zero, three := 0, 3
	first.Index, fourth.Index = &zero, &three
	fourth.Hash = "DEADBEEF"
	share.Sigils = []LoadoutShareSigil{first, fourth}

	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.ConstructedSigils) != 2 || draft.ConstructedSigils[1].Index != 3 {
		t.Fatalf("组合哈希因子没有按指纹回建到第 4 格: %+v", draft.ConstructedSigils)
	}
	if len(draft.Missing) != 0 {
		t.Fatalf("可以用主副词条指纹回建的因子不应报缺失: %v", draft.Missing)
	}
}

func TestLoadoutShareBuildsExactDraftForCombinationAbsentFromCatalog(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	var damageCap *TraitDef
	for index := range catalog.Traits {
		if cnTrait(catalog.Traits[index].DisplayName) == "伤害上限" {
			damageCap = &catalog.Traits[index]
			break
		}
	}
	if damageCap == nil {
		t.Fatal("目录中没有伤害上限词条")
	}
	slot := 0
	share.Sigils = []LoadoutShareSigil{{
		Index: &slot, Hash: "80C94A24", Name: "怒发冲冠 + 伤害上限", Level: 15,
		PrimaryTraitHash: "7EDD69D0", PrimaryTraitLevel: 15,
		SecondaryTraitHash: damageCap.Hash, SecondaryTraitLevel: 15,
	}}

	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.ConstructedSigils) != 1 {
		t.Fatalf("精确组合因子草稿数量=%d，期望 1", len(draft.ConstructedSigils))
	}
	got := draft.ConstructedSigils[0]
	if got.ExactSigilHash != "80C94A24" ||
		got.ExactPrimaryTraitHash != "7EDD69D0" ||
		!strings.EqualFold(got.ExactSecondaryTraitHash, damageCap.Hash) {
		t.Fatalf("精确组合因子哈希未保留: %+v", got)
	}
	if got.Item.Level != 15 || got.Item.PrimaryLevel != 15 || got.Item.SecondaryLevel != 15 {
		t.Fatalf("精确组合因子等级未保留: %+v", got.Item)
	}
}

func TestLoadoutShareV2RejectsInvalidSigilIndices(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	for _, tc := range []struct {
		name    string
		indices []int
	}{
		{name: "重复", indices: []int{2, 2}},
		{name: "越界", indices: []int{0, loadoutMaxSigils}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copyShare := *share
			copyShare.Sigils = append([]LoadoutShareSigil(nil), share.Sigils[:2]...)
			for i := range copyShare.Sigils {
				index := tc.indices[i]
				copyShare.Sigils[i].Index = &index
			}
			if _, err := resolveLoadoutShare(path, share.CharaHash, &copyShare); err == nil {
				t.Fatalf("v2 因子索引 %v 应被拒绝", tc.indices)
			}
		})
	}
}

func TestLoadoutShareReadsLegacyV1DenseSigils(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	share.Version = 1
	share.Summons = nil
	share.Sigils = append([]LoadoutShareSigil(nil), share.Sigils[:2]...)
	for i := range share.Sigils {
		share.Sigils[i].Index = nil
	}

	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.SigilSlotIDs) != 2 || len(draft.ConstructedSigils) != 2 || draft.ConstructedSigils[0].Index != 0 || draft.ConstructedSigils[1].Index != 1 {
		t.Fatalf("旧 v1 dense 因子没有按顺序转为构造草稿: slots=%v constructed=%+v", draft.SigilSlotIDs, draft.ConstructedSigils)
	}
}

func TestSingleTraitShareImportReplacesOpaqueSigilNameAndKeepsSecondaryEmpty(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const (
		sigilHash   = uint32(0x42BB0C1C) // GEEN_151_04, local 2.0.2 gem.tbl
		primaryHash = uint32(0x57AB5B10) // SKILL_151_00
	)
	draft, err := loadoutShareConstructedSigil(catalog, LoadoutShareSigil{
		Hash: hashText(sigilHash), Name: "0x42BB0C1C", Level: 15,
		PrimaryTraitHash: hashText(primaryHash), PrimaryTraitLevel: 15,
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantName := sigilDisplayName(sigilHash)
	if wantName == "" || draft.Item.SigilName != wantName || strings.Contains(strings.ToLower(draft.Item.SigilName), "0x42bb0c1c") {
		t.Fatalf("opaque single-trait name was not replaced: got=%q want=%q", draft.Item.SigilName, wantName)
	}
	if draft.Item.SecondaryTraitID != "" || draft.Item.SecondaryTraitName != "" ||
		draft.Item.SecondaryLevel != 0 || draft.ExactSecondaryTraitHash != "" {
		t.Fatalf("single-trait import fabricated a secondary trait: %+v", draft)
	}
	prepared, err := prepareExactLoadoutSigil(catalog, draft)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.hasSecondary || prepared.secondaryHash != EmptyHash || prepared.secondaryLevel != 0 {
		t.Fatalf("single-trait write was not prepared as an empty secondary slot: %+v", prepared)
	}
}

func TestCombinationShareImportDerivesNameFromTraitsInsteadOfKeepingOpaqueHash(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const (
		sigilHash   = uint32(0x80C94A24)
		primaryHash = uint32(0x7EDD69D0)
	)
	var secondary *TraitDef
	for index := range catalog.Traits {
		if catalog.Traits[index].InternalID == "SKILL_000_00" {
			secondary = &catalog.Traits[index]
			break
		}
	}
	if secondary == nil {
		t.Fatal("catalog lacks the ATK secondary trait fixture")
	}
	draft, err := loadoutShareConstructedSigil(catalog, LoadoutShareSigil{
		Hash: hashText(sigilHash), Name: hashText(sigilHash), Level: 15,
		PrimaryTraitHash: hashText(primaryHash), PrimaryTraitLevel: 15,
		SecondaryTraitHash: secondary.Hash, SecondaryTraitLevel: 15,
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantName := draft.Item.PrimaryTraitName + " + " + draft.Item.SecondaryTraitName
	if draft.Item.SigilName != wantName || isOpaqueLoadoutShareName(draft.Item.SigilName, sigilHash) {
		t.Fatalf("opaque combination name was not reconstructed: got=%q want=%q", draft.Item.SigilName, wantName)
	}
}

func TestNamedLocalTableAliasesNeverFallBackToOpaqueHashes(t *testing.T) {
	previous := getCurrentLanguage()
	t.Cleanup(func() { setCurrentLanguage(previous) })
	for _, locale := range []struct {
		language string
		names    map[uint32]string
	}{
		{language: "zh", names: map[uint32]string{
			0x2D85102A: "属性克制转换+",
			0x99E8B892: "狂战士+",
			0x97CF485D: "万能药+",
			0x4AE72C9E: "斯巴达+",
		}},
		{language: "en", names: map[uint32]string{
			0x2D85102A: "War Elemental+",
			0x99E8B892: "Berserker Echo+",
			0x97CF485D: "Potent Greens+",
			0x4AE72C9E: "Spartan Echo+",
		}},
	} {
		setCurrentLanguage(locale.language)
		for hash, want := range locale.names {
			if got := sigilDisplayName(hash); got != want {
				t.Errorf("%s factor %08X name=%q, want %q", locale.language, hash, got, want)
			}
		}
	}
}

func TestLoadoutShareV1ThroughV5WithoutSummonsRemainCompatible(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	for _, version := range []int{loadoutShareLegacyVersion, 2, 3, 4, loadoutShareVersion} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			legacy := *share
			legacy.Version = version
			legacy.Summons = nil
			payload, err := json.Marshal(&legacy)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(payload), "\"summons\"") {
				t.Fatalf("无召唤石旧文件不应写出 summons 字段: %s", payload)
			}
			draft, err := resolveLoadoutShare(path, legacy.CharaHash, &legacy)
			if err != nil {
				t.Fatal(err)
			}
			if draft.SummonSlotIDs != nil {
				t.Fatalf("无 summons 的 v%d 文件不应生成召唤石草稿: %v", version, draft.SummonSlotIDs)
			}
			for _, missing := range draft.Missing {
				if strings.Contains(missing, "召唤石") {
					t.Fatalf("无 summons 的 v%d 文件不应报告召唤石缺失: %v", version, draft.Missing)
				}
			}
		})
	}
}

func TestLoadoutShareV2ExportsFourStableSummonFingerprints(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	ctx, err := (&App{}).LoadoutStatContext(path, share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.EquippedSummons) != 4 {
		t.Fatalf("真实存档已装备召唤石=%d，期望4: warnings=%v", len(ctx.EquippedSummons), ctx.Warnings)
	}

	payload, err := json.Marshal(share)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Summons []struct {
			TypeHash       string `json:"typeHash"`
			Name           string `json:"name"`
			MainTraitHash  string `json:"mainTraitHash"`
			MainTraitLevel int    `json:"mainTraitLevel"`
			SubParamHash   string `json:"subParamHash"`
			SubParamLevel  int    `json:"subParamLevel"`
			Rank           int    `json:"rank"`
		} `json:"summons"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Summons) != 4 {
		t.Fatalf("v2 导出召唤石指纹=%d，期望4: %s", len(document.Summons), payload)
	}
	for index, got := range document.Summons {
		want := ctx.EquippedSummons[index]
		if got.TypeHash != want.TypeHash || got.Name != want.Name ||
			got.MainTraitHash != want.MainTraitHash || got.MainTraitLevel != want.MainTraitLevel ||
			got.SubParamHash != want.SubParamHash || got.SubParamLevel != want.SubParamLevel ||
			got.Rank != want.Rank {
			t.Fatalf("第%d槽召唤石指纹未按1451顺序稳定导出: got=%+v want=%+v", index+1, got, want)
		}
	}

	var generic map[string]any
	if err := json.Unmarshal(payload, &generic); err != nil {
		t.Fatal(err)
	}
	for index, raw := range generic["summons"].([]any) {
		fields := raw.(map[string]any)
		if _, exists := fields["slotId"]; exists {
			t.Fatalf("第%d槽分享指纹泄露了存档局部 slotId: %v", index+1, fields)
		}
		if _, exists := fields["unitId"]; exists {
			t.Fatalf("第%d槽分享指纹泄露了存档局部 unitId: %v", index+1, fields)
		}
	}
}

func TestLoadoutShareV2ResolvesSummonFingerprintsToRealSlotsInOrder(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	ctx, err := (&App{}).LoadoutStatContext(path, share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(draft)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SummonSlotIDs []uint32 `json:"summonSlotIds"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.SummonSlotIDs) != 4 {
		t.Fatalf("导入草稿召唤石槽=%v，期望固定四槽: %s", decoded.SummonSlotIDs, payload)
	}
	for index, slotID := range decoded.SummonSlotIDs {
		if slotID != ctx.EquippedSummonSlotIDs[index] {
			t.Fatalf("第%d槽映射到%d，期望当前存档真实实例%d", index+1, slotID, ctx.EquippedSummonSlotIDs[index])
		}
	}
}

func TestLoadoutShareV3KeepsMissingSummonPositionAndBuildsIt(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	share.Summons = append([]LoadoutShareSummon(nil), share.Summons...)
	share.Summons[0].TypeHash = "DEADBEEF"
	share.Summons[0].Name = "不存在的测试召唤石"

	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.SummonSlotIDs) != 4 || draft.SummonSlotIDs[0] != 0 {
		t.Fatalf("缺失召唤石没有在原始第1槽留空: %v", draft.SummonSlotIDs)
	}
	for index := 1; index < 4; index++ {
		if draft.SummonSlotIDs[index] == 0 {
			t.Fatalf("存在的第%d槽召唤石被错误清空: %v", index+1, draft.SummonSlotIDs)
		}
	}
	if draft.ApplyPayload == nil || len(draft.ApplyPayload.ConstructedSummons) != 1 ||
		draft.ApplyPayload.ConstructedSummons[0].Index != 0 || draft.ApplyPayload.ConstructedSummons[0].Name != "不存在的测试召唤石" {
		t.Fatalf("缺失召唤石没有生成原始第1槽构造草稿: %+v", draft.ApplyPayload)
	}
	if len(draft.Missing) != 0 {
		t.Fatalf("可自动生成的召唤石不应锁定写入: %v", draft.Missing)
	}
}

func shareSummonFingerprint(summon LoadoutSummon) LoadoutShareSummon {
	return LoadoutShareSummon{
		TypeHash: summon.TypeHash, Name: summon.Name,
		MainTraitHash: summon.MainTraitHash, MainTraitLevel: summon.MainTraitLevel,
		SubParamHash: summon.SubParamHash, SubParamLevel: summon.SubParamLevel,
		Rank: summon.Rank,
	}
}

func exactShareTestUnit(t *testing.T, save *SaveData, idType, unitID uint32, valueCount int) *unitEntry {
	t.Helper()
	var result *unitEntry
	for _, entry := range save.findAllUnitsByType(idType) {
		if entry.UnitID != unitID || entry.ValueCnt != valueCount ||
			entry.ValueOff < 0 || entry.ValueOff+valueCount*4 > len(save.data) {
			continue
		}
		if result != nil {
			t.Fatalf("IDType=%d UnitID=%d 存在多个%d值候选", idType, unitID, valueCount)
		}
		result = entry
	}
	if result == nil {
		t.Fatalf("找不到 IDType=%d UnitID=%d 的%d值记录", idType, unitID, valueCount)
	}
	return result
}

func cloneSummonFingerprintForShareTest(t *testing.T, path string, sourceUnitID, targetUnitID uint32) {
	t.Helper()
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		idType, valueCount uint32
	}{
		{idType: 1457, valueCount: 1},
		{idType: 1458, valueCount: 2},
		{idType: 1459, valueCount: 2},
		{idType: 1460, valueCount: 1},
	} {
		source := exactShareTestUnit(t, save, field.idType, sourceUnitID, int(field.valueCount))
		target := exactShareTestUnit(t, save, field.idType, targetUnitID, int(field.valueCount))
		for index := 0; index < int(field.valueCount); index++ {
			value, err := source.Uint32At(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := target.SetUint32At(index, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, save.data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeShareTestSummonSlots(t *testing.T, path string, slotIDs []uint32) {
	t.Helper()
	if len(slotIDs) != 4 {
		t.Fatalf("测试1451需要4个值，得到%d", len(slotIDs))
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := exactShareTestUnit(t, save, 1451, 0, 4)
	for index, slotID := range slotIDs {
		if err := entry.SetUint32At(index, slotID); err != nil {
			t.Fatal(err)
		}
	}
	if err := save.FixChecksums(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, save.data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func shareTestLoadoutUnitID(t *testing.T, path string, share *LoadoutShare) uint32 {
	t.Helper()
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		for _, loadout := range group.Loadouts {
			if !loadout.IsParty && strings.EqualFold(loadout.CharaHash, share.CharaHash) && loadout.Name == share.Name {
				return loadout.UnitID
			}
		}
	}
	t.Fatalf("存档副本中找不到分享源配装 %s/%s", share.CharaHash, share.Name)
	return 0
}

func TestLoadoutShareV3PicksOneEquivalentDuplicateSummon(t *testing.T) {
	_, share := actualLoadoutShareFixture(t)
	path := copyStatsSave(t)
	ctx, err := (&App{}).LoadoutStatContext(path, share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Summons) < 2 {
		t.Fatal("真实存档背包不足两个召唤石")
	}
	source, target := ctx.Summons[0], ctx.Summons[1]
	cloneSummonFingerprintForShareTest(t, path, source.UnitID, target.UnitID)
	ambiguous := shareSummonFingerprint(source)

	share.Summons = append([]LoadoutShareSummon(nil), share.Summons...)
	share.Summons[0] = ambiguous
	draft, err := resolveLoadoutShare(path, share.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.SummonSlotIDs) != 4 || draft.SummonSlotIDs[0] == 0 {
		t.Fatalf("等价重复实例应稳定选择一个已有 SlotID: %v", draft.SummonSlotIDs)
	}
	if len(draft.Missing) != 0 {
		t.Fatalf("字段完全相同的重复实例不应阻止导入: %v", draft.Missing)
	}
}

func TestLoadoutShareV2RejectsDuplicateEquippedSummonOnExport(t *testing.T) {
	_, share := actualLoadoutShareFixture(t)
	path := copyStatsSave(t)
	ctx, err := (&App{}).LoadoutStatContext(path, share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.EquippedSummonSlotIDs) != 4 {
		t.Fatalf("真实测试配置不是四召唤石: %v", ctx.EquippedSummonSlotIDs)
	}
	duplicate := append([]uint32(nil), ctx.EquippedSummonSlotIDs...)
	duplicate[1] = duplicate[0]
	writeShareTestSummonSlots(t, path, duplicate)

	unitID := shareTestLoadoutUnitID(t, path, share)
	if _, err := buildLoadoutShare(path, unitID); err == nil || !strings.Contains(err.Error(), "重复") {
		t.Fatalf("1451重复引用必须拒绝导出，得到: %v", err)
	}
}

func TestLoadoutShareV2RejectsIncompleteEquippedSummonOnExport(t *testing.T) {
	_, share := actualLoadoutShareFixture(t)
	path := copyStatsSave(t)
	ctx, err := (&App{}).LoadoutStatContext(path, share.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	incomplete := append([]uint32(nil), ctx.EquippedSummonSlotIDs...)
	incomplete[3] = 0
	writeShareTestSummonSlots(t, path, incomplete)

	unitID := shareTestLoadoutUnitID(t, path, share)
	if _, err := buildLoadoutShare(path, unitID); err == nil || !strings.Contains(err.Error(), "不完整") {
		t.Fatalf("1451空槽必须拒绝导出，得到: %v", err)
	}
}

func TestLoadoutShareV2RejectsPartialSummonFingerprintVectorOnImport(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	share.Summons = append([]LoadoutShareSummon(nil), share.Summons[:3]...)
	if _, err := resolveLoadoutShare(path, share.CharaHash, share); err == nil || !strings.Contains(err.Error(), "4") {
		t.Fatalf("非空 summons 必须恰好四槽，得到: %v", err)
	}
}
