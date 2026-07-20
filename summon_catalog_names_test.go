package main

import (
	"encoding/json"
	"testing"
)

func TestSummonMainSkillNamesUseCanonicalProductChinese(t *testing.T) {
	var payload summonSkillFile
	if err := json.Unmarshal(summonSkillsJSON, &payload); err != nil {
		t.Fatalf("decode summon skills: %v", err)
	}
	got := make(map[string]string, len(payload.Skills))
	for _, skill := range payload.Skills {
		got[skill.Hash] = skill.DisplayName
	}
	want := map[string]string{
		"0x7C2E4D64": "躲避仇火",
		"0x1B0D9897": "钳蟹的报恩",
		"0x4C588C27": "属性克制转换",
		"0xDBE1D775": "α秘纹",
		"0x8D2ADB6E": "β秘纹",
		"0x5C862E13": "γ秘纹",
	}
	for hash, name := range want {
		if got[hash] != name {
			t.Errorf("summon skill %s = %q, want %q", hash, got[hash], name)
		}
	}
}
