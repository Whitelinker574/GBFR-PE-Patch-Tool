const portraitLayout = {
  gran:       [[1145, 1600], [0, 0.1588, 0.5563, 0.7875], [0.25, 0.27]],
  djeeta:     [[1207, 1600], [0.0008, 0.2381, 0.4963, 0.7081], [0.33, 0.30]],
  katalina:   [[1600, 1547], [0.01, 0.159, 0.8363, 0.788], [0.43, 0.25]],
  rackam:     [[1327, 1600], [0.0068, 0.1694, 0.5019, 0.7106], [0.30, 0.28]],
  io:         [[1564, 1600], [0.0051, 0.2531, 0.7263, 0.6563], [0.35, 0.34]],
  eugen:      [[1527, 1600], [0.0046, 0.1269, 0.7007, 0.8194], [0.42, 0.20]],
  rosetta:    [[1600, 1564], [0.0075, 0.1669, 0.7181, 0.7794], [0.38, 0.27]],
  ferry:      [[1564, 1600], [0.0051, 0.1569, 0.7257, 0.7894], [0.43, 0.27]],
  lancelot:   [[1418, 1600], [0.0049, 0.0931, 0.6312, 0.8531], [0.35, 0.20]],
  vane:       [[1527, 1600], [0.0033, 0, 0.6994, 0.9463], [0.35, 0.15]],
  percival:   [[1600, 1547], [0.0044, 0.084, 0.775, 0.8623], [0.43, 0.20]],
  siegfried:  [[1455, 1600], [0.0124, 0.0144, 0.6316, 0.9319], [0.35, 0.16]],
  charlotta:  [[1400, 1600], [0.0143, 0.1769, 0.565, 0.63], [0.43, 0.30]],
  yodarha:    [[1218, 1600], [0.0115, 0.2806, 0.3604, 0.5262], [0.20, 0.38]],
  narmaya:    [[1509, 1600], [0.0133, 0.1744, 0.6514, 0.7075], [0.36, 0.28]],
  ghandagoza: [[1600, 1600], [0.01, 0.0044, 0.715, 0.9419], [0.34, 0.15]],
  zeta:       [[1600, 1547], [0.0025, 0.0052, 0.8119, 0.9412], [0.42, 0.15]],
  vaseraga:   [[1600, 1437], [0.005, 0, 0.915, 0.9464], [0.46, 0.14]],
  cagliostro: [[1364, 1600], [0.0117, 0.2256, 0.5139, 0.6488], [0.36, 0.33]],
  id:         [[1564, 1600], [0.0064, 0.06, 0.6535, 0.8862], [0.40, 0.20]],
  sandalphon: [[1600, 1145], [0.0031, 0, 0.9969, 0.9459], [0.50, 0.30]],
  seofon:     [[1545, 1600], [0.011, 0, 0.7489, 0.9463], [0.40, 0.18]],
  tweyen:     [[1600, 1530], [0.0119, 0.0941, 0.7731, 0.8523], [0.42, 0.22]],
  gallanza:   [[1600, 1193], [0.0169, 0.0872, 0.7937, 0.8592], [0.45, 0.20]],
  maglielle:  [[1600, 1328], [0.0094, 0.1506, 0.9906, 0.6807], [0.52, 0.30]],
  beatrix:    [[1600, 1530], [0.01, 0.2641, 0.6731, 0.6575], [0.39, 0.37]],
  eustace:    [[1600, 1408], [0.0069, 0, 0.6687, 0.9467], [0.35, 0.18]],
  fraux:      [[1491, 1600], [0.0094, 0.1694, 0.6821, 0.7769], [0.40, 0.24]],
  fediel:     [[1600, 1341], [0.0025, 0.0425, 0.8044, 0.9038], [0.52, 0.22]],
}

const rosterRows = [
  ['2A26B1B2', 'PL0000', 'gran', '古兰', 'Gran'],
  ['A4ACBA76', 'PL0100', 'djeeta', '姬塔', 'Djeeta'],
  ['18E2F9F9', 'PL0200', 'katalina', '卡塔莉娜', 'Katalina'],
  ['079DF0CC', 'PL0300', 'rackam', '拉卡姆', 'Rackam'],
  ['4D0A60C3', 'PL0400', 'io', '伊欧', 'Io'],
  ['DD7A151E', 'PL0500', 'eugen', '欧根', 'Eugen'],
  ['C8616284', 'PL0600', 'rosetta', '萝赛塔', 'Rosetta'],
  ['C3FFD418', 'PL0700', 'ferry', '菲莉', 'Ferry'],
  ['22E437E5', 'PL0800', 'lancelot', '兰斯洛特', 'Lancelot'],
  ['2EBE91D5', 'PL0900', 'vane', '巴恩', 'Vane'],
  ['BDEF7181', 'PL1000', 'percival', '珀西瓦尔', 'Percival'],
  ['627BCB0D', 'PL1100', 'siegfried', '齐格飞', 'Siegfried'],
  ['FD3BE362', 'PL1200', 'charlotta', '夏洛特', 'Charlotta'],
  ['FC6CDF7B', 'PL1300', 'yodarha', '尤达拉哈', 'Yodarha'],
  ['E7053919', 'PL1400', 'narmaya', '娜露梅', 'Narmaya'],
  ['978E4B18', 'PL1500', 'ghandagoza', '冈达葛萨', 'Ghandagoza'],
  ['0D21B430', 'PL1600', 'zeta', '泽塔', 'Zeta'],
  ['F0EB77EF', 'PL1700', 'vaseraga', '巴萨拉卡', 'Vaseraga'],
  ['AA66178A', 'PL1800', 'cagliostro', '卡莉奥丝特罗', 'Cagliostro'],
  ['A3A3CB2F', 'PL1900', 'id', '伊德', 'Id'],
  ['718E1A14', 'PL2100', 'sandalphon', '圣德芬', 'Sandalphon'],
  ['296471BE', 'PL2200', 'seofon', '希耶提', 'Seofon'],
  ['BAD16E3B', 'PL2300', 'tweyen', '索恩', 'Tweyen'],
  ['1BB37EF0', 'PL2400', 'gallanza', '伽兰查', 'Gallanza'],
  ['25D46F4B', 'PL2500', 'maglielle', '玛琪拉菲菈', 'Maglielle'],
  ['9A8AF295', 'PL2600', 'beatrix', '贝阿朵丽丝', 'Beatrix'],
  ['9B15CFB1', 'PL2700', 'eustace', '尤斯提斯', 'Eustace'],
  ['646C3168', 'PL2800', 'fraux', '芙劳', 'Fraux'],
  ['74DD4C79', 'PL2900', 'fediel', '菲迪埃尔', 'Fediel'],
]

function portraitProfile(slug) {
  const [intrinsicSize, safeFrame, focus] = portraitLayout[slug]
  return Object.freeze({
    path: `/share-portraits/${slug}.webp`,
    intrinsicSize: Object.freeze([...intrinsicSize]),
    faceFocus: Object.freeze([...focus]),
    weaponSafeFrame: Object.freeze([...safeFrame]),
    anchors: Object.freeze({
      landscape: Object.freeze({ fit: 'cover', focus: Object.freeze([...focus]) }),
      portrait: Object.freeze({ fit: 'cover', focus: Object.freeze([...focus]) }),
      square: Object.freeze({ fit: 'cover', focus: Object.freeze([...focus]) }),
    }),
  })
}

export const characterRoster = Object.freeze(rosterRows.map(([hash, plId, slug, nameZh, nameEn]) => Object.freeze({
  hash,
  plId,
  slug,
  nameZh,
  nameEn,
  portrait: portraitProfile(slug),
})))

const byHash = new Map(characterRoster.map(item => [item.hash, item]))
const byPLID = new Map(characterRoster.map(item => [item.plId, item]))

export function characterIdentityByHash(hash) {
  return byHash.get(String(hash || '').replace(/^0x/i, '').toUpperCase()) || null
}

export function characterIdentityByPLID(plId) {
  return byPLID.get(String(plId || '').toUpperCase()) || null
}

export function characterSlug(hash) {
  return characterIdentityByHash(hash)?.slug || ''
}

export function characterNameByPLID(plId, locale = 'zh') {
  const identity = characterIdentityByPLID(plId)
  if (!identity) return ''
  return locale === 'en' ? identity.nameEn : identity.nameZh
}

export function characterNamePairByPLID(plId) {
  const identity = characterIdentityByPLID(plId)
  return identity ? [identity.nameZh, identity.nameEn] : null
}

export function characterSharePortrait(hash) {
  return characterIdentityByHash(hash)?.portrait.path || ''
}

export function characterSharePortraitProfile(hash) {
  return characterIdentityByHash(hash)?.portrait || null
}

export const characterSharePortraits = Object.freeze(Object.fromEntries(characterRoster.map(item => [item.hash, item.portrait.path])))
