package main

import (
	"strings"
	"testing"
)

func TestDeriveMasterGrowthUsesAuditedThresholdsAndClampsUnlocksAt50(t *testing.T) {
	at20 := deriveMasterGrowth(136500)
	if at20.ProgressIndex != 20 || at20.MasterLevel != 20 || at20.HP != 2400 || at20.ATK != 1200 || at20.DamageCap != 29 {
		t.Fatalf("MSP 136500 growth = %+v, want MLv20/+2400 HP/+1200 ATK/+29%% cap", at20)
	}

	post50 := deriveMasterGrowth(3309499)
	if post50.ProgressIndex != 55 {
		t.Fatalf("MSP 3309499 progress index = %d, want 55", post50.ProgressIndex)
	}
	if post50.MasterLevel != 50 || post50.HP != 6000 || post50.ATK != 3000 || post50.DamageCap != 100 {
		t.Fatalf("post-50 unlock growth = %+v, want MLv50 clamp/+6000 HP/+3000 ATK/+100%% cap", post50)
	}
}

func TestDeriveFateGrowthUsesCompletedEpisodeMaskAndTableThresholds(t *testing.T) {
	partial, known := deriveFateGrowth(0x4D0A60C3, 0x1F)
	if !known || partial.EpisodeCount != 5 || partial.HP != 70 || partial.ATK != 25 {
		t.Fatalf("Io first five Fate episodes = %+v known=%v, want 5/+70 HP/+25 ATK", partial, known)
	}

	complete, known := deriveFateGrowth(0x4D0A60C3, 0x7FF)
	if !known || complete.EpisodeCount != 11 || complete.HP != 640 || complete.ATK != 165 {
		t.Fatalf("Io complete Fate growth = %+v known=%v, want 11/+640 HP/+165 ATK", complete, known)
	}
}

func TestLoadoutStatContextReadsRealIoPermanentBaseline(t *testing.T) {
	requireStatsSave(t)
	ctx, err := (&App{}).LoadoutStatContext(testStatsSave, testIoHash)
	if err != nil {
		t.Fatal(err)
	}

	growth := ctx.PermanentGrowth
	if growth.FateEpisodeMask != 0x7FF || growth.FateEpisodeCount != 11 || growth.FateHP != 640 || growth.FateATK != 165 {
		t.Fatalf("real Io Fate growth = %+v", growth)
	}
	if growth.MasterTotalMSP != 3309499 || growth.MasterProgressIndex != 55 || growth.MasterLevel != 50 ||
		growth.MasterHP != 6000 || growth.MasterATK != 3000 || growth.MasterDamageCap != 100 {
		t.Fatalf("real Io Master growth = %+v", growth)
	}
	if ctx.BaselineHP != 9796 || ctx.BaselineATK != 3831 || ctx.BaselineStun != 8 || ctx.BaselineCritRate != 5 || ctx.BaselineDamageCap != 100 {
		t.Fatalf("real Io permanent baseline = HP %d ATK %d Stun %g Crit %g Cap %g", ctx.BaselineHP, ctx.BaselineATK, ctx.BaselineStun, ctx.BaselineCritRate, ctx.BaselineDamageCap)
	}

	simulation, err := (&App{}).LoadoutSimulateBuild(testStatsSave, testIoHash, 0, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if simulation.FinalStats == nil || simulation.FinalStats.HP != 9796 || simulation.FinalStats.Attack != 3831 || simulation.FinalStats.CritRate != 5 {
		t.Fatalf("offline estimate did not start from the real permanent baseline: %+v", simulation.FinalStats)
	}
	if simulation.FinalStats.DamageCap != 100 || simulation.FinalStats.NormalDamageCap != 120 ||
		simulation.FinalStats.AbilityDamageCap != 120 || simulation.FinalStats.SkyboundDamageCap != 100 {
		t.Fatalf("Master damage-cap baseline did not enter the offline estimate: %+v", simulation.FinalStats)
	}
	for _, total := range simulation.Totals {
		for _, source := range total.Sources {
			if strings.Contains(source, "命运篇章") || strings.Contains(source, "角色强化") {
				t.Fatalf("permanent baseline was disguised as a changeable bonus: %+v", total)
			}
		}
	}
	if !strings.Contains(simulation.FinalStats.Scope, "固定基准") || !strings.Contains(simulation.FinalStats.Scope, "加成明细默认只汇总") {
		t.Fatalf("final-stat scope does not distinguish permanent baseline from changeable bonuses: %q", simulation.FinalStats.Scope)
	}
}
