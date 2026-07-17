package main

import "testing"

func characterCountByName(stats []CharacterStat, name string) (int32, bool) {
	for _, stat := range stats {
		if stat.Name == name {
			return stat.Count, true
		}
	}
	return 0, false
}

func TestCharacterStatsSupportsConvertedAndNewDLCSaves(t *testing.T) {
	data := &SaveDataBinary{UIntTable: []UIntSaveDataUnit{
		{IDType: SaveID_CharacterQuestUse, UnitID: 10009, ValueData: []uint32{91}},
		{IDType: SaveID_CharacterQuestUse, UnitID: 10010, ValueData: []uint32{102}},
		{IDType: SaveID_CharacterQuestUse, UnitID: 10034, ValueData: []uint32{343}},
		{IDType: SaveID_CharacterQuestUse, UnitID: 10040, ValueData: []uint32{404}},
	}}

	oldStats := characterStatsForSave(data, false)
	if got, ok := characterCountByName(oldStats, "菲莉"); !ok || got != 91 {
		t.Fatalf("旧版转换存档菲莉次数 = %d, found=%v; want 91", got, ok)
	}
	if got, ok := characterCountByName(oldStats, "芙劳"); !ok || got != 404 {
		t.Fatalf("旧版转换存档芙劳次数 = %d, found=%v; want 404", got, ok)
	}

	newStats := characterStatsForSave(data, true)
	if got, ok := characterCountByName(newStats, "菲莉"); !ok || got != 102 {
		t.Fatalf("DLC 新建存档菲莉次数 = %d, found=%v; want 102", got, ok)
	}
	if got, ok := characterCountByName(newStats, "芙劳"); !ok || got != 343 {
		t.Fatalf("DLC 新建存档芙劳次数 = %d, found=%v; want 343", got, ok)
	}
}

func TestQuestCodesPreserveDLCAlphanumericIDs(t *testing.T) {
	if got := storedToQuestCode(0x40A301); got != "40A301" {
		t.Fatalf("quest code = %q; want 40A301", got)
	}
	if got := storedToQuestID(0x409321); got != 409321 {
		t.Fatalf("decimal quest id = %d; want 409321", got)
	}
	if got := storedToQuestID(0x40A301); got != 0x40A301 {
		t.Fatalf("alphanumeric quest id must retain packed value, got %X", got)
	}
	// 0x406367 is a base-game quest with no entry in any available text table,
	// so it should still fall back to the uncatalogued format. (0x40A301 now
	// resolves to a real name, "新世界秩序", after the quest table was expanded.)
	if got := questIDToNameCN(0x406367); got != "未收录任务 · 406367" {
		t.Fatalf("DLC fallback name = %q", got)
	}
}
