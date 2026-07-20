package main

import "testing"

func TestDLCTraitChineseNamesComeFromLocalSimplifiedChineseCatalog(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	setCurrentLanguage("zh")
	defer setCurrentLanguage(previousLanguage)

	want := map[string]string{
		"Gladiator's Frenzy":      "狼王的激昂",
		"Gladiator's Top":         "狼王的大转轮",
		"Gladiator's Warpath":     "狼王的战气",
		"Bladequeen's Serenade":   "刃姬的小夜曲",
		"Bladequeen's Circuit":    "刃姬的轮舞曲",
		"Bladequeen's Warpath":    "刃姬的战气",
		"Ultramarine's Flash":     "群青的剑光",
		"Ultramarine's Adversity": "群青的逆境",
		"Ultramarine's Warpath":   "群青的战气",
		"Thunderwolf's Recharge":  "雷狼的弹匣",
		"Thunderwolf's Acuity":    "雷狼的慧眼",
		"Thunderwolf's Warpath":   "雷狼的战气",
		"Enchantress's Blessing":  "转世的恩宠",
		"Enchantress's Rhythm":    "转世的跃动",
		"Enchantress's Warpath":   "转世的战气",
		"The Black's Mark":        "黑龙的咒印",
		"The Black's Impulse":     "黑龙的折跃",
		"The Black's Warpath":     "黑龙的战气",
	}
	for english, chinese := range want {
		if got := cnTrait(english); got != chinese {
			t.Errorf("cnTrait(%q) = %q, want %q", english, got, chinese)
		}
	}

	if got := sigilMemoryNameByHash(sigilMemoryTraits, 0x9ACE140B); got != "刃姬的轮舞曲" {
		t.Errorf("memory trait 0x9ACE140B = %q, want %q", got, "刃姬的轮舞曲")
	}
	if got := sigilMemoryNameByHash(sigilMemorySigils, 0x96D6FE5E); got != "刃姬的轮舞曲" {
		t.Errorf("memory sigil 0x96D6FE5E = %q, want %q", got, "刃姬的轮舞曲")
	}
	if got := ctName(0x9ACE140B); got != "刃姬的轮舞曲" {
		t.Errorf("ct name 0x9ACE140B = %q, want %q", got, "刃姬的轮舞曲")
	}
}

func TestOverdriveTraitChineseNamesUseTheGameCatalogCasing(t *testing.T) {
	previousLanguage := getCurrentLanguage()
	setCurrentLanguage("zh")
	defer setCurrentLanguage(previousLanguage)

	want := map[uint32]string{
		0x6F2CF65F: "Overdrive特效",
		0xA9D17F55: "Overdrive特效",
		0x3973C1C4: "Overdrive特效V+",
	}
	for hash, name := range want {
		if got := ctName(hash); got != name {
			t.Errorf("ctName(%08X) = %q, want %q", hash, got, name)
		}
	}
}
