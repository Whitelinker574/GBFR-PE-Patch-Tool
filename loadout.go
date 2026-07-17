package main

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// ── 配装预设（游戏内 Loadout / 装备方案）──────────────────────────────
//
// 结构通过存档差分实测确认（改一项 → 存档只变对应字段）：
//
//	UnitID = 20000 + (角色序号-1)*15 + 槽位          每角色 15 个槽（游戏 UI 的 Loadout 01..15）
//	  3002 (vec)  配装名称，UTF-8 C 字符串
//	  3003 (1)    角色 hash（= SaveID_CharacterID 的值）；EmptyHash 表示该槽未保存
//	  1402 (1)    武器 —— 存的是武器的 SlotID（weaponSlotIDType 2802 的值），不是武器 hash
//	  1403 (13)   因子 ×12(+1 填充) —— 存的是因子的 SlotID（gemSlotIDType 2702 的值）
//	  1404 (4)    技能 ×4（技能 hash）
//	  3007 (50)   专精 —— 技能盘节点 hash（对应 DB skillboard_layout.Key）
//
// 关键点：1402/1403 是「SlotID 引用」，只能指向存档里真实存在的武器/因子，
// 这与游戏内手动保存的配装完全同构。
const (
	loadoutNameIDType    uint32 = 3002
	loadoutCharIDType    uint32 = 3003
	loadoutWeaponIDType  uint32 = 1402
	loadoutSigilsIDType  uint32 = 1403
	loadoutSkillsIDType  uint32 = 1404
	loadoutMasteryIDType uint32 = 3007

	gemSlotIDType uint32 = 2702 // 因子的 SlotID（与武器的 2802 对称）

	loadoutBase          = 20000  // 保存的预设：UnitID = 20000 + (角色序号-1)*15 + 槽位
	loadoutSlotsPerChara = 15     // 每角色 15 个预设槽（游戏 UI 的 Loadout 01..15）
	partyLoadoutBase     = 104000 // 当前队伍 4 名成员的实时配装（非玩家保存的预设）
)

// ── 技能盘节点表（专精翻译）────────────────────────────────────────────
// data/skillboard_nodes.json 由游戏表提取：skillboard_layout/effect/action_parts
// + 简中文本；说明里的 {n} 占位符已按 Value(n+1) 顺序映射填入实际数值。

//go:embed data/skillboard_nodes.json
var skillboardNodesJSON []byte

type SkillboardNode struct {
	Hash string `json:"hash"`
	Char string `json:"char"` // PL0400 等
	Cat  string `json:"cat"`  // SB_ATK / SB_DEF / SB_LIMIT / 基础盘
	Grp  string `json:"grp"`
	Name string `json:"name"` // 具名大节点（真谛/觉醒/秘义），多数节点为空
	Desc string `json:"desc"` // 效果说明（数值已填充）
}

var (
	skillboardOnce   sync.Once
	skillboardByHash map[uint32]SkillboardNode
)

func skillboardNodeForHash(hash uint32) (SkillboardNode, bool) {
	skillboardOnce.Do(func() {
		var payload struct {
			Nodes []SkillboardNode `json:"nodes"`
		}
		skillboardByHash = map[uint32]SkillboardNode{}
		if err := json.Unmarshal(skillboardNodesJSON, &payload); err != nil {
			return
		}
		for _, n := range payload.Nodes {
			if h, err := ParseHashHex(n.Hash); err == nil {
				skillboardByHash[h] = n
			}
		}
	})
	n, ok := skillboardByHash[hash]
	return n, ok
}

type LoadoutMasteryNode struct {
	Hash string `json:"hash"`
	Cat  string `json:"cat"`
	Name string `json:"name"` // 具名节点名（可为空）
	Desc string `json:"desc"` // 效果说明
}

// ── 技能名表 ──────────────────────────────────────────────────────────
// data/skill_names.json：ability.Key 经 XXHash32Custom 求 hash + 简中名。

//go:embed data/skill_names.json
var skillNamesJSON []byte

type LoadoutSkill struct {
	Hash string `json:"hash"`
	Name string `json:"name"`
}

var (
	skillNamesOnce sync.Once
	skillNameByHash map[uint32]string
)

func skillNameForHash(hash uint32) string {
	skillNamesOnce.Do(func() {
		var payload struct {
			Skills map[string]struct {
				Name string `json:"name"`
			} `json:"skills"`
		}
		skillNameByHash = map[uint32]string{}
		if err := json.Unmarshal(skillNamesJSON, &payload); err != nil {
			return
		}
		for hex, s := range payload.Skills {
			if h, err := ParseHashHex(hex); err == nil {
				skillNameByHash[h] = s.Name
			}
		}
	})
	return skillNameByHash[hash]
}

type LoadoutSigil struct {
	SlotID  uint32 `json:"slotId"`  // 1403 里存的值
	UnitID  uint32 `json:"unitId"`  // 对应的因子槽 UnitID（30000+）
	Hash    string `json:"hash"`    // 因子 hash
	Name    string `json:"name"`    // 中文名
	Level   int    `json:"level"`   // 因子等级
	Missing bool   `json:"missing"` // SlotID 在存档里找不到对应因子
}

type LoadoutEntry struct {
	UnitID    uint32         `json:"unitId"`
	Slot      int            `json:"slot"`      // 该角色下的第几个槽（1..15），无法推断时为 0
	IsParty   bool           `json:"isParty"`   // true = 当前队伍成员的实时配装（UnitID 104000+），不是玩家保存的预设
	Name      string         `json:"name"`
	CharaHash string         `json:"charaHash"`
	CharaName string         `json:"charaName"`
	WeaponSlotID uint32      `json:"weaponSlotId"`
	WeaponHash   string      `json:"weaponHash"`
	WeaponName   string      `json:"weaponName"`
	Sigils    []LoadoutSigil       `json:"sigils"`
	Skills    []LoadoutSkill       `json:"skills"`  // 4 个技能（含中文名）
	Mastery   []LoadoutMasteryNode `json:"mastery"` // 专精（技能盘）节点，含中文效果
}

type CharacterLoadouts struct {
	CharaName string         `json:"charaName"`
	CharaHash string         `json:"charaHash"`
	Loadouts  []LoadoutEntry `json:"loadouts"`
}

// entryText 把 ValueData 当 UTF-8 C 字符串读（遇 NUL 截断）。
func entryText(e *unitEntry) string {
	if e == nil || e.ValueCnt == 0 {
		return ""
	}
	buf := make([]byte, 0, e.ValueCnt*4)
	for i := 0; i < e.ValueCnt; i++ {
		v, err := e.Uint32At(i)
		if err != nil {
			break
		}
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], v)
		buf = append(buf, b[:]...)
	}
	if i := strings.IndexByte(string(buf), 0); i >= 0 {
		buf = buf[:i]
	}
	s := string(buf)
	if !utf8.ValidString(s) {
		return ""
	}
	return s
}

// LoadoutList 读出存档里全部已保存的配装预设，按角色分组。
func (a *App) LoadoutList(path string) ([]CharacterLoadouts, error) {
	if _, err := loadProgressionCatalog(); err != nil {
		return nil, err
	}
	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}

	// 角色 hash -> 名字 / 序号
	charName := map[uint32]string{}
	charIdx := map[uint32]int{}
	for _, e := range save.findAllUnitsByType(SaveID_CharacterID) {
		idx := int(e.UnitID) - 10000
		if idx >= 0 && idx < len(charaNames) && charaNames[idx] != "" {
			charName[e.Uint32()] = charaNames[idx]
			charIdx[e.Uint32()] = idx
		}
	}

	// 因子 SlotID -> 因子槽 UnitID
	gemBySlotID := map[uint32]uint32{}
	for _, e := range save.findAllUnitsByType(gemSlotIDType) {
		gemBySlotID[e.Uint32()] = e.UnitID
	}
	gemHash := entriesByUnitID(save.findAllUnitsByType(GemIDType))
	gemLevel := entriesByUnitID(save.findAllUnitsByType(GemLevelIDType))

	// 武器 SlotID -> 武器槽 UnitID
	wepBySlotID := map[uint32]uint32{}
	for _, e := range save.findAllUnitsByType(weaponSlotIDType) {
		wepBySlotID[e.Uint32()] = e.UnitID
	}
	wepHash := entriesByUnitID(save.findAllUnitsByType(weaponIDType))

	names := entriesByUnitID(save.findAllUnitsByType(loadoutNameIDType))
	weapons := entriesByUnitID(save.findAllUnitsByType(loadoutWeaponIDType))
	sigils := entriesByUnitID(save.findAllUnitsByType(loadoutSigilsIDType))
	skills := entriesByUnitID(save.findAllUnitsByType(loadoutSkillsIDType))
	mastery := entriesByUnitID(save.findAllUnitsByType(loadoutMasteryIDType))

	byChara := map[uint32]*CharacterLoadouts{}
	for _, ce := range save.findAllUnitsByType(loadoutCharIDType) {
		ch := ce.Uint32()
		name, ok := charName[ch]
		if !ok || ch == EmptyHash || ch == 0 {
			continue // 空槽
		}
		u := ce.UnitID

		lo := LoadoutEntry{
			UnitID:    u,
			Name:      entryText(names[u]),
			CharaHash: fmt.Sprintf("%08X", ch),
			CharaName: name,
		}
		// UnitID 104000+ 是「当前队伍 4 名成员」的实时配装，不是玩家保存的预设槽
		if u >= partyLoadoutBase {
			lo.IsParty = true
			lo.Slot = int(u-partyLoadoutBase) + 1
		} else if idx, ok := charIdx[ch]; ok && idx >= 1 {
			// 保存的预设：UnitID = 20000 + (角色序号-1)*15 + 槽位
			base := uint32(loadoutBase + (idx-1)*loadoutSlotsPerChara)
			if u >= base && u < base+loadoutSlotsPerChara {
				lo.Slot = int(u-base) + 1
			}
		}

		// 武器：1402 = 武器 SlotID
		if e := weapons[u]; e != nil {
			sid := e.Uint32()
			lo.WeaponSlotID = sid
			if wu, ok := wepBySlotID[sid]; ok {
				if h := wepHash[wu]; h != nil {
					hv := h.Uint32()
					lo.WeaponHash = fmt.Sprintf("%08X", hv)
					if d, ok := progressionWeaponDefForHash(hv); ok {
						lo.WeaponName = progressionWeaponName(d)
					}
				}
			}
		}

		// 因子：1403 = 因子 SlotID ×12(+1 填充)
		if e := sigils[u]; e != nil {
			for i := 0; i < e.ValueCnt; i++ {
				sid, err := e.Uint32At(i)
				if err != nil || sid == 0 || sid == EmptyHash {
					continue
				}
				s := LoadoutSigil{SlotID: sid}
				gu, ok := gemBySlotID[sid]
				if !ok {
					s.Missing = true
					lo.Sigils = append(lo.Sigils, s)
					continue
				}
				s.UnitID = gu
				if h := gemHash[gu]; h != nil {
					hv := h.Uint32()
					s.Hash = fmt.Sprintf("%08X", hv)
					s.Name = sigilDisplayName(hv)
				}
				if l := gemLevel[gu]; l != nil {
					s.Level = int(l.Int32())
				}
				lo.Sigils = append(lo.Sigils, s)
			}
		}

		// 技能：1404 = 4 个技能 hash（经 skill_names.json 翻译）
		if e := skills[u]; e != nil {
			for i := 0; i < e.ValueCnt; i++ {
				v, err := e.Uint32At(i)
				if err != nil || v == EmptyHash || v == 0 {
					continue
				}
				lo.Skills = append(lo.Skills, LoadoutSkill{
					Hash: fmt.Sprintf("%08X", v),
					Name: skillNameForHash(v),
				})
			}
		}

		// 专精：3007 = 技能盘节点 hash（经 skillboard_nodes.json 翻译成中文效果）
		if e := mastery[u]; e != nil {
			for i := 0; i < e.ValueCnt; i++ {
				v, err := e.Uint32At(i)
				if err != nil || v == EmptyHash || v == 0 {
					continue
				}
				node := LoadoutMasteryNode{Hash: fmt.Sprintf("%08X", v)}
				if n, ok := skillboardNodeForHash(v); ok {
					node.Cat = n.Cat
					node.Name = n.Name
					node.Desc = n.Desc
				}
				lo.Mastery = append(lo.Mastery, node)
			}
		}

		g := byChara[ch]
		if g == nil {
			g = &CharacterLoadouts{CharaName: name, CharaHash: fmt.Sprintf("%08X", ch)}
			byChara[ch] = g
		}
		g.Loadouts = append(g.Loadouts, lo)
	}

	result := make([]CharacterLoadouts, 0, len(byChara))
	for _, g := range byChara {
		sort.Slice(g.Loadouts, func(i, j int) bool { return g.Loadouts[i].UnitID < g.Loadouts[j].UnitID })
		result = append(result, *g)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CharaName < result[j].CharaName })
	return result, nil
}

// sigilDisplayName 尽量给出因子中文名。
func sigilDisplayName(hash uint32) string {
	if hash == EmptyHash {
		return ""
	}
	if n, ok := ctHashToName[hash]; ok {
		return n
	}
	return ""
}
