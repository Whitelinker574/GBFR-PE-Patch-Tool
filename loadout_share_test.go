package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

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
	if len(share.Sigils) != 12 || len(share.Skills) != 4 || len(share.MasteryHashes) != 50 {
		t.Fatalf("导出丢字段: 因子%d 技能%d 专精%d", len(share.Sigils), len(share.Skills), len(share.MasteryHashes))
	}

	draft, err := resolveLoadoutShare(path, source.CharaHash, share)
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Missing) != 0 {
		t.Fatalf("同一真实存档导入不应缺资源: %v", draft.Missing)
	}
	if len(draft.SigilSlotIDs) != 12 || len(draft.SkillHashes) != 4 || len(draft.MasteryHashes) != 50 {
		t.Fatalf("导入解析丢字段: 因子%d 技能%d 专精%d", len(draft.SigilSlotIDs), len(draft.SkillHashes), len(draft.MasteryHashes))
	}
	ctx, err := app.LoadoutEditContext(path, source.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Skills) < 8 {
		t.Fatalf("伊欧技能池不完整：得到 %d，至少应有 8 个", len(ctx.Skills))
	}
	selected := map[uint32]bool{}
	for _, slotID := range draft.SigilSlotIDs {
		selected[slotID] = true
	}
	for _, sigil := range ctx.Sigils {
		if selected[sigil.SlotID] && sigil.PrimaryTraitName == "" {
			t.Fatalf("真实因子 %s(%d) 未显示主词条", sigil.Name, sigil.SlotID)
		}
	}
}

func TestLoadoutShareV2PreservesSparseSigilIndices(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	if share.Version != 2 {
		t.Fatalf("新导出格式版本=%d，期望 2", share.Version)
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
	if draft.SigilSlotIDs[0] == 0 || draft.SigilSlotIDs[3] == 0 {
		t.Fatalf("稀疏因子没有回到原始 0/3 槽: %v", draft.SigilSlotIDs)
	}
	for i, slotID := range draft.SigilSlotIDs {
		if i != 0 && i != 3 && slotID != 0 {
			t.Fatalf("稀疏导入把因子写到了第 %d 槽: %v", i, draft.SigilSlotIDs)
		}
	}
}

func TestLoadoutShareV2KeepsMissingSigilPositionEmpty(t *testing.T) {
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
	if draft.SigilSlotIDs[0] == 0 || draft.SigilSlotIDs[3] != 0 {
		t.Fatalf("缺失因子没有在原第 4 格留下空位: %v", draft.SigilSlotIDs)
	}
	if len(draft.Missing) != 1 {
		t.Fatalf("缺失项=%v，期望恰好 1 项", draft.Missing)
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
	if len(draft.SigilSlotIDs) != 2 || draft.SigilSlotIDs[0] == 0 || draft.SigilSlotIDs[1] == 0 {
		t.Fatalf("旧 v1 dense 因子没有按顺序兼容导入: %v", draft.SigilSlotIDs)
	}
}

func TestLoadoutShareV1AndV2WithoutSummonsRemainCompatible(t *testing.T) {
	path, share := actualLoadoutShareFixture(t)
	for _, version := range []int{loadoutShareLegacyVersion, loadoutShareVersion} {
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

func TestLoadoutShareV2KeepsMissingSummonPositionAndReportsIt(t *testing.T) {
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
	if len(draft.Missing) != 1 || !strings.Contains(draft.Missing[0], "召唤石") ||
		!strings.Contains(draft.Missing[0], "不存在的测试召唤石") {
		t.Fatalf("缺失召唤石没有加入 Missing: %v", draft.Missing)
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

func TestLoadoutShareV2ReportsAmbiguousSummonFingerprintInsteadOfPickingOne(t *testing.T) {
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
	if len(draft.SummonSlotIDs) != 4 || draft.SummonSlotIDs[0] != 0 {
		t.Fatalf("歧义指纹不应擅自选择一个真实实例: %v", draft.SummonSlotIDs)
	}
	foundAmbiguous := false
	for _, item := range draft.Missing {
		if strings.Contains(item, "召唤石") && strings.Contains(item, "歧义") {
			foundAmbiguous = true
		}
	}
	if !foundAmbiguous {
		t.Fatalf("歧义召唤石没有加入 Missing 并说明原因: %v", draft.Missing)
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
