package main

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// ── 配装写入（LoadoutApply）────────────────────────────────────────────
//
// 安全优先的预设槽写入：把用户从「该存档已有资源」里拼出的一套配装原地写入
// 指定预设槽（UnitID 20000..20614）。只引用存档里真实存在、且已被游戏判为合法
// 的资源，绝不新造因子、绝不生成专精、绝不跨角色搬运。任何引用无法解析或归属
// 存疑一律拒绝写入。落地依据见 配装写入实现计划.md。
//
// 字段填充语义（实测，务必区分）：
//   3002 名称  Byte 表，64 字节，UTF-8 + NUL 填充（走 SetBytes，绝不能按 uint32 步长写）
//   1402 武器  1 值 = 武器 SlotID（2802）
//   1403 因子  13 值，前≤12 = 因子 SlotID（2702），其余填 0（**不是** EmptyHash），第 13 位恒 0
//   1404 技能  4 值，≤4 技能 hash，其余填 EmptyHash
//   3007 专精  50 值，≤50 节点 hash，其余填 EmptyHash
//   3003 角色  1 值 = 角色 hash

const (
	loadoutNameMaxBytes = 63 // 3002 缓冲 64 字节，留 1 字节 NUL
	loadoutMaxSigils    = 12
	loadoutMaxSkills    = 4
	loadoutMaxMastery   = 50
)

// LoadoutWrite 是一次预设槽写入请求。
type LoadoutWrite struct {
	UnitID          uint32   `json:"unitId"`          // 目标预设槽 20000..20614
	ExpectCharaHash string   `json:"expectCharaHash"` // 前端认定的槽位归属角色（8 位 hex），写前与块内实测比对
	Op              string   `json:"op"`              // "write" | "clone" | "clear"
	Name            string   `json:"name"`            // 配装名称 UTF-8，≤63 字节
	WeaponSlotID    uint32   `json:"weaponSlotId"`    // 1402
	SigilSlotIDs    []uint32 `json:"sigilSlotIds"`    // 1403，≤12
	SkillHashes     []string `json:"skillHashes"`     // 1404，≤4，8 位 hex
	MasteryHashes   []string `json:"masteryHashes"`   // 3007，≤50，8 位 hex
	CloneFromUnitID uint32   `json:"cloneFromUnitId"` // Op=="clone" 时的源槽
}

// LoadoutApplyResult 汇报写入结果。
type LoadoutApplyResult struct {
	OutputPath     string `json:"outputPath"`
	BackupPath     string `json:"backupPath"`
	SlotsWritten   int    `json:"slotsWritten"`
	SlotsCleared   int    `json:"slotsCleared"`
	VerifiedFields int    `json:"verifiedFields"` // 回读后逐字段命中的数量
}

// ── 只读编辑上下文：给前端一份「该角色可安全引用的资源池」──────────────

type LoadoutSlotInfo struct {
	UnitID   uint32 `json:"unitId"`
	Slot     int    `json:"slot"`
	Occupied bool   `json:"occupied"`
	Name     string `json:"name"`
}

type LoadoutPickWeapon struct {
	SlotID    uint32 `json:"slotId"`
	Hash      string `json:"hash"`
	Name      string `json:"name"`
	OwnerCode string `json:"ownerCode"` // 空 = 通用武器
}

type LoadoutPickSigil struct {
	SlotID              uint32 `json:"slotId"`
	Hash                string `json:"hash"`
	Name                string `json:"name"`
	Level               int    `json:"level"`
	PrimaryTraitName    string `json:"primaryTraitName"`
	PrimaryTraitLevel   int    `json:"primaryTraitLevel"`
	SecondaryTraitName  string `json:"secondaryTraitName"`
	SecondaryTraitLevel int    `json:"secondaryTraitLevel"`
	Generic             bool   `json:"generic"` // true = 非角色因子（任意角色可装）
}

type LoadoutPickSkill struct {
	Hash string `json:"hash"`
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

type LoadoutMasterySource struct {
	UnitID     uint32   `json:"unitId"`
	Slot       int      `json:"slot"`
	Name       string   `json:"name"`
	NodeCount  int      `json:"nodeCount"`
	NodeHashes []string `json:"nodeHashes"`
}

type LoadoutEditContext struct {
	CharaHash      string                 `json:"charaHash"`
	CharaName      string                 `json:"charaName"`
	OwnerCode      string                 `json:"ownerCode"` // 推导出的 PLxxxx，空=无法确定
	BlockBase      uint32                 `json:"blockBase"`
	Slots          []LoadoutSlotInfo      `json:"slots"`
	Weapons        []LoadoutPickWeapon    `json:"weapons"`
	Sigils         []LoadoutPickSigil     `json:"sigils"`
	Skills         []LoadoutPickSkill     `json:"skills"`
	MasterySources []LoadoutMasterySource `json:"masterySources"`
}

// ── 存档索引：一次解析、读写共用（"所见即所写"）────────────────────────

type loadoutIndex struct {
	gemBySlotID map[uint32]uint32     // 因子 SlotID -> 因子槽 UnitID（仅非空因子）
	gemHash     map[uint32]*unitEntry // 因子槽 UnitID -> 2703
	gemLevel    map[uint32]*unitEntry // 因子槽 UnitID -> 2704
	wepBySlotID map[uint32]uint32     // 武器 SlotID -> 武器槽 UnitID
	wepHash     map[uint32]*unitEntry // 武器槽 UnitID -> 2803
	charName    map[uint32]string     // 角色 hash -> 名字
}

func buildLoadoutIndex(save *SaveData) *loadoutIndex {
	ix := &loadoutIndex{
		gemBySlotID: map[uint32]uint32{},
		wepBySlotID: map[uint32]uint32{},
		charName:    map[uint32]string{},
	}
	for _, e := range save.findAllUnitsByType(SaveID_CharacterID) {
		idx := int(e.UnitID) - 10000
		if idx >= 0 && idx < len(charaNames) && charaNames[idx] != "" {
			ix.charName[e.Uint32()] = charaNames[idx]
		}
	}
	ix.gemHash = entriesByUnitID(save.findAllUnitsByType(GemIDType))
	ix.gemLevel = entriesByUnitID(save.findAllUnitsByType(GemLevelIDType))
	for _, e := range save.findAllUnitsByType(gemSlotIDType) {
		if h := ix.gemHash[e.UnitID]; h == nil || h.Uint32() == EmptyHash || h.Uint32() == 0 {
			continue // 跳过已清空的因子记录，避免悬空引用命中空记录
		}
		ix.gemBySlotID[e.Uint32()] = e.UnitID
	}
	ix.wepHash = entriesByUnitID(save.findAllUnitsByType(weaponIDType))
	for _, e := range save.findAllUnitsByType(weaponSlotIDType) {
		ix.wepBySlotID[e.Uint32()] = e.UnitID
	}
	return ix
}

// resolveBlockChara 由目标槽所在的 15 槽块推断归属角色（读块内占用槽的 3003）。
// 块全空（无法确定角色）或多个非空 3003 互相冲突 → ok=false。
func resolveBlockChara(save *SaveData, unitID uint32) (charaHash uint32, ok bool) {
	if unitID < loadoutBase || unitID >= partyLoadoutBase {
		return 0, false
	}
	blockBase := loadoutBase + ((unitID-loadoutBase)/loadoutSlotsPerChara)*loadoutSlotsPerChara
	var found uint32
	for u := blockBase; u < blockBase+loadoutSlotsPerChara; u++ {
		e, exists := save.findUnitExact(loadoutCharIDType, u)
		if !exists {
			continue
		}
		v := e.Uint32()
		if v == EmptyHash || v == 0 {
			continue
		}
		if found == 0 {
			found = v
		} else if found != v {
			return 0, false // 同块出现两个不同角色 = 数据异常，拒绝
		}
	}
	if found == 0 {
		return 0, false
	}
	return found, true
}

// deriveOwnerCode 推导角色的 PLxxxx：从该角色现有配装的专精节点 char（权威）
// 与武器 ownerCode 双源交叉。两源须一致；无法确定时返回空串（=只放行通用武器）。
func (ix *loadoutIndex) deriveOwnerCode(save *SaveData, charaHash uint32) string {
	fromMastery := ""
	fromWeapon := ""
	for _, ce := range save.findAllUnitsByType(loadoutCharIDType) {
		if ce.Uint32() != charaHash || ce.UnitID >= partyLoadoutBase {
			continue
		}
		u := ce.UnitID
		// 专精节点 -> char
		if fromMastery == "" {
			if me, ok := save.findUnitExact(loadoutMasteryIDType, u); ok {
				for i, n := 0, vecLen(me); i < n; i++ {
					v, err := me.Uint32At(i)
					if err != nil {
						break
					}
					if v == EmptyHash || v == 0 {
						continue
					}
					if node, ok := skillboardNodeForHash(v); ok && node.Char != "" {
						fromMastery = node.Char
						break
					}
				}
			}
		}
		// 武器 ownerCode
		if fromWeapon == "" {
			if we, ok := save.findUnitExact(loadoutWeaponIDType, u); ok {
				if wu, ok := ix.wepBySlotID[we.Uint32()]; ok {
					if h := ix.wepHash[wu]; h != nil {
						if def, ok := progressionWeaponDefForHash(h.Uint32()); ok && def.OwnerCode != "" {
							fromWeapon = def.OwnerCode
						}
					}
				}
			}
		}
		if fromMastery != "" && fromWeapon != "" {
			break
		}
	}
	switch {
	case fromMastery != "" && fromWeapon != "":
		if fromMastery == fromWeapon {
			return fromMastery
		}
		return "" // 两源冲突，保守起见不确定
	case fromMastery != "":
		return fromMastery
	case fromWeapon != "":
		return fromWeapon
	default:
		return ""
	}
}

// charaPrecedent 收集该角色「游戏已亲自接受过」的因子/技能 hash 白名单：
// 凡出现在该角色任一现有配装（20000+）或实时装备（10000+，同 charaHash）中的，
// 都视为合法可再引用。用于对角色专属因子/技能做安全校验（无 hash→角色映射时的替代）。
func (ix *loadoutIndex) charaPrecedent(save *SaveData, charaHash uint32) (sigils, skills map[uint32]bool) {
	sigils = map[uint32]bool{}
	skills = map[uint32]bool{}
	for _, ce := range save.findAllUnitsByType(loadoutCharIDType) {
		if ce.Uint32() != charaHash {
			continue
		}
		u := ce.UnitID
		if se, ok := save.findUnitExact(loadoutSigilsIDType, u); ok {
			for i, n := 0, vecLen(se); i < n; i++ {
				sid, err := se.Uint32At(i)
				if err != nil {
					break
				}
				if gu, ok := ix.gemBySlotID[sid]; ok {
					if h := ix.gemHash[gu]; h != nil {
						sigils[h.Uint32()] = true
					}
				}
			}
		}
		if ke, ok := save.findUnitExact(loadoutSkillsIDType, u); ok {
			for i, n := 0, vecLen(ke); i < n; i++ {
				v, err := ke.Uint32At(i)
				if err != nil {
					break
				}
				if v != EmptyHash && v != 0 {
					skills[v] = true
				}
			}
		}
	}
	return sigils, skills
}

// isCharacterSigil 判断因子是否是「角色专属因子」（只有某角色能装）。
func isCharacterSigil(cat *Catalog, hash uint32) bool {
	if cat == nil {
		return false
	}
	if def := cat.LookupSigilByHash(hash); def != nil && def.Category != nil {
		return *def.Category == "character_sigil"
	}
	return false
}

// ── 字段写入原语 ──────────────────────────────────────────────────────

// writeLoadoutName 写 3002 名称：断言 ValueCnt==64，UTF-8 编码 + NUL 填满 64 字节。
func (s *SaveData) writeLoadoutName(unitID uint32, name string) error {
	e, ok := s.findUnitExact(loadoutNameIDType, unitID)
	if !ok {
		return fmt.Errorf("找不到配装名称字段 3002 @%d", unitID)
	}
	if e.ValueCnt != 64 {
		return fmt.Errorf("配装名称字段 ValueCnt=%d，期望 64", e.ValueCnt)
	}
	buf := []byte(name)
	if len(buf) > loadoutNameMaxBytes {
		return fmt.Errorf("配装名称 %d 字节超过上限 %d（约 21 个汉字）", len(buf), loadoutNameMaxBytes)
	}
	return e.SetBytes(buf) // SetBytes 会先清零整块 64 字节
}

// writeLoadoutVector 写定长 uint32 向量：断言 ValueCnt==wantCnt，前 len(vals) 写值、其余补 pad。
func (s *SaveData) writeLoadoutVector(idType, unitID uint32, vals []uint32, wantCnt int, pad uint32) error {
	e, ok := s.findUnitExact(idType, unitID)
	if !ok {
		return fmt.Errorf("找不到字段 IDType=%d @%d", idType, unitID)
	}
	if e.ValueCnt != wantCnt {
		return fmt.Errorf("字段 IDType=%d @%d ValueCnt=%d，期望 %d", idType, unitID, e.ValueCnt, wantCnt)
	}
	if len(vals) > wantCnt {
		return fmt.Errorf("字段 IDType=%d 写入 %d 个值超过容量 %d", idType, len(vals), wantCnt)
	}
	for i := 0; i < wantCnt; i++ {
		v := pad
		if i < len(vals) {
			v = vals[i]
		}
		if err := e.SetUint32At(i, v); err != nil {
			return err
		}
	}
	return nil
}

// ── 校验 + 落盘编排 ──────────────────────────────────────────────────

// resolvedWrite 是一条 LoadoutWrite 经完整预校验后的、可直接落盘的结果。
type resolvedWrite struct {
	unitID     uint32
	op         string
	charaHash  uint32   // 写入 3003 的实测推导值
	name       string   // Op=="write"
	weaponSID  uint32   // 1402
	sigilSIDs  []uint32 // 1403（≤12）
	skills     []uint32 // 1404（≤4）
	mastery    []uint32 // 3007（≤50）
	keepWeapon bool     // clear 时保持 1402 原值
}

// validateLoadoutWrite 对一条写入请求做完整预校验，返回可落盘的 resolvedWrite。
// 任一不合法即返回 error（此时缓冲区尚未被触碰）。
func validateLoadoutWrite(save *SaveData, ix *loadoutIndex, cat *Catalog, w LoadoutWrite) (*resolvedWrite, error) {
	// 槽位：必须是玩家预设槽（非队伍、非实时装备）
	slot, isParty := loadoutSlotOf(w.UnitID)
	if isParty || slot < 1 || w.UnitID < loadoutBase || w.UnitID >= partyLoadoutBase {
		return nil, fmt.Errorf("UnitID %d 不是可写的预设槽（20000..20614）", w.UnitID)
	}
	// 六字段必须都存在且 ValueCnt 恰为设计前提值
	for _, f := range []struct {
		id   uint32
		want int
		name string
	}{
		{loadoutCharIDType, 1, "角色"}, {loadoutNameIDType, 64, "名称"},
		{loadoutWeaponIDType, 1, "武器"}, {loadoutSigilsIDType, 13, "因子"},
		{loadoutSkillsIDType, 4, "技能"}, {loadoutMasteryIDType, 50, "专精"},
	} {
		e, ok := save.findUnitExact(f.id, w.UnitID)
		if !ok {
			return nil, fmt.Errorf("槽 %d 缺少%s字段（IDType %d）", w.UnitID, f.name, f.id)
		}
		if e.ValueCnt != f.want {
			return nil, fmt.Errorf("槽 %d 的%s字段 ValueCnt=%d，期望 %d（存档结构异常，拒绝写入）", w.UnitID, f.name, e.ValueCnt, f.want)
		}
	}
	// 归属角色：块内实测推导，必须与前端认定一致
	blockChara, ok := resolveBlockChara(save, w.UnitID)
	if !ok {
		return nil, fmt.Errorf("槽 %d 所在角色块为空或角色冲突，无法确定归属角色（请选择已有≥1 套配装的角色）", w.UnitID)
	}
	expect, err := ParseHashHex(w.ExpectCharaHash)
	if err != nil {
		return nil, fmt.Errorf("ExpectCharaHash 无效: %v", err)
	}
	if expect != blockChara {
		return nil, fmt.Errorf("槽 %d 归属角色不符（存档实测 %08X，前端认定 %08X）——存档可能已变，请刷新", w.UnitID, blockChara, expect)
	}

	rw := &resolvedWrite{unitID: w.UnitID, op: w.Op, charaHash: blockChara}

	if w.Op == "clear" {
		rw.keepWeapon = true
		return rw, nil
	}

	if w.Op == "clone" {
		src := w.CloneFromUnitID
		sc, ok := resolveBlockChara(save, src)
		if !ok || sc != blockChara {
			return nil, fmt.Errorf("克隆源槽 %d 与目标不属于同一角色", src)
		}
		if e, ok := save.findUnitExact(loadoutWeaponIDType, src); ok {
			rw.weaponSID = e.Uint32()
		}
		rw.sigilSIDs = readVec(save, loadoutSigilsIDType, src, loadoutMaxSigils)
		rw.skills = readVec(save, loadoutSkillsIDType, src, loadoutMaxSkills)
		rw.mastery = readVec(save, loadoutMasteryIDType, src, loadoutMaxMastery)
		rw.name = entryTextAt(save, src)
		return rw, nil
	}

	if w.Op != "write" {
		return nil, fmt.Errorf("未知操作 %q（应为 write/clone/clear）", w.Op)
	}

	// ── Op == "write"：逐字段校验用户自定义内容 ──
	if !utf8.ValidString(w.Name) {
		return nil, fmt.Errorf("配装名称不是合法 UTF-8")
	}
	if len(w.Name) > loadoutNameMaxBytes {
		return nil, fmt.Errorf("配装名称 %d 字节超过上限 %d", len(w.Name), loadoutNameMaxBytes)
	}
	rw.name = w.Name

	ownerCode := ix.deriveOwnerCode(save, blockChara)

	// 武器（1402）：SlotID -> 现存武器，hash 非空，ownerCode 须匹配该角色或通用
	if _, present := ix.wepBySlotID[w.WeaponSlotID]; present || w.WeaponSlotID != 0 {
		wu, ok := ix.wepBySlotID[w.WeaponSlotID]
		if !ok {
			return nil, fmt.Errorf("武器 SlotID %d 在存档里找不到对应武器", w.WeaponSlotID)
		}
		h := ix.wepHash[wu]
		if h == nil || h.Uint32() == EmptyHash || h.Uint32() == 0 {
			return nil, fmt.Errorf("武器 SlotID %d 指向空武器槽", w.WeaponSlotID)
		}
		if def, ok := progressionWeaponDefForHash(h.Uint32()); ok && def.OwnerCode != "" {
			if ownerCode == "" {
				return nil, fmt.Errorf("无法确定该角色的武器归属码，只能装备通用武器；「%s」是角色专属武器", progressionWeaponName(def))
			}
			if def.OwnerCode != ownerCode {
				return nil, fmt.Errorf("武器「%s」属于 %s，不能装到该角色（%s）", progressionWeaponName(def), def.OwnerCode, ownerCode)
			}
		}
		rw.weaponSID = w.WeaponSlotID
	}

	// 因子（1403）：≤12，各 SlotID 解析到现存非空因子，不得重复；角色因子走 precedent 白名单
	if len(w.SigilSlotIDs) > loadoutMaxSigils {
		return nil, fmt.Errorf("因子最多 %d 个，收到 %d", loadoutMaxSigils, len(w.SigilSlotIDs))
	}
	precSigils, precSkills := ix.charaPrecedent(save, blockChara)
	seenSID := map[uint32]bool{}
	for _, sid := range w.SigilSlotIDs {
		if sid == 0 {
			continue
		}
		if seenSID[sid] {
			return nil, fmt.Errorf("因子 SlotID %d 被重复装备", sid)
		}
		seenSID[sid] = true
		gu, ok := ix.gemBySlotID[sid]
		if !ok {
			return nil, fmt.Errorf("因子 SlotID %d 在存档里找不到对应因子（拒绝写悬空引用）", sid)
		}
		h := ix.gemHash[gu]
		if h == nil {
			return nil, fmt.Errorf("因子 SlotID %d 无 hash", sid)
		}
		hv := h.Uint32()
		if isCharacterSigil(cat, hv) && !precSigils[hv] {
			return nil, fmt.Errorf("因子「%s」是角色专属因子，且该角色的现有配装从未用过它，无法确认可装（v1 保守拒绝）", sigilDisplayNameOr(hv))
		}
		rw.sigilSIDs = append(rw.sigilSIDs, sid)
	}

	// 技能（1404）：≤4，优先按解包 skill_names 的角色归属校验；旧档未知项再回退 precedent。
	if len(w.SkillHashes) > loadoutMaxSkills {
		return nil, fmt.Errorf("技能最多 %d 个，收到 %d", loadoutMaxSkills, len(w.SkillHashes))
	}
	for _, hx := range w.SkillHashes {
		v, err := ParseHashHex(hx)
		if err != nil {
			return nil, fmt.Errorf("技能 hash 无效: %v", err)
		}
		if v == EmptyHash || v == 0 {
			continue
		}
		if !skillBelongsToOwner(v, ownerCode) && !precSkills[v] {
			return nil, fmt.Errorf("技能 %08X 不属于该角色（%s）", v, ownerCode)
		}
		rw.skills = append(rw.skills, v)
	}

	// 专精（3007）：≤50，逐节点须属于该角色 PLxxxx（skillboard_nodes.json 的 char）
	if len(w.MasteryHashes) > loadoutMaxMastery {
		return nil, fmt.Errorf("专精节点最多 %d 个，收到 %d", loadoutMaxMastery, len(w.MasteryHashes))
	}
	for _, hx := range w.MasteryHashes {
		v, err := ParseHashHex(hx)
		if err != nil {
			return nil, fmt.Errorf("专精 hash 无效: %v", err)
		}
		if v == EmptyHash || v == 0 {
			continue
		}
		node, ok := skillboardNodeForHash(v)
		if !ok {
			return nil, fmt.Errorf("专精节点 %08X 未收录", v)
		}
		if ownerCode != "" && node.Char != "" && node.Char != ownerCode {
			return nil, fmt.Errorf("专精节点 %08X 属于 %s，不属于该角色（%s）", v, node.Char, ownerCode)
		}
		rw.mastery = append(rw.mastery, v)
	}
	// 配额校验：每档不超 10/10/10/20（防止写出游戏可能拒绝的非法盘）。
	// 不强制点满（允许半盘/低级盘）；满级由前端配置器保证正好 50。
	if len(rw.mastery) > 0 {
		if _, err := validateMasteryQuota(rw.mastery, ownerCode, false); err != nil {
			return nil, err
		}
	}

	return rw, nil
}

// readVec 读一个 uint32 向量的前 maxN 个非填充值（跳过 0 与 EmptyHash）。
func readVec(save *SaveData, idType, unitID uint32, maxN int) []uint32 {
	e, ok := save.findUnitExact(idType, unitID)
	if !ok {
		return nil
	}
	var out []uint32
	for i, n := 0, vecLen(e); i < n && len(out) < maxN; i++ {
		v, err := e.Uint32At(i)
		if err != nil {
			break
		}
		if v == EmptyHash || v == 0 {
			continue
		}
		out = append(out, v)
	}
	return out
}

func entryTextAt(save *SaveData, unitID uint32) string {
	if e, ok := save.findUnitExact(loadoutNameIDType, unitID); ok {
		return entryText(e)
	}
	return ""
}

func sigilDisplayNameOr(hash uint32) string {
	if n := sigilDisplayName(hash); n != "" {
		return n
	}
	return fmt.Sprintf("%08X", hash)
}

// applyResolvedWrite 把一条已校验的写入落到缓冲区（内存态）。
func applyResolvedWrite(save *SaveData, rw *resolvedWrite) error {
	if rw.op == "clear" {
		if err := save.patchUintExact(loadoutCharIDType, rw.unitID, EmptyHash); err != nil {
			return err
		}
		if err := save.writeLoadoutName(rw.unitID, ""); err != nil {
			return err
		}
		if err := save.writeLoadoutVector(loadoutSigilsIDType, rw.unitID, nil, 13, 0); err != nil {
			return err
		}
		if err := save.writeLoadoutVector(loadoutSkillsIDType, rw.unitID, nil, 4, EmptyHash); err != nil {
			return err
		}
		if err := save.writeLoadoutVector(loadoutMasteryIDType, rw.unitID, nil, 50, EmptyHash); err != nil {
			return err
		}
		return nil // 1402 保持原值
	}
	// write / clone
	if err := save.patchUintExact(loadoutCharIDType, rw.unitID, rw.charaHash); err != nil {
		return err
	}
	if err := save.writeLoadoutName(rw.unitID, rw.name); err != nil {
		return err
	}
	if err := save.patchUintExact(loadoutWeaponIDType, rw.unitID, rw.weaponSID); err != nil {
		return err
	}
	if err := save.writeLoadoutVector(loadoutSigilsIDType, rw.unitID, rw.sigilSIDs, 13, 0); err != nil {
		return err
	}
	if err := save.writeLoadoutVector(loadoutSkillsIDType, rw.unitID, rw.skills, 4, EmptyHash); err != nil {
		return err
	}
	if err := save.writeLoadoutVector(loadoutMasteryIDType, rw.unitID, rw.mastery, 50, EmptyHash); err != nil {
		return err
	}
	return nil
}

// verifyResolvedWrite 回读逐字段比对，返回命中的字段数。
func verifyResolvedWrite(save *SaveData, rw *resolvedWrite) (int, error) {
	hit := 0
	if e, ok := save.findUnitExact(loadoutCharIDType, rw.unitID); ok {
		want := rw.charaHash
		if rw.op == "clear" {
			want = EmptyHash
		}
		if e.Uint32() != want {
			return hit, fmt.Errorf("回读 3003 不符")
		}
		hit++
	}
	if got := entryTextAt(save, rw.unitID); got == rw.name || (rw.op == "clear" && got == "") {
		hit++
	} else {
		return hit, fmt.Errorf("回读名称不符：%q != %q", got, rw.name)
	}
	if !vecMatches(save, loadoutSigilsIDType, rw.unitID, rw.sigilSIDs, 13, 0) {
		return hit, fmt.Errorf("回读因子向量不符")
	}
	hit++
	if !vecMatches(save, loadoutSkillsIDType, rw.unitID, rw.skills, 4, EmptyHash) {
		return hit, fmt.Errorf("回读技能向量不符")
	}
	hit++
	if !vecMatches(save, loadoutMasteryIDType, rw.unitID, rw.mastery, 50, EmptyHash) {
		return hit, fmt.Errorf("回读专精向量不符")
	}
	hit++
	if rw.op != "clear" {
		if e, ok := save.findUnitExact(loadoutWeaponIDType, rw.unitID); ok && e.Uint32() == rw.weaponSID {
			hit++
		} else {
			return hit, fmt.Errorf("回读武器 SlotID 不符")
		}
	}
	return hit, nil
}

func vecMatches(save *SaveData, idType, unitID uint32, vals []uint32, wantCnt int, pad uint32) bool {
	e, ok := save.findUnitExact(idType, unitID)
	if !ok || e.ValueCnt != wantCnt {
		return false
	}
	for i := 0; i < wantCnt; i++ {
		want := pad
		if i < len(vals) {
			want = vals[i]
		}
		got, err := e.Uint32At(i)
		if err != nil || got != want {
			return false
		}
	}
	return true
}

// LoadoutApply 是配装写入闸门：全量预校验通过后统一落盘，任一失败即在触碰磁盘前返回。
func (a *App) LoadoutApply(inputPath, outputPath string, changes []LoadoutWrite) (*LoadoutApplyResult, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("没有要写入的配装")
	}
	if outputPath == "" {
		outputPath = inputPath
	}
	if samePath(inputPath, outputPath) {
		if _, err := findProcessByName(charaProcessName); err == nil {
			return nil, fmt.Errorf("写入存档前请先完全退出游戏，避免游戏把旧数据写回")
		}
	}
	if _, err := loadProgressionCatalog(); err != nil {
		return nil, err
	}
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	save, err := LoadSave(inputPath)
	if err != nil {
		return nil, err
	}
	ix := buildLoadoutIndex(save)

	// 第一段：全量预校验（不触碰缓冲区）
	seen := map[uint32]bool{}
	resolved := make([]*resolvedWrite, 0, len(changes))
	for _, w := range changes {
		if seen[w.UnitID] {
			return nil, fmt.Errorf("槽 %d 被重复提交", w.UnitID)
		}
		seen[w.UnitID] = true
		rw, err := validateLoadoutWrite(save, ix, cat, w)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, rw)
	}

	// 第二段：统一落盘
	result := &LoadoutApplyResult{}
	for _, rw := range resolved {
		if err := applyResolvedWrite(save, rw); err != nil {
			return nil, fmt.Errorf("写入槽 %d 失败: %w", rw.unitID, err)
		}
		if rw.op == "clear" {
			result.SlotsCleared++
		} else {
			result.SlotsWritten++
		}
	}
	if err := save.FixChecksums(); err != nil {
		return nil, fmt.Errorf("修复存档校验失败: %w", err)
	}
	if err := save.Write(outputPath); err != nil {
		return nil, err
	}
	result.OutputPath = outputPath
	result.BackupPath = save.LastBackupPath()

	verify, err := LoadSave(outputPath)
	if err != nil {
		return nil, fmt.Errorf("配装已写入，但重新读取失败: %w", err)
	}
	for _, rw := range resolved {
		n, err := verifyResolvedWrite(verify, rw)
		if err != nil {
			return nil, fmt.Errorf("配装已写入，但槽 %d 回读验证失败（%v）；请用备份恢复", rw.unitID, err)
		}
		result.VerifiedFields += n
	}
	return result, nil
}

// LoadoutEditContext 给前端一份「该角色可安全引用的资源池」，杜绝前端瞎猜引用。
// charaHex 是 8 位十六进制的角色 hash（= LoadoutList 返回的 CharaHash）。
func (a *App) LoadoutEditContext(path, charaHex string) (*LoadoutEditContext, error) {
	if _, err := loadProgressionCatalog(); err != nil {
		return nil, err
	}
	cat, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	charaHash, err := ParseHashHex(charaHex)
	if err != nil {
		return nil, fmt.Errorf("角色 hash 无效: %v", err)
	}
	save, err := LoadSave(path)
	if err != nil {
		return nil, err
	}
	ix := buildLoadoutIndex(save)

	name := ix.charName[charaHash]
	if name == "" {
		return nil, fmt.Errorf("存档里找不到角色 %s", charaHex)
	}

	// 找该角色的块基址（任一占用槽）
	var blockBase uint32
	found := false
	for _, ce := range save.findAllUnitsByType(loadoutCharIDType) {
		if ce.Uint32() == charaHash && ce.UnitID >= loadoutBase && ce.UnitID < partyLoadoutBase {
			blockBase = loadoutBase + ((ce.UnitID-loadoutBase)/loadoutSlotsPerChara)*loadoutSlotsPerChara
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("角色「%s」在该存档里没有任何已保存的配装，无法确定预设槽块（请先在游戏里保存至少一套配装）", name)
	}

	ownerCode := ix.deriveOwnerCode(save, charaHash)
	precSigils, precSkills := ix.charaPrecedent(save, charaHash)

	ctx := &LoadoutEditContext{
		CharaHash: fmt.Sprintf("%08X", charaHash),
		CharaName: name,
		OwnerCode: ownerCode,
		BlockBase: blockBase,
	}

	// 15 个槽的占用/名称
	for u := blockBase; u < blockBase+loadoutSlotsPerChara; u++ {
		info := LoadoutSlotInfo{UnitID: u, Slot: int(u-blockBase) + 1}
		if ce, ok := save.findUnitExact(loadoutCharIDType, u); ok {
			v := ce.Uint32()
			if v != EmptyHash && v != 0 {
				info.Occupied = true
				info.Name = entryTextAt(save, u)
			}
		}
		ctx.Slots = append(ctx.Slots, info)
	}

	// 武器池：该角色专属 + 通用（去重，按 SlotID）
	seenWep := map[uint32]bool{}
	for sid, wu := range ix.wepBySlotID {
		h := ix.wepHash[wu]
		if h == nil {
			continue
		}
		hv := h.Uint32()
		if hv == EmptyHash || hv == 0 {
			continue
		}
		def, ok := progressionWeaponDefForHash(hv)
		oc := ""
		nm := fmt.Sprintf("%08X", hv)
		if ok {
			oc = def.OwnerCode
			nm = progressionWeaponName(def)
		}
		if oc != "" && oc != ownerCode {
			continue
		}
		if seenWep[sid] {
			continue
		}
		seenWep[sid] = true
		ctx.Weapons = append(ctx.Weapons, LoadoutPickWeapon{
			SlotID: sid, Hash: fmt.Sprintf("%08X", hv), Name: nm, OwnerCode: oc,
		})
	}

	// 因子池：通用因子 + 该角色 precedent 里出现过的角色因子（按 SlotID）。
	// 词条表先按 UnitID 建索引，避免对背包每颗因子反复全表扫描。
	traitHashByUnit := entriesByUnitID(save.findAllUnitsByType(TraitHashIDType))
	traitLevelByUnit := entriesByUnitID(save.findAllUnitsByType(TraitLevelIDType))
	indexedTraits := func(gemUnitID uint32) (uint32, int, uint32, int) {
		gemIndex := int(gemUnitID) - GemSlotBaseID
		if gemIndex < 0 {
			return 0, 0, 0, 0
		}
		primaryUnit := uint32(TraitSlotBase + gemIndex*100)
		secondaryUnit := primaryUnit + 1
		var primaryHash, secondaryHash uint32
		var primaryLevel, secondaryLevel int
		if entry := traitHashByUnit[primaryUnit]; entry != nil {
			primaryHash = entry.Uint32()
		}
		if entry := traitLevelByUnit[primaryUnit]; entry != nil {
			primaryLevel = int(entry.Int32())
		}
		if entry := traitHashByUnit[secondaryUnit]; entry != nil {
			secondaryHash = entry.Uint32()
		}
		if entry := traitLevelByUnit[secondaryUnit]; entry != nil {
			secondaryLevel = int(entry.Int32())
		}
		return primaryHash, primaryLevel, secondaryHash, secondaryLevel
	}
	for sid, gu := range ix.gemBySlotID {
		h := ix.gemHash[gu]
		if h == nil {
			continue
		}
		hv := h.Uint32()
		generic := !isCharacterSigil(cat, hv)
		if !generic && !precSigils[hv] {
			continue
		}
		lvl := 0
		if l := ix.gemLevel[gu]; l != nil {
			lvl = int(l.Int32())
		}
		primaryHash, primaryLevel, secondaryHash, secondaryLevel := indexedTraits(gu)
		traitName := func(hash uint32) string {
			if hash == 0 || hash == EmptyHash {
				return ""
			}
			if trait := cat.LookupTraitByHash(hash); trait != nil {
				return cnTrait(trait.DisplayName)
			}
			if name := ctName(hash); name != "" {
				return name
			}
			return fmt.Sprintf("%08X", hash)
		}
		ctx.Sigils = append(ctx.Sigils, LoadoutPickSigil{
			SlotID: sid, Hash: fmt.Sprintf("%08X", hv), Name: sigilDisplayNameOr(hv),
			Level: lvl, PrimaryTraitName: traitName(primaryHash), PrimaryTraitLevel: primaryLevel,
			SecondaryTraitName: traitName(secondaryHash), SecondaryTraitLevel: secondaryLevel, Generic: generic,
		})
	}
	sort.Slice(ctx.Weapons, func(i, j int) bool {
		if ctx.Weapons[i].Name != ctx.Weapons[j].Name {
			return ctx.Weapons[i].Name < ctx.Weapons[j].Name
		}
		return ctx.Weapons[i].SlotID < ctx.Weapons[j].SlotID
	})
	sort.Slice(ctx.Sigils, func(i, j int) bool {
		if ctx.Sigils[i].Name != ctx.Sigils[j].Name {
			return ctx.Sigils[i].Name < ctx.Sigils[j].Name
		}
		return ctx.Sigils[i].SlotID < ctx.Sigils[j].SlotID
	})

	// 技能池：使用解包数据中的完整角色技能表；仅在未收录角色上回退已有配装 precedent。
	ctx.Skills = skillPoolForOwnerCode(ownerCode)
	if len(ctx.Skills) == 0 {
		for h := range precSkills {
			ctx.Skills = append(ctx.Skills, LoadoutPickSkill{Hash: fmt.Sprintf("%08X", h), Name: skillNameForHash(h)})
		}
	}

	// 专精来源：该角色每套已存配装（可整段复制）
	for _, ce := range save.findAllUnitsByType(loadoutCharIDType) {
		if ce.Uint32() != charaHash || ce.UnitID < loadoutBase || ce.UnitID >= partyLoadoutBase {
			continue
		}
		nodes := readVec(save, loadoutMasteryIDType, ce.UnitID, loadoutMaxMastery)
		if len(nodes) == 0 {
			continue
		}
		hexes := make([]string, len(nodes))
		for i, n := range nodes {
			hexes[i] = fmt.Sprintf("%08X", n)
		}
		slot, _ := loadoutSlotOf(ce.UnitID)
		ctx.MasterySources = append(ctx.MasterySources, LoadoutMasterySource{
			UnitID: ce.UnitID, Slot: slot, Name: entryTextAt(save, ce.UnitID),
			NodeCount: len(nodes), NodeHashes: hexes,
		})
	}

	return ctx, nil
}
