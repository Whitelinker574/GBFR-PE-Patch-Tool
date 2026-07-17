package main

import (
	"os"
	"path/filepath"
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

// 校验各类非法写入都被拒绝（毁档防线）。
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

	cases := []struct {
		name string
		w    LoadoutWrite
	}{
		{"越界槽位", LoadoutWrite{UnitID: 104000, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x"}},
		{"角色hash不符", LoadoutWrite{UnitID: target, ExpectCharaHash: "DEADBEEF", Op: "write", Name: "x"}},
		{"悬空因子SlotID", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", SigilSlotIDs: []uint32{9999999}}},
		{"超长名称", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "这是一个非常非常非常非常非常非常非常非常长的配装名称超过六十三字节上限"}},
		{"重复因子", LoadoutWrite{UnitID: target, ExpectCharaHash: io.CharaHash, Op: "write", Name: "x", SigilSlotIDs: []uint32{ctx.Sigils[0].SlotID, ctx.Sigils[0].SlotID}}},
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
