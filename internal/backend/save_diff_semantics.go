package backend

// saveDiffSemantic is intentionally conservative. "known" rows are backed by
// the current parser plus an audited read/write or table path. "inferred" rows
// have a stable code-level use, but their complete in-game effect is not
// closed. IDs absent from this dictionary stay "unknown"; numeric proximity is
// never used to invent a category or purpose.
type saveDiffSemantic struct {
	Category   string
	CategoryZh string
	CategoryEn string
	NameZh     string
	NameEn     string
	PurposeZh  string
	PurposeEn  string
	Confidence string
}

type saveDiffCategoryLabel struct {
	Zh string
	En string
}

var saveDiffCategoryLabels = map[string]saveDiffCategoryLabel{
	"system":      {Zh: "存档结构", En: "Save Structure"},
	"currency":    {Zh: "货币与点数", En: "Currency & Points"},
	"character":   {Zh: "角色成长", En: "Character Progress"},
	"inventory":   {Zh: "物品背包", En: "Inventory"},
	"quest":       {Zh: "任务进度", En: "Quest Progress"},
	"trait":       {Zh: "装备词条", En: "Equipment Traits"},
	"sigil":       {Zh: "因子", En: "Sigils"},
	"wrightstone": {Zh: "祝福石", En: "Wrightstones"},
	"summon":      {Zh: "召唤石", En: "Summons"},
	"weapon":      {Zh: "武器", En: "Weapons"},
	"loadout":     {Zh: "配装预设", En: "Loadout Presets"},
	"title":       {Zh: "称号与收藏", En: "Titles & Collection"},
	"unlock":      {Zh: "开放状态", En: "Unlock State"},
	"unknown":     {Zh: "未识别", En: "Unidentified"},
}

func knownSaveDiffSemantic(category, nameZh, nameEn, purposeZh, purposeEn string) saveDiffSemantic {
	return newSaveDiffSemantic(category, nameZh, nameEn, purposeZh, purposeEn, "known")
}

func inferredSaveDiffSemantic(category, nameZh, nameEn, purposeZh, purposeEn string) saveDiffSemantic {
	return newSaveDiffSemantic(category, nameZh, nameEn, purposeZh, purposeEn, "inferred")
}

func newSaveDiffSemantic(category, nameZh, nameEn, purposeZh, purposeEn, confidence string) saveDiffSemantic {
	label, ok := saveDiffCategoryLabels[category]
	if !ok {
		label = saveDiffCategoryLabels["unknown"]
		category = "unknown"
	}
	return saveDiffSemantic{
		Category: category, CategoryZh: label.Zh, CategoryEn: label.En,
		NameZh: nameZh, NameEn: nameEn, PurposeZh: purposeZh, PurposeEn: purposeEn,
		Confidence: confidence,
	}
}

var saveDiffSemantics = map[uint32]saveDiffSemantic{
	SaveID_HashSeed: knownSaveDiffSemantic(
		"system", "校验种子", "Hash Seed",
		"存档校验计算使用的种子，由目标存档自身维护，不能跨存档复制。",
		"Seed used by save checksum calculation. It belongs to the target save and cannot be copied across saves.",
	),
	SaveID_Rupees: knownSaveDiffSemantic(
		"currency", "卢比", "Rupees",
		"记录当前卢比数量；变化通常会影响可用货币。",
		"Stores the current Rupee count; changes normally affect spendable currency.",
	),
	SaveID_Commendations: knownSaveDiffSemantic(
		"currency", "表彰章", "Commendations",
		"记录表彰章数量；具体商店用途以游戏当前表和界面为准。",
		"Stores the Commendations count; exact shop use follows current game tables and UI.",
	),
	SaveID_MasteryPoints: knownSaveDiffSemantic(
		"currency", "MSP 点数", "Mastery Points",
		"记录可用 MSP 点数；不是角色各自的累计专精经验。",
		"Stores spendable MSP; this is not each character's accumulated mastery experience.",
	),
	SaveID_CurrentStageID: inferredSaveDiffSemantic(
		"quest", "当前关卡标识", "Current Stage ID",
		"代码将该值作为当前关卡标识读取；仅凭此字段不能证明任务解锁或完成。",
		"Code reads this as the current stage identifier; this field alone does not prove quest unlock or completion.",
	),
	SaveID_PartyHealth: inferredSaveDiffSemantic(
		"character", "队伍生命状态", "Party Health State",
		"代码将该记录作为队伍生命状态读取，但完整的游戏内消费路径尚未闭环。",
		"Code reads this as party health state, but its complete in-game consumption path is not closed.",
	),
	SaveID_CharacterID: knownSaveDiffSemantic(
		"character", "角色 ID", "Character ID",
		"把角色槽位或记录关联到具体角色 Hash。",
		"Associates a character slot or record with a character hash.",
	),
	1308: knownSaveDiffSemantic(
		"character", "角色等级", "Character Level",
		"记录对应 UnitID 角色的当前等级。",
		"Stores the current level of the character identified by UnitID.",
	),
	1309: knownSaveDiffSemantic(
		"character", "角色基础 HP", "Character Base HP",
		"记录角色表基础 HP；最终面板还会叠加命运篇章、专精、武器和装备效果。",
		"Stores table-derived base HP; the final panel also includes Fate, mastery, weapon, and equipment effects.",
	),
	1310: knownSaveDiffSemantic(
		"character", "角色基础攻击", "Character Base ATK",
		"记录角色表基础攻击；不是最终面板攻击力。",
		"Stores table-derived base ATK, not final panel ATK.",
	),
	1312: knownSaveDiffSemantic(
		"character", "角色基础昏厥值", "Character Base Stun",
		"以 float32 位模式记录角色基础昏厥值；不是命运篇章加成。",
		"Stores character base stun as float32 bits; it is not a Fate bonus.",
	),
	1313: knownSaveDiffSemantic(
		"character", "角色基础暴击率", "Character Base Critical Rate",
		"记录角色基础暴击率；最终面板仍会叠加其他来源。",
		"Stores character base critical rate; the final panel includes other sources.",
	),
	SaveID_CharacterQuestUse: inferredSaveDiffSemantic(
		"character", "角色任务使用记录", "Character Quest Use Record",
		"代码把该记录与角色 UnitID 关联；完整计数规则和最终效果尚未闭环。",
		"Code associates this record with a character UnitID; its complete counting rules and final effect are not closed.",
	),
	1318: knownSaveDiffSemantic(
		"character", "命运篇章完成位", "Fate Episode Completion Bits",
		"低 11 位记录角色命运篇章完成状态，并参与已完成篇章的永久 HP/攻击加成汇总。",
		"The low 11 bits store character Fate Episode completion and contribute confirmed permanent HP/ATK bonuses.",
	),
	1323: knownSaveDiffSemantic(
		"character", "角色累计专精 MSP", "Character Total Mastery MSP",
		"记录角色自己的累计 MSP，用于推导 Master 等级、节点容量和永久成长。",
		"Stores character-specific accumulated MSP used to derive Master level, node capacity, and permanent growth.",
	),
	loadoutWeaponIDType: knownSaveDiffSemantic(
		"loadout", "配装武器引用", "Loadout Weapon Reference",
		"保存配装所选武器的 SlotID 引用，不是武器 Hash。",
		"Stores the SlotID reference of the weapon selected by a loadout, not the weapon hash.",
	),
	loadoutSigilsIDType: knownSaveDiffSemantic(
		"loadout", "配装因子引用", "Loadout Sigil References",
		"保存十二个因子槽的独立 SlotID 引用及填充位。",
		"Stores the twelve sigil-slot SlotID references plus the padding entry.",
	),
	loadoutSkillsIDType: knownSaveDiffSemantic(
		"loadout", "配装技能", "Loadout Skills",
		"保存配装选择的四个角色技能 Hash。",
		"Stores the four character-skill hashes selected by the loadout.",
	),
	SummonEquippedIDType: knownSaveDiffSemantic(
		"summon", "已装备召唤石", "Equipped Summons",
		"保存当前四个召唤石装备槽的实例引用。",
		"Stores instance references for the four equipped summon slots.",
	),
	SummonCatalogIDType: knownSaveDiffSemantic(
		"summon", "召唤石图鉴记录", "Summon Catalog Record",
		"记录召唤石图鉴中的类型条目。",
		"Stores summon-type entries in the summon catalog.",
	),
	SummonRegisteredIDType: inferredSaveDiffSemantic(
		"summon", "召唤石登记状态", "Summon Registration State",
		"代码将该字段作为召唤石登记状态维护；具体 UI 展示效果未单独闭环。",
		"Code maintains this as summon registration state; its exact UI effect is not independently closed.",
	),
	SummonMaxSlotIDType: knownSaveDiffSemantic(
		"summon", "召唤石最大 SlotID", "Summon Maximum SlotID",
		"记录召唤石实例分配使用的最大 SlotID。",
		"Stores the maximum SlotID used for summon instance allocation.",
	),
	SummonUnlockedIDType: knownSaveDiffSemantic(
		"summon", "召唤石系统开放状态", "Summon System Unlock State",
		"0/1 标志，记录召唤石系统是否开放。",
		"0/1 flag storing whether the summon system is unlocked.",
	),
	SummonSlotIDType: knownSaveDiffSemantic(
		"summon", "召唤石实例 SlotID", "Summon Instance SlotID",
		"记录每个召唤石实例的唯一 SlotID，装备引用依赖该值。",
		"Stores each summon instance's unique SlotID used by equipment references.",
	),
	SummonTypeIDType: knownSaveDiffSemantic(
		"summon", "召唤石类型", "Summon Type",
		"记录召唤石实例的类型 Hash。",
		"Stores the summon instance's type hash.",
	),
	SummonTraitsIDType: knownSaveDiffSemantic(
		"summon", "召唤石主副效果", "Summon Main/Sub Effects",
		"记录召唤石主加护与副参数 Hash。",
		"Stores summon main-aura and sub-parameter hashes.",
	),
	SummonLevelsIDType: knownSaveDiffSemantic(
		"summon", "召唤石效果等级", "Summon Effect Levels",
		"记录主加护与副参数各自的等级。",
		"Stores levels for the main aura and sub parameter.",
	),
	SummonRankIDType: knownSaveDiffSemantic(
		"summon", "召唤石 Rank", "Summon Rank",
		"记录召唤石实例的 Rank。",
		"Stores the summon instance rank.",
	),
	1606: knownSaveDiffSemantic(
		"character", "上限突破属性", "Over-Mastery Attribute",
		"记录角色上限突破四槽中的属性 Hash。",
		"Stores the attribute hash in one of the four over-mastery slots.",
	),
	1607: knownSaveDiffSemantic(
		"character", "上限突破等级", "Over-Mastery Level",
		"以单 bit 等级记录上限突破属性档位。",
		"Stores the over-mastery attribute tier as a single-bit level.",
	),
	TraitHashIDType: knownSaveDiffSemantic(
		"trait", "装备词条 Hash", "Equipment Trait Hash",
		"记录因子或祝福石实例引用的词条 Hash；具体归属由 UnitID 关联。",
		"Stores a trait hash referenced by a sigil or wrightstone instance; UnitID determines ownership.",
	),
	TraitLevelIDType: knownSaveDiffSemantic(
		"trait", "装备词条等级", "Equipment Trait Level",
		"记录对应词条实例的存储等级。",
		"Stores the saved level of the corresponding trait instance.",
	),
	SaveID_ItemID: knownSaveDiffSemantic(
		"inventory", "物品 ID", "Item ID",
		"记录背包条目的物品 Hash。",
		"Stores the item hash for an inventory entry.",
	),
	SaveID_ItemCount: knownSaveDiffSemantic(
		"inventory", "物品数量", "Item Count",
		"记录对应物品条目的背包数量。",
		"Stores the inventory quantity for the corresponding item entry.",
	),
	SaveID_ItemFlags: inferredSaveDiffSemantic(
		"inventory", "物品状态标志", "Item State Flags",
		"记录物品条目的状态位；各 bit 的完整含义可能随物品类型变化。",
		"Stores item-state bits; complete bit meanings may vary by item type.",
	),
	SaveID_CurioRewardItemID: knownSaveDiffSemantic(
		"inventory", "鉴定奖励物品", "Curio Reward Item",
		"记录鉴定结果对应的奖励物品 Hash。",
		"Stores the reward-item hash associated with a curio result.",
	),
	SaveID_CurioIDs: knownSaveDiffSemantic(
		"inventory", "遗物 ID", "Curio ID",
		"记录背包或鉴定流程中的遗物条目 ID。",
		"Stores curio entry IDs used by inventory or appraisal flow.",
	),
	WrightstoneMaxSlotIDType: knownSaveDiffSemantic(
		"wrightstone", "祝福石最大 SlotID", "Wrightstone Maximum SlotID",
		"记录祝福石实例分配使用的最大 SlotID。",
		"Stores the maximum SlotID used for wrightstone instance allocation.",
	),
	WrightstoneItemIDType: knownSaveDiffSemantic(
		"wrightstone", "祝福石类型", "Wrightstone Type",
		"记录祝福石实例的物品 Hash。",
		"Stores the item hash of a wrightstone instance.",
	),
	WrightstoneSlotIDType: knownSaveDiffSemantic(
		"wrightstone", "祝福石实例 SlotID", "Wrightstone Instance SlotID",
		"记录祝福石实例的唯一 SlotID。",
		"Stores the unique SlotID of a wrightstone instance.",
	),
	WrightstoneBoolIDType: inferredSaveDiffSemantic(
		"wrightstone", "祝福石状态位", "Wrightstone State Flag",
		"写入路径维护的布尔状态位；完整游戏内用途尚未独立闭环。",
		"Boolean state maintained by the write path; its complete in-game purpose is not independently closed.",
	),
	WrightstoneFlagsIDType: inferredSaveDiffSemantic(
		"wrightstone", "祝福石标志", "Wrightstone Flags",
		"记录祝福石实例状态标志；当前合法新实例使用已验证的普通标志值。",
		"Stores wrightstone instance flags; legal new instances use the verified normal flag value.",
	),
	SaveID_QuestIDs: knownSaveDiffSemantic(
		"quest", "任务 ID", "Quest ID",
		"记录任务条目的任务 ID。",
		"Stores the quest ID for a quest record.",
	),
	SaveID_QuestCompleteCount: knownSaveDiffSemantic(
		"quest", "任务完成次数", "Quest Completion Count",
		"记录对应任务的完成次数。",
		"Stores completion count for the corresponding quest.",
	),
	GemMaxSlotIDType: knownSaveDiffSemantic(
		"sigil", "因子最大 SlotID", "Sigil Maximum SlotID",
		"记录因子实例分配使用的最大 SlotID。",
		"Stores the maximum SlotID used for sigil instance allocation.",
	),
	GemSlotIDType: knownSaveDiffSemantic(
		"sigil", "因子实例 SlotID", "Sigil Instance SlotID",
		"记录因子实例的唯一 SlotID，十二槽配装引用依赖该值。",
		"Stores the unique SlotID of a sigil instance used by twelve-slot loadout references.",
	),
	SaveID_GemID: knownSaveDiffSemantic(
		"sigil", "因子 ID", "Sigil ID",
		"记录因子实例的合法物品壳 Hash。",
		"Stores the legal item-shell hash of a sigil instance.",
	),
	GemLevelIDType: knownSaveDiffSemantic(
		"sigil", "因子等级", "Sigil Level",
		"记录因子物品实例的等级。",
		"Stores the item level of a sigil instance.",
	),
	SaveID_GemWornBy: knownSaveDiffSemantic(
		"sigil", "因子装备角色", "Sigil Equipped By",
		"记录因子当前装备到的角色引用。",
		"Stores the character reference currently equipping the sigil.",
	),
	GemFlagsIDType: inferredSaveDiffSemantic(
		"sigil", "因子状态标志", "Sigil State Flags",
		"记录因子实例状态标志；当前合法新实例使用已验证的普通标志值。",
		"Stores sigil instance flags; legal new instances use the verified normal flag value.",
	),
	weaponMaxSlotIDType: knownSaveDiffSemantic(
		"weapon", "武器最大 SlotID", "Weapon Maximum SlotID",
		"记录武器实例分配使用的最大 SlotID。",
		"Stores the maximum SlotID used for weapon instance allocation.",
	),
	weaponSlotIDType: knownSaveDiffSemantic(
		"weapon", "武器实例 SlotID", "Weapon Instance SlotID",
		"记录武器实例的唯一 SlotID，配装武器引用依赖该值。",
		"Stores the unique SlotID of a weapon instance used by loadout references.",
	),
	weaponIDType: knownSaveDiffSemantic(
		"weapon", "武器 ID", "Weapon ID",
		"记录武器实例的武器 Hash。",
		"Stores the weapon hash of a weapon instance.",
	),
	weaponXPIDType: knownSaveDiffSemantic(
		"weapon", "武器经验", "Weapon XP",
		"记录武器经验值，用于推导当前等级。",
		"Stores weapon experience used to derive current level.",
	),
	weaponUncapIDType: knownSaveDiffSemantic(
		"weapon", "武器上限突破", "Weapon Uncap",
		"记录武器上限突破阶段。",
		"Stores the weapon uncap stage.",
	),
	weaponMirageIDType: knownSaveDiffSemantic(
		"weapon", "武器强化加值", "Weapon Mirage Bonus",
		"记录武器强化加值进度。",
		"Stores weapon plus-value enhancement progress.",
	),
	weaponAwakeIDType: knownSaveDiffSemantic(
		"weapon", "武器觉醒阶段", "Weapon Awakening Stage",
		"记录武器觉醒阶段。",
		"Stores the weapon awakening stage.",
	),
	weaponFlagsIDType: inferredSaveDiffSemantic(
		"weapon", "武器状态标志", "Weapon State Flags",
		"记录武器实例状态位；不能仅凭该字段推断收藏或装备状态。",
		"Stores weapon instance state bits; this field alone does not prove collection or equipment state.",
	),
	weaponVariantIDType: inferredSaveDiffSemantic(
		"weapon", "武器变体", "Weapon Variant",
		"代码把该值作为武器实例变体读取；具体显示效果依武器类型而定。",
		"Code reads this as a weapon-instance variant; its display effect depends on weapon type.",
	),
	weaponStateIDType: inferredSaveDiffSemantic(
		"weapon", "武器实例状态", "Weapon Instance State",
		"写入路径维护的武器实例状态；完整枚举含义尚未闭环。",
		"Weapon-instance state maintained by the write path; the complete enum meaning is not closed.",
	),
	weaponStoneSubType: inferredSaveDiffSemantic(
		"weapon", "武器祝福引用", "Weapon Wrightstone Reference",
		"代码把该值作为武器佩戴祝福石的引用；空值与有效引用需结合实例表判断。",
		"Code reads this as the weapon's equipped-wrightstone reference; empty and valid references require the instance table.",
	),
	weaponTranscendenceIDType: knownSaveDiffSemantic(
		"weapon", "武器超越阶段", "Weapon Transcendence Stage",
		"记录武器当前超越阶段。",
		"Stores the current weapon transcendence stage.",
	),
	weaponExtraIDType: knownSaveDiffSemantic(
		"weapon", "武器附加技能", "Weapon Extra Skills",
		"保存位置敏感的五个武器附加/超越技能 Hash。",
		"Stores five position-sensitive weapon extra/transcendence skill hashes.",
	),
	loadoutNameIDType: knownSaveDiffSemantic(
		"loadout", "配装名称", "Loadout Name",
		"固定容量 UTF-8 字节向量，保存配装预设名称。",
		"Fixed-capacity UTF-8 byte vector storing the loadout preset name.",
	),
	loadoutCharIDType: knownSaveDiffSemantic(
		"loadout", "配装角色", "Loadout Character",
		"记录配装归属角色 Hash；空 Hash 表示未保存槽位。",
		"Stores the owner-character hash; the empty hash means an unused preset slot.",
	),
	loadoutWeaponSkillsIDType: knownSaveDiffSemantic(
		"loadout", "配装武器技能快照", "Loadout Weapon Skill Snapshot",
		"保存当前配装武器的五个位置敏感技能 Hash。",
		"Stores five position-sensitive weapon-skill hashes for the loadout.",
	),
	loadoutMasteryIDType: knownSaveDiffSemantic(
		"loadout", "配装专精节点", "Loadout Mastery Nodes",
		"保存最多 50 个专精节点 Hash；容量不等于当前 Master 等级。",
		"Stores up to 50 mastery-node hashes; capacity is not the current Master level.",
	),
	SaveID_FavoriteChara: knownSaveDiffSemantic(
		"character", "收藏角色", "Favorite Character",
		"记录当前收藏角色 Hash。",
		"Stores the currently favorited character hash.",
	),
	SaveID_BadgeUnlocked: knownSaveDiffSemantic(
		"title", "称号解锁状态", "Title Unlock State",
		"记录称号是否已解锁。",
		"Stores whether a title is unlocked.",
	),
	SaveID_BadgeRewardClaimed: knownSaveDiffSemantic(
		"title", "称号奖励领取状态", "Title Reward Claim State",
		"记录称号奖励是否已领取。",
		"Stores whether a title reward has been claimed.",
	),
	SaveID_BadgeViewed: knownSaveDiffSemantic(
		"title", "称号已查看状态", "Title Viewed State",
		"记录称号是否已查看。",
		"Stores whether a title has been viewed.",
	),
	SaveID_IsUnlocked: inferredSaveDiffSemantic(
		"unlock", "开放状态", "Unlock State",
		"通用开放标记；具体解锁对象必须结合 UnitID 和独立字段证据判断。",
		"Generic unlock flag; the unlocked object requires UnitID and independent field evidence.",
	),
}

func saveDiffSemanticFor(idType uint32) saveDiffSemantic {
	if semantic, ok := saveDiffSemantics[idType]; ok {
		return semantic
	}
	label := saveDiffCategoryLabels["unknown"]
	return saveDiffSemantic{
		Category: "unknown", CategoryZh: label.Zh, CategoryEn: label.En,
		NameZh: "未知字段", NameEn: "Unknown Field",
		PurposeZh:  "尚无可重复证据说明该字段的用途；这里只保留原始 ID、类型、位置和值摘要，不猜测效果。",
		PurposeEn:  "No repeatable evidence currently explains this field. Only its raw ID, type, location, and value summary are retained; no effect is inferred.",
		Confidence: "unknown",
	}
}
