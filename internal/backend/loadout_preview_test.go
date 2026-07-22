package backend

import "testing"

func TestLoadoutPreviewListKeepsCurrentLoadoutFirstAndCalculatesApproximatePanel(t *testing.T) {
	requireStatsSave(t)
	result, err := (&App{}).LoadoutPreviewList(statsSaveFixturePath(), testIoHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 || !result[0].IsParty {
		t.Fatalf("current loadout must be first: %+v", result)
	}
	found := false
	for _, entry := range result {
		if entry.Error != "" || entry.FinalStats == nil {
			continue
		}
		found = true
		if entry.FinalStats.HP <= 0 || entry.FinalStats.Attack <= 0 {
			t.Fatalf("preview lacks approximate HP/attack values: %+v", entry.FinalStats)
		}
		if entry.RuntimeBaseline && entry.BaselineEvidence == "" {
			t.Fatal("runtime-backed preview must expose its fixed-growth evidence")
		}
		break
	}
	if !found {
		t.Fatalf("no preview could be calculated: %+v", result)
	}
}
