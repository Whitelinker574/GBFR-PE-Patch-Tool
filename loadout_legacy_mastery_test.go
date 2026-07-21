package main

import "testing"

func TestLegacyMasteryFullIoFourTabsFrom202Tables(t *testing.T) {
	weapons := []legacyMasteryWeaponState{
		{InternalID: "WEP_PL0400_01", Level: 150, Uncap: 6, Transcendence: 6},
		{InternalID: "WEP_PL0400_02", Level: 150, Uncap: 6, Transcendence: 6},
		{InternalID: "WEP_PL0400_03", Level: 150, Uncap: 6, Awakening: 10, Transcendence: 7},
		{InternalID: "WEP_PL0400_04", Level: 150, Uncap: 6, Transcendence: 7},
		{InternalID: "WEP_PL0400_05", Level: 150, Uncap: 6, Transcendence: 6},
		{InternalID: "WEP_PL0400_06", Level: 150, Uncap: 6, Awakening: 10, Transcendence: 6},
	}
	growth, err := deriveLegacyMasteryGrowth("PL0400", 1159403, weapons)
	if err != nil {
		t.Fatal(err)
	}
	if !growth.Complete || growth.Evidence != "extracted-2.0.2-tables+save-completion" {
		t.Fatalf("Io legacy mastery completion=%+v", growth)
	}
	if growth.AttackTab.Attack != 4682 || growth.AttackTab.CritRate != 40 || growth.AttackTab.StunRaw != 6.1 {
		t.Fatalf("Io attack tab=%+v", growth.AttackTab)
	}
	if growth.DefenseTab.HP != 33300 {
		t.Fatalf("Io defense tab=%+v", growth.DefenseTab)
	}
	if growth.CollectionTab.Attack != 600 || growth.CollectionTab.HP != 10000 || growth.CollectionTab.CritRate != 5 || growth.CollectionTab.StunRaw != 1.9 {
		t.Fatalf("Io collection tab=%+v", growth.CollectionTab)
	}
	if growth.TranscendenceTab.Attack != 3300 || growth.TranscendenceTab.HP != 12400 || growth.TranscendenceTab.CritRate != 33 || growth.TranscendenceTab.StunRaw != 2.5 {
		t.Fatalf("Io transcendence tab=%+v", growth.TranscendenceTab)
	}
	if growth.Total.Attack != 8582 || growth.Total.HP != 55700 || growth.Total.CritRate != 78 || growth.Total.StunRaw != 10.5 || growth.Total.StunPanel != 105 {
		t.Fatalf("Io legacy mastery total=%+v", growth.Total)
	}
}

func TestLegacyMasteryDoesNotInventPartialProgress(t *testing.T) {
	growth, err := deriveLegacyMasteryGrowth("PL0400", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if growth.Complete || growth.Total != (LoadoutPermanentPanelStats{}) || growth.Evidence != "partial-save-progress-unresolved" {
		t.Fatalf("partial legacy mastery must remain unresolved: %+v", growth)
	}
}

func TestRuntimeLegacyMasteryAggregateSubtractsOverLimitRawUnits(t *testing.T) {
	observed := LoadoutPermanentPanelStats{Attack: 8582, HP: 55700, CritRate: 78, StunRaw: 14.5}
	overLimit := []LoadoutOverLimitBonus{
		{Name: "昏厥值", RawValue: 2, Value: 20, DisplayScale: 10},
		{Name: "昏厥值", RawValue: 2, Value: 20, DisplayScale: 10},
	}
	got := legacyMasteryFromRuntimeAggregate(observed, overLimit)
	if got.Attack != 8582 || got.HP != 55700 || got.CritRate != 78 || got.StunRaw != 10.5 || got.StunPanel != 105 {
		t.Fatalf("runtime aggregate minus over-limit=%+v", got)
	}
}
