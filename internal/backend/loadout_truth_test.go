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

func TestUncataloguedLoadoutSigilNameUsesItsStoredTraits(t *testing.T) {
	previous := getCurrentLanguage()
	setCurrentLanguage("zh")
	t.Cleanup(func() { setCurrentLanguage(previous) })

	if got := loadoutSigilDisplayNameFromTraits(0x6CBA6B0D, "怒涛", "攻击力"); got != "怒涛 + 攻击力" {
		t.Fatalf("未知因子名=%q，期望由实际主副词条组成", got)
	}
	if got := loadoutSigilDisplayNameFromTraits(0xDEADBEEF, "", ""); got != "未收录因子" {
		t.Fatalf("完全未知因子名=%q，不应显示八位 hash", got)
	}
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if got := loadoutTraitDisplayName(cat, 0xDEADBEEF); got != "未收录词条" {
		t.Fatalf("未知词条名=%q，不应显示八位 hash", got)
	}
	if got := sigilDisplayNameOr(0xDEADBEEF); got != "未收录因子" {
		t.Fatalf("未知因子回退名=%q，不应显示八位 hash", got)
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
