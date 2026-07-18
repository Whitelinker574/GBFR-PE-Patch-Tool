package main

import "testing"

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

func TestMasterySummaryFollowsThreeDirectionActivationRules(t *testing.T) {
	// 1阶三方向各 3 项均可生效；2阶只能觉醒达到 6 项；3阶沿用觉醒；EX 不产生第四种专精类型。
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
		"12AA6898", "1F55589B", "1AD4D530", // R1 觉醒 3
		"64D850F3", "DACCAB76", "EC9C657D", // R1 秘义 3
		"2904689B", "6ED1BE0F", "DF5883DE", "D69522F1", "AD6575D4", "82BE5E7C", // R2 觉醒 6
		"3AA0FBEF", "E0E6FF0C", "7580D66D", "16271F83", "E03C3AD2", "A66398F9", // R3 觉醒 6
		"5B124A50", // EX 节点
	)
	summary, err := summarizeMasteryHashes("PL0400", hashes)
	if err != nil {
		t.Fatal(err)
	}
	if summary.PrimaryCat != "SB_DEF" || summary.PrimaryLabel != "觉醒：专注强化" {
		t.Fatalf("主方向=%s/%s，期望觉醒：专注强化", summary.PrimaryCat, summary.PrimaryLabel)
	}
	r1 := summary.rank("R1")
	if r1 == nil || r1.activeCategoryCount() != 3 {
		t.Fatalf("1阶应三方向生效: %+v", r1)
	}
	r2 := summary.rank("R2")
	if r2 == nil || !r2.category("SB_DEF").Active || r2.activeCategoryCount() != 1 {
		t.Fatalf("2阶应仅觉醒生效: %+v", r2)
	}
	r3 := summary.rank("R3")
	if r3 == nil || !r3.category("SB_DEF").Active || r3.activeCategoryCount() != 1 {
		t.Fatalf("3阶应沿用觉醒: %+v", r3)
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
	pick := func(rank string, n int) []uint32 {
		var out []uint32
		for _, p := range pools {
			if p.Rank != rank {
				continue
			}
			for i := 0; i < n && i < len(p.Nodes); i++ {
				h, _ := ParseHashHex(p.Nodes[i].Hash)
				out = append(out, h)
			}
		}
		return out
	}
	// 正好 10/10/10/20 = 50，requireFull 应通过
	var full []uint32
	full = append(full, pick("R1", 10)...)
	full = append(full, pick("R2", 10)...)
	full = append(full, pick("R3", 10)...)
	full = append(full, pick("EX", 20)...)
	if _, err := validateMasteryQuota(full, "PL0400", true); err != nil {
		t.Errorf("满盘 10/10/10/20 应通过，得错误: %v", err)
	}
	// R1 超一个（11），应拒绝
	over := append(pick("R1", 11), pick("R2", 10)...)
	if _, err := validateMasteryQuota(over, "PL0400", false); err == nil {
		t.Error("R1 档 11 个应被拒绝")
	}
}
