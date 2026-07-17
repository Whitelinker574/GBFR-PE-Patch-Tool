package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// ── 配装模拟器：把一套配装的因子换算成「词条加成汇总」──────────────────
//
// 规则（经 29-agent 研究工作流对抗验证，数据源=游戏原表 skill_status）：
//  1. 逐颗因子取出主词条(hash,等级)+副词条(hash,等级)。
//  2. 按词条 internalId 分组，把同名词条所有出现的等级**相加**，封顶到该词条 maxLevel。
//  3. 用合并后等级查 trait_values.json，按格式串占位符 {N}→LevelValue(N+1) 取数值与单位（带%为百分比）。
//
// v1 只做「因子→词条加成总和」（用户诉求），不做整机最终面板（武器多表/召唤石/限界突破未端到端校准）。

//go:embed data/trait_values.json
var traitValuesJSON []byte

type traitPlaceholder struct {
	Ph     int       `json:"ph"`
	Col    int       `json:"col"`
	Unit   string    `json:"unit"` // "pct" | "flat"
	Values []float64 `json:"values"`
}

type traitValueDef struct {
	Name         string             `json:"name"`
	Cat          *int               `json:"cat"`
	CatLabel     string             `json:"catLabel"`
	MaxLevel     int                `json:"maxLevel"`
	Format       string             `json:"format"`
	Placeholders []traitPlaceholder `json:"placeholders"`
}

var (
	traitValuesOnce sync.Once
	traitValuesByID map[string]*traitValueDef
)

func loadTraitValues() map[string]*traitValueDef {
	traitValuesOnce.Do(func() {
		var payload struct {
			Traits map[string]*traitValueDef `json:"traits"`
		}
		traitValuesByID = map[string]*traitValueDef{}
		if err := json.Unmarshal(traitValuesJSON, &payload); err != nil {
			return
		}
		traitValuesByID = payload.Traits
	})
	return traitValuesByID
}

// TraitBonus 是模拟器输出的一行：某词条的合并加成。
type TraitBonus struct {
	TraitID    string           `json:"traitId"`
	Name       string           `json:"name"`
	CatLabel   string           `json:"catLabel"`
	Level      int              `json:"level"`    // 合并后（封顶后）等级
	RawLevel   int              `json:"rawLevel"` // 封顶前的等级之和
	MaxLevel   int              `json:"maxLevel"`
	Capped     bool             `json:"capped"`
	Effect     string           `json:"effect"`     // 渲染好的中文效果串（数值已代入）
	Components []BonusComponent `json:"components"` // 结构化数值，供聚合（如总HP）
}

// BonusComponent 是一个词条效果里的一个数值分量。
type BonusComponent struct {
	Unit  string  `json:"unit"` // pct | flat
	Value float64 `json:"value"`
}

// SimSigilInput 是模拟器的单颗因子输入（主/副词条 hash + 等级）。
type SimSigilInput struct {
	PrimaryHash    uint32
	PrimaryLevel   int
	SecondaryHash  uint32
	SecondaryLevel int
}

// simulateTraits 把若干 (词条hash,等级) 聚合成加成汇总。
// hashToID：词条 hash → internalId（来自 catalog.traitByHash）。
func simulateTraits(pairs []struct {
	hash  uint32
	level int
}, hashToID map[uint32]string) []TraitBonus {
	tv := loadTraitValues()

	// 1. 按 internalId 分组累加等级
	sumByID := map[string]int{}
	order := []string{}
	for _, p := range pairs {
		if p.hash == EmptyHash || p.hash == 0 || p.level <= 0 {
			continue
		}
		id := hashToID[p.hash]
		if id == "" {
			continue // 未知词条，跳过
		}
		if _, seen := sumByID[id]; !seen {
			order = append(order, id)
		}
		sumByID[id] += p.level
	}

	var out []TraitBonus
	for _, id := range order {
		def := tv[id]
		if def == nil {
			continue
		}
		raw := sumByID[id]
		lv := raw
		capped := false
		if def.MaxLevel > 0 && lv > def.MaxLevel {
			lv = def.MaxLevel
			capped = true
		}
		b := TraitBonus{
			TraitID: id, Name: def.Name, CatLabel: def.CatLabel,
			Level: lv, RawLevel: raw, MaxLevel: def.MaxLevel, Capped: capped,
		}
		b.Effect, b.Components = renderTraitEffect(def, lv)
		out = append(out, b)
	}
	// 攻击类靠前、按分类再按等级排
	sort.SliceStable(out, func(i, j int) bool {
		ci, cj := catRank(out[i].CatLabel), catRank(out[j].CatLabel)
		if ci != cj {
			return ci < cj
		}
		return out[i].Level > out[j].Level
	})
	return out
}

func catRank(label string) int {
	switch label {
	case "攻击类":
		return 0
	case "基础能力":
		return 1
	case "防御类":
		return 2
	case "支援类":
		return 3
	default:
		return 4
	}
}

// renderTraitEffect 把格式串在指定等级代入数值，返回中文效果串 + 结构化分量。
func renderTraitEffect(def *traitValueDef, level int) (string, []BonusComponent) {
	if level < 1 {
		return "", nil
	}
	idx := level - 1
	// 占位符 ph -> 该级数值
	valByPh := map[int]float64{}
	var comps []BonusComponent
	for _, p := range def.Placeholders {
		v := 0.0
		if idx >= 0 && idx < len(p.Values) {
			v = p.Values[idx]
		}
		valByPh[p.Ph] = v
		comps = append(comps, BonusComponent{Unit: p.Unit, Value: v})
	}
	return stripMarkup(substituteFormat(def.Format, valByPh)), comps
}

// stripMarkup 去掉游戏富文本标记（如 <d>…</d> 的属性色标签），只留纯文本。
func stripMarkup(s string) string {
	out := make([]rune, 0, len(s))
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			out = append(out, r)
		}
	}
	return string(out)
}

// substituteFormat 把 "攻击力+{0}" / "攻击力+{1:.1f}%" 里的 {N}[:spec] 替换成数值。
func substituteFormat(format string, valByPh map[int]float64) string {
	out := make([]rune, 0, len(format))
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' {
			out = append(out, runes[i])
			continue
		}
		// 找到 '}'
		j := i + 1
		for j < len(runes) && runes[j] != '}' {
			j++
		}
		if j >= len(runes) {
			out = append(out, runes[i])
			continue
		}
		inner := string(runes[i+1 : j]) // 如 "0" 或 "1:.1f"
		ph := 0
		oneDecimal := false
		// 解析 ph 与是否 .1f
		n := 0
		k := 0
		for k < len(inner) && inner[k] >= '0' && inner[k] <= '9' {
			n = n*10 + int(inner[k]-'0')
			k++
		}
		ph = n
		if k < len(inner) && inner[k] == ':' {
			if containsSub(inner[k:], ".1f") {
				oneDecimal = true
			}
		}
		v := valByPh[ph]
		out = append(out, []rune(formatNum(v, oneDecimal))...)
		i = j
	}
	return string(out)
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func formatNum(v float64, oneDecimal bool) string {
	if oneDecimal {
		return fmt.Sprintf("%.1f", v)
	}
	// 整数则不带小数
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

// buildTraitHashToID 从 catalog 建「词条 hash -> internalId」映射（供模拟器 join trait_values）。
func buildTraitHashToID(cat *Catalog) map[uint32]string {
	m := map[uint32]string{}
	if cat == nil {
		return m
	}
	for i := range cat.Traits {
		if h, err := ParseHashHex(cat.Traits[i].Hash); err == nil {
			m[h] = cat.Traits[i].InternalID
		}
	}
	return m
}

// readSigilTraits 读一颗因子（因子槽 UnitID）的主/副词条 (hash, level)。
func readSigilTraits(save *SaveData, gemUnitID uint32) (primHash uint32, primLv int, secHash uint32, secLv int) {
	gemIndex := int(gemUnitID) - GemSlotBaseID
	if gemIndex < 0 {
		return
	}
	primUnit := uint32(TraitSlotBase + gemIndex*100)
	secUnit := primUnit + 1
	if e, ok := save.findUnit(TraitHashIDType, primUnit); ok {
		primHash = e.Uint32()
	}
	if e, ok := save.findUnit(TraitLevelIDType, primUnit); ok {
		primLv = int(e.Int32())
	}
	if e, ok := save.findUnit(TraitHashIDType, secUnit); ok {
		secHash = e.Uint32()
	}
	if e, ok := save.findUnit(TraitLevelIDType, secUnit); ok {
		secLv = int(e.Int32())
	}
	return
}

// LoadoutSimulate 对一组因子 SlotID 计算「词条加成总和」。
// sigilSlotIDs 是 1403 里存的因子 SlotID（前端编辑器当前勾选的因子）。
func (a *App) LoadoutSimulate(path string, sigilSlotIDs []uint32) ([]TraitBonus, error) {
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	ix := buildLoadoutIndex(save)
	hashToID := buildTraitHashToID(cat)

	var pairs []struct {
		hash  uint32
		level int
	}
	for _, sid := range sigilSlotIDs {
		gu, ok := ix.gemBySlotID[sid]
		if !ok {
			continue
		}
		ph, pl, sh, sl := readSigilTraits(save, gu)
		if ph != 0 && ph != EmptyHash && pl > 0 {
			pairs = append(pairs, struct {
				hash  uint32
				level int
			}{ph, pl})
		}
		if sh != 0 && sh != EmptyHash && sl > 0 {
			pairs = append(pairs, struct {
				hash  uint32
				level int
			}{sh, sl})
		}
	}
	return simulateTraits(pairs, hashToID), nil
}
