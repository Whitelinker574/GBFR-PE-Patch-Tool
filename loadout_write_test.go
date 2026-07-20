package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testUnlockedSave = `D:\gbf\SaveData1.dat`
const testLoadoutSave = `D:\gbf\SaveData2(3).dat`

func haveSave(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// 名称字段（3002）是 Byte 表，ValueCnt=64=字节数。验证：
//  1. entryText 走字节路径能读出正确名称；
//  2. SetBytes 写入不越界、不触碰相邻记录（对比写前写后除本 64 字节外全档一致）。
func TestLoadoutNameByteRoundtrip(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	save, err := LoadSave(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	// 伊欧块基址 20045，已知名称「装备方案01」
	e, ok := save.findUnitExact(loadoutNameIDType, 20045)
	if !ok {
		t.Fatal("找不到 3002 @20045")
	}
	if e.ValueCnt != 64 {
		t.Fatalf("3002 ValueCnt=%d，期望 64（字节数）", e.ValueCnt)
	}
	name := entryText(e)
	if name == "" {
		t.Fatal("名称读出为空")
	}
	t.Logf("读出名称: %q (%d 字节)", name, len(e.Bytes()))

	// 快照整档，改这 64 字节，验证只有这段变化
	before := make([]byte, len(save.data))
	copy(before, save.data)
	region := e.Bytes()
	regionStart := e.ValueOff

	if err := e.SetBytes([]byte("测试配装名XYZ")); err != nil {
		t.Fatalf("SetBytes 失败: %v", err)
	}
	// region 应为 UTF-8 + NUL 填充
	if got := entryText(e); got != "测试配装名XYZ" {
		t.Fatalf("回读名称=%q，期望 测试配装名XYZ", got)
	}
	// 尾部必须是 0 填充
	raw := e.Bytes()
	tail := len("测试配装名XYZ")
	for i := tail; i < len(raw); i++ {
		if raw[i] != 0 {
			t.Fatalf("尾部字节 %d 非零: %d", i, raw[i])
		}
	}
	// 除 [regionStart, regionStart+64) 外，全档必须与写前一致
	for i := 0; i < len(save.data); i++ {
		if i >= regionStart && i < regionStart+len(region) {
			continue
		}
		if save.data[i] != before[i] {
			t.Fatalf("写名称越界！偏移 %d 被改动（区域 [%d,%d)）", i, regionStart, regionStart+len(region))
		}
	}
	t.Log("字节写入未触碰相邻数据 ✓")

	// 超长必须报错，不截断
	huge := make([]byte, 65)
	if err := e.SetBytes(huge); err == nil {
		t.Fatal("SetBytes 应拒绝超过 64 字节的写入")
	}
}

// 校验写入闸门的存在性与只读上下文（不实际写盘，只走内存态往返）。
func TestLoadoutApplyDryRoundtripInMemory(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	// 拷到临时目录，避免碰用户真实存档
	tmp := filepath.Join(t.TempDir(), "sd.dat")
	in, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp, in, 0644); err != nil {
		t.Fatal(err)
	}
	save, err := LoadSave(tmp)
	if err != nil {
		t.Fatal(err)
	}
	// 冒烟：三个向量字段的 ValueCnt 与设计前提一致
	for _, tc := range []struct {
		id   uint32
		unit uint32
		want int
	}{
		{loadoutNameIDType, 20045, 64},
		{loadoutWeaponIDType, 20045, 1},
		{loadoutSigilsIDType, 20045, 13},
		{loadoutSkillsIDType, 20045, 4},
		{loadoutMasteryIDType, 20045, 50},
		{loadoutCharIDType, 20045, 1},
	} {
		e, ok := save.findUnitExact(tc.id, tc.unit)
		if !ok {
			t.Fatalf("找不到 IDType=%d @%d", tc.id, tc.unit)
		}
		if e.ValueCnt != tc.want {
			t.Fatalf("IDType=%d @%d ValueCnt=%d，期望 %d", tc.id, tc.unit, e.ValueCnt, tc.want)
		}
	}
	t.Log("六字段 ValueCnt 与设计前提一致 ✓")
}

// 在真实存档的副本上端到端验证 LoadoutApply：clone / write / clear 三种操作 + 回读。
func TestLoadoutApplyEndToEnd(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	app := &App{}

	// 拷贝到临时目录，绝不碰用户真实存档
	work := filepath.Join(t.TempDir(), "sd.dat")
	in, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, in, 0644); err != nil {
		t.Fatal(err)
	}

	// 用 LoadoutList 找伊欧（有 6 套配装的角色）及其一个空槽
	groups, err := app.LoadoutList(work)
	if err != nil {
		t.Fatal(err)
	}
	var io *CharacterLoadouts
	for i := range groups {
		if groups[i].CharaName == "伊欧" {
			io = &groups[i]
			break
		}
	}
	if io == nil {
		t.Skip("测试存档里没有伊欧")
	}
	ctx, err := app.LoadoutEditContext(work, io.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("伊欧 ownerCode=%q，武器池=%d，因子池=%d，技能池=%d，专精来源=%d",
		ctx.OwnerCode, len(ctx.Weapons), len(ctx.Sigils), len(ctx.Skills), len(ctx.MasterySources))
	if ctx.OwnerCode == "" {
		t.Error("未能推导伊欧的 ownerCode（应为 PL0400）")
	}

	// 找一个空槽做写入目标
	var emptySlot uint32
	var srcSlot uint32
	for _, s := range ctx.Slots {
		if !s.Occupied && emptySlot == 0 {
			emptySlot = s.UnitID
		}
		if s.Occupied && srcSlot == 0 {
			srcSlot = s.UnitID
		}
	}
	if emptySlot == 0 || srcSlot == 0 {
		t.Skip("伊欧没有同时具备空槽和占用槽")
	}

	// ── 1. clone：把占用槽整套克隆到空槽 ──
	res, err := app.LoadoutApply(work, work, []LoadoutWrite{{
		UnitID: emptySlot, ExpectCharaHash: io.CharaHash, Op: "clone", CloneFromUnitID: srcSlot,
	}})
	if err != nil {
		t.Fatalf("clone 失败: %v", err)
	}
	t.Logf("clone: 写入=%d 验证字段=%d 有备份=%v", res.SlotsWritten, res.VerifiedFields, res.BackupPath != "")
	if res.SlotsWritten != 1 || res.VerifiedFields < 6 {
		t.Errorf("clone 结果异常: %+v", res)
	}
	// 验证克隆后目标槽与源槽的因子/专精一致
	g2, _ := app.LoadoutList(work)
	var clonedName, srcName string
	for _, gg := range g2 {
		if gg.CharaName != "伊欧" {
			continue
		}
		for _, lo := range gg.Loadouts {
			if lo.UnitID == emptySlot {
				clonedName = lo.Name
			}
			if lo.UnitID == srcSlot {
				srcName = lo.Name
			}
		}
	}
	if clonedName != srcName {
		t.Errorf("克隆后名称=%q，源=%q", clonedName, srcName)
	}

	// ── 2. write：自定义写入（用资源池里的第一件武器 + 前 3 个因子 + 专精来源整套）──
	w := LoadoutWrite{
		UnitID: emptySlot, ExpectCharaHash: io.CharaHash, Op: "write",
		Name: "自动测试配装",
	}
	if len(ctx.Weapons) > 0 {
		w.WeaponSlotID = ctx.Weapons[0].SlotID
	}
	for i, s := range ctx.Sigils {
		if i >= 3 {
			break
		}
		w.SigilSlotIDs = append(w.SigilSlotIDs, s.SlotID)
	}
	for i, s := range ctx.Skills {
		if i >= 2 {
			break
		}
		w.SkillHashes = append(w.SkillHashes, s.Hash)
	}
	if len(ctx.MasterySources) > 0 {
		w.MasteryHashes = ctx.MasterySources[0].NodeHashes
	}
	res, err = app.LoadoutApply(work, work, []LoadoutWrite{w})
	if err != nil {
		t.Fatalf("write 失败: %v", err)
	}
	t.Logf("write: 验证字段=%d", res.VerifiedFields)
	if res.VerifiedFields < 6 {
		t.Errorf("write 验证字段不足: %d", res.VerifiedFields)
	}
	// 回读名称必须是新名
	g3, _ := app.LoadoutList(work)
	ok := false
	for _, gg := range g3 {
		for _, lo := range gg.Loadouts {
			if lo.UnitID == emptySlot && lo.Name == "自动测试配装" {
				ok = true
			}
		}
	}
	if !ok {
		t.Error("write 后未读到新名称")
	}

	// ── 3. clear：清空 ──
	res, err = app.LoadoutApply(work, work, []LoadoutWrite{{
		UnitID: emptySlot, ExpectCharaHash: io.CharaHash, Op: "clear",
	}})
	if err != nil {
		t.Fatalf("clear 失败: %v", err)
	}
	if res.SlotsCleared != 1 {
		t.Errorf("clear 结果异常: %+v", res)
	}
	// 清空后该槽不再出现在 LoadoutList（3003=EmptyHash 被跳过）
	g4, _ := app.LoadoutList(work)
	for _, gg := range g4 {
		for _, lo := range gg.Loadouts {
			if lo.UnitID == emptySlot && !lo.IsParty {
				t.Errorf("clear 后槽 %d 仍出现在列表", emptySlot)
			}
		}
	}
}

func TestLoadoutApplyPreservesSparse1403PositionsAndZeroPadding(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	work := filepath.Join(t.TempDir(), "sparse-loadout.dat")
	raw, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, raw, 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	groups, err := app.LoadoutList(work)
	if err != nil {
		t.Fatal(err)
	}
	var group *CharacterLoadouts
	for i := range groups {
		if groups[i].CharaName == "伊欧" {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		t.Skip("测试存档里没有伊欧")
	}
	ctx, err := app.LoadoutEditContext(work, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Sigils) < 3 {
		t.Skip("伊欧没有足够的可验证因子")
	}
	var target, cloneTarget uint32
	for _, slot := range ctx.Slots {
		if !slot.Occupied {
			if target == 0 {
				target = slot.UnitID
			} else {
				cloneTarget = slot.UnitID
				break
			}
		}
	}
	if target == 0 || cloneTarget == 0 {
		t.Skip("伊欧没有两个空配装槽")
	}

	want := make([]uint32, loadoutMaxSigils)
	want[0] = ctx.Sigils[0].SlotID
	want[4] = ctx.Sigils[1].SlotID
	want[10] = ctx.Sigils[2].SlotID
	if _, err := app.LoadoutApply(work, work, []LoadoutWrite{{
		UnitID: target, ExpectCharaHash: group.CharaHash, Op: "write",
		Name: "稀疏槽位测试", SigilSlotIDs: want,
	}}); err != nil {
		t.Fatalf("固定 12 格因子向量应允许中间空槽: %v", err)
	}

	after, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := after.findUnitExact(loadoutSigilsIDType, target)
	if !ok {
		t.Fatal("回读找不到目标 1403")
	}
	if entry.ValueCnt != 13 {
		t.Fatalf("回读 1403 ValueCnt=%d，期望 13", entry.ValueCnt)
	}
	for index := 0; index < loadoutMaxSigils; index++ {
		got, err := entry.Uint32At(index)
		if err != nil {
			t.Fatal(err)
		}
		if got != want[index] {
			t.Fatalf("1403[%d]=%d，期望 %d", index, got, want[index])
		}
	}
	padding, err := entry.Uint32At(loadoutMaxSigils)
	if err != nil {
		t.Fatal(err)
	}
	if padding != 0 {
		t.Fatalf("1403 第 13 位填充值=%08X，期望恒为 0", padding)
	}

	if _, err := app.LoadoutApply(work, work, []LoadoutWrite{{
		UnitID: cloneTarget, ExpectCharaHash: group.CharaHash, Op: "clone", CloneFromUnitID: target,
	}}); err != nil {
		t.Fatalf("克隆稀疏配装失败: %v", err)
	}
	afterClone, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	cloneEntry, ok := afterClone.findUnitExact(loadoutSigilsIDType, cloneTarget)
	if !ok {
		t.Fatal("克隆后找不到目标 1403")
	}
	for index := 0; index < loadoutMaxSigils; index++ {
		got, err := cloneEntry.Uint32At(index)
		if err != nil {
			t.Fatal(err)
		}
		if got != want[index] {
			t.Fatalf("克隆后 1403[%d]=%d，期望保留稀疏值 %d", index, got, want[index])
		}
	}
	if padding, err := cloneEntry.Uint32At(loadoutMaxSigils); err != nil || padding != 0 {
		t.Fatalf("克隆后 1403 第 13 位=%08X err=%v，期望 0", padding, err)
	}

	groups, err = app.LoadoutList(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range groups {
		for _, loadout := range item.Loadouts {
			if loadout.UnitID != target {
				continue
			}
			if len(loadout.Sigils) != 3 {
				t.Fatalf("稀疏回读因子数=%d，期望 3", len(loadout.Sigils))
			}
			for i, index := range []int{0, 4, 10} {
				if loadout.Sigils[i].Index != index || loadout.Sigils[i].SlotID != want[index] {
					t.Fatalf("稀疏回读第 %d 项=%+v，期望 index=%d SlotID=%d", i, loadout.Sigils[i], index, want[index])
				}
			}
			return
		}
	}
	t.Fatal("写后 LoadoutList 没有读到目标配装")
}

// 配装编辑器的「构造新因子」必须走独立生成器实例，并且只在存档副本上新增、回读成功。
func TestLoadoutConstructSigilOnSaveCopy(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	work := filepath.Join(t.TempDir(), "construct.dat")
	raw, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, raw, 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewSigilGen()
	sigils, err := gen.GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	var chosen SigilInfo
	for _, sigil := range sigils {
		if sigil.Constructible && sigil.SupportsSecondaryTrait && len(sigil.AllowedSigilLevels) > 0 && len(sigil.AllowedFirstTraitLevels) > 0 {
			chosen = sigil
			break
		}
	}
	if chosen.InternalID == "" {
		t.Fatal("目录中没有可构造的双词条因子")
	}
	secondaries, err := gen.GetCompatibleSecondaryTraits(chosen.InternalID)
	if err != nil || len(secondaries) == 0 {
		t.Fatalf("读取副词条失败: %v (%d)", err, len(secondaries))
	}
	var secondary TraitInfo
	var secondaryLevels []int
	for _, candidate := range secondaries {
		levels, levelErr := gen.GetSecondaryTraitLevels(chosen.InternalID, candidate.InternalID)
		if levelErr == nil && len(levels) > 0 {
			secondary = candidate
			secondaryLevels = levels
			break
		}
	}
	if secondary.InternalID == "" {
		t.Fatal("目录中没有带合法等级的副词条")
	}
	item := QueueItem{
		SigilID: chosen.InternalID, Level: chosen.AllowedSigilLevels[len(chosen.AllowedSigilLevels)-1],
		PrimaryLevel:     chosen.AllowedFirstTraitLevels[len(chosen.AllowedFirstTraitLevels)-1],
		SecondaryTraitID: secondary.InternalID, SecondaryLevel: secondaryLevels[len(secondaryLevels)-1], Quantity: 1,
	}

	before, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	beforeCount := before.GetOccupiedGemCount()
	result, err := (&App{}).LoadoutConstructSigil(work, item)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || result.VerifiedCount != 1 {
		t.Fatalf("构造结果异常: %+v", result)
	}
	if len(result.SlotIDs) != 1 || result.SlotIDs[0] == 0 {
		t.Fatalf("构造结果没有返回可用于配装的因子槽位: %+v", result)
	}
	if result.BackupPath == "" {
		t.Fatalf("构造结果没有返回写入前备份路径: %+v", result)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("构造结果返回的备份不存在 %q: %v", result.BackupPath, err)
	}
	after, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.GetOccupiedGemCount(); got != beforeCount+1 {
		t.Fatalf("因子数量=%d，期望 %d", got, beforeCount+1)
	}
}

func naturalConstructedSigilItem(t *testing.T) QueueItem {
	t.Helper()
	gen := NewSigilGen()
	sigils, err := gen.GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	for _, sigil := range sigils {
		if !sigil.Constructible || sigil.InternalID == "GEEN_142_02" || sigil.Category == "character_sigil" ||
			!sigil.SupportsSecondaryTrait || len(sigil.AllowedSigilLevels) == 0 || sigil.FirstTraitMaxLevel < 15 {
			continue
		}
		secondaries, err := gen.GetCompatibleSecondaryTraits(sigil.InternalID)
		if err != nil {
			continue
		}
		for _, secondary := range secondaries {
			levels, err := gen.GetSecondaryTraitLevels(sigil.InternalID, secondary.InternalID)
			if err != nil || !containsNaturalLevel(levels, 15) || secondary.InternalID == sigil.PrimaryTraitID {
				continue
			}
			return QueueItem{
				SigilID: sigil.InternalID, Level: sigil.AllowedSigilLevels[len(sigil.AllowedSigilLevels)-1],
				PrimaryLevel:     15,
				SecondaryTraitID: secondary.InternalID, SecondaryLevel: 15, Quantity: 1,
			}
		}
	}
	t.Fatal("目录中没有可用于原子配装写入测试的自然双词条因子")
	return QueueItem{}
}

// 构造草稿不是一次独立的背包写入：它必须和配装替换共用同一个
// SaveData 缓冲、一次 FixChecksums/Write 和一次严格回读。
func TestLoadoutApplyAtomicallyConstructsAndBindsSigilOnSaveCopy(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	work := filepath.Join(t.TempDir(), "atomic-construct.dat")
	raw, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, raw, 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	groups, err := app.LoadoutList(work)
	if err != nil {
		t.Fatal(err)
	}
	var group *CharacterLoadouts
	for i := range groups {
		if groups[i].CharaName == "伊欧" {
			group = &groups[i]
			break
		}
	}
	if group == nil {
		t.Skip("测试存档里没有伊欧")
	}
	ctx, err := app.LoadoutEditContext(work, group.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	var target uint32
	for _, slot := range ctx.Slots {
		if slot.Occupied {
			target = slot.UnitID
			break
		}
	}
	if target == 0 {
		t.Skip("伊欧没有可用于替换测试的已有配装")
	}

	before, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	beforeCount := before.GetOccupiedGemCount()
	beforeMaxSlotID, err := before.GetMaxSlotID()
	if err != nil {
		t.Fatal(err)
	}
	weaponSID := uint32(0)
	if entry, ok := before.findUnitExact(loadoutWeaponIDType, target); ok {
		weaponSID = entry.Uint32()
	}
	toHex := func(values []uint32) []string {
		out := make([]string, len(values))
		for i, value := range values {
			out[i] = fmt.Sprintf("%08X", value)
		}
		return out
	}
	item := naturalConstructedSigilItem(t)
	write := LoadoutWrite{
		UnitID: target, ExpectCharaHash: group.CharaHash, Op: "write",
		Name: entryTextAt(before, target), WeaponSlotID: weaponSID,
		SigilSlotIDs:      readVec(before, loadoutSigilsIDType, target, loadoutMaxSigils),
		SkillHashes:       toHex(readVec(before, loadoutSkillsIDType, target, loadoutMaxSkills)),
		MasteryHashes:     toHex(readVec(before, loadoutMasteryIDType, target, loadoutMaxMastery)),
		ConstructedSigils: []LoadoutConstructedSigil{{Index: 0, Item: item}},
	}

	result, err := app.LoadoutApply(work, work, []LoadoutWrite{write})
	if err != nil {
		t.Fatal(err)
	}
	if result.CreatedCount != 1 || result.VerifiedCount != 1 || len(result.SlotIDs) != 1 {
		t.Fatalf("原子构造结果异常: %+v", result)
	}
	if result.SlotIDs[0] != uint32(beforeMaxSlotID+1) {
		t.Fatalf("新 SlotID=%d，期望 %d", result.SlotIDs[0], beforeMaxSlotID+1)
	}
	backups, err := filepath.Glob(work + ".pre-edit-*.bak")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("应只有一次落盘备份，实际 %d 个: %v", len(backups), backups)
	}

	after, err := LoadSave(work)
	if err != nil {
		t.Fatal(err)
	}
	if got := after.GetOccupiedGemCount(); got != beforeCount+1 {
		t.Fatalf("因子占用数=%d，期望 %d", got, beforeCount+1)
	}
	loadoutSigils, ok := after.findUnitExact(loadoutSigilsIDType, target)
	if !ok {
		t.Fatal("回读找不到目标配装因子字段")
	}
	boundSlotID, err := loadoutSigils.Uint32At(0)
	if err != nil {
		t.Fatal(err)
	}
	if boundSlotID != result.SlotIDs[0] {
		t.Fatalf("目标配装第 1 个因子引用=%d，期望新因子 %d", boundSlotID, result.SlotIDs[0])
	}
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{Index: 0, Item: item})
	if err != nil {
		t.Fatal(err)
	}
	afterIndex := buildLoadoutIndex(after)
	gemUnitID, ok := afterIndex.gemBySlotID[result.SlotIDs[0]]
	if !ok {
		t.Fatalf("背包索引找不到新因子 SlotID %d", result.SlotIDs[0])
	}
	if err := after.VerifySigil(
		int(gemUnitID), result.SlotIDs[0], prepared.sigilHash, prepared.item.Level,
		prepared.primaryHash, prepared.item.PrimaryLevel,
		prepared.secondaryHash, prepared.secondaryLevel, prepared.hasSecondary,
	); err != nil {
		t.Fatalf("新因子全部字段回读不符: %v", err)
	}
}

func TestPrepareLoadoutSigilRejectsNonNaturalDraftValues(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	item := naturalConstructedSigilItem(t)

	badLevel := item
	badLevel.Level = 999
	if _, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{Index: 0, Item: badLevel}); err == nil || !strings.Contains(err.Error(), "自然范围") {
		t.Fatalf("应拒绝目录外因子等级，实际错误: %v", err)
	}
	badPrimaryLevel := item
	badPrimaryLevel.PrimaryLevel = 16
	if _, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{Index: 0, Item: badPrimaryLevel}); err == nil || !strings.Contains(err.Error(), "自然范围") {
		t.Fatalf("应拒绝存储范围内但非自然的主词条等级，实际错误: %v", err)
	}

	sigil, err := cat.RequireSigil(item.SigilID)
	if err != nil {
		t.Fatal(err)
	}
	badSecondary := item
	badSecondary.SecondaryTraitID = sigil.PrimaryTraitID
	if _, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{Index: 0, Item: badSecondary}); err == nil || !strings.Contains(err.Error(), "主副词条") {
		t.Fatalf("应拒绝不兼容的同名主副词条，实际错误: %v", err)
	}

	badSecondaryLevel := item
	badSecondaryLevel.SecondaryLevel = 16
	if _, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{Index: 0, Item: badSecondaryLevel}); err == nil || !strings.Contains(err.Error(), "自然范围") {
		t.Fatalf("应拒绝存储范围内但非自然的副词条等级，实际错误: %v", err)
	}

	broadFallback := QueueItem{
		SigilID: "GEEN_146_24", Level: 15, PrimaryLevel: 15,
		SecondaryTraitID: "SKILL_000_00", SecondaryLevel: 15, Quantity: 1,
	}
	if _, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{Index: 0, Item: broadFallback}); err == nil ||
		!strings.Contains(err.Error(), "可信") || !strings.Contains(err.Error(), "自然生成") {
		t.Fatalf("应拒绝只有临时宽泛池、没有显式兼容白名单的副词条组合，实际错误: %v", err)
	}
}

func TestLoadoutComplianceReportMatchesWritePreflightWithoutMutatingSave(t *testing.T) {
	path := badgeTestSaveCopy(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	app := &App{}
	groups, err := app.LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	var chara *CharacterLoadouts
	for index := range groups {
		if groups[index].CharaName == "伊欧" {
			chara = &groups[index]
			break
		}
	}
	if chara == nil {
		t.Skip("测试存档里没有伊欧")
	}
	ctx, err := app.LoadoutEditContext(path, chara.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Slots) == 0 {
		t.Skip("伊欧没有配装槽")
	}

	valid := LoadoutWrite{
		UnitID: ctx.Slots[0].UnitID, ExpectCharaHash: chara.CharaHash, Op: "write", Name: "合规预检",
		ConstructedSigils: []LoadoutConstructedSigil{{Index: 0, Item: naturalConstructedSigilItem(t)}},
	}
	report, err := app.LoadoutCheckCompliance(path, valid)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Writable || report.Status != LegalityLegal || len(report.Items) != 1 || report.Items[0].Status != LegalityLegal {
		t.Fatalf("valid natural draft report=%+v", report)
	}

	invalid := valid
	invalid.ConstructedSigils = append([]LoadoutConstructedSigil(nil), valid.ConstructedSigils...)
	invalid.ConstructedSigils[0].Item.Level = 999
	report, err = app.LoadoutCheckCompliance(path, invalid)
	if err != nil {
		t.Fatal(err)
	}
	if report.Writable || report.Status != LegalityImpossible || len(report.Items) != 1 || report.Items[0].Status != LegalityImpossible {
		t.Fatalf("impossible draft report=%+v", report)
	}
	if !strings.Contains(report.Message, "自然范围") || !strings.Contains(report.Items[0].Message, "自然范围") {
		t.Fatalf("compliance reasons must explain the rejected value: %+v", report)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("read-only compliance check changed the save")
	}
}

func TestPrepareLoadoutSigilRejectsCatalogEntriesWithoutConstructibleProof(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	sigil, err := cat.RequireSigil("GEEN_000_24")
	if err != nil {
		t.Fatal(err)
	}
	if cat.IsSigilConstructible(sigil) {
		t.Fatal("测试前提失效：GEEN_000_24 不应是可信可构造项")
	}
	sigilLevels, err := cat.RequireSigilLevels(sigil)
	if err != nil || len(sigilLevels) == 0 {
		t.Fatalf("读取因子等级失败: %v", err)
	}
	primaryLevels, err := cat.RequirePrimaryTraitLevels(sigil)
	if err != nil || len(primaryLevels) == 0 {
		t.Fatalf("读取主词条等级失败: %v", err)
	}
	_, err = prepareLoadoutSigil(cat, LoadoutConstructedSigil{
		Index: 0,
		Item: QueueItem{
			SigilID: sigil.InternalID, Level: sigilLevels[0], PrimaryLevel: primaryLevels[0],
		},
	})
	if err == nil || !strings.Contains(err.Error(), "可信") {
		t.Fatalf("配装原子构造必须拒绝非可信目录项，实际错误: %v", err)
	}
}

func TestLoadoutSigilConstructionRejectsUnverifiedSevenNet(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareLoadoutSigil(cat, LoadoutConstructedSigil{
		Index: 0, Item: QueueItem{SigilID: "GEEN_142_02", Level: 1, PrimaryLevel: 1},
	}); err == nil || !strings.Contains(err.Error(), "GEEN_142_02") {
		t.Fatalf("配装原子构造应拒绝 GEEN_142_02，实际错误: %v", err)
	}

	originalFinder := loadoutFindProcessByName
	loadoutFindProcessByName = func(string) (uint32, error) { return 0, errors.New("not running") }
	t.Cleanup(func() { loadoutFindProcessByName = originalFinder })
	_, err = (&App{}).LoadoutConstructSigil(filepath.Join(t.TempDir(), "missing.dat"), QueueItem{SigilID: "geen_142_02"})
	if err == nil || !strings.Contains(err.Error(), "GEEN_142_02") {
		t.Fatalf("独立构造兼容入口也应拒绝 GEEN_142_02，实际错误: %v", err)
	}
}

func TestLoadoutConstructSigilRejectsWhenGameRunning(t *testing.T) {
	originalFinder := loadoutFindProcessByName
	loadoutFindProcessByName = func(name string) (uint32, error) {
		if name != charaProcessName {
			t.Fatalf("进程闸门检查了错误的进程名 %q", name)
		}
		return 4242, nil
	}
	t.Cleanup(func() { loadoutFindProcessByName = originalFinder })

	_, err := (&App{}).LoadoutConstructSigil(filepath.Join(t.TempDir(), "missing.dat"), QueueItem{})
	if err == nil || !strings.Contains(err.Error(), "完全退出游戏") {
		t.Fatalf("游戏运行时应在读取存档前拒绝构造，实际错误: %v", err)
	}
}

func queuedSigilGeneratorOnSaveCopy(t *testing.T) (*SigilGen, string, uint32) {
	t.Helper()
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	work := filepath.Join(t.TempDir(), "strict-verify.dat")
	raw, err := os.ReadFile(testLoadoutSave)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(work, raw, 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewSigilGen()
	if _, err := gen.LoadSaveFile(work); err != nil {
		t.Fatal(err)
	}
	items, err := gen.GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	var chosen SigilInfo
	for _, item := range items {
		if len(item.AllowedSigilLevels) > 0 && len(item.AllowedFirstTraitLevels) > 0 {
			chosen = item
			break
		}
	}
	if chosen.InternalID == "" {
		t.Fatal("目录中没有可用于严格回读测试的因子")
	}
	if err := gen.AddToQueue(QueueItem{
		SigilID:      chosen.InternalID,
		Level:        chosen.AllowedSigilLevels[len(chosen.AllowedSigilLevels)-1],
		PrimaryLevel: chosen.AllowedFirstTraitLevels[len(chosen.AllowedFirstTraitLevels)-1],
		Quantity:     1,
	}); err != nil {
		t.Fatal(err)
	}
	maxSlotID, err := gen.save.GetMaxSlotID()
	if err != nil {
		t.Fatal(err)
	}
	return gen, work, uint32(maxSlotID + 1)
}

func TestSigilApplyQueueReturnsReadbackLoadError(t *testing.T) {
	gen, work, _ := queuedSigilGeneratorOnSaveCopy(t)
	sentinel := errors.New("forced readback load failure")
	gen.loadSaveForVerification = func(string) (*SaveData, error) { return nil, sentinel }

	_, err := gen.ApplyQueue(work)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ApplyQueue 应返回回读加载错误，实际: %v", err)
	}
}

func TestSigilApplyQueueRejectsSlotIDReadbackMismatch(t *testing.T) {
	gen, work, expectedSlotID := queuedSigilGeneratorOnSaveCopy(t)
	gen.loadSaveForVerification = func(path string) (*SaveData, error) {
		verifySave, err := LoadSave(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range verifySave.findAllUnitsByType(GemSlotIDType) {
			if entry.Uint32() == expectedSlotID {
				entry.SetUint32(expectedSlotID + 1000)
				return verifySave, nil
			}
		}
		return nil, fmt.Errorf("回读存档中找不到新槽位 %d", expectedSlotID)
	}

	_, err := gen.ApplyQueue(work)
	if err == nil || !strings.Contains(err.Error(), "槽位ID") {
		t.Fatalf("ApplyQueue 应拒绝 2702 SlotID 回读不符，实际: %v", err)
	}
}

// 校验各类非法写入都被拒绝（毁档防线）。
func TestSigilConstructorCatalogLocalizesPrimaryTraitName(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	setCurrentLanguage("zh")
	defer setCurrentLanguage(previousLanguage)
	if got := cnTrait("Mage's Savvy"); got != "魔法师的伶俐" {
		t.Fatalf("Mage's Savvy 译名=%q", got)
	}

	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.DisplayName == "霸体" {
			if item.PrimaryTraitName != "霸体" {
				t.Fatalf("构造器主词条没有本地化：%q", item.PrimaryTraitName)
			}
			return
		}
	}
	t.Fatal("构造目录中未找到游戏原名霸体")
}

func TestSigilConstructorCatalogExposesCategory(t *testing.T) {
	items, err := NewSigilGen().GetSigilList()
	if err != nil {
		t.Fatal(err)
	}
	characterSigils := 0
	for _, item := range items {
		if item.Category == "character_sigil" {
			characterSigils++
		}
	}
	if characterSigils != 91 {
		t.Fatalf("角色专属因子分类数=%d，期望 91", characterSigils)
	}
}

func TestLoadoutApplyRejections(t *testing.T) {
	if !haveSave(testLoadoutSave) {
		t.Skipf("测试存档不存在: %s", testLoadoutSave)
	}
	app := &App{}
	work := filepath.Join(t.TempDir(), "sd.dat")
	in, _ := os.ReadFile(testLoadoutSave)
	os.WriteFile(work, in, 0644)

	groups, err := app.LoadoutList(work)
	if err != nil {
		t.Fatal(err)
	}
	var io *CharacterLoadouts
	for i := range groups {
		if groups[i].CharaName == "伊欧" {
			io = &groups[i]
		}
	}
	if io == nil {
		t.Skip("无伊欧")
	}
	ctx, _ := app.LoadoutEditContext(work, io.CharaHash)
	if len(ctx.Skills) == 0 {
		t.Fatal("伊欧技能池为空，无法验证重复技能拒绝")
	}
	if len(ctx.Weapons) == 0 {
		t.Fatal("伊欧武器池为空，无法构造有效的重复技能写入")
	}
	var target uint32
	for _, s := range ctx.Slots {
		if !s.Occupied {
			target = s.UnitID
			break
		}
	}
	if target == 0 {
		t.Skip("伊欧无空槽")
	}
	invalidNaturalDraft := naturalConstructedSigilItem(t)
	invalidNaturalDraft.Level = 999

	cases := []struct {
		name string
		w    LoadoutWrite
	}{
		{"越界槽位", LoadoutWrite{UnitID: 104000, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x"}},
		{"角色hash不符", LoadoutWrite{UnitID: target, ExpectCharaHash: "DEADBEEF", Op: "write", Name: "x"}},
		{"悬空因子SlotID", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", SigilSlotIDs: []uint32{9999999}}},
		{"超长名称", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "这是一个非常非常非常非常非常非常非常非常长的配装名称超过六十三字节上限"}},
		{"重复因子", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", SigilSlotIDs: []uint32{ctx.Sigils[0].SlotID, ctx.Sigils[0].SlotID}}},
		{"重复技能", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", WeaponSlotID: ctx.Weapons[0].SlotID, SkillHashes: []string{ctx.Skills[0].Hash, ctx.Skills[0].Hash}}},
		{"构造因子目录外等级", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", ConstructedSigils: []LoadoutConstructedSigil{{Index: 0, Item: invalidNaturalDraft}}}},
		{"构造因子索引越界", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", ConstructedSigils: []LoadoutConstructedSigil{{Index: 12, Item: naturalConstructedSigilItem(t)}}}},
		{"未验证构造因子", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", ConstructedSigils: []LoadoutConstructedSigil{{Index: 0, Item: QueueItem{SigilID: "GEEN_142_02", Level: 1, PrimaryLevel: 1}}}}},
		{"未知操作", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "frobnicate"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 记录写前文件字节
			before, _ := os.ReadFile(work)
			_, err := app.LoadoutApply(work, work, []LoadoutWrite{c.w})
			if err == nil {
				t.Fatalf("非法写入 %q 竟未被拒绝", c.name)
			}
			if c.name == "重复技能" && !strings.Contains(err.Error(), "重复") {
				t.Fatalf("重复技能应由去重校验拒绝，实际错误: %v", err)
			}
			// 拒绝时磁盘文件不得被触碰
			after, _ := os.ReadFile(work)
			if len(before) != len(after) {
				t.Fatalf("%q 被拒但文件大小变了", c.name)
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("%q 被拒但文件字节 %d 被改动", c.name, i)
				}
			}
		})
	}
}

func TestLoadoutWriteLimitsMasteryByTargetCharacterMasterLevel(t *testing.T) {
	path := requireIsolatedSaveQA(t)
	parsed, err := LoadSaveFile(path)
	if err != nil {
		t.Fatal(err)
	}
	chara, ok := uintUnitExact(parsed.SlotData, 1301, 10002)
	if !ok || len(chara.ValueData) != 1 {
		t.Fatal("isolated save lacks scalar 1301 UnitID 10002")
	}
	charaHash := chara.ValueData[0]

	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadProgressionCatalog(); err != nil {
		t.Fatal(err)
	}
	index := buildLoadoutIndex(save)
	var target uint32
	for _, entry := range save.findAllUnitsByType(loadoutCharIDType) {
		if entry.Uint32() == charaHash && entry.UnitID >= loadoutBase && entry.UnitID < partyLoadoutBase {
			target = entry.UnitID
			break
		}
	}
	if target == 0 {
		t.Fatalf("UnitID 10002 character %08X has no saved loadout block", charaHash)
	}
	ownerCode := index.deriveOwnerCode(save, charaHash)
	if ownerCode == "" {
		t.Fatalf("cannot derive owner code for UnitID 10002 character %08X", charaHash)
	}
	pools, err := (&App{}).MasteryNodePool(ownerCode)
	if err != nil {
		t.Fatal(err)
	}
	var hashes []string
	for _, pool := range pools {
		if pool.Rank != "R1" {
			continue
		}
		for _, node := range pool.Nodes {
			hashes = append(hashes, node.Hash)
			if len(hashes) == 2 {
				break
			}
		}
	}
	if len(hashes) != 2 {
		t.Fatalf("%s has fewer than two R1 nodes", ownerCode)
	}
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateLoadoutWrite(save, index, catalog, LoadoutWrite{
		UnitID: target, ExpectCharaHash: hashText(charaHash), Op: "write", Name: "level-cap-check",
		MasteryHashes: hashes[:1],
	}); err != nil {
		t.Fatalf("Lv1 target should accept exactly one R1 mastery node: %v", err)
	}
	_, err = validateLoadoutWrite(save, index, catalog, LoadoutWrite{
		UnitID: target, ExpectCharaHash: hashText(charaHash), Op: "write", Name: "level-cap-check",
		MasteryHashes: hashes,
	})
	if err == nil {
		t.Fatalf("Lv1 target accepted two R1 mastery nodes: %v", hashes)
	}
	if !strings.Contains(err.Error(), "R1") || !strings.Contains(err.Error(), "1") {
		t.Fatalf("capacity rejection does not identify the target rank/cap: %v", err)
	}
}

func TestLoadoutCloneCannotBypassTargetCharacterMasteryCapacity(t *testing.T) {
	path := requireIsolatedSaveQA(t)
	parsed, err := LoadSaveFile(path)
	if err != nil {
		t.Fatal(err)
	}
	chara, ok := uintUnitExact(parsed.SlotData, 1301, 10002)
	if !ok || len(chara.ValueData) != 1 {
		t.Fatal("isolated save lacks scalar 1301 UnitID 10002")
	}
	charaHash := chara.ValueData[0]
	save, err := LoadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadProgressionCatalog(); err != nil {
		t.Fatal(err)
	}
	index := buildLoadoutIndex(save)

	var source uint32
	for _, entry := range save.findAllUnitsByType(loadoutCharIDType) {
		if entry.Uint32() != charaHash || entry.UnitID < loadoutBase || entry.UnitID >= partyLoadoutBase {
			continue
		}
		source = entry.UnitID
		break
	}
	if source == 0 {
		t.Fatalf("UnitID 10002 character %08X has no saved loadout source", charaHash)
	}
	ownerCode := index.deriveOwnerCode(save, charaHash)
	if ownerCode == "" {
		t.Fatalf("cannot derive owner code for UnitID 10002 character %08X", charaHash)
	}
	pools, err := (&App{}).MasteryNodePool(ownerCode)
	if err != nil {
		t.Fatal(err)
	}
	var mastery []uint32
	for _, pool := range pools {
		if pool.Rank != "R1" {
			continue
		}
		for _, node := range pool.Nodes {
			hash, err := ParseHashHex(node.Hash)
			if err != nil {
				t.Fatal(err)
			}
			mastery = append(mastery, hash)
			if len(mastery) == 2 {
				break
			}
		}
	}
	if len(mastery) != 2 {
		t.Fatalf("%s has fewer than two R1 nodes", ownerCode)
	}
	if err := save.writeLoadoutVector(loadoutMasteryIDType, source, mastery, loadoutMaxMastery, EmptyHash); err != nil {
		t.Fatal(err)
	}
	blockBase := loadoutBase + ((source - loadoutBase) / loadoutSlotsPerChara * loadoutSlotsPerChara)
	target := blockBase
	if target == source {
		target++
	}
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateLoadoutWrite(save, index, catalog, LoadoutWrite{
		UnitID: target, ExpectCharaHash: hashText(charaHash), Op: "clone", CloneFromUnitID: source,
	})
	if err == nil {
		t.Fatalf("Lv1 target cloned over-cap mastery from slot %d", source)
	}
	if !strings.Contains(err.Error(), "Master Lv1") || !strings.Contains(err.Error(), "容量") {
		t.Fatalf("clone capacity rejection is not explicit: %v", err)
	}
}

// Opt-in end-to-end QA for the user-provided disposable save. It writes the
// existing Io loadout back to the same slot, exercising the real backup,
// checksum repair and disk re-read path without changing the logical build.
func TestLoadoutApplyActualSaveOptIn(t *testing.T) {
	if os.Getenv("GBFR_REAL_SAVE_WRITE_QA") != "1" {
		t.Skip("set GBFR_REAL_SAVE_WRITE_QA=1 to exercise the approved real-save write")
	}
	const path = `D:\gbf\Saved\SaveGames\SaveData2.dat`
	groups, err := (&App{}).LoadoutList(path)
	if err != nil {
		t.Fatal(err)
	}
	var source LoadoutEntry
	for _, group := range groups {
		if group.CharaHash != "4D0A60C3" {
			continue
		}
		for _, loadout := range group.Loadouts {
			if loadout.UnitID == 20049 && !loadout.IsParty {
				source = loadout
				break
			}
		}
	}
	if source.UnitID == 0 {
		t.Fatal("真实存档里没有伊欧 UnitID 20049 配装")
	}
	sigilSlotIDs := make([]uint32, loadoutMaxSigils)
	for _, sigil := range source.Sigils {
		if sigil.Index >= 0 && sigil.Index < len(sigilSlotIDs) {
			sigilSlotIDs[sigil.Index] = sigil.SlotID
		}
	}
	skillHashes := make([]string, 0, len(source.Skills))
	for _, skill := range source.Skills {
		skillHashes = append(skillHashes, skill.Hash)
	}
	masteryHashes := make([]string, 0, len(source.Mastery))
	for _, mastery := range source.Mastery {
		masteryHashes = append(masteryHashes, mastery.Hash)
	}
	statContext, err := (&App{}).LoadoutStatContext(path, source.CharaHash)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (&App{}).LoadoutApply(path, path, []LoadoutWrite{{
		UnitID: source.UnitID, ExpectCharaHash: source.CharaHash, Op: "write", Name: source.Name,
		WeaponSlotID: source.WeaponSlotID, SigilSlotIDs: sigilSlotIDs, SkillHashes: skillHashes,
		MasteryHashes: masteryHashes, SummonSlotIDs: statContext.EquippedSummonSlotIDs,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.SlotsWritten != 1 || result.VerifiedFields < 7 || result.BackupPath == "" {
		t.Fatalf("真实写入/回读结果不完整: %+v", result)
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("真实写入没有留下可读取备份 %q: %v", result.BackupPath, err)
	}
	t.Logf("real save write verified: fields=%d backup=%s", result.VerifiedFields, result.BackupPath)
}
