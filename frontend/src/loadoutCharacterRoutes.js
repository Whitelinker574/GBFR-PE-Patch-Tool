export const LOADOUT_CHARACTER_ROUTE_VERSION = '2.0.2-community-routes-3'

const source = (titleZh, titleEn, url, kind = 'community') => Object.freeze({
  titleZh,
  titleEn,
  url,
  kind,
})

export const COMMUNITY_ROUTE_SOURCES = Object.freeze({
  universal: source(
    '全角色通用因子配装攻略（2026-07-12）',
    'Universal Sigil Build Guide (2026-07-12)',
    'https://www.bilibili.com/video/BV1ZYNT6aEXy/',
  ),
  characterIndex: source(
    '2.0.2 全角色流派配置（2026-07-29）',
    '2.0.2 Character Build Routes (2026-07-29)',
    'https://www.bilibili.com/opus/1229753109457141792',
  ),
  ioMagicChain: source(
    '伊欧 · 魔法连锁详细配装（2026-07-29）',
    'Io · Magic Chain Build (2026-07-29)',
    'https://www.bilibili.com/video/BV13c3y66EAT/',
  ),
  ioOnlineFocusChain: source(
    '伊欧 · 联机毕业配装：专注与连锁（2026-07-22）',
    'Io · Online Graduation Build: Focus & Chain (2026-07-22)',
    'https://www.bilibili.com/video/BV1Ckgz6VEsn/',
  ),
  ioDlcGraduation: source(
    '伊欧 · 无尽黄昏 DLC 毕业因子（2026-07-18）',
    'Io · Endless Twilight DLC Graduation Sigils (2026-07-18)',
    'https://www.bilibili.com/video/BV1NiKc6FE1x/',
  ),
  classLevel: source(
    '古兰 / 姬塔 · Class Lv 强化详细配装（2026-07-28）',
    'Gran / Djeeta · Class Lv Build (2026-07-28)',
    'https://www.bilibili.com/video/BV1Lv3v6xEDM/',
  ),
  katalinaAres: source(
    '卡塔莉娜 · 阿瑞斯强袭特化详细配装（2026-07-27）',
    'Katalina · Ares Assault Build (2026-07-27)',
    'https://www.bilibili.com/video/BV1SCgf65Exo/',
  ),
  rackamAftershock: source(
    '拉卡姆 · 战地余波详细配装（2026-07-25）',
    'Rackam · Battlefield Aftershock Build (2026-07-25)',
    'https://www.bilibili.com/video/BV1bF3M6vEN5/',
  ),
  eugenGrenadeFist: source(
    '欧根 · 榴弹拳 DOT 配装（2026-07-29）',
    'Eugen · Grenade-Fist DOT Build (2026-07-29)',
    'https://www.bilibili.com/video/BV1fx3y6pEeK/',
  ),
  rosettaRose: source(
    '萝赛塔 · 玫瑰强化详细配装（2026-07-28）',
    'Rosetta · Rose Enhancement Build (2026-07-28)',
    'https://www.bilibili.com/video/BV1Lv3v6xEom/',
  ),
  ferryPets: source(
    '菲莉 · 宠物强化详细配装（2026-07-25）',
    'Ferry · Pet Enhancement Build (2026-07-25)',
    'https://www.bilibili.com/video/BV19h3M6VEh1/',
  ),
  lancelotAbility: source(
    '兰斯洛特 · 高速能力详细配装（2026-07-26）',
    'Lancelot · High-Speed Ability Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1yK376HE5S/',
  ),
  vaneGuard: source(
    '巴恩 · 勇往直前格挡配装（2026-07-25）',
    'Vane · Heroic Guard Build (2026-07-25)',
    'https://www.bilibili.com/video/BV1Ur3M6iEip/',
  ),
  percivalGraduation: source(
    '珀西瓦尔 · 无尽黄昏毕业配装（2026-07-24）',
    'Percival · Endless Twilight Graduation Build (2026-07-24)',
    'https://www.bilibili.com/video/BV16Pgq6hE5k/',
  ),
  siegfriedDragonClimb: source(
    '齐格飞 · 攻击登龙毕业配装（2026-07-29）',
    'Siegfried · Dragon-Climb Graduation Build (2026-07-29)',
    'https://www.bilibili.com/video/BV11Q3J6bEhw/',
  ),
  charlottaGraduation: source(
    '夏洛特 · 无尽黄昏 DLC 毕业配装（2026-07-27）',
    'Charlotta · Endless Twilight DLC Graduation Build (2026-07-27)',
    'https://www.bilibili.com/video/BV1gQ3A6xELG/',
  ),
  yodarhaThreeMarks: source(
    '尤达拉哈 · 三幕心得详细配装（2026-07-27）',
    'Yodarha · Triple-Shroud Detailed Build (2026-07-27)',
    'https://www.bilibili.com/video/BV1uCgf65EER/',
  ),
  narmayaGraduation: source(
    '娜露梅 · 无尽黄昏 DLC 毕业配装（2026-07-26）',
    'Narmaya · Endless Twilight DLC Graduation Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1ha3L6dEWw/',
  ),
  zetaBlackCrab: source(
    '泽塔 · 漆黑钳蟹最终配装（2026-07-19）',
    'Zeta · Dread Black Pincer Crab Final Build (2026-07-19)',
    'https://www.bilibili.com/video/BV1rrKv6XEBM/',
  ),
  ghandagozaEternalRage: source(
    '冈达葛萨 · 威武雄姿特化配装（2026-07-28）',
    'Ghandagoza · Eternal Rage Build (2026-07-28)',
    'https://www.bilibili.com/video/BV1Vz3v6XEtc/',
  ),
  vaseragaFullHp: source(
    '巴萨拉卡 · 满血减伤毕业配装（2026-07-26）',
    'Vaseraga · Full-HP Mitigation Graduation Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1Vug96AEwb/',
  ),
  cagliostroGraduation: source(
    '卡莉奥丝特罗 · 4.7 亿打桩毕业配装（2026-07-12）',
    'Cagliostro · 470M Dummy Graduation Build (2026-07-12)',
    'https://www.bilibili.com/video/BV1X6NT6dE7T/',
  ),
  idGraduation: source(
    '伊德 · 无尽黄昏毕业配装（2026-07-18）',
    'Id · Endless Twilight Graduation Build (2026-07-18)',
    'https://www.bilibili.com/video/BV1vcKP6SEFB/',
  ),
  sandalphonPrimarch: source(
    '圣德芬 · 天司长灵威配装（2026-07-25）',
    'Sandalphon · Supreme Primarch Build (2026-07-25)',
    'https://www.bilibili.com/video/BV1393T6zECC/',
  ),
  seofonWarpath: source(
    '希耶提 · 剑圣战气配装（2026-07-27）',
    'Seofon · Spirit Edge Warpath Build (2026-07-27)',
    'https://www.bilibili.com/video/BV1Tmgf6rEpj/',
  ),
  tweyenAwakening: source(
    '索恩 · 魔眼觉醒配装（2026-07-28）',
    'Tweyen · Dark Huntress Awakening Build (2026-07-28)',
    'https://www.bilibili.com/video/BV1Vz3v6XE9d/',
  ),
  gallanzaWarpath: source(
    '伽兰查 · 狼王战气配装（2026-07-26）',
    'Gallanza · Gladiator Warpath Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1W63V6kEt6/',
  ),
  maglielleBladequeen: source(
    '玛琪拉菲菈 · 刃姬轮舞配装（2026-07-27）',
    'Maglielle · Bladequeen Circuit Build (2026-07-27)',
    'https://www.bilibili.com/video/BV1uCgf65EMq/',
  ),
  maglielleFatebreaker: source(
    '玛琪拉菲菈 · 浪迹天涯毕业配装（2026-07-26）',
    'Maglielle · Fatebreaker Graduation Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1NA3L62EaN/',
  ),
  beatrixWarpath: source(
    '贝阿朵丽丝 · 群青战气配装（2026-07-25）',
    'Beatrix · Ultramarine Warpath Build (2026-07-25)',
    'https://www.bilibili.com/video/BV18f3T6KEgZ/',
  ),
  eustaceWarpath: source(
    '尤斯提斯 · 雷狼战气蓄力配装（2026-07-26）',
    'Eustace · Thunderwolf Charge Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1ku376JEDZ/',
  ),
  frauxWarpath: source(
    '芙劳 · 转世恩宠配装（2026-07-26）',
    'Fraux · Enchantress Blessing Build (2026-07-26)',
    'https://www.bilibili.com/video/BV1GM3j6wENG/',
  ),
  fedielBalanced: source(
    '菲迪埃尔 · 黑龙均衡配装（2026-07-17）',
    'Fediel · Balanced Black Dragon Build (2026-07-17)',
    'https://www.bilibili.com/video/BV1ujKJ6REZg/',
  ),
})

const trait = (traitId, targetLevel, reasonZh, reasonEn, extra = {}) => Object.freeze({
  traitId,
  targetLevel,
  reasonZh,
  reasonEn,
  ...extra,
})

const ioOnlineFactorCore = () => [
  trait('SKILL_004_00', 30, '视频实际使用 2 枚昏厥 V+，合计把昏厥补到 30 级。', 'The recorded build uses two Stun Power V+ sigils for Stun Power Lv30.', { slotCount: 2 }),
  trait('SKILL_087_00', 15, '视频第三槽是不动 V（无加号），用于蓄力与连续施法时维持动作稳定。', 'The third recorded slot is Firm Stance V without a plus secondary, keeping charged casts stable.'),
  trait('SKILL_063_00', 15, '躲避性能用于取消后摇并提高高压联机容错。', 'Improved Dodge cancels recovery and improves safety in difficult online fights.'),
  trait('SKILL_166_00', 15, '金刚补足 HP，使联机路线保留稳定生存空间。', 'Greater Aegis adds HP so the online route keeps a reliable survival margin.'),
  trait('SKILL_070_00', 15, '怒涛利用高频命中回收能力冷却。', 'Cascade refunds cooldown through Io’s frequent hits.'),
  trait('SKILL_069_00', 15, '迅捷能力直接缩短能力循环空档。', 'Quick Cooldown directly shortens gaps in the ability loop.'),
  trait('SKILL_020_00', 30, '视频实际使用 2 枚伤害上限 V+；其他上限由武器、召唤石和专精继续补。', 'The recorded build uses two DMG Cap V+ sigils; weapon, summons, and mastery supply the remaining cap.', { slotCount: 2 }),
  trait('BF78FBFC', 20, '保留视频中的“可怕的漆黑钳蟹因子”，不能用普通攻击词条替代。', 'Keeps the recorded Dread Black Pincer Crab Sigil instead of replacing it with a generic ATK trait.'),
  trait('SKILL_117_02', 15, '魔法师的战气是这条能力循环的专属增伤核心。', 'Mage’s Warpath is the character-specific damage core for this ability loop.'),
  trait('SKILL_117_00', 15, '魔法师的心愿缩短蓄力时间，支撑能力后接星梦。', 'Mage’s Aspiration shortens charge time so Stargaze can follow abilities.'),
]

const ioMagicChainFactorCore = () => [
  trait('SKILL_004_00', 45, '视频实际使用 3 枚昏厥 V+，合计昏厥 45 级。', 'The recorded build uses three Stun Power V+ sigils for Stun Power Lv45.', { slotCount: 3 }),
  trait('SKILL_106_00', 15, '明镜止水用于精准闪避后的技能循环与奥义积累。', 'Nimble Onslaught supports the post-dodge ability loop and SBA gain.'),
  trait('SKILL_069_00', 15, '迅捷能力直接压缩能力循环空档。', 'Quick Cooldown directly shortens gaps in the ability loop.'),
  trait('SKILL_023_00', 15, '保留视频里的天然单技能“万能药+”，不为它虚构副词条。', 'Keeps the recorded natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_166_00', 15, '金刚为高压任务补足 HP。', 'Greater Aegis adds HP for high-pressure encounters.'),
  trait('SKILL_096_00', 15, '坚持与固定来源的霸体配合，降低连续承伤。', 'Steel Nerves works with fixed-source Stout Heart to reduce repeated damage.'),
  trait('SKILL_063_00', 15, '躲避性能用于取消后摇并扩大容错。', 'Improved Dodge cancels recovery and improves safety.'),
  trait('SKILL_159_00', 15, '摇曳步是视频实际使用的闪避型生存槽，不等同于超级精准闪避。', 'Flight over Fight is the recorded dodge-based survival slot; it is not Super Ultimate Perfect Dodge.'),
  trait('SKILL_117_00', 15, '魔法师的心愿缩短蓄力时间。', 'Mage’s Aspiration shortens charge time.'),
  trait('SKILL_117_02', 15, '魔法师的战气提供魔法连锁核心增伤。', 'Mage’s Warpath provides the core Magic Chain damage bonus.'),
]

const ioDlcGraduationFactorCore = () => [
  trait('SKILL_117_00', 15, '魔法师的心愿缩短蓄力时间。', 'Mage’s Aspiration shortens charge time.'),
  trait('SKILL_004_00', 45, '视频实际使用 3 枚昏厥 V+，合计昏厥 45 级。', 'The recorded build uses three Stun Power V+ sigils for Stun Power Lv45.', { slotCount: 3 }),
  trait('SKILL_001_00', 30, '视频实际使用 2 枚体力 V+，合计体力 30 级。', 'The recorded build uses two HP V+ sigils for HP Lv30.', { slotCount: 2 }),
  trait('SKILL_063_00', 15, '躲避性能提高高压任务容错。', 'Improved Dodge adds safety in difficult encounters.'),
  trait('SKILL_117_02', 15, '魔法师的战气提供专属增伤。', 'Mage’s Warpath supplies the character-specific damage bonus.'),
  trait('SKILL_106_00', 15, '明镜止水支撑精准闪避后的输出循环。', 'Nimble Onslaught supports the post-dodge damage loop.'),
  trait('SKILL_036_00', 15, '坚守把低血量阶段转为更高防御。', 'Garrison adds defense at lower HP.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('BF78FBFC', 20, '保留视频里的“可怕的漆黑钳蟹因子”。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
]

const ioMagicChainFinalChecks = () => [
  trait('SKILL_020_00', 63, '逐帧汇总为 18+45=63；这些等级来自因子以外的固定装备和专精时也应计入。', 'The recorded final total is 18+45=63; fixed gear and mastery sources count toward it.'),
  trait('SKILL_151_00', 45, '逐帧汇总确认追击达到毕业路线所需档位。', 'The frame-verified summary confirms the route’s supplemental-damage tier.'),
  trait('SKILL_146_00', 15, '逐帧汇总与召唤石画面确认属性克制转换。', 'The summary and summon frames confirm War Elemental.'),
  trait('SKILL_233_00', 15, '狂战士来自召唤石，不强占因子槽。', 'Berserker comes from a summon and must not consume a sigil slot.', { condition: 'base-attack-25000' }),
  trait('SKILL_234_00', 15, '斯巴达来自召唤石，不强占因子槽。', 'Spartan comes from a summon and must not consume a sigil slot.', { condition: 'base-hp-80000' }),
]

const classLevelFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_006_00', 15, '奋勇是满血阶段的基础攻击槽。', 'Stamina supplies the full-HP attack slot.'),
  trait('SKILL_166_00', 15, '金刚提高高压任务的 HP。', 'Greater Aegis raises HP for difficult encounters.'),
  trait('SKILL_144_00', 15, '刚健补足满血防御。', 'Stronghold supplies full-HP defense.'),
  trait('SKILL_036_00', 15, '坚守覆盖失血后的防御区间。', 'Garrison covers the lower-HP defense zone.'),
  trait('SKILL_159_00', 15, '摇曳步是视频实际使用的容错槽。', 'Flight over Fight is the recorded safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_114_00', 15, '英勇神速缩短高 Class Lv 下的能力冷却。', 'Heroic Swiftness shortens cooldowns at higher Class Lv.'),
  trait('SKILL_114_02', 15, '英勇之心是 Class Lv 强化路线的角色增伤槽。', 'Heroic Heart is the character damage slot for the Class Lv route.'),
]

const katalinaAresFactorCore = () => [
  trait('SKILL_001_00', 30, '视频使用 2 枚体力 V+。', 'The recorded build uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_146_00', 15, '属性克制转换在这套 12 槽中直接占一槽。', 'War Elemental directly occupies one of the twelve recorded slots.'),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健覆盖满血防御。', 'Stronghold covers full-HP defense.'),
  trait('SKILL_036_00', 15, '坚守覆盖失血后的防御。', 'Garrison covers lower-HP defense.'),
  trait('SKILL_159_00', 15, '摇曳步提高强袭路线的实战容错。', 'Flight over Fight improves real-fight safety.'),
  trait('SKILL_115_00', 15, '守护者的决心提供追击与上限，是阿瑞斯强袭核心。', 'Guardian’s Conviction supplies echo and cap for the Ares assault route.'),
]

const rackamAftershockFactorCore = () => [
  trait('SKILL_000_00', 15, '视频使用 1 枚攻击力 V+。', 'The recorded build uses one ATK V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_151_00', 15, '视频使用 1 枚追击 V+。', 'The recorded build uses one Supplementary DMG V+ sigil.'),
  trait('SKILL_036_00', 45, '战地余波常驻低血量，视频明确使用 3 枚坚守 V+。', 'Battlefield Aftershock stays at low HP, and the recorded build uses three Garrison V+ sigils.', { slotCount: 3 }),
  trait('SKILL_096_00', 15, '坚持降低屏障站桩期间的承伤。', 'Steel Nerves reduces damage while holding position behind the barrier.'),
  trait('SKILL_159_00', 15, '摇曳步用于取消动作并留出生存空间。', 'Flight over Fight cancels recovery and preserves safety.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_116_00', 15, '舵手的指引增加攻击判定，是战地余波输出核心。', 'Helmsman’s Guidance adds an attack hit and anchors the Aftershock route.'),
]

const eugenGrenadeFistFactorCore = () => [
  trait('SKILL_004_00', 30, '视频使用 2 枚昏厥 V+。', 'The recorded build uses two Stun Power V+ sigils.', { slotCount: 2 }),
  trait('SKILL_001_00', 30, '视频前后共使用 2 枚体力 V+。', 'The recorded build uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_146_00', 15, '属性克制转换在原画面直接占一槽。', 'War Elemental directly occupies one recorded slot.'),
  trait('SKILL_151_00', 15, '追击用于榴弹拳多段收益。', 'Supplementary DMG benefits the grenade-fist multihits.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_111_00', 15, '快速蓄力缩短榴弹和升龙拳准备时间。', 'Quick Charge shortens grenade and rising-fist preparation.'),
  trait('SKILL_036_00', 15, '坚守覆盖低血量阶段。', 'Garrison covers lower-HP phases.'),
  trait('SKILL_144_00', 15, '刚健覆盖满血阶段。', 'Stronghold covers the full-HP phase.'),
  trait('SKILL_070_00', 15, '怒涛利用多段命中回收能力。', 'Cascade refunds cooldown through multihits.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
]

const rosettaRoseFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_006_00', 15, '奋勇覆盖玫瑰稳定维持的满血阶段。', 'Stamina covers the full-HP phase maintained by the roses.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健提供满血防御。', 'Stronghold supplies full-HP defense.'),
  trait('SKILL_036_00', 15, '坚守覆盖失血阶段。', 'Garrison covers lower-HP phases.'),
  trait('SKILL_159_00', 15, '摇曳步是视频实际使用的生存槽。', 'Flight over Fight is the recorded safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_119_00', 15, '玫瑰早绽提高玫瑰自动攻击频率。', 'Rose’s early-bloom trait speeds up automatic rose attacks.'),
  trait('SKILL_119_02', 15, '玫瑰的战气阻止等级衰减并强化自动攻击。', 'Rose’s Warpath prevents level decay and strengthens automatic attacks.'),
]

const ferryPetFactorCore = () => [
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_146_00', 15, '属性克制转换在 12 槽中直接占一槽。', 'War Elemental directly occupies one recorded slot.'),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+。', 'Keeps the natural single-trait Potent Greens+.'),
  trait('SKILL_070_00', 15, '怒涛利用宠物和乱鞭多段回收冷却。', 'Cascade refunds cooldown through pet and whip multihits.'),
  trait('SKILL_069_00', 15, '迅捷能力缩短能力循环空档。', 'Quick Cooldown shortens ability-loop downtime.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健提供满血防御。', 'Stronghold supplies full-HP defense.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_120_00', 15, '幽幻之谊是宠物伤害核心。', 'Phantasm’s bond is the pet-damage core.'),
  trait('SKILL_120_02', 15, '幽幻的战气提供角色增伤并减少宠物消失。', 'Phantasm’s Warpath supplies character damage and pet retention.'),
]

const lancelotAbilityFactorCore = () => [
  trait('SKILL_001_00', 30, '视频使用 2 枚体力 V+。', 'The recorded build uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_069_00', 30, '视频使用 2 枚迅捷能力 V+，支撑南十字星循环。', 'The recorded build uses two Quick Cooldown V+ sigils to sustain the Southern Cross loop.', { slotCount: 2 }),
  trait('SKILL_070_00', 15, '怒涛利用高速连击继续回收冷却。', 'Cascade uses fast multihits to refund more cooldown.'),
  trait('SKILL_106_00', 15, '明镜止水强化精准闪避后的循环。', 'Nimble Onslaught strengthens the post-dodge loop.'),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+。', 'Keeps the natural single-trait Potent Greens+.'),
  trait('SKILL_159_00', 15, '摇曳步是该视频采用的容错槽，不等同于超级精准闪避。', 'Flight over Fight is the recorded safety slot and is not Super Ultimate Perfect Dodge.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
]

const vaneGuardFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_006_00', 15, '奋勇覆盖格挡维持的高血量阶段。', 'Stamina covers the high-HP phase sustained by guarding.'),
  trait('SKILL_085_00', 15, '守护提供基础 HP。', 'Aegis supplies base HP.'),
  trait('SKILL_166_00', 15, '金刚进一步提高 HP。', 'Greater Aegis further raises HP.'),
  trait('SKILL_144_00', 15, '刚健补足满血防御。', 'Stronghold supplies full-HP defense.'),
  trait('SKILL_096_00', 15, '坚持配合格挡和霸体降低承伤。', 'Steel Nerves works with guard and super armor to reduce damage.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_122_00', 15, '勇士之觉醒+的主词条是勇士的信念；固定的毅力不另占槽。', 'Hero’s Awakening+ uses Hero’s Creed as its primary; its fixed Will secondary does not consume another slot.'),
  trait('SKILL_122_02', 15, '勇士的战气提高伤害和巨斧槽获取。', 'Hero’s Warpath improves damage and Beatdown gauge gain.'),
]

const percivalGraduationFactorCore = () => [
  trait('SKILL_123_02', 15, '王者的战气是画面直接装备的角色专属输出槽。', "Lord's Warpath is the recorded character-specific damage slot."),
  trait('SKILL_146_00', 15, '属性克制转换直接占据画面中的一个因子槽。', 'War Elemental directly occupies one recorded sigil slot.'),
  trait('SKILL_123_00', 15, '王者行进是第二个独立角色专属槽，不能误并为王者之觉醒。', "Lord's Procession is a second independent character slot and must not be collapsed into an Awakening sigil."),
  trait('SKILL_063_00', 15, '躲避性能用于实战取消与容错。', 'Improved Dodge supplies practical cancel timing and safety.'),
  trait('SKILL_001_00', 45, '视频装备页使用 3 枚体力 V+。', 'The recorded equipment page uses three HP V+ sigils.', { slotCount: 3 }),
  trait('SKILL_004_00', 30, '视频装备页使用 2 枚昏厥 V+。', 'The recorded equipment page uses two Stun Power V+ sigils.', { slotCount: 2 }),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_166_00', 15, '金刚继续抬高实战 HP。', 'Greater Aegis raises practical HP further.'),
  trait('SKILL_070_00', 15, '怒涛利用连续命中回收能力冷却。', 'Cascade refunds ability cooldown through repeated hits.'),
]

const siegfriedDragonClimbFactorCore = () => [
  trait('SKILL_124_02', 15, '屠龙者的战气是画面直接装备的角色专属增伤槽。', "Dragonslayer's Warpath is the recorded character-specific damage slot."),
  trait('SKILL_124_00', 15, '屠龙者的威猛是第二个独立专属槽，不能误并为屠龙之觉醒。', "Dragonslayer's Dominance is a second independent character slot and must not be collapsed into an Awakening sigil."),
  trait('SKILL_063_00', 15, '躲避性能用于精准连段与实战容错。', 'Improved Dodge supports precise strings and practical safety.'),
  trait('SKILL_036_00', 15, '坚守补充低血量阶段防御。', 'Garrison adds defense at lower HP.'),
  trait('SKILL_144_00', 15, '刚健补充高血量阶段防御。', 'Stronghold adds defense at higher HP.'),
  trait('SKILL_166_00', 15, '金刚提高最大 HP。', 'Greater Aegis raises maximum HP.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_001_00', 45, '视频装备页使用 3 枚体力 V+。', 'The recorded equipment page uses three HP V+ sigils.', { slotCount: 3 }),
  trait('SKILL_004_00', 30, '视频装备页使用 2 枚昏厥 V+。', 'The recorded equipment page uses two Stun Power V+ sigils.', { slotCount: 2 }),
]

const charlottaGraduationFactorCore = () => [
  trait('SKILL_004_00', 45, '视频装备页使用 3 枚昏厥 V+。', 'The recorded equipment page uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_001_00', 30, '视频装备页使用 2 枚体力 V+。', 'The recorded equipment page uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_125_02', 15, '圣骑士的战气是角色专属增伤槽。', "Holy Knight's Warpath is the character-specific damage slot."),
  trait('SKILL_125_00', 15, '圣骑士之觉醒+固定包含剑辉与威光，只占一个物理槽。', "Holy Knight's Awakening+ has fixed Luster and Grandeur traits while occupying one physical slot.", { sigilId: 'GEEN_125_90' }),
  trait('SKILL_063_00', 15, '躲避性能用于连续攻击中的实战容错。', 'Improved Dodge supplies practical safety during continuous attacks.'),
  trait('SKILL_146_00', 15, '属性克制转换直接占一个画面槽位。', 'War Elemental directly occupies one recorded slot.'),
  trait('SKILL_106_00', 15, '明镜止水强化精准闪避后的循环。', 'Nimble Onslaught strengthens the post-dodge loop.'),
  trait('SKILL_166_00', 15, '金刚提高最大 HP。', 'Greater Aegis raises maximum HP.'),
  trait('BF78FBFC', 20, '保留视频中的可怕的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
]

const yodarhaThreeMarksFactorCore = () => [
  trait('SKILL_001_00', 15, '视频装备页使用 1 枚体力 V+。', 'The recorded equipment page uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频装备页使用 3 枚昏厥 V+。', 'The recorded equipment page uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_006_00', 15, '奋勇覆盖高血量输出阶段。', 'Stamina covers high-HP damage windows.'),
  trait('SKILL_036_00', 15, '坚守覆盖失血阶段防御。', 'Garrison covers lower-HP defense.'),
  trait('SKILL_166_00', 15, '金刚提高最大 HP。', 'Greater Aegis raises maximum HP.'),
  trait('SKILL_144_00', 15, '刚健补充高血量防御。', 'Stronghold adds high-HP defense.'),
  trait('SKILL_063_00', 15, '躲避性能提供连击收招时的操作容错。', 'Improved Dodge supplies safety around combo finishers.'),
  trait('BF78FBFC', 20, '保留视频中的可怕的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_126_01', 15, '变幻自如的妖剑士是画面直接装备的角色专属槽。', "Swordmaster's Art is the recorded character-specific slot."),
  trait('SKILL_126_02', 15, '变幻自如的战气提供角色专属造成伤害。', "Swordmaster's Warpath supplies character-specific damage."),
]

const narmayaGraduationFactorCore = () => [
  trait('SKILL_127_00', 15, '斩姬之觉醒+固定包含蝶舞与武艺，只占一个物理槽。', "Butterfly's Awakening+ has fixed Grace and Valor traits while occupying one physical slot.", { sigilId: 'GEEN_127_90', secondaryTraitId: 'SKILL_127_01' }),
  trait('SKILL_127_02', 15, '斩姬的战气是角色专属增伤槽。', "Butterfly's Warpath is the character-specific damage slot."),
  trait('SKILL_004_00', 45, '视频装备页使用 3 枚昏厥 V+。', 'The recorded equipment page uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_001_00', 30, '视频装备页使用 2 枚体力 V+。', 'The recorded equipment page uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_166_00', 15, '金刚提高最大 HP。', 'Greater Aegis raises maximum HP.'),
  trait('MEMORY_TRAIT_D029FE08', 15, '0:35 明细确认该槽为浪迹天涯，不是超级奋勇或超级精准闪避。', 'The 0:35 detail frame confirms Fatebreaker rather than Super Stamina or Super Ultimate Perfect Dodge.', { sigilId: 'MEMORY_SIGIL_5BF84FD1' }),
  trait('SKILL_063_00', 15, '躲避性能支撑精准切换与实战容错。', 'Improved Dodge supports precise stance switching and practical safety.'),
  trait('SKILL_151_00', 15, '追击直接占一个画面槽位。', 'Supplementary DMG directly occupies one recorded slot.'),
  trait('BF78FBFC', 20, '保留视频中的可怕的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
]

const zetaBlackCrabFactorCore = () => [
  trait('SKILL_131_00', 15, '真红烈焰是画面直接装备的角色专属槽。', "Crimson's Clout is the recorded character-specific slot."),
  trait('SKILL_131_02', 15, '真红的战气提供角色专属造成伤害与暴击支持。', "Crimson's Warpath supplies character-specific damage and critical support."),
  trait('SKILL_004_00', 30, '视频装备页使用 2 枚昏厥 V+。', 'The recorded equipment page uses two Stun Power V+ sigils.', { slotCount: 2 }),
  trait('SKILL_001_00', 15, '视频装备页使用 1 枚体力 V+。', 'The recorded equipment page uses one HP V+ sigil.'),
  trait('BF78FBFC', 20, '保留视频直接装备的可怕的漆黑钳蟹因子，等级按本地 2.0.2 真实曲线上限计算。', 'Keeps the recorded Dread Black Pincer Crab Sigil and evaluates it at the real local 2.0.2 trait-curve cap.'),
  trait('SKILL_020_00', 15, '伤害上限直接占一个画面槽位。', 'DMG Cap directly occupies one recorded slot.'),
  trait('MEMORY_TRAIT_73220725', 15, '天星之止息使用运行时补充目录里的真实 Hash。', 'Celestial Ventus uses its real runtime-supplement hash.'),
  trait('MEMORY_TRAIT_A898E283', 15, '天星之雪使用运行时补充目录里的真实 Hash。', 'Celestial Aqua uses its real runtime-supplement hash.'),
  trait('MEMORY_TRAIT_D029FE08', 15, '浪迹天涯使用运行时补充目录里的真实 Hash。', 'Fatebreaker uses its real runtime-supplement hash.'),
  trait('SKILL_151_00', 30, '视频装备页使用 2 枚追击 V+。', 'The recorded equipment page uses two Supplementary DMG V+ sigils.', { slotCount: 2 }),
]

const ghandagozaEternalRageFactorCore = () => [
  trait('SKILL_001_00', 15, '视频装备页使用 1 枚体力 V+。', 'The recorded equipment page uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频装备页使用 3 枚昏厥 V+。', 'The recorded equipment page uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_106_00', 15, '明镜止水支撑精准闪避后的输出循环。', 'Nimble Onslaught supports the post-dodge damage loop.'),
  trait('SKILL_036_00', 60, '威武雄姿特化路线使用 4 枚坚守 V+，不能压缩成普通通用模板。', 'The Eternal Rage route records four Garrison V+ sigils and must not be collapsed into a generic template.', { slotCount: 4 }),
  trait('SKILL_063_00', 15, '躲避性能是画面中的实战容错槽。', 'Improved Dodge is the recorded practical safety slot.'),
  trait('SKILL_087_00', 15, '不动用于蓄力拳过程中维持动作稳定。', 'Firm Stance keeps charged punches stable.'),
  trait('SKILL_128_01', 15, '古今无双的强者是画面直接装备的角色专属槽。', "Eternal Rage's Ethos is the character-specific slot shown on the equipment page."),
]

const vaseragaFullHpFactorCore = () => [
  trait('SKILL_132_00', 15, '冥暗之觉醒+固定包含冥暗刚刃与冥暗自若，只占一个物理槽。', "Ebony's Awakening+ has fixed Presence and Poise traits while occupying one physical slot.", { sigilId: 'GEEN_132_90' }),
  trait('BF78FBFC', 20, '保留视频中的可怕的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_151_00', 15, '追击直接占一个画面槽位。', 'Supplementary DMG directly occupies one recorded slot.'),
  trait('MEMORY_TRAIT_D029FE08', 15, '浪迹天涯使用运行时补充目录里的真实 Hash。', 'Fatebreaker uses its real runtime-supplement hash.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_063_00', 15, '躲避性能提供满血路线的操作容错。', 'Improved Dodge supplies practical safety for the full-HP route.'),
  trait('MEMORY_TRAIT_F26BAEA5', 15, '分歧使用运行时补充目录里的真实 Hash。', 'Divergence uses its real runtime-supplement hash.'),
  trait('SKILL_087_00', 15, '不动维持蓄力动作稳定。', 'Firm Stance keeps charged actions stable.'),
  trait('SKILL_069_00', 15, '迅捷能力缩短能力循环空档。', 'Quick Cooldown shortens ability-loop downtime.'),
  trait('SKILL_004_00', 45, '视频装备页使用 3 枚昏厥 V+。', 'The recorded equipment page uses three Stun Power V+ sigils.', { slotCount: 3 }),
]

const cagliostroGraduationFactorCore = () => [
  trait('SKILL_129_00', 30, '极致谋略+与极致之觉醒+各占一槽；觉醒固定包含谋略与真理。', "Founder's Strategy+ and Founder's Awakening+ occupy one slot each; Awakening has fixed Strategy and Truth traits.", { slotCount: 2, exactSigilIds: ['GEEN_129_91', 'GEEN_129_90'] }),
  trait('SKILL_129_02', 15, '极致的战气是角色专属增伤槽。', "Founder's Warpath is the character-specific damage slot."),
  trait('SKILL_154_00', 15, '狂战士直接占一个画面槽位，不能误写成狂战士回响。', 'Berserker directly occupies one recorded slot and must not be mislabeled as Berserker Echo.'),
  trait('MEMORY_TRAIT_73220725', 15, '天星之止息使用运行时补充目录里的真实 Hash。', 'Celestial Ventus uses its real runtime-supplement hash.'),
  trait('MEMORY_TRAIT_9232DC17', 15, '天星之界使用运行时补充目录里的真实 Hash。', 'Celestial Terra uses its real runtime-supplement hash.'),
  trait('MEMORY_TRAIT_D029FE08', 15, '浪迹天涯使用运行时补充目录里的真实 Hash。', 'Fatebreaker uses its real runtime-supplement hash.'),
  trait('SKILL_001_00', 30, '视频装备页使用 2 枚体力 V+。', 'The recorded equipment page uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_004_00', 45, '视频装备页使用 3 枚昏厥 V+。', 'The recorded equipment page uses three Stun Power V+ sigils.', { slotCount: 3 }),
]

const idGraduationFactorCore = () => [
  trait('MEMORY_TRAIT_73220725', 15, '天星之止息 V+ 使用运行时补充目录里的真实 Hash，画面副词条为天星之煌。', 'Celestial Ventus V+ uses its real runtime-supplement hash with the recorded Celestial Lumen secondary.', { sigilId: 'MEMORY_SIGIL_9300FADB', secondaryTraitId: 'MEMORY_TRAIT_A7726190' }),
  trait('SKILL_130_01', 15, '异能战意是画面直接装备的角色专属槽。', 'Versalis Ignition is the recorded character-specific slot.', { sigilId: 'GEEN_130_92' }),
  trait('SKILL_130_02', 15, '异能之心延长形态并提供角色专属造成伤害。', 'Versalis Heart extends the form and supplies character-specific damage.', { sigilId: 'GEEN_130_93' }),
  trait('SKILL_063_00', 15, '躲避性能提供高速动作中的实战容错。', 'Improved Dodge supplies safety during fast actions.'),
  trait('SKILL_003_00', 15, '暴击率直接占一个画面槽位。', 'Critical Hit Rate directly occupies one recorded slot.'),
  trait('SKILL_000_00', 30, '视频装备页使用 2 枚攻击力 V+。', 'The recorded equipment page uses two ATK V+ sigils.', { slotCount: 2 }),
  trait('SKILL_159_00', 15, '摇曳步是画面采用的闪避型容错槽。', 'Flight over Fight is the recorded dodge-based safety slot.'),
  trait('SKILL_144_00', 15, '刚健补充高血量防御。', 'Stronghold adds high-HP defense.'),
  trait('SKILL_166_00', 15, '金刚提高最大 HP。', 'Greater Aegis raises maximum HP.'),
  trait('SKILL_004_00', 30, '视频装备页使用 2 枚昏厥 V+。', 'The recorded equipment page uses two Stun Power V+ sigils.', { slotCount: 2 }),
]

const sandalphonPrimarchFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_144_00', 15, '刚健覆盖高血量阶段。', 'Stronghold covers the high-HP phase.'),
  trait('SKILL_085_00', 15, '守护提供基础 HP。', 'Aegis supplies base HP.'),
  trait('SKILL_166_00', 15, '金刚进一步提高 HP。', 'Greater Aegis further raises HP.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_159_00', 15, '摇曳步是画面中实际采用的容错槽。', 'Flight over Fight is the recorded safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_172_00', 15, '天司长的灵威是本路线的角色专属核心。', 'Supreme Primarch’s Awe is the character-specific core of this route.'),
  trait('SKILL_172_02', 15, '天司长的战气提供角色增伤。', 'Supreme Primarch’s Warpath supplies character damage.'),
]

const seofonWarpathFactorCore = () => [
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_106_00', 15, '明镜止水强化精准闪避后的攻击与奥义循环。', 'Nimble Onslaught strengthens the post-dodge attack and SBA loop.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健覆盖高血量阶段。', 'Stronghold covers the high-HP phase.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_063_00', 15, '躲避性能用于取消动作并扩大容错。', 'Improved Dodge cancels recovery and expands the safety window.'),
  trait('SKILL_159_00', 15, '摇曳步是视频实际使用的闪避型生存槽。', 'Flight over Fight is the recorded dodge-based safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_170_02', 15, '剑圣的战气是这套实战路线的角色专属增伤槽。', 'Spirit Edge’s Warpath is the character-specific damage slot for this route.'),
]

const tweyenAwakeningFactorCore = () => [
  trait('SKILL_000_00', 45, '视频使用 3 枚攻击力 V+。', 'The recorded build uses three ATK V+ sigils.', { slotCount: 3 }),
  trait('SKILL_004_00', 15, '视频使用 1 枚昏厥 V+。', 'The recorded build uses one Stun Power V+ sigil.'),
  trait('SKILL_151_00', 15, '追击提高索恩多段攻击的收益。', 'Supplementary DMG benefits Tweyen’s multihit attacks.'),
  trait('SKILL_027_00', 30, '视频使用 2 枚暴君 V+。', 'The recorded build uses two Tyranny V+ sigils.', { slotCount: 2 }),
  trait('MEMORY_TRAIT_9232DC17', 15, '视频的天星之界 V+直接占一个因子槽。', 'The recorded Celestial Terra V+ directly occupies one sigil slot.'),
  trait('MEMORY_TRAIT_A7726190', 15, '视频的天星之煌 V+直接占一个因子槽。', 'The recorded Celestial Lumen V+ directly occupies one sigil slot.'),
  trait('SKILL_159_00', 15, '摇曳步为高压输出保留闪避容错。', 'Flight over Fight preserves dodge safety for the high-pressure damage route.'),
  trait('SKILL_171_00', 15, '魔眼之觉醒+以魔眼的飞矢为主词条，固定第二词条不另占槽。', 'Dark Huntress’s Awakening+ uses Volley as its primary; the fixed secondary does not consume another slot.'),
  trait('SKILL_171_02', 15, '魔眼的战气提供角色专属增伤。', 'Dark Huntress’s Warpath supplies character-specific damage.'),
]

const gallanzaWarpathFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_106_00', 15, '明镜止水强化精准闪避后的输出循环。', 'Nimble Onslaught strengthens the post-dodge damage loop.'),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健覆盖高血量阶段。', 'Stronghold covers the high-HP phase.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_159_00', 15, '摇曳步是画面中的闪避型生存槽。', 'Flight over Fight is the recorded dodge-based safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_173_02', 15, '狼王的战气是这条实战路线的角色专属增伤槽。', 'Gladiator’s Warpath is the character-specific damage slot for this route.'),
]

const maglielleBladequeenFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健覆盖高血量阶段。', 'Stronghold covers the high-HP phase.'),
  trait('SKILL_063_00', 15, '躲避性能用于取消动作并扩大容错。', 'Improved Dodge cancels recovery and expands the safety window.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_174_01', 15, '刃姬的轮舞曲是本路线的角色循环核心。', 'Bladequeen’s Circuit is the character-loop core of this route.'),
  trait('SKILL_174_02', 15, '刃姬的战气提供角色专属增伤。', 'Bladequeen’s Warpath supplies character-specific damage.'),
]

const maglielleFatebreakerFactorCore = () => [
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_001_00', 30, '视频使用 2 枚体力 V+。', 'The recorded build uses two HP V+ sigils.', { slotCount: 2 }),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_151_00', 15, '追击补充多段攻击收益。', 'Supplementary DMG benefits multihit attacks.'),
  trait('SKILL_063_00', 15, '躲避性能用于取消动作并扩大容错。', 'Improved Dodge cancels recovery and expands the safety window.'),
  trait('SKILL_106_00', 15, '明镜止水强化精准闪避后的循环。', 'Nimble Onslaught strengthens the post-dodge loop.'),
  trait('SKILL_174_02', 15, '刃姬的战气提供角色专属增伤。', 'Bladequeen’s Warpath supplies character-specific damage.'),
  trait('SKILL_174_01', 15, '刃姬的轮舞曲支撑角色循环。', 'Bladequeen’s Circuit supports the character loop.'),
  trait('MEMORY_TRAIT_D029FE08', 15, '浪迹天涯 V+ 是该视频直接装备的任务特化槽。', 'The recorded Fatebreaker V+ is the quest-specific slot for this route.'),
]

const beatrixWarpathFactorCore = () => [
  trait('SKILL_001_00', 30, '视频使用 2 枚体力 V+。', 'The recorded build uses two HP V+ sigils.', { slotCount: 2 }),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健覆盖高血量阶段。', 'Stronghold covers the high-HP phase.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_063_00', 15, '躲避性能用于取消动作并扩大容错。', 'Improved Dodge cancels recovery and expands the safety window.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_175_02', 15, '群青的战气是本路线的角色专属增伤槽。', 'Ultramarine’s Warpath is the character-specific damage slot for this route.'),
]

const eustaceWarpathFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_111_00', 30, '视频使用 2 枚快速蓄力 V+。', 'The recorded build uses two Quick Charge V+ sigils.', { slotCount: 2 }),
  trait('SKILL_060_00', 30, '视频使用 2 枚格挡性能 V+。', 'The recorded build uses two Improved Guard V+ sigils.', { slotCount: 2 }),
  trait('SKILL_159_00', 15, '摇曳步是画面中的闪避型生存槽。', 'Flight over Fight is the recorded dodge-based safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_176_02', 15, '雷狼的战气是蓄力路线的角色专属增伤槽。', 'Thunderwolf’s Warpath is the character-specific damage slot for the charge route.'),
]

const frauxWarpathFactorCore = () => [
  trait('SKILL_001_00', 15, '视频使用 1 枚体力 V+。', 'The recorded build uses one HP V+ sigil.'),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_023_00', 15, '保留天然单技能万能药+，不虚构副词条。', 'Keeps the natural single-trait Potent Greens+ without inventing a secondary.'),
  trait('SKILL_166_00', 15, '金刚补足 HP。', 'Greater Aegis supplies HP.'),
  trait('SKILL_144_00', 15, '刚健覆盖高血量阶段。', 'Stronghold covers the high-HP phase.'),
  trait('SKILL_096_00', 15, '坚持降低霸体期间的承伤。', 'Steel Nerves reduces damage while Stout Heart is active.'),
  trait('SKILL_159_00', 15, '摇曳步是画面中的闪避型生存槽。', 'Flight over Fight is the recorded dodge-based safety slot.'),
  trait('BF78FBFC', 20, '保留视频中的漆黑钳蟹因子。', 'Keeps the recorded Dread Black Pincer Crab Sigil.'),
  trait('SKILL_177_00', 15, '转世的恩宠是本路线的角色专属核心。', 'Enchantress’s Blessing is the character-specific core of this route.'),
  trait('SKILL_177_02', 15, '转世的战气提供角色专属增伤。', 'Enchantress’s Warpath supplies character-specific damage.'),
]

const fedielBalancedFactorCore = () => [
  trait('SKILL_178_02', 15, '黑龙的战气是这条均衡路线的角色专属增伤槽。', 'The Black’s Warpath is the character-specific damage slot for this balanced route.'),
  trait('SKILL_146_00', 15, '属性克制转换在视频的 12 槽中直接占一槽。', 'War Elemental directly occupies one of the twelve recorded slots.'),
  trait('MEMORY_TRAIT_A7726190', 15, '视频的天星之煌 V+直接占一个因子槽。', 'The recorded Celestial Lumen V+ directly occupies one sigil slot.'),
  trait('SKILL_020_00', 15, '视频使用 1 枚伤害上限 V+。', 'The recorded build uses one DMG Cap V+ sigil.'),
  trait('SKILL_154_00', 15, '视频槽位是普通因子“狂战士+”，不是召唤石来源的狂战士追击。', 'The recorded slot is the Berserker+ sigil, not the summon-sourced Berserker Echo.'),
  trait('SKILL_234_00', 15, '斯巴达+在视频里直接占一个因子槽。', 'Spartan+ directly occupies one recorded sigil slot.'),
  trait('SKILL_000_00', 30, '视频使用 2 枚攻击力 V+。', 'The recorded build uses two ATK V+ sigils.', { slotCount: 2 }),
  trait('SKILL_004_00', 45, '视频使用 3 枚昏厥 V+。', 'The recorded build uses three Stun Power V+ sigils.', { slotCount: 3 }),
  trait('SKILL_085_00', 15, '守护为均衡路线提供基础 HP。', 'Aegis supplies base HP for the balanced route.'),
]

const optional = (traitId, targetLevel, reasonZh, reasonEn, priority = 1, extra = {}) => trait(
  traitId,
  targetLevel,
  reasonZh,
  reasonEn,
  { priority, ...extra },
)

const routes = Object.freeze({
  '2A26B1B2': Object.freeze([
    Object.freeze({
      id: 'gran-class-level-20260728',
      ownerCode: 'PL0000',
      nameZh: 'Class Lv 强化 · 逐帧 12 槽',
      nameEn: 'Class Lv Enhancement · Frame-Verified 12 Slots',
      summaryZh: '古兰与姬塔共用的 Class Lv 强化路线。精确 12 槽来自 7 月 28 日装备页；伤害上限、属性转换、武器和召唤石不从其他视频硬塞进因子槽。',
      summaryEn: 'A shared Class Lv route for Gran and Djeeta. The exact twelve slots come from the July 28 equipment page; cap, elemental conversion, weapon, and summon sources are not copied into sigil slots.',
      actionType: 'normal',
      required: Object.freeze(classLevelFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.classLevel, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  A4ACBA76: Object.freeze([
    Object.freeze({
      id: 'djeeta-class-level-20260728',
      ownerCode: 'PL0100',
      nameZh: 'Class Lv 强化 · 逐帧 12 槽',
      nameEn: 'Class Lv Enhancement · Frame-Verified 12 Slots',
      summaryZh: '姬塔与古兰共用的 Class Lv 强化路线。精确 12 槽来自 7 月 28 日装备页；“挺身而出”和木桩输出是独立替代玩法，不混进本路线。',
      summaryEn: 'A shared Class Lv route for Djeeta and Gran. The exact twelve slots come from the July 28 equipment page; Last-Stand and dummy-output alternatives remain separate.',
      actionType: 'normal',
      required: Object.freeze(classLevelFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.classLevel, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '18E2F9F9': Object.freeze([
    Object.freeze({
      id: 'katalina-ares-assault-20260727',
      ownerCode: 'PL0200',
      nameZh: '阿瑞斯强袭 · 逐帧 12 槽',
      nameEn: 'Ares Assault · Frame-Verified 12 Slots',
      summaryZh: '阿瑞斯强袭输出路线。属性克制转换和守护者的决心确实在 12 槽内；其余伤害上限由当前武器、召唤石和专精实际回读核对。',
      summaryEn: 'An Ares assault damage route. War Elemental and Guardian’s Conviction are recorded in the twelve slots; remaining cap is checked from the current weapon, summons, and mastery.',
      actionType: 'ability',
      required: Object.freeze(katalinaAresFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.katalinaAres, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '079DF0CC': Object.freeze([
    Object.freeze({
      id: 'rackam-aftershock-barrier-20260725',
      ownerCode: 'PL0300',
      nameZh: '战地余波 · 1 HP 屏障',
      nameEn: 'Battlefield Aftershock · 1 HP Barrier',
      summaryZh: '战地余波常驻 1 HP 的专用路线，所以 3 枚坚守是画面事实，不是拉卡姆的通用模板。应用前会保持武器、召唤石与专精不变。',
      summaryEn: 'A route dedicated to Battlefield Aftershock at 1 HP, so the three Garrison sigils are recorded facts rather than a universal Rackam template. Fixed equipment remains unchanged.',
      actionType: 'ability',
      required: Object.freeze(rackamAftershockFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.rackamAftershock, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '4D0A60C3': Object.freeze([
    Object.freeze({
      id: 'io-magic-chain-20260729',
      ownerCode: 'PL0400',
      nameZh: '魔法连锁 · 7 月 29 日逐帧版',
      nameEn: 'Magic Chain · July 29 Frame-Verified',
      summaryZh: '完整逐帧确认的 12 槽：3 昏厥、明镜止水、迅捷能力、万能药、金刚、坚持、躲避性能、摇曳步，以及魔法师的心愿和战气。狂战士、斯巴达、属性克制转换和大量伤害上限来自召唤石、武器与专精，不重复塞进因子。',
      summaryEn: 'Frame-verified 12 slots: 3 Stun Power, Nimble Onslaught, Quick Cooldown, Potent Greens, Greater Aegis, Steel Nerves, Improved Dodge, Flight over Fight, Mage’s Aspiration, and Mage’s Warpath. Berserker, Spartan, War Elemental, and much of the cap come from fixed sources.',
      actionType: 'ability',
      required: Object.freeze(ioMagicChainFactorCore()),
      finalChecks: Object.freeze(ioMagicChainFinalChecks()),
      optional: Object.freeze([]),
      sources: Object.freeze([
        COMMUNITY_ROUTE_SOURCES.ioMagicChain,
        COMMUNITY_ROUTE_SOURCES.characterIndex,
      ]),
      evidence: 'frame-verified-community-build-plus-local-2.0.2-table',
    }),
    Object.freeze({
      id: 'io-online-focus-chain',
      ownerCode: 'PL0400',
      nameZh: '联机毕业 · 专注与魔法连锁',
      nameEn: 'Online Graduation · Focus & Magic Chain',
      summaryZh: '按 7 月 22 日视频逐帧确认的实际 12 槽求解：2 昏厥、2 伤害上限、不动、躲避性能、金刚、怒涛、迅捷能力、漆黑钳蟹，以及心愿和战气。该视频没有清楚展开全部固定来源，因此不套用其他视频的召唤石、武器或专精结论。',
      summaryEn: 'Uses the exact 12-slot build recorded on July 22: 2 Stun Power, 2 DMG Cap, Firm Stance, Improved Dodge, Greater Aegis, Cascade, Quick Cooldown, Dread Black Pincer Crab, Aspiration, and Warpath. The video does not clearly expose every fixed source, so summon, weapon, and mastery conclusions are not copied from another build.',
      actionType: 'ability',
      required: Object.freeze(ioOnlineFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([
        optional('SKILL_073_00', 15, '若副词条允许，可补药水携带数提高联机容错。', 'Potion Hoarder is a useful secondary for online safety.', 4),
        optional('SKILL_111_00', 15, '不改变主槽结构时，可从副词条补一档快速蓄力。', 'Quick Charge can be added through a secondary without changing the core slots.', 3),
        optional('SKILL_072_00', 15, '需要更稳的高难容错时，可从副词条补自动复活。', 'Autorevive is a defensive secondary for difficult encounters.', 2),
      ]),
      sources: Object.freeze([
        COMMUNITY_ROUTE_SOURCES.ioOnlineFocusChain,
        COMMUNITY_ROUTE_SOURCES.ioMagicChain,
        COMMUNITY_ROUTE_SOURCES.ioDlcGraduation,
        COMMUNITY_ROUTE_SOURCES.characterIndex,
      ]),
      evidence: 'frame-verified-community-build-plus-local-2.0.2-table',
    }),
    Object.freeze({
      id: 'io-dlc-graduation-20260718',
      ownerCode: 'PL0400',
      nameZh: 'DLC 毕业 · 体力与坚守',
      nameEn: 'DLC Graduation · HP & Garrison',
      summaryZh: '7 月 18 日视频完整逐帧确认的 12 槽：心愿、3 昏厥、2 体力、躲避性能、战气、明镜止水、坚守、坚持和漆黑钳蟹。视频没有清楚逐项展开固定装备，因此不会套用其他视频的召唤石与武器来源。',
      summaryEn: 'Frame-verified July 18 route: Aspiration, 3 Stun Power, 2 HP, Improved Dodge, Warpath, Nimble Onslaught, Garrison, Steel Nerves, and Dread Black Pincer Crab. Fixed-source details are not copied from a different video.',
      actionType: 'ability',
      required: Object.freeze(ioDlcGraduationFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([
        COMMUNITY_ROUTE_SOURCES.ioDlcGraduation,
      ]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  DD7A151E: Object.freeze([
    Object.freeze({
      id: 'eugen-grenade-fist-dot-20260729',
      ownerCode: 'PL0500',
      nameZh: '榴弹拳 DOT · 逐帧 12 槽',
      nameEn: 'Grenade-Fist DOT · Frame-Verified 12 Slots',
      summaryZh: '榴弹拳多段与 DOT 路线，和瞄准爆裂射击分开。原作者说明实战可从属性转换副词条调整出“不动”，该修正只作为提示，不伪装成原画面槽位。',
      summaryEn: 'A grenade-fist multihit and DOT route kept separate from aimed Burst Fire. The creator’s Firm Stance correction remains a note and is not presented as an original recorded slot.',
      actionType: 'ability',
      required: Object.freeze(eugenGrenadeFistFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.eugenGrenadeFist]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  C8616284: Object.freeze([
    Object.freeze({
      id: 'rosetta-rose-enhancement-20260728',
      ownerCode: 'PL0600',
      nameZh: '玫瑰强化 · 自动攻击',
      nameEn: 'Rose Enhancement · Automatic Attacks',
      summaryZh: '以玫瑰早绽和玫瑰的战气维持玫瑰等级与自动攻击的逐帧路线；不把单人 Link Time 木桩数据当成独立精确槽位。',
      summaryEn: 'A frame-verified route using Rose’s early bloom and Warpath to sustain rose level and automatic attacks; solo Link Time dummy results do not invent a different slot list.',
      actionType: 'ability',
      required: Object.freeze(rosettaRoseFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.rosettaRose, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  C3FFD418: Object.freeze([
    Object.freeze({
      id: 'ferry-pet-whip-20260725',
      ownerCode: 'PL0700',
      nameZh: '宠物强化 · 乱鞭循环',
      nameEn: 'Pet Enhancement · Whip Loop',
      summaryZh: '幽幻之谊与幽幻的战气驱动的宠物/乱鞭路线。SBA 控轴和极限减伤是独立替代玩法，不混入默认毕业槽。',
      summaryEn: 'A pet and whip-loop route driven by Phantasm’s bond and Warpath. SBA control and extreme mitigation remain separate alternatives.',
      actionType: 'ability',
      required: Object.freeze(ferryPetFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.ferryPets, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '22E437E5': Object.freeze([
    Object.freeze({
      id: 'lancelot-high-speed-ability-20260726',
      ownerCode: 'PL0800',
      nameZh: '高速能力 · 南十字星循环',
      nameEn: 'High-Speed Abilities · Southern Cross Loop',
      summaryZh: '以双迅捷、怒涛和明镜止水维持高速能力循环。摇曳步是画面中的容错槽，不会再错误写成“超级精准闪避”。',
      summaryEn: 'Uses double Quick Cooldown, Cascade, and Nimble Onslaught for the high-speed ability loop. The recorded Flight over Fight slot is never mislabeled as Super Ultimate Perfect Dodge.',
      actionType: 'ability',
      required: Object.freeze(lancelotAbilityFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.lancelotAbility, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '2EBE91D5': Object.freeze([
    Object.freeze({
      id: 'vane-heroic-guard-20260725',
      ownerCode: 'PL0900',
      nameZh: '勇往直前 · Y 格挡',
      nameEn: 'Heroic Advance · Y Guard',
      summaryZh: '利用 Y 全程格挡、勇士之觉醒和勇士的战气的舒适路线。勇士之觉醒+固定携带毅力，但只占一个物理因子槽。',
      summaryEn: 'A comfortable Y-guard route using Hero’s Awakening and Warpath. Hero’s Awakening+ includes its fixed Will secondary while occupying one physical sigil slot.',
      actionType: 'normal',
      required: Object.freeze(vaneGuardFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.vaneGuard, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  BDEF7181: Object.freeze([
    Object.freeze({
      id: 'percival-endless-twilight-graduation-20260724',
      ownerCode: 'PL1000',
      nameZh: '王者战气 · 无尽黄昏毕业',
      nameEn: "Lord's Warpath · Endless Twilight Graduation",
      summaryZh: '7 月 24 日完整装备页逐帧确认的 12 槽：王者的战气、属性克制转换、王者行进、躲避性能、3 体力、2 昏厥、坚持、金刚和怒涛。王者行进与王者的战气是两个独立槽，不会错误合并成觉醒因子。',
      summaryEn: "A complete twelve-slot build recorded on July 24: Lord's Warpath, War Elemental, Lord's Procession, Improved Dodge, 3 HP, 2 Stun Power, Steel Nerves, Greater Aegis, and Cascade. Procession and Warpath remain two independent slots.",
      actionType: 'ability',
      required: Object.freeze(percivalGraduationFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.percivalGraduation, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '627BCB0D': Object.freeze([
    Object.freeze({
      id: 'siegfried-dragon-climb-graduation-20260729',
      ownerCode: 'PL1100',
      nameZh: '攻击登龙 · 双专属毕业',
      nameEn: 'Dragon Climb · Dual-Character Graduation',
      summaryZh: '7 月 29 日完整装备页逐帧确认的 12 槽：屠龙者的战气、屠龙者的威猛、躲避性能、坚守、刚健、金刚、坚持、3 体力和 2 昏厥。两个角色专属因子各占一个物理槽，不会误合成觉醒因子。',
      summaryEn: "A complete twelve-slot build recorded on July 29: Dragonslayer's Warpath, Dragonslayer's Dominance, Improved Dodge, Garrison, Stronghold, Greater Aegis, Steel Nerves, 3 HP, and 2 Stun Power. The two character sigils remain independent physical slots.",
      actionType: 'normal',
      required: Object.freeze(siegfriedDragonClimbFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.siegfriedDragonClimb, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  FD3BE362: Object.freeze([
    Object.freeze({
      id: 'charlotta-dlc-graduation-20260727',
      ownerCode: 'PL1200',
      nameZh: '圣骑士战气 · DLC 毕业',
      nameEn: "Holy Knight's Warpath · DLC Graduation",
      summaryZh: '7 月 27 日完整装备页逐帧确认的 12 槽：3 昏厥、2 体力、圣骑士的战气、圣骑士之觉醒、躲避性能、属性克制转换、明镜止水、金刚和漆黑钳蟹。觉醒的剑辉与威光是同一固定壳，不会拆成两个槽。',
      summaryEn: "A complete twelve-slot build recorded on July 27: 3 Stun Power, 2 HP, Holy Knight's Warpath, Holy Knight's Awakening, Improved Dodge, War Elemental, Nimble Onslaught, Greater Aegis, and Dread Black Pincer Crab. Awakening's fixed Luster and Grandeur traits remain one physical slot.",
      actionType: 'normal',
      required: Object.freeze(charlottaGraduationFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.charlottaGraduation, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  FC6CDF7B: Object.freeze([
    Object.freeze({
      id: 'yodarha-three-marks-20260727',
      ownerCode: 'PL1300',
      nameZh: '三幕心得 · 双专属实战',
      nameEn: 'Triple-Shroud · Dual-Character Practical Build',
      summaryZh: '7 月 27 日完整装备页逐帧确认的 12 槽：体力、3 昏厥、奋勇、坚守、金刚、刚健、躲避性能、漆黑钳蟹、变幻自如的妖剑士和战气。两个专属各占一个槽，不会错误合并成觉醒因子。',
      summaryEn: "A complete twelve-slot build recorded on July 27: HP, 3 Stun Power, Stamina, Garrison, Greater Aegis, Stronghold, Improved Dodge, Dread Black Pincer Crab, Swordmaster's Art, and Swordmaster's Warpath. The two character sigils remain independent slots.",
      actionType: 'normal',
      required: Object.freeze(yodarhaThreeMarksFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.yodarhaThreeMarks, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  E7053919: Object.freeze([
    Object.freeze({
      id: 'narmaya-dlc-graduation-20260726',
      ownerCode: 'PL1400',
      nameZh: '斩姬战气 · DLC 毕业',
      nameEn: "Butterfly's Warpath · DLC Graduation",
      summaryZh: '7 月 26 日完整装备页逐帧确认的 12 槽：斩姬之觉醒、斩姬的战气、3 昏厥、2 体力、金刚、浪迹天涯、躲避性能、追击和漆黑钳蟹。0:35 明细确认第 9 槽是浪迹天涯，不会误写成超级奋勇或超级精准闪避。',
      summaryEn: "A complete twelve-slot build recorded on July 26: Butterfly's Awakening, Butterfly's Warpath, 3 Stun Power, 2 HP, Greater Aegis, Fatebreaker, Improved Dodge, Supplementary DMG, and Dread Black Pincer Crab. The 0:35 detail frame confirms Fatebreaker.",
      actionType: 'normal',
      required: Object.freeze(narmayaGraduationFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.narmayaGraduation, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '0D21B430': Object.freeze([
    Object.freeze({
      id: 'zeta-black-crab-final-20260719',
      ownerCode: 'PL1600',
      nameZh: '漆黑钳蟹 · 真红最终配装',
      nameEn: 'Dread Black Pincer Crab · Crimson Final Build',
      summaryZh: '7 月 19 日装备页逐帧确认的完整 12 槽：真红烈焰、真红的战气、2 昏厥、体力、漆黑钳蟹、伤害上限、天星之止息、天星之雪、浪迹天涯和 2 追击。天星与浪迹天涯均使用 2.0.2 运行时补充目录的真实 Hash。',
      summaryEn: "A complete twelve-slot build recorded on July 19: Crimson's Clout, Crimson's Warpath, 2 Stun Power, HP, Dread Black Pincer Crab, DMG Cap, Celestial Ventus, Celestial Aqua, Fatebreaker, and 2 Supplementary DMG. Runtime-only sigils keep their real 2.0.2 hashes.",
      actionType: 'normal',
      required: Object.freeze(zetaBlackCrabFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.zetaBlackCrab, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  F0EB77EF: Object.freeze([
    Object.freeze({
      id: 'vaseraga-full-hp-mitigation-20260726',
      ownerCode: 'PL1700',
      nameZh: '满血减伤 · 拒绝 1 HP',
      nameEn: 'Full-HP Mitigation · No 1-HP Lock',
      summaryZh: '7 月 26 日完整装备页逐帧确认的 12 槽：冥暗之觉醒、漆黑钳蟹、追击、浪迹天涯、坚持、躲避性能、分歧、不动、迅捷能力和 3 昏厥。觉醒固定的冥暗刚刃与冥暗自若只占一个物理槽。',
      summaryEn: "A complete twelve-slot build recorded on July 26: Ebony's Awakening, Dread Black Pincer Crab, Supplementary DMG, Fatebreaker, Steel Nerves, Improved Dodge, Divergence, Firm Stance, Quick Cooldown, and 3 Stun Power. Awakening's fixed Presence and Poise traits occupy one physical slot.",
      actionType: 'normal',
      required: Object.freeze(vaseragaFullHpFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.vaseragaFullHp, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  AA66178A: Object.freeze([
    Object.freeze({
      id: 'cagliostro-dummy-graduation-20260712',
      ownerCode: 'PL1800',
      nameZh: '极致谋略 · 4.7 亿打桩',
      nameEn: "Founder's Strategy · 470M Dummy Build",
      summaryZh: '7 月 12 日完整装备页逐帧确认的 12 槽：极致谋略、极致的战气、狂战士、天星之止息、天星之界、浪迹天涯、2 体力、3 昏厥和极致之觉醒。觉醒固定的谋略与真理不拆成额外槽，狂战士也不会误写成狂战士回响。',
      summaryEn: "A complete twelve-slot build recorded on July 12: Founder's Strategy, Founder's Warpath, Berserker, Celestial Ventus, Celestial Terra, Fatebreaker, 2 HP, 3 Stun Power, and Founder's Awakening. Awakening's fixed Strategy and Truth remain one slot, and Berserker is not mislabeled as Berserker Echo.",
      actionType: 'ability',
      required: Object.freeze(cagliostroGraduationFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.cagliostroGraduation, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  A3A3CB2F: Object.freeze([
    Object.freeze({
      id: 'id-endless-twilight-graduation-20260718',
      ownerCode: 'PL1900',
      nameZh: '异能之心 · 无尽黄昏毕业',
      nameEn: 'Versalis Heart · Endless Twilight Graduation',
      summaryZh: '7 月 18 日完整装备页逐帧确认的 12 槽：天星之止息、异能战意、异能之心、躲避性能、暴击率、2 攻击力、摇曳步、刚健、金刚和 2 昏厥。天星之止息的画面副词条是天星之煌，作为同一 V+ 壳精确匹配，不另算物理槽。',
      summaryEn: 'A complete twelve-slot build recorded on July 18: Celestial Ventus, Versalis Ignition, Versalis Heart, Improved Dodge, Critical Hit Rate, 2 ATK, Flight over Fight, Stronghold, Greater Aegis, and 2 Stun Power. Celestial Lumen is matched as the recorded secondary on the same Ventus V+ shell.',
      actionType: 'normal',
      required: Object.freeze(idGraduationFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.idGraduation, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '978E4B18': Object.freeze([
    Object.freeze({
      id: 'ghandagoza-eternal-rage-20260728',
      ownerCode: 'PL1500',
      nameZh: '威武雄姿特化 · 坚守蓄力拳',
      nameEn: 'Eternal Rage · Garrison Charge',
      summaryZh: '7 月 28 日装备页逐帧确认的完整 12 槽：体力、3 昏厥、明镜止水、4 坚守、躲避性能、不动和古今无双的强者。武器、召唤石与专精在后续画面单独展示，不重复塞进因子槽。',
      summaryEn: "A complete twelve-slot build recorded on July 28: HP, 3 Stun Power, Nimble Onslaught, 4 Garrison, Improved Dodge, Firm Stance, and Eternal Rage's Ethos. Weapon, summons, and mastery are shown separately and are not duplicated in sigil slots.",
      actionType: 'normal',
      required: Object.freeze(ghandagozaEternalRageFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.ghandagozaEternalRage, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '718E1A14': Object.freeze([
    Object.freeze({
      id: 'sandalphon-primarch-awe-20260725',
      ownerCode: 'PL2100',
      nameZh: '天司长灵威 · 实战生存',
      nameEn: 'Supreme Primarch’s Awe · Practical Survival',
      summaryZh: '7 月 25 日画面完整确认的 12 槽。以灵威和战气为角色核心，体力、守护、金刚、刚健、坚持与摇曳步保证实战容错；十二翼和 71 万血为独立路线，不混进这里。',
      summaryEn: 'A complete twelve-slot build recorded on July 25. Awe and Warpath provide the character core while HP, Aegis, Greater Aegis, Stronghold, Steel Nerves, and Flight over Fight preserve practical safety. Twelve-Wing and 710k-HP alternatives remain separate.',
      actionType: 'ability',
      required: Object.freeze(sandalphonPrimarchFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.sandalphonPrimarch, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  '296471BE': Object.freeze([
    Object.freeze({
      id: 'seofon-warpath-practical-20260727',
      ownerCode: 'PL2200',
      nameZh: '剑圣战气 · 精准闪避循环',
      nameEn: 'Spirit Edge Warpath · Perfect-Dodge Loop',
      summaryZh: '7 月 27 日画面完整确认的 12 槽。明镜止水、躲避性能和摇曳步服务于实战循环；召唤流是另一条路线，不会把未展开的槽位猜进本方案。',
      summaryEn: 'A complete twelve-slot build recorded on July 27. Nimble Onslaught, Improved Dodge, and Flight over Fight support the practical loop; the summon route remains separate because its full slots were not shown.',
      actionType: 'normal',
      required: Object.freeze(seofonWarpathFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.seofonWarpath, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-legality',
    }),
  ]),
  BAD16E3B: Object.freeze([
    Object.freeze({
      id: 'tweyen-awakening-celestial-20260728',
      ownerCode: 'PL2300',
      nameZh: '魔眼觉醒 · 天星上限',
      nameEn: 'Dark Huntress Awakening · Celestial Cap',
      summaryZh: '7 月 28 日画面完整确认的高攻击路线：3 攻击、2 暴君、追击，以及天星之界与天星之煌共同补上限。天星因子使用运行时 2.0.2 补充目录中的真实 Hash，不伪造成普通 gem.tbl 因子。',
      summaryEn: 'A complete high-attack route recorded on July 28: 3 ATK, 2 Tyranny, Supplementary DMG, plus Celestial Terra and Lumen for cap. Celestial sigils use their real 2.0.2 runtime-supplement hashes rather than being misrepresented as ordinary gem.tbl rows.',
      actionType: 'normal',
      required: Object.freeze(tweyenAwakeningFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.tweyenAwakening]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '1BB37EF0': Object.freeze([
    Object.freeze({
      id: 'gallanza-warpath-practical-20260726',
      ownerCode: 'PL2400',
      nameZh: '狼王战气 · 实战循环',
      nameEn: 'Gladiator Warpath · Practical Loop',
      summaryZh: '7 月 26 日画面完整确认的 12 槽，以狼王战气为角色核心，配合明镜止水、万能药、金刚、刚健、坚持和摇曳步。陀螺流为独立分支，不会用不完整画面替换本方案。',
      summaryEn: 'A complete twelve-slot build recorded on July 26. Gladiator’s Warpath anchors the character route alongside Nimble Onslaught, Potent Greens, Greater Aegis, Stronghold, Steel Nerves, and Flight over Fight. The spin route remains a separate branch.',
      actionType: 'normal',
      required: Object.freeze(gallanzaWarpathFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.gallanzaWarpath, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '25D46F4B': Object.freeze([
    Object.freeze({
      id: 'maglielle-bladequeen-circuit-20260727',
      ownerCode: 'PL2500',
      nameZh: '刃姬轮舞 · 实战生存',
      nameEn: 'Bladequeen Circuit · Practical Survival',
      summaryZh: '7 月 27 日完整装备页确认的 12 槽，以轮舞曲和战气为角色核心，配合体力、金刚、刚健、坚持与躲避性能。此路线作为默认首选，不混入另一条浪迹天涯方案。',
      summaryEn: 'A complete twelve-slot equipment page recorded on July 27. Circuit and Warpath anchor the character route alongside HP, Greater Aegis, Stronghold, Steel Nerves, and Improved Dodge. It remains separate from the Fatebreaker alternative.',
      actionType: 'ability',
      required: Object.freeze(maglielleBladequeenFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.maglielleBladequeen, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
    Object.freeze({
      id: 'maglielle-fatebreaker-20260726',
      ownerCode: 'PL2500',
      nameZh: '浪迹天涯 · 追击轮舞',
      nameEn: 'Fatebreaker · Supplemental Circuit',
      summaryZh: '7 月 26 日另一份完整 12 槽毕业路线：双体力、3 昏厥、追击、躲避性能、明镜止水、轮舞曲、战气与浪迹天涯。浪迹天涯使用运行时 2.0.2 补充目录的真实 Hash。',
      summaryEn: 'A second complete twelve-slot graduation route recorded on July 26: double HP, 3 Stun Power, Supplementary DMG, Improved Dodge, Nimble Onslaught, Circuit, Warpath, and Fatebreaker. Fatebreaker uses its real 2.0.2 runtime-supplement hash.',
      actionType: 'ability',
      required: Object.freeze(maglielleFatebreakerFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.maglielleFatebreaker]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '9A8AF295': Object.freeze([
    Object.freeze({
      id: 'beatrix-ultramarine-warpath-20260725',
      ownerCode: 'PL2600',
      nameZh: '群青战气 · 高血量实战',
      nameEn: 'Ultramarine Warpath · High-HP Practical',
      summaryZh: '7 月 25 日完整装备页确认的 12 槽。双体力、金刚、刚健、坚持和躲避性能托住实战容错，群青战气负责角色专属增伤。',
      summaryEn: 'A complete twelve-slot equipment page recorded on July 25. Double HP, Greater Aegis, Stronghold, Steel Nerves, and Improved Dodge preserve practical safety while Ultramarine’s Warpath supplies character damage.',
      actionType: 'normal',
      required: Object.freeze(beatrixWarpathFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.beatrixWarpath, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '9B15CFB1': Object.freeze([
    Object.freeze({
      id: 'eustace-thunderwolf-charge-20260726',
      ownerCode: 'PL2700',
      nameZh: '雷狼战气 · 蓄力格挡',
      nameEn: 'Thunderwolf Warpath · Charge & Guard',
      summaryZh: '7 月 26 日完整装备页确认的 12 槽。双快速蓄力与双格挡性能服务于蓄力实战循环，摇曳步和漆黑钳蟹保留容错，雷狼战气提供专属增伤。',
      summaryEn: 'A complete twelve-slot equipment page recorded on July 26. Double Quick Charge and double Improved Guard support the charged combat loop; Flight over Fight and the crab sigil preserve safety while Thunderwolf’s Warpath supplies character damage.',
      actionType: 'normal',
      required: Object.freeze(eustaceWarpathFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.eustaceWarpath, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '646C3168': Object.freeze([
    Object.freeze({
      id: 'fraux-enchantress-blessing-20260726',
      ownerCode: 'PL2800',
      nameZh: '转世恩宠 · 实战生存',
      nameEn: 'Enchantress Blessing · Practical Survival',
      summaryZh: '7 月 26 日完整装备页确认的 12 槽。转世恩宠与战气是角色核心，体力、金刚、刚健、坚持和摇曳步组成实战防线。',
      summaryEn: 'A complete twelve-slot equipment page recorded on July 26. Enchantress’s Blessing and Warpath anchor the character route while HP, Greater Aegis, Stronghold, Steel Nerves, and Flight over Fight form its practical defense.',
      actionType: 'ability',
      required: Object.freeze(frauxWarpathFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.frauxWarpath, COMMUNITY_ROUTE_SOURCES.characterIndex]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
  '74DD4C79': Object.freeze([
    Object.freeze({
      id: 'fediel-balanced-black-dragon-20260717',
      ownerCode: 'PL2900',
      nameZh: '黑龙战气 · 均衡输出',
      nameEn: 'The Black’s Warpath · Balanced Damage',
      summaryZh: '作者明确称为“相对均衡”的完整 12 槽路线：黑龙战气、属性克制转换、天星之煌、伤害上限、狂战士、斯巴达、双攻击、3 昏厥与守护。肉装和高秒伤路线目前没有完整逐槽画面，不会混入。',
      summaryEn: 'A complete twelve-slot route explicitly described as relatively balanced: The Black’s Warpath, War Elemental, Celestial Lumen, DMG Cap, Berserker, Spartan, double ATK, 3 Stun Power, and Aegis. Tank and higher-DPS alternatives are not merged without full slot evidence.',
      actionType: 'ability',
      required: Object.freeze(fedielBalancedFactorCore()),
      finalChecks: Object.freeze([]),
      optional: Object.freeze([]),
      sources: Object.freeze([COMMUNITY_ROUTE_SOURCES.fedielBalanced]),
      evidence: 'frame-verified-sigil-slots-local-2.0.2-runtime-supplement',
    }),
  ]),
})

export function characterBuildRoutes(charaHash) {
  return routes[String(charaHash || '').replace(/^0x/i, '').toUpperCase()] || []
}

export function routeTraitTargets(route, atlas) {
  const traitById = new Map((atlas?.traits || []).map(item => [item.internalId, item]))
  const requiredIds = new Set((route?.required || []).map(item => item.traitId))
  const merged = new Map()
  for (const item of [...(route?.required || []), ...(route?.optional || [])]) {
    const current = merged.get(item.traitId)
    if (!current || Number(item.targetLevel || 0) > Number(current.targetLevel || 0)) merged.set(item.traitId, item)
  }
  return [...merged.values()].map(item => {
    const catalog = traitById.get(item.traitId)
    return {
      ...item,
      name: catalog?.displayName || item.traitId,
      cap: Math.max(1, Number(item.targetLevel || catalog?.maxLevel || 15)),
      weight: requiredIds.has(item.traitId) ? 100 : Math.max(1, Number(item.priority || 1)),
      required: requiredIds.has(item.traitId),
    }
  })
}
