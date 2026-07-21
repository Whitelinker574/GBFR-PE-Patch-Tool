package backend

import (
	"encoding/binary"
	"testing"
)

// 槽位号只能取块内偏移。曾用 20000+(角色序号-1)*15 反推块基址，被实测推翻：
// SaveData1 的伊欧在 20060，而 SaveData2 的 20060 属于欧根（存档有转换/DLC
// 两套角色布局），且古兰（序号 0）会算出负基址、全部预设显示成「槽00」。
func TestLoadoutSlotOf(t *testing.T) {
	cases := []struct {
		name      string
		unit      uint32
		wantSlot  int
		wantParty bool
	}{
		// 实测 SaveData2(3).dat：伊欧 6 套预设连续落在块 20045。
		{"伊欧槽1", 20045, 1, false},
		{"伊欧槽6", 20050, 6, false},
		// 实测 SaveData1.dat：同一个伊欧却在块 20060——块基址与角色序号无关。
		{"另一存档的伊欧仍是槽1", 20060, 1, false},
		// 首个块（实测为姬塔）；也是古兰当主角时会落到的位置。
		{"块起始即槽1", 20000, 1, false},
		{"块内末槽", 20014, 15, false},
		{"下一块回到槽1", 20015, 1, false},
		// 队伍实时配装，不是玩家保存的预设。
		{"队伍成员1", 104000, 1, true},
		{"队伍成员4", 104003, 4, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			slot, party := loadoutSlotOf(c.unit)
			if slot != c.wantSlot || party != c.wantParty {
				t.Errorf("loadoutSlotOf(%d) = (槽%d, party=%v), 期望 (槽%d, party=%v)",
					c.unit, slot, party, c.wantSlot, c.wantParty)
			}
		})
	}
}

// 预设槽位必须恒落在游戏 UI 的 1..15，任何 UnitID 都不得算出 0 或 16+。
func TestLoadoutSlotAlwaysInRange(t *testing.T) {
	for u := uint32(loadoutBase); u < loadoutBase+15*40; u++ {
		slot, party := loadoutSlotOf(u)
		if party {
			t.Fatalf("UnitID %d 不应判为队伍配装", u)
		}
		if slot < 1 || slot > loadoutSlotsPerChara {
			t.Fatalf("UnitID %d 算出越界槽位 %d", u, slot)
		}
	}
}

// vecLen 必须钳制 ValueCnt：tryReadUnitEntry 不校验它与剩余字节的关系，
// 损坏/伪造存档可给出巨大的 ValueCnt，照此预分配会直接 OOM。
func TestVecLenClampsHostileValueCnt(t *testing.T) {
	if n := vecLen(nil); n != 0 {
		t.Errorf("vecLen(nil) = %d, 期望 0", n)
	}
	if n := vecLen(&unitEntry{ValueCnt: 0}); n != 0 {
		t.Errorf("vecLen(空) = %d, 期望 0", n)
	}
	if n := vecLen(&unitEntry{ValueCnt: 50}); n != 50 {
		t.Errorf("vecLen(50) = %d, 期望 50（专精 3007 的实际长度）", n)
	}
	if n := vecLen(&unitEntry{ValueCnt: 1 << 30}); n != maxLoadoutVec {
		t.Errorf("vecLen(2^30) = %d, 期望钳制到 %d", n, maxLoadoutVec)
	}
	if n := vecLen(&unitEntry{ValueCnt: -1}); n != 0 {
		t.Errorf("vecLen(-1) = %d, 期望 0", n)
	}
}

func TestReadLoadoutSigilSlotsPreservesSparse1403Indexes(t *testing.T) {
	values := []uint32{3311, 0, EmptyHash, 3314, 0, 0, 0, 0, 0, 0, 3321, 0, 0}
	data := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(data[i*4:], value)
	}
	entry := &unitEntry{ValueCnt: len(values), data: data}

	got := readLoadoutSigilSlots(entry)
	if len(got) != 3 {
		t.Fatalf("读取到 %d 个有效因子，期望 3 个: %+v", len(got), got)
	}
	wantIndexes := []int{0, 3, 10}
	wantSlotIDs := []uint32{3311, 3314, 3321}
	for i := range got {
		if got[i].Index != wantIndexes[i] || got[i].SlotID != wantSlotIDs[i] {
			t.Fatalf("因子 %d = index %d / SlotID %d，期望 index %d / SlotID %d",
				i, got[i].Index, got[i].SlotID, wantIndexes[i], wantSlotIDs[i])
		}
	}
}
