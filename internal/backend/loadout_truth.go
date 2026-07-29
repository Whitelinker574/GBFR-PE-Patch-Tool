package backend

import (
	"fmt"
	"strings"
)

// loadoutSigilAccess classifies a factor without turning "not in our catalog"
// into "universal". Known generic factors are always selectable; known
// character factors and unknown real-save factors require an existing use by
// the same character. The generic result is true only when the catalog proves
// that classification.
func loadoutSigilAccess(cat *Catalog, hash uint32, precedent map[uint32]bool) (generic, allowed bool) {
	if cat == nil || hash == 0 || hash == EmptyHash {
		return false, false
	}
	def := cat.LookupSigilByHash(hash)
	if def == nil {
		return false, precedent[hash]
	}
	if def.Category != nil && *def.Category == "character_sigil" {
		return false, precedent[hash]
	}
	return true, true
}

// loadoutSigilDisplayNameFromTraits keeps traits in their own fields and derives
// the game's V/V+/+ item-family suffix when a combination uses an instance hash.
func loadoutSigilDisplayNameFromTraits(hash uint32, primaryName, secondaryName string) string {
	return loadoutSigilDisplayNameForTraits(hash, primaryName, secondaryName)
}

func sigilHasFixedCatalogTitle(sigil *SigilDef) bool {
	if sigil == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(derefStr(sigil.Category))) {
	case "character_sigil", "dlc_supplement":
		return true
	default:
		return false
	}
}

func loadoutSigilDisplayNameForTraits(hash uint32, primaryName, secondaryName string) string {
	if cat, err := LoadCatalog(); err == nil {
		if sigil := cat.LookupSigilByHash(hash); sigil != nil {
			if !sigilHasFixedCatalogTitle(sigil) && strings.TrimSpace(primaryName) != "" {
				if name := synthesizeSigilNameForTraits(cat, primaryName, strings.TrimSpace(secondaryName) != "", useChinese()); name != "" {
					return name
				}
			}
			return displaySigilName(sigil)
		}
		if name := strings.TrimSpace(localizedRuntimeName(hash)); name != "" {
			if useChinese() {
				return normalizeChineseSigilItemName(name)
			}
			return name
		}
		if name := synthesizeSigilNameForTraits(cat, primaryName, strings.TrimSpace(secondaryName) != "", useChinese()); name != "" {
			return name
		}
	}
	if useChinese() {
		return fallbackSynthesizedSigilName(primaryName, secondaryName, true)
	}
	return fallbackSynthesizedSigilName(primaryName, secondaryName, false)
}

func fallbackSynthesizedSigilName(primaryName, secondaryName string, chinese bool) string {
	primaryName = strings.TrimSpace(primaryName)
	if primaryName == "" {
		if chinese {
			return "因子"
		}
		return "Sigil"
	}
	if isNoSuffixSpecialSigilPrimary(primaryName) {
		return primaryName
	}
	if strings.TrimSpace(secondaryName) == "" {
		return primaryName
	}
	if chinese && primaryName == "躲避性能" {
		return primaryName + "+"
	}
	return primaryName + " V+"
}

func isNoSuffixSpecialSigilPrimary(name string) bool {
	switch strings.TrimSpace(name) {
	case "可怕的漆黑钳蟹因子", "相扑斗力", "漆黑之谊", "Crabby Resonance", "Crabmiration", "Crabvestment Returns", "Seven Net":
		return true
	default:
		return false
	}
}

func synthesizeSigilNameForTraits(cat *Catalog, primaryName string, hasSecondary, chinese bool) string {
	primaryName = strings.TrimSpace(primaryName)
	if cat == nil || primaryName == "" {
		if hasSecondary {
			return fallbackSynthesizedSigilName(primaryName, "secondary", chinese)
		}
		return fallbackSynthesizedSigilName(primaryName, "", chinese)
	}
	base := ""
	suffixes := make(map[string]bool)
	for index := range cat.Sigils {
		sigil := &cat.Sigils[index]
		if sigil.PrimaryTraitName == nil {
			continue
		}
		catalogName := strings.TrimSpace(*sigil.PrimaryTraitName)
		chineseName := strings.TrimSpace(traitCN[catalogName])
		if !strings.EqualFold(primaryName, catalogName) && (chineseName == "" || !strings.EqualFold(primaryName, chineseName)) {
			continue
		}
		if base == "" {
			base = catalogName
			if chinese && chineseName != "" {
				base = chineseName
			}
		}
		candidate := sigilDisplayNameForLanguage(sigil, chinese)
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, base) {
			suffixes[strings.TrimSpace(strings.TrimPrefix(candidate, base))] = true
		}
	}
	if base == "" {
		base = primaryName
	}
	if isNoSuffixSpecialSigilPrimary(primaryName) {
		return base
	}
	if hasSecondary {
		if suffixes["V+"] {
			return base + " V+"
		}
		if suffixes["+"] {
			return base + "+"
		}
		if suffixes["V"] {
			return base + " V+"
		}
		return fallbackSynthesizedSigilName(base, "secondary", chinese)
	}
	// A hashless instance without a secondary trait represents the single-trait
	// family shape. Do not let a newly cataloged V+ shell rename it to V+.
	for _, suffix := range []string{"V", "+", ""} {
		if suffixes[suffix] {
			if suffix == "" {
				return base
			}
			if suffix == "+" {
				return base + "+"
			}
			return base + " " + suffix
		}
	}
	if suffixes["V+"] {
		return base + " V"
	}
	return fallbackSynthesizedSigilName(base, "", chinese)
}

func sigilDisplayNameForLanguage(sigil *SigilDef, chinese bool) string {
	if sigil == nil {
		return ""
	}
	if chinese {
		if name := strings.TrimSpace(sigilCN[sigil.DisplayName]); name != "" {
			return normalizeChineseSigilItemName(name)
		}
		if hash, err := ParseHashHex(sigil.Hash); err == nil {
			return normalizeChineseSigilItemName(runtimeNameCN[hash])
		}
	}
	return strings.TrimSpace(sigil.DisplayName)
}

func validateLoadoutWeaponDefinition(hash uint32, ownerCode string) (ProgressionWeaponDef, error) {
	def, ok := progressionWeaponDefForLoadout(hash)
	if !ok {
		return ProgressionWeaponDef{}, fmt.Errorf("武器 %08X 未收录、是哨兵值或属于内部隐藏记录", hash)
	}
	if def.OwnerCode == "" {
		return def, nil
	}
	if ownerCode == "" {
		return ProgressionWeaponDef{}, fmt.Errorf("无法确定角色武器归属码，不能装备「%s」", progressionWeaponName(def))
	}
	if def.OwnerCode != ownerCode {
		return ProgressionWeaponDef{}, fmt.Errorf("武器「%s」属于 %s，不能装到 %s", progressionWeaponName(def), def.OwnerCode, ownerCode)
	}
	return def, nil
}
