package main

import (
	"os"
	"testing"
)

// 用用户提供的真实存档做单套配装分享往返测试；测试只读，不会写入原档。
func TestLoadoutShareRoundTripWithActualSave(t *testing.T) {
	const path = `D:\gbf\Saved\SaveGames\SaveData2.dat`
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
