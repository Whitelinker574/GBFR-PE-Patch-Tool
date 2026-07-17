package main

import "testing"

// 模拟器聚合规则：同名词条等级相加→封顶→查一次表。锚点用游戏原表实测值。
func TestSimulateTraitsAnchors(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	hashToID := buildTraitHashToID(cat)
	// ATK = SKILL_000_00 = 0x50079A1C（flat，Lv30=? Lv50=2000）
	const atk = 0x50079A1C
	if hashToID[atk] != "SKILL_000_00" {
		t.Fatalf("ATK hash 未映射到 SKILL_000_00，得 %q", hashToID[atk])
	}

	mk := func(hash uint32, lv int) struct {
		hash  uint32
		level int
	} {
		return struct {
			hash  uint32
			level int
		}{hash, lv}
	}

	// 两颗 ATK 各 15 → 合并 30（未封顶，max50）
	got := simulateTraits([]struct {
		hash  uint32
		level int
	}{mk(atk, 15), mk(atk, 15)}, hashToID)
	if len(got) != 1 {
		t.Fatalf("期望 1 行，得 %d", len(got))
	}
	if got[0].Level != 30 || got[0].Capped {
		t.Errorf("合并等级=%d capped=%v，期望 30/false", got[0].Level, got[0].Capped)
	}
	if got[0].Name != "攻击力" {
		t.Errorf("名字=%q，期望 攻击力", got[0].Name)
	}
	t.Logf("两颗ATK15 → %s（Lv%d）", got[0].Effect, got[0].Level)

	// 封顶：四颗 ATK 各 15 → 合并 60 → 封顶 50，值应为 2000
	got = simulateTraits([]struct {
		hash  uint32
		level int
	}{mk(atk, 15), mk(atk, 15), mk(atk, 15), mk(atk, 15)}, hashToID)
	if got[0].Level != 50 || !got[0].Capped || got[0].RawLevel != 60 {
		t.Errorf("封顶结果 lv=%d raw=%d capped=%v，期望 50/60/true", got[0].Level, got[0].RawLevel, got[0].Capped)
	}
	if len(got[0].Components) != 1 || got[0].Components[0].Value != 2000 || got[0].Components[0].Unit != "flat" {
		t.Errorf("ATK Lv50 分量=%+v，期望 flat 2000", got[0].Components)
	}
	t.Logf("四颗ATK15 → %s（Lv%d/%d，raw%d）", got[0].Effect, got[0].Level, got[0].MaxLevel, got[0].RawLevel)

	// 暴击率 SKILL_003_00 = 0x8D78A19B（pct，Lv45=50）
	const crit = 0x8D78A19B
	got = simulateTraits([]struct {
		hash  uint32
		level int
	}{mk(crit, 45)}, hashToID)
	if len(got) != 1 || got[0].Components[0].Unit != "pct" || got[0].Components[0].Value != 50 {
		t.Errorf("暴击率 Lv45 = %+v，期望 pct 50", got)
	}
	t.Logf("暴击率45 → %s", got[0].Effect)
}

// LoadoutSimulate 端到端：对真实存档某套配装的因子跑模拟。
func TestLoadoutSimulateReal(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("无测试存档")
	}
	app := &App{}
	groups, err := app.LoadoutList(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	// 找一套有因子的配装，取它的因子 SlotID
	var slotIDs []uint32
	for _, g := range groups {
		for _, lo := range g.Loadouts {
			if len(lo.Sigils) >= 6 {
				for _, s := range lo.Sigils {
					if s.SlotID != 0 {
						slotIDs = append(slotIDs, s.SlotID)
					}
				}
				break
			}
		}
		if len(slotIDs) > 0 {
			break
		}
	}
	if len(slotIDs) == 0 {
		t.Skip("没有可模拟的配装")
	}
	bonuses, err := app.LoadoutSimulate(testLoadoutSave, slotIDs)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("对 %d 颗因子模拟出 %d 条词条加成:", len(slotIDs), len(bonuses))
	for _, b := range bonuses {
		cap := ""
		if b.Capped {
			cap = "(封顶)"
		}
		t.Logf("  [%s] %s Lv%d%s → %s", b.CatLabel, b.Name, b.Level, cap, b.Effect)
	}
	if len(bonuses) == 0 {
		t.Error("模拟结果为空——可能是主/副词条读取或 join 失败")
	}
}
