package backend

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

func TestSummonOptionsSelectOfficialLanguageCatalog(t *testing.T) {
	app := &App{}
	setCurrentLanguage("zh")
	zh, err := app.SummonGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	setCurrentLanguage("en")
	en, err := app.SummonGetOptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(zh.Types) != 189 || len(zh.Traits) != 83 || len(zh.SubParams) != 22 {
		t.Fatalf("unexpected Chinese summon catalog sizes: %d/%d/%d", len(zh.Types), len(zh.Traits), len(zh.SubParams))
	}
	find := func(options []SummonOption, hash uint32) string {
		for _, option := range options {
			if option.Hash == hash {
				return option.Name
			}
		}
		return ""
	}
	checks := []struct {
		options []SummonOption
		hash    uint32
		want    string
	}{
		{en.Traits, 0x0DE887A0, "Celestial Nyx"},
		{en.SubParams, 0x00D171E0, "Critical Hit Rate (Low · Max 20%)"},
		{zh.Traits, 0x0DE887A0, "天星之炼"},
		{zh.Traits, 0xB6E31F76, "不动（非天然）"},
	}
	for _, check := range checks {
		if got := find(check.options, check.hash); got != check.want {
			t.Errorf("summon option %08X = %q, want %q", check.hash, got, check.want)
		}
	}
}
