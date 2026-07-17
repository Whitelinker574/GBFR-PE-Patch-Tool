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
	for _, p := range pools {
		if p.Cap != wantCap[p.Rank] {
			t.Errorf("%s 档配额=%d，期望 %d", p.Rank, p.Cap, wantCap[p.Rank])
		}
		if len(p.Nodes) < p.Cap {
			t.Errorf("%s 档池子 %d 个 < 配额 %d，不够选", p.Rank, len(p.Nodes), p.Cap)
		}
		t.Logf("%s(%s) 配额%d 池%d", p.Rank, p.Label, p.Cap, len(p.Nodes))
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
