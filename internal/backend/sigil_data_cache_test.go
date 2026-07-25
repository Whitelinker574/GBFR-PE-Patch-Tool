package backend

import "testing"

func TestLoadCatalogReusesImmutableCatalog(t *testing.T) {
	first, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("embedded catalog was parsed more than once")
	}
	if len(first.Sigils) == 0 || len(first.Traits) == 0 || len(first.Rules) == 0 {
		t.Fatal("cached catalog is incomplete")
	}
}
