package backend

import (
	"regexp"
	"testing"
)

func TestLoadoutSigilAccessFailsClosedForUnknownHashes(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	const knownGeneric = uint32(0x2D7F2E70) // Attack Power V+
	var knownCharacter uint32
	for _, def := range cat.Sigils {
		if def.Category != nil && *def.Category == "character_sigil" {
			knownCharacter, err = ParseHashHex(def.Hash)
			if err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if knownCharacter == 0 {
		t.Fatal("测试目录里没有角色专属因子")
	}

	if generic, allowed := loadoutSigilAccess(cat, knownGeneric, nil); !generic || !allowed {
		t.Fatalf("已知通用因子应放行且标为通用: generic=%v allowed=%v", generic, allowed)
	}
	if generic, allowed := loadoutSigilAccess(cat, knownCharacter, nil); generic || allowed {
		t.Fatalf("无先例的角色因子应拒绝: generic=%v allowed=%v", generic, allowed)
	}
	if generic, allowed := loadoutSigilAccess(cat, knownCharacter, map[uint32]bool{knownCharacter: true}); generic || !allowed {
		t.Fatalf("有本角色先例的角色因子应放行但不得标为通用: generic=%v allowed=%v", generic, allowed)
	}

	const unknown = uint32(0x6CBA6B0D) // 实档中出现、目录未收录的因子 hash
	if cat.LookupSigilByHash(unknown) != nil {
		t.Fatalf("测试前提失效：%08X 已进入目录", unknown)
	}
	if generic, allowed := loadoutSigilAccess(cat, unknown, nil); generic || allowed {
		t.Fatalf("未知因子不得跨角色当通用因子暴露: generic=%v allowed=%v", generic, allowed)
	}
	if generic, allowed := loadoutSigilAccess(cat, unknown, map[uint32]bool{unknown: true}); generic || !allowed {
		t.Fatalf("未知因子仅可按本角色已有先例放行，且不得伪称通用: generic=%v allowed=%v", generic, allowed)
	}
}

func TestLoadoutSigilNameUsesOnlyVerifiedItemIdentity(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("zh")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	if got := loadoutSigilDisplayNameFromTraits(0x46ABA3C0, "怒发冲冠", "伤害上限"); got != "怒发冲冠 V+" {
		t.Fatalf("目录因子名=%q，不能把强制副词条拼进物品标题", got)
	}
	if got := loadoutSigilDisplayNameFromTraits(0x80C94A24, "怒发冲冠", "伤害上限"); got != "怒发冲冠 V+" {
		t.Fatalf("组合实例因子名=%q，应由唯一主词条对应到合法物品名", got)
	}
	for _, test := range []struct {
		hash      uint32
		primary   string
		secondary string
		want      string
	}{
		{hash: 0xB5B23F02, primary: "体力", secondary: "金刚", want: "体力 V+"},
		{hash: 0x80C94A24, primary: "怒发冲冠", secondary: "伤害上限", want: "怒发冲冠 V+"},
		{hash: 0xF1D8F754, primary: "分歧", secondary: "天星之炼", want: "分歧 V+"},
	} {
		if got := loadoutSigilDisplayNameFromTraits(test.hash, test.primary, test.secondary); got != test.want {
			t.Errorf("实档因子 %08X 的标题=%q，期望按真实主副词条显示 %q", test.hash, got, test.want)
		}
	}
	if got := loadoutSigilDisplayNameFromTraits(0x673C5D8F, "勇士的信念", "勇士的毅力"); got != "勇士之觉醒+" {
		t.Fatalf("角色专属觉醒因子标题=%q，不能被通用 V+ 规则覆盖", got)
	}
	for hash, want := range map[uint32]string{
		0x426AD20E: "永恒钳蟹因子+",
		0x95CC3CB8: "群青之觉醒+",
		0xD8A464F1: "刃姬之觉醒+",
		0x23953FD4: "雷狼之觉醒+",
	} {
		if got := loadoutSigilDisplayNameFromTraits(hash, "DLC专属主词条", "DLC专属副词条"); got != want {
			t.Errorf("DLC 专属因子 %08X 的标题=%q，期望固定物品名 %q", hash, got, want)
		}
	}
	for _, test := range []struct {
		primary   string
		secondary string
		want      string
	}{
		{primary: "怒发冲冠", want: "怒发冲冠 V"},
		{primary: "体力", secondary: "伤害上限", want: "体力 V+"},
		{primary: "躲避性能", secondary: "伤害上限", want: "躲避性能+"},
		{primary: "可怕的漆黑钳蟹因子", secondary: "伤害上限", want: "可怕的漆黑钳蟹因子"},
	} {
		if got := loadoutSigilDisplayNameFromTraits(0xDEADBEEF, test.primary, test.secondary); got != test.want {
			t.Errorf("主词条 %q、副词条 %q 的因子名=%q，期望 %q", test.primary, test.secondary, got, test.want)
		}
	}
	if got := loadoutSigilDisplayNameFromTraits(0x6CBA6B0D, "不存在的主词条", "攻击力"); got != "不存在的主词条 V+" {
		t.Fatalf("无法匹配目录时也应按主副词条形态显示，不应显示占位语: %q", got)
	}
	if got := loadoutSigilDisplayNameFromTraits(0xDEADBEEF, "", ""); got != "因子" {
		t.Fatalf("完全未知因子名=%q，不应显示占位语或八位 hash", got)
	}
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if got := loadoutTraitDisplayName(cat, 0xDEADBEEF); got != "未收录词条" {
		t.Fatalf("未知词条名=%q，不应显示八位 hash", got)
	}
	if got := sigilDisplayNameOr(0xDEADBEEF); got != "因子" {
		t.Fatalf("未知因子回退名=%q，不应显示八位 hash", got)
	}
}

func TestLoadoutTraitNamesPreferHashSpecificExtractedGameText(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("zh")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for hash, want := range map[uint32]string{
		0x0CD6C625: "变幻自如的迅刃",
		0xA3B49220: "变幻自如的妖剑士",
		0x77C809F5: "剑圣的炼气",
	} {
		if got := loadoutTraitDisplayName(cat, hash); got != want {
			t.Errorf("解包词条 %08X 的名称=%q，期望 %q", hash, got, want)
		}
	}
}

func TestValidateLoadoutWeaponDefinitionRejectsUnknownHiddenAndWrongOwner(t *testing.T) {
	if _, err := loadProgressionCatalog(); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []uint32{0xC8736136, 0xDEADBEEF, 0xEE1EBC2E} {
		if _, err := validateLoadoutWeaponDefinition(hash, "PL0100"); err == nil {
			t.Errorf("未收录/隐藏武器 %08X 应被写入校验拒绝", hash)
		}
	}
	if _, err := validateLoadoutWeaponDefinition(0xC2D446F7, "PL0400"); err == nil {
		t.Error("姬塔武器不应允许写给伊欧")
	}
	def, err := validateLoadoutWeaponDefinition(0xC2D446F7, "PL0100")
	if err != nil || def.OwnerCode != "PL0100" || def.NameCN != "启程" {
		t.Fatalf("真实 WeaponId2 别名应按真实归属放行: def=%+v err=%v", def, err)
	}
}

func TestRealLoadoutContextDoesNotExposeUnknownSigilsAsGeneric(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	app := &App{}
	groups, err := app.LoadoutList(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	hexOnly := regexp.MustCompile(`(?i)^[0-9a-f]{8}$`)
	checked := 0
	for _, group := range groups {
		if len(group.Loadouts) == 0 {
			continue
		}
		ctx, err := app.LoadoutEditContext(testLoadoutSave, group.CharaHash)
		if err != nil {
			continue
		}
		for _, factor := range ctx.Sigils {
			hash, err := ParseHashHex(factor.Hash)
			if err != nil {
				t.Fatal(err)
			}
			if cat.LookupSigilByHash(hash) == nil && factor.Generic {
				t.Fatalf("%s 的未知因子 %s 被错误标为通用", group.CharaName, factor.Hash)
			}
			if hexOnly.MatchString(factor.Name) {
				t.Fatalf("%s 的因子名称仍显示裸 hash: %q", group.CharaName, factor.Name)
			}
		}
		for _, weapon := range ctx.Weapons {
			hash, err := ParseHashHex(weapon.Hash)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := progressionWeaponDefForLoadout(hash); !ok {
				t.Fatalf("%s 的武器池含未知/隐藏武器 %s (%s)", group.CharaName, weapon.Hash, weapon.Name)
			}
			if hexOnly.MatchString(weapon.Name) {
				t.Fatalf("%s 的武器名称仍显示裸 hash: %q", group.CharaName, weapon.Name)
			}
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("真实存档没有可校验的配装编辑上下文")
	}
}
