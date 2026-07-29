package backend

import (
	"strings"
	"testing"
)

func TestInfinityRuleCatalogUsesLocalizedSubstitutedTableText(t *testing.T) {
	catalog, err := loadInfinityRuleCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Rules) != 25 || len(catalog.Difficulties) != 5 {
		t.Fatalf("unexpected catalog shape: %+v", catalog)
	}
	rule := catalog.Rules[1]
	if rule.QuestID != "0040B301" || rule.NameZh != "精准格挡待诞可获得额外效果" ||
		!strings.Contains(rule.DescriptionZh, "持续15秒") || !strings.Contains(rule.DescriptionZh, "伤害+300%") {
		t.Fatalf("ordered effect parameters were not substituted: %+v", rule)
	}
	for _, entry := range catalog.Rules {
		if strings.Contains(entry.DescriptionZh, "{") || strings.Contains(entry.DescriptionEn, "{") {
			t.Fatalf("unresolved localized placeholder: %+v", entry)
		}
	}
}
