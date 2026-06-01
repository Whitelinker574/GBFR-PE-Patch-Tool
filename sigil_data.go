package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed data/sigils.json data/traits.json data/secondary-trait-rules.json
var dataFiles embed.FS

type SigilDef struct {
	InternalID                     string                         `json:"internalId"`
	Hash                           string                         `json:"hash"`
	DisplayName                    string                         `json:"displayName"`
	Category                       *string                        `json:"category"`
	IsPlusSigil                    *bool                          `json:"isPlusSigil"`
	SupportsSecondaryTrait         *bool                          `json:"supportsSecondaryTrait"`
	AllowedSigilLevels             []int                          `json:"allowedSigilLevels"`
	DefaultSigilLevel              *int                           `json:"defaultSigilLevel"`
	MaxSigilLevel                  *int                           `json:"maxSigilLevel"`
	PrimaryTraitID                 string                         `json:"primaryTraitId"`
	PrimaryTraitName               *string                        `json:"primaryTraitName"`
	FirstTraitMaxLevel             *int                           `json:"firstTraitMaxLevel"`
	AllowedFirstTraitLevels        []int                          `json:"allowedFirstTraitLevels"`
	AllowedSecondaryTraitIDs       []string                       `json:"allowedSecondaryTraitIds"`
	DisallowedSecondaryTraitIDs    []string                       `json:"disallowedSecondaryTraitIds"`
	DefaultSecondaryTraitID        *string                        `json:"defaultSecondaryTraitId"`
	DefaultSecondaryTraitName      *string                        `json:"defaultSecondaryTraitName"`
	SecondaryTraitLevelOverrides   map[string]TraitLevelOverride  `json:"secondaryTraitLevelOverrides"`
}

type TraitDef struct {
	InternalID                  string  `json:"internalId"`
	Hash                        string  `json:"hash"`
	DisplayName                 string  `json:"displayName"`
	Category                    *string `json:"category"`
	MaxLevel                    *int    `json:"maxLevel"`
	AllowedLevels               []int   `json:"allowedLevels"`
	ObservedLevels              []int   `json:"observedLevels"`
	CanAppearAsPrimary          *bool   `json:"canAppearAsPrimary"`
	CanAppearAsSecondary        *bool   `json:"canAppearAsSecondary"`
	BannedAsSecondaryOnPlusSigils *bool `json:"bannedAsSecondaryOnPlusSigils"`
}

type TraitLevelOverride struct {
	MaxLevel      *int  `json:"maxLevel"`
	AllowedLevels []int `json:"allowedLevels"`
	ObservedLevels []int `json:"observedLevels"`
}

type CompatibilityRule struct {
	ID                    string  `json:"id"`
	Type                  string  `json:"type"`
	SigilID               *string `json:"sigilId"`
	PrimaryTraitID        *string `json:"primaryTraitId"`
	SecondaryTraitID      *string `json:"secondaryTraitId"`
	AllowedSecondaryLevels []int  `json:"allowedSecondaryLevels"`
}

type RuleFile struct {
	Rules []CompatibilityRule `json:"rules"`
}

type Catalog struct {
	Sigils      []SigilDef
	Traits      []TraitDef
	Rules       []CompatibilityRule
	sigilByID   map[string]*SigilDef
	traitByID   map[string]*TraitDef
}

func LoadCatalog() (*Catalog, error) {
	c := &Catalog{}

	sigils, err := loadJSON[struct{ Sigils []SigilDef `json:"sigils"` }]("data/sigils.json")
	if err != nil {
		return nil, fmt.Errorf("加载因子数据失败: %w", err)
	}
	c.Sigils = sigils.Sigils

	traits, err := loadJSON[struct{ Traits []TraitDef `json:"traits"` }]("data/traits.json")
	if err != nil {
		return nil, fmt.Errorf("加载特性数据失败: %w", err)
	}
	c.Traits = traits.Traits

	rules, err := loadJSON[RuleFile]("data/secondary-trait-rules.json")
	if err != nil {
		return nil, fmt.Errorf("加载规则数据失败: %w", err)
	}
	c.Rules = rules.Rules

	c.sigilByID = make(map[string]*SigilDef, len(c.Sigils))
	for i := range c.Sigils {
		c.sigilByID[c.Sigils[i].InternalID] = &c.Sigils[i]
	}
	c.traitByID = make(map[string]*TraitDef, len(c.Traits))
	for i := range c.Traits {
		c.traitByID[c.Traits[i].InternalID] = &c.Traits[i]
	}

	return c, nil
}

func loadJSON[T any](path string) (T, error) {
	var result T
	data, err := dataFiles.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	// 预处理: 去掉 JS 风格的 // 和 /* */ 注释，以及尾部逗号
	cleaned := cleanJSON(string(data))
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return result, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	return result, nil
}

func cleanJSON(s string) string {
	// 移除 UTF-8 BOM (EF BB BF)
	s = strings.TrimPrefix(s, "\xEF\xBB\xBF")
	// 移除 /* */ 注释
	for {
		start := strings.Index(s, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+2:], "*/")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+2+end+2:]
	}
	// 移除 // 行注释（仅当 // 前是空白或行首时）
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue // 整行注释，跳过
		}
		// 内联注释: 查找不在字符串内的 //
		inString := false
		commentStart := -1
		for i := 0; i < len(line)-1; i++ {
			if line[i] == '"' && (i == 0 || line[i-1] != '\\') {
				inString = !inString
			}
			if !inString && line[i] == '/' && line[i+1] == '/' {
				commentStart = i
				break
			}
		}
		if commentStart >= 0 {
			// 保留注释前的内容，但去掉尾部空白
			line = strings.TrimRight(line[:commentStart], " \t")
		}
		out = append(out, line)
	}
	s = strings.Join(out, "\n")
	// 移除尾部逗号 (在 ] 或 } 之前的逗号)
	s = strings.ReplaceAll(s, ",\n]", "\n]")
	s = strings.ReplaceAll(s, ",\n}", "\n}")
	s = strings.ReplaceAll(s, ", ]", " ]")
	s = strings.ReplaceAll(s, ", }", " }")
	s = strings.ReplaceAll(s, ",\t]", "\t]")
	s = strings.ReplaceAll(s, ",\t}", "\t}")
	s = strings.ReplaceAll(s, ",  ]", "  ]")
	s = strings.ReplaceAll(s, ",  }", "  }")
	return s
}

func (c *Catalog) RequireSigil(id string) (*SigilDef, error) {
	if s, ok := c.sigilByID[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("未知因子 ID: %s", id)
}

func (c *Catalog) RequireTrait(id string) (*TraitDef, error) {
	if id == "" {
		return nil, fmt.Errorf("缺少已验证的特性 ID")
	}
	if t, ok := c.traitByID[id]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("未知特性 ID: %s", id)
}

func (c *Catalog) GetAllowedSecondaryTraits(sigil *SigilDef) ([]*TraitDef, error) {
	if sigil.SupportsSecondaryTrait == nil || !*sigil.SupportsSecondaryTrait {
		return nil, nil
	}

	disallowed := make(map[string]bool)
	for _, id := range sigil.DisallowedSecondaryTraitIDs {
		disallowed[id] = true
	}

	var result []*TraitDef
	for _, id := range sigil.AllowedSecondaryTraitIDs {
		if disallowed[id] {
			continue
		}
		trait, err := c.RequireTrait(id)
		if err != nil {
			continue
		}
		if trait.CanAppearAsSecondary != nil && !*trait.CanAppearAsSecondary {
			continue
		}
		if trait.BannedAsSecondaryOnPlusSigils != nil && *trait.BannedAsSecondaryOnPlusSigils {
			continue
		}
		result = append(result, trait)
	}
	return result, nil
}

func (c *Catalog) GetDefaultSecondaryTrait(sigil *SigilDef) *TraitDef {
	if sigil.DefaultSecondaryTraitID == nil || *sigil.DefaultSecondaryTraitID == "" {
		return nil
	}
	trait, err := c.RequireTrait(*sigil.DefaultSecondaryTraitID)
	if err != nil {
		return nil
	}
	return trait
}

func (c *Catalog) RequireSigilLevels(sigil *SigilDef) ([]int, error) {
	if len(sigil.AllowedSigilLevels) > 0 {
		return sigil.AllowedSigilLevels, nil
	}
	if sigil.MaxSigilLevel != nil {
		levels := make([]int, *sigil.MaxSigilLevel)
		for i := range levels {
			levels[i] = i + 1
		}
		return levels, nil
	}
	return nil, fmt.Errorf("因子 %s 缺少已验证的等级范围", sigil.DisplayName)
}

func (c *Catalog) RequirePrimaryTraitLevels(sigil *SigilDef) ([]int, error) {
	if len(sigil.AllowedFirstTraitLevels) > 0 {
		return sigil.AllowedFirstTraitLevels, nil
	}
	if sigil.FirstTraitMaxLevel != nil {
		levels := make([]int, *sigil.FirstTraitMaxLevel)
		for i := range levels {
			levels[i] = i + 1
		}
		return levels, nil
	}
	trait, err := c.RequireTrait(sigil.PrimaryTraitID)
	if err != nil {
		return nil, err
	}
	return requireTraitLevels(trait, "主特性")
}

func (c *Catalog) RequireSecondaryTraitLevels(sigil *SigilDef, trait *TraitDef) ([]int, error) {
	if sigil.SecondaryTraitLevelOverrides != nil {
		if override, ok := sigil.SecondaryTraitLevelOverrides[trait.InternalID]; ok {
			if len(override.AllowedLevels) > 0 {
				return override.AllowedLevels, nil
			}
			if override.MaxLevel != nil {
				levels := make([]int, *override.MaxLevel)
				for i := range levels {
					levels[i] = i + 1
				}
				return levels, nil
			}
		}
	}
	return requireTraitLevels(trait, "副特性")
}

func requireTraitLevels(trait *TraitDef, label string) ([]int, error) {
	if len(trait.AllowedLevels) > 0 {
		return trait.AllowedLevels, nil
	}
	if trait.MaxLevel != nil {
		levels := make([]int, *trait.MaxLevel)
		for i := range levels {
			levels[i] = i + 1
		}
		return levels, nil
	}
	return nil, fmt.Errorf("特性 %s 缺少已验证的等级范围 (%s)", trait.DisplayName, label)
}

func ParseHashHex(s string) (uint32, error) {
	text := strings.TrimPrefix(s, "0x")
	text = strings.TrimPrefix(text, "0X")
	v, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("无效的哈希值: %s", s)
	}
	return uint32(v), nil
}

// GetSigilSortedList returns sigils sorted by group then display name (matching C# sort order).
func (c *Catalog) GetSigilSortedList() []*SigilDef {
	sorted := make([]*SigilDef, len(c.Sigils))
	for i := range c.Sigils {
		sorted[i] = &c.Sigils[i]
	}
	sort.Slice(sorted, func(i, j int) bool {
		gi := sigilSortKey(sorted[i].InternalID)
		gj := sigilSortKey(sorted[j].InternalID)
		if gi != gj {
			return gi < gj
		}
		return sorted[i].DisplayName < sorted[j].DisplayName
	})
	return sorted
}

func sigilSortKey(id string) int {
	if len(id) >= 8 && strings.HasPrefix(strings.ToUpper(id), "GEEN_") {
		// id format: GEEN_XXX_YY
		parts := strings.Split(id, "_")
		if len(parts) >= 3 {
			if n, err := strconv.Atoi(parts[1]); err == nil {
				return n
			}
		}
	}
	return 1 << 30 // max int-ish sentinel
}
