package backend

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSummonMainTraitSafetyCeilingMatchesLocalCurve(t *testing.T) {
	if summonMainTraitSafetyMaxLevel != 15 {
		t.Fatalf("summon main-trait safety ceiling = %d, want per-stone local summon_curve maximum 15", summonMainTraitSafetyMaxLevel)
	}
}

func TestSummonMainSkillCatalogMatchesLocal202Pool(t *testing.T) {
	wantHashes := []uint32{
		0x50079A1C, 0xF372F096, 0x8D78A19B, 0xCEB700EE, 0x3F488339,
		0x2FC8FBFF, 0x1C360C63, 0x3FEC5F80, 0xC0979A17, 0x6B694D6D,
		0xA7A45F28, 0xB360801D, 0xDC584F60, 0xF17850B9, 0x71F11A9B,
		0xC35B111B, 0x4F1A3683, 0xA9D17F55, 0xE6CDBA9C, 0xA1A8E39D,
		0xE69A4694, 0x973B49AF, 0x7C84A6B3, 0xD54F8CA7, 0xFB572681,
		0x2242921F, 0x0AA20846, 0x3C2B57B0, 0x8B3BF60C, 0x7C2E4D64,
		0x9389CC06, 0x6085DA25, 0x7CCFF74F, 0x95F3FA86, 0x318D12E9,
		0x05F2ECDC, 0xB5FF9FD3, 0x24883AF3, 0xDC607D75, 0x6018372B,
		0xF687C5EF, 0xC86F3082, 0xEAE321EB, 0xE0ABFDFE, 0xA2FA9685,
		0x9702860F, 0x0053599E, 0x1470F860, 0x0EAD65E0, 0x09AA7DB5,
		0xD2C8E10A, 0x29B292A8, 0x7EDD69D0, 0x8F502F0D, 0x84078CB0,
		0x66DE60B1, 0x1B0D9897, 0x74AA75D6, 0xDC225C96, 0x4C588C27,
		0x5E422AE5, 0x57AB5B10, 0x82CE278D, 0x1568E0E4, 0x70395731,
		0xCD18A77D, 0xA8A3163B, 0xDBE1D775, 0x8D2ADB6E, 0x5C862E13,
		0x48A95B8D, 0xF71F8997, 0xEE85CD1F, 0x3D8153A1, 0x0DE887A0,
		0xA7726190, 0x9232DC17, 0x36E3848D, 0xA898E283, 0xD029FE08,
		0x73220725, 0xF26BAEA5,
	}

	want := make(map[uint32]struct{}, len(wantHashes))
	for _, hash := range wantHashes {
		want[hash] = struct{}{}
	}
	if len(want) != 82 {
		t.Fatalf("test fixture has %d unique hashes, want 82", len(want))
	}

	var payload summonSkillFile
	if err := json.Unmarshal(summonSkillsJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Skills) != len(want) {
		t.Fatalf("summon main-skill catalog has %d rows, want the 82 skills referenced by local summon_lot + summon_preset", len(payload.Skills))
	}
	seen := make(map[uint32]struct{}, len(payload.Skills))
	for _, skill := range payload.Skills {
		hash, err := ParseHashHex(skill.Hash)
		if err != nil {
			t.Errorf("invalid hash %q: %v", skill.Hash, err)
			continue
		}
		if _, ok := want[hash]; !ok {
			t.Errorf("catalog contains non-summon trait/item hash 0x%08X (%s)", hash, skill.DisplayName)
		}
		if skill.MaxLevel != 15 {
			t.Errorf("summon main trait 0x%08X has per-stone max %d, want local summon_curve cap 15", hash, skill.MaxLevel)
		}
		if _, duplicate := seen[hash]; duplicate {
			t.Errorf("catalog contains duplicate hash 0x%08X", hash)
		}
		seen[hash] = struct{}{}
	}
}

func TestDLC202SummonMainSkillsUseLocalTableCurves(t *testing.T) {
	tests := []struct {
		id, name string
		first    map[int]float64
		last     map[int]float64
	}{
		{"SKILL_320_00", "天星之炼", map[int]float64{0: 16, 1: 50, 2: 25}, map[int]float64{0: 30, 1: 120, 2: 25}},
		{"SKILL_321_00", "天星之煌", map[int]float64{0: 6, 1: 2, 2: 75}, map[int]float64{0: 20, 1: 70, 2: 75}},
		{"SKILL_322_00", "天星之界", map[int]float64{0: 30, 1: 1}, map[int]float64{0: 30, 1: 70}},
		{"SKILL_323_00", "天星之焰", map[int]float64{0: 10}, map[int]float64{0: 50}},
		{"SKILL_324_00", "天星之雪", map[int]float64{0: 1}, map[int]float64{0: 15}},
		{"SKILL_325_00", "浪迹天涯", map[int]float64{0: 10, 1: 1, 2: 10}, map[int]float64{0: 50, 1: 5, 2: 50}},
		{"SKILL_326_00", "天星之止息", map[int]float64{0: 1}, map[int]float64{0: 10}},
		{"SKILL_327_00", "分歧", map[int]float64{0: 0.6, 1: 1}, map[int]float64{0: 2, 1: 15}},
	}

	definitions := loadTraitValues()
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			definition := definitions[tt.id]
			if definition == nil {
				t.Fatalf("missing data-driven definition %s", tt.id)
			}
			if definition.Name != tt.name || definition.MaxLevel != 15 || definition.Format == "" {
				t.Fatalf("%s metadata = name %q max %d format %q", tt.id, definition.Name, definition.MaxLevel, definition.Format)
			}
			byPlaceholder := make(map[int]traitPlaceholder, len(definition.Placeholders))
			for _, placeholder := range definition.Placeholders {
				byPlaceholder[placeholder.Ph] = placeholder
			}
			for placeholder, want := range tt.first {
				values := byPlaceholder[placeholder].Values
				if len(values) != 15 || values[0] != want {
					t.Errorf("%s placeholder %d Lv1 = %v, want %v (15-level curve)", tt.id, placeholder, values, want)
				}
			}
			for placeholder, want := range tt.last {
				values := byPlaceholder[placeholder].Values
				if len(values) != 15 || values[14] != want {
					t.Errorf("%s placeholder %d Lv15 = %v, want %v (15-level curve)", tt.id, placeholder, values, want)
				}
			}
			for _, level := range []int{1, 15} {
				effect, _ := renderTraitEffect(definition, level)
				if effect == "" || effect == fmt.Sprintf("%s Lv%d", tt.name, level) {
					t.Errorf("%s Lv%d did not render its official format", tt.id, level)
				}
			}
		})
	}
}

func TestCelestialTerraPreservesIntermediateLocalCurve(t *testing.T) {
	definition := loadTraitValues()["SKILL_322_00"]
	if definition == nil {
		t.Fatal("missing data-driven definition SKILL_322_00")
	}
	var damageCap []float64
	for _, placeholder := range definition.Placeholders {
		if placeholder.Ph == 1 {
			damageCap = placeholder.Values
			break
		}
	}
	for _, check := range []struct {
		level int
		want  float64
	}{{2, 5}, {10, 45}, {14, 65}} {
		if len(damageCap) != 15 || damageCap[check.level-1] != check.want {
			t.Errorf("SKILL_322_00 damage-cap Lv%d = %v, want %v from local skill_status.tbl", check.level, damageCap, check.want)
		}
	}
}
