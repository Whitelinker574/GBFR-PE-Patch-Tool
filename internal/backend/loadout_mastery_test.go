package backend

import (
	"strings"
	"testing"
)

// 专精配置器：节点池按 10/10/10/20 配额分档，各角色池子足够选。
func TestMasteryNodePool(t *testing.T) {
	app := &App{}
	pools, err := app.MasteryNodePool("PL0400") // 伊欧
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) != 4 {
		t.Fatalf("期望 4 档，得 %d", len(pools))
	}
	wantCap := map[string]int{"R1": 10, "R2": 10, "R3": 10, "EX": 20}
	wantLabel := map[string]string{"R1": "1阶专精技能", "R2": "2阶专精技能", "R3": "3阶专精技能", "EX": "EX阶专精技能"}
	for _, p := range pools {
		if p.Cap != wantCap[p.Rank] {
			t.Errorf("%s 档配额=%d，期望 %d", p.Rank, p.Cap, wantCap[p.Rank])
		}
		if len(p.Nodes) < p.Cap {
			t.Errorf("%s 档池子 %d 个 < 配额 %d，不够选", p.Rank, len(p.Nodes), p.Cap)
		}
		if p.Label != wantLabel[p.Rank] {
			t.Errorf("%s 官方标签=%q，期望 %q", p.Rank, p.Label, wantLabel[p.Rank])
		}
		t.Logf("%s(%s) 配额%d 池%d", p.Rank, p.Label, p.Cap, len(p.Nodes))
	}
}

func TestMasteryNodePoolExcludesStageUnlockRowsFromThe50SlotVector(t *testing.T) {
	pools, err := (&App{}).MasteryNodePool("PL0400")
	if err != nil {
		t.Fatal(err)
	}
	unlockRows := map[string]bool{
		"80490676": true, "58E2FE10": true, "49E65B86": true,
		"88DCAC00": true, "81F4970A": true, "E63217E6": true,
	}
	for _, pool := range pools {
		for _, node := range pool.Nodes {
			if unlockRows[node.Hash] || node.Specialization || strings.TrimSpace(node.Name) != "" {
				t.Errorf("%s 的阶段解锁行 %s 不应占用 3007 配置槽: %+v", pool.Rank, node.Hash, node)
			}
		}
	}
	if effect := masterySpecializationEffect("PL0400", "R3", "SB_ATK"); strings.TrimSpace(effect) == "" {
		t.Fatal("阶段解锁行虽不进入 3007，但必须继续提供逐阶效果说明")
	}
}

func TestMasteryNodePoolHidesNodesWithoutVerifiedEffects(t *testing.T) {
	pools, err := (&App{}).MasteryNodePool("PL1200")
	if err != nil {
		t.Fatal(err)
	}
	for _, pool := range pools {
		for _, node := range pool.Nodes {
			if strings.TrimSpace(node.Desc) == "" {
				t.Errorf("%s/%s 节点 %s 没有可核对的效果文本，不应进入自由配置池", pool.Rank, node.Cat, node.Hash)
			}
			if node.Hash == "F2D81718" {
				t.Error("未解明的 F2D81718 不应进入专精配置池")
			}
		}
	}
}

func TestSkillPoolUsesFullUnpackedCharacterCatalog(t *testing.T) {
	got := skillPoolForOwnerCode("PL0400")
	if len(got) != 8 {
		t.Fatalf("伊欧完整技能池应有 8 个，得 %d: %+v", len(got), got)
	}
	want := []string{"治愈之风", "寒冰", "雷霆", "魔洞", "火焰", "专注", "花耀七闪", "魔力漩涡"}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("技能 %d=%q，期望 %q", i+1, got[i].Name, want[i])
		}
	}
}

func TestLoadoutMasteryNodeCarriesOfficialRank(t *testing.T) {
	node, ok := loadoutMasteryNodeForHash(0x5B124A50) // 伊欧 EX：魔力漩涡额外赋予防御UP
	if !ok {
		t.Fatal("未找到伊欧 EX 节点 5B124A50")
	}
	if node.Rank != "EX" || node.RankLabel != "EX阶专精技能" {
		t.Fatalf("节点阶级=%s/%s，期望 EX/EX阶专精技能", node.Rank, node.RankLabel)
	}
	if node.Desc == "" {
		t.Fatal("EX 节点必须保留真实效果文本")
	}
}

func TestLoadoutMasteryStunUsesVerifiedPanelScaleButKeepsRawText(t *testing.T) {
	node, ok := loadoutMasteryNodeForHash(0x1F52146F)
	if !ok {
		t.Fatal("未找到伊欧 EX 昏厥节点 1F52146F")
	}
	if node.Desc != "昏厥值+4（原始 f32 0.4 ×10 面板）" || node.RawDesc != "昏厥值+0.4" || node.DisplayScale != 10 {
		t.Fatalf("昏厥节点没有同时保留原始值和真实面板值: %+v", node)
	}
	if node.Evidence != "2.0.2-table+runtime-display-scale" {
		t.Fatalf("昏厥节点证据等级=%q", node.Evidence)
	}
}

func TestMasterySummaryFollowsRealSaveCountActivationRules(t *testing.T) {
	// 伊欧实档的 3007 向量不包含阶段解锁行：1阶三个方向各达到3项；
	// 2阶真谛达到6项；3阶继续真谛达到6项，必须直接显示真谛3/3。
	toHashes := func(values ...string) []uint32 {
		out := make([]uint32, 0, len(values))
		for _, value := range values {
			h, err := ParseHashHex(value)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, h)
		}
		return out
	}
	hashes := toHashes(
		"8330DD47", "F5F495BE", "C20A0EC1", // R1 真谛 3
		"1F55589B", "1AD4D530", "82EE504B", // R1 觉醒 3
		"7EBE8989", "DACCAB76", "EC9C657D", // R1 秘义 3
		"082EBAC9", "F84D6E7C", "B09E1B0F", "774106C7", "A7E5980C", "2C218F9E", // R2 真谛 6
		"729ED65E", "75DB4733", "CBD26730", "8EE99564", "5C5381FA", "DE9B3B33", // R3 真谛 6
		"5B124A50", // EX 节点
	)
	summary, err := summarizeMasteryHashes("PL0400", hashes)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PrimaryCat != "SB_ATK" || summary.PrimaryLabel != "真谛：魔法连锁" {
		t.Fatalf("主方向=%s/%s，期望真谛：魔法连锁", summary.PrimaryCat, summary.PrimaryLabel)
	}
	r1 := summary.rank("R1")
	if r1 == nil || r1.activeCategoryCount() != 3 {
		t.Fatalf("1阶应三方向生效: %+v", r1)
	}
	r2 := summary.rank("R2")
	if r2 == nil || !r2.category("SB_ATK").Active || r2.activeCategoryCount() != 1 {
		t.Fatalf("2阶应仅真谛生效: %+v", r2)
	}
	if strings.TrimSpace(r2.category("SB_ATK").Effect) == "" {
		t.Fatalf("2阶方向摘要必须直接携带真实专精效果: %+v", r2.category("SB_ATK"))
	}
	r3 := summary.rank("R3")
	if r3 == nil || !r3.category("SB_ATK").Active || r3.activeCategoryCount() != 1 {
		t.Fatalf("3阶应沿用真谛: %+v", r3)
	}
	ex := summary.rank("EX")
	if ex == nil || ex.Label != "EX阶专精技能" || ex.activeCategoryCount() != 0 {
		t.Fatalf("EX阶不是第四专精方向: %+v", ex)
	}
}

// 配额校验：超档拒绝、正好 10/10/10/20 通过。
func TestMasteryQuota(t *testing.T) {
	app := &App{}
	pools, err := app.MasteryNodePool("PL0400")
	if err != nil {
		t.Fatal(err)
	}
	pick := func(rank, cat string, n int) []uint32 {
		var out []uint32
		for _, p := range pools {
			if p.Rank != rank {
				continue
			}
			for _, node := range p.Nodes {
				if cat != "" && node.Cat != cat {
					continue
				}
				h, _ := ParseHashHex(node.Hash)
				out = append(out, h)
				if len(out) == n {
					break
				}
			}
		}
		if len(out) != n {
			t.Fatalf("%s/%s 需要 %d 个测试节点，实际只有 %d 个", rank, cat, n, len(out))
		}
		return out
	}
	pickSubtraits := func(rank, cat string, n int) []uint32 {
		var out []uint32
		for _, p := range pools {
			if p.Rank != rank {
				continue
			}
			for _, node := range p.Nodes {
				if node.Cat != cat || node.Specialization {
					continue
				}
				h, _ := ParseHashHex(node.Hash)
				out = append(out, h)
				if len(out) == n {
					break
				}
			}
		}
		if len(out) != n {
			t.Fatalf("%s/%s 需要 %d 个普通子词条，实际只有 %d 个", rank, cat, n, len(out))
		}
		return out
	}
	pickDirectionalRank := func(rank, primary string) []uint32 {
		secondary := []string{"SB_ATK", "SB_DEF", "SB_LIMIT"}
		var out []uint32
		out = append(out, pick(rank, primary, 6)...)
		for _, cat := range secondary {
			if cat != primary {
				out = append(out, pickSubtraits(rank, cat, 2)...)
			}
		}
		return out
	}
	// 正好 10/10/10/20 = 50，且 2/3 阶沿用同一主方向，应通过。
	var full []uint32
	full = append(full, pick("R1", "", 10)...)
	full = append(full, pickDirectionalRank("R2", "SB_DEF")...)
	full = append(full, pickDirectionalRank("R3", "SB_DEF")...)
	full = append(full, pick("EX", "", 20)...)
	if _, err := validateMasteryQuota(full, "PL0400", true); err != nil {
		t.Errorf("满盘 10/10/10/20 应通过，得错误: %v", err)
	}
	// 方向是对已选节点的推导结果，不是写入限制；混合方向必须原样保留。
	wrongDirection := append([]uint32{}, pick("R1", "", 10)...)
	wrongDirection = append(wrongDirection, pickDirectionalRank("R2", "SB_DEF")...)
	wrongDirection = append(wrongDirection, pickDirectionalRank("R3", "SB_ATK")...)
	wrongDirection = append(wrongDirection, pick("EX", "", 20)...)
	if _, err := validateMasteryQuota(wrongDirection, "PL0400", false); err != nil {
		t.Errorf("2/3 阶混合方向应可写并由摘要提示，得错误: %v", err)
	}
	// 2 阶 4/3/3 没有达到任一主方向门槛，也只能提示而不能阻止写入。
	noPrimary := append([]uint32{}, pick("R1", "", 10)...)
	noPrimary = append(noPrimary, pickSubtraits("R2", "SB_ATK", 4)...)
	noPrimary = append(noPrimary, pickSubtraits("R2", "SB_DEF", 3)...)
	noPrimary = append(noPrimary, pickSubtraits("R2", "SB_LIMIT", 3)...)
	noPrimary = append(noPrimary, pickDirectionalRank("R3", "SB_DEF")...)
	noPrimary = append(noPrimary, pick("EX", "", 20)...)
	if _, err := validateMasteryQuota(noPrimary, "PL0400", true); err != nil {
		t.Errorf("没有六节点主方向的满盘仍应可写，得错误: %v", err)
	}
	if _, err := validateMasteryQuota(pick("R1", "", 3), "PL0400", true); err != nil {
		t.Errorf("部分专精向量可编码，只能提示未点满，得错误: %v", err)
	}
	// 实档证明阶段效果不占用3007槽；达到3/6/6数量门槛即激活。
	countActivated := append([]uint32{}, pick("R1", "", 10)...)
	countActivated = append(countActivated, pickDirectionalRank("R2", "SB_DEF")...)
	countActivated = append(countActivated, pickDirectionalRank("R3", "SB_DEF")...)
	countActivated = append(countActivated, pick("EX", "", 20)...)
	if _, err := validateMasteryQuota(countActivated, "PL0400", true); err != nil {
		t.Errorf("不含阶段解锁行的真实50槽配置应可保存: %v", err)
	}
	countSummary, err := summarizeMasteryHashes("PL0400", countActivated)
	if err != nil {
		t.Fatal(err)
	}
	if countSummary.PrimaryCat != "SB_DEF" || !countSummary.rank("R2").category("SB_DEF").Active || !countSummary.rank("R3").category("SB_DEF").Active {
		t.Fatalf("3/6/6数量门槛没有生成3阶觉醒效果: %+v", countSummary)
	}
	// R1 超一个（11），应拒绝。
	over := append(pick("R1", "", 11), pick("R2", "", 10)...)
	if _, err := validateMasteryQuota(over, "PL0400", false); err == nil {
		t.Error("R1 档 11 个应被拒绝")
	}
	duplicate := pick("R1", "", 10)
	duplicate[1] = duplicate[0]
	if _, err := validateMasteryQuota(duplicate, "PL0400", false); err == nil {
		t.Error("重复专精节点应被拒绝")
	}
}
