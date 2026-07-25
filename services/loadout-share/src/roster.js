const roster = [
  { slug: 'gran', name: '格兰', nameEn: 'Gran', plId: 'pl0000', hash: '2A26B1B2', dlc: false, accent: '#85704b' },
  { slug: 'djeeta', name: '姬塔', nameEn: 'Djeeta', plId: 'pl0100', hash: 'A4ACBA76', dlc: false, accent: '#9c543f' },
  { slug: 'katalina', name: '卡塔莉娜', nameEn: 'Katalina', plId: 'pl0200', hash: '18E2F9F9', dlc: false, accent: '#50798e' },
  { slug: 'rackam', name: '拉卡姆', nameEn: 'Rackam', plId: 'pl0300', hash: '079DF0CC', dlc: false, accent: '#8e4f3a' },
  { slug: 'io', name: '伊欧', nameEn: 'Io', plId: 'pl0400', hash: '4D0A60C3', dlc: false, accent: '#758d45' },
  { slug: 'eugen', name: '欧根', nameEn: 'Eugen', plId: 'pl0500', hash: 'DD7A151E', dlc: false, accent: '#78614a' },
  { slug: 'rosetta', name: '萝赛塔', nameEn: 'Rosetta', plId: 'pl0600', hash: 'C8616284', dlc: false, accent: '#8d526c', aliases: ['罗赛塔'] },
  { slug: 'ferry', name: '菲莉', nameEn: 'Ferry', plId: 'pl0700', hash: 'C3FFD418', dlc: false, accent: '#7e6f9c' },
  { slug: 'lancelot', name: '兰斯洛特', nameEn: 'Lancelot', plId: 'pl0800', hash: '22E437E5', dlc: false, accent: '#4b7799' },
  { slug: 'vane', name: '巴恩', nameEn: 'Vane', plId: 'pl0900', hash: '2EBE91D5', dlc: false, accent: '#7b6a43' },
  { slug: 'percival', name: '珀西瓦尔', nameEn: 'Percival', plId: 'pl1000', hash: 'BDEF7181', dlc: false, accent: '#94443b', aliases: ['帕西瓦尔'] },
  { slug: 'siegfried', name: '齐格飞', nameEn: 'Siegfried', plId: 'pl1100', hash: '627BCB0D', dlc: false, accent: '#59624d' },
  { slug: 'charlotta', name: '夏洛特', nameEn: 'Charlotta', plId: 'pl1200', hash: 'FD3BE362', dlc: false, accent: '#8e743f' },
  { slug: 'yodarha', name: '尤达拉哈', nameEn: 'Yodarha', plId: 'pl1300', hash: 'FC6CDF7B', dlc: false, accent: '#6d7d5a' },
  { slug: 'narmaya', name: '娜露梅', nameEn: 'Narmaya', plId: 'pl1400', hash: 'E7053919', dlc: false, accent: '#875d8d', aliases: ['娜露梅亚'] },
  { slug: 'ghandagoza', name: '冈达葛萨', nameEn: 'Ghandagoza', plId: 'pl1500', hash: '978E4B18', dlc: false, accent: '#8d593b', aliases: ['刚达葛萨'] },
  { slug: 'zeta', name: '泽塔', nameEn: 'Zeta', plId: 'pl1600', hash: '0D21B430', dlc: false, accent: '#a04d40', aliases: ['塞达', '婕塔'] },
  { slug: 'vaseraga', name: '巴萨拉卡', nameEn: 'Vaseraga', plId: 'pl1700', hash: 'F0EB77EF', dlc: false, accent: '#5c536d' },
  { slug: 'cagliostro', name: '卡莉奥丝特罗', nameEn: 'Cagliostro', plId: 'pl1800', hash: 'AA66178A', dlc: false, accent: '#a06e51', aliases: ['卡莉奥斯特萝'] },
  { slug: 'id', name: '伊德', nameEn: 'Id', plId: 'pl1900', hash: 'A3A3CB2F', dlc: false, accent: '#4f6176' },
  { slug: 'sandalphon', name: '圣德芬', nameEn: 'Sandalphon', plId: 'pl2100', hash: '718E1A14', dlc: true, accent: '#77664f' },
  { slug: 'seofon', name: '希耶提', nameEn: 'Seofon', plId: 'pl2200', hash: '296471BE', dlc: true, accent: '#587a86', aliases: ['西欧丰'] },
  { slug: 'tweyen', name: '索恩', nameEn: 'Tweyen', plId: 'pl2300', hash: 'BAD16E3B', dlc: true, accent: '#8a6f47', aliases: ['图维恩'] },
  { slug: 'gallanza', name: '伽兰查', nameEn: 'Gallanza', plId: 'pl2400', hash: '1BB37EF0', dlc: true, accent: '#735443', aliases: ['加兰查'] },
  { slug: 'maglielle', name: '玛琪拉菲菈', nameEn: 'Maglielle', plId: 'pl2500', hash: '25D46F4B', dlc: true, accent: '#756080' },
  { slug: 'beatrix', name: '贝阿朵丽丝', nameEn: 'Beatrix', plId: 'pl2600', hash: '9A8AF295', dlc: true, accent: '#9a5144' },
  { slug: 'eustace', name: '尤斯提斯', nameEn: 'Eustace', plId: 'pl2700', hash: '9B15CFB1', dlc: true, accent: '#596851', aliases: ['尤斯塔斯'] },
  { slug: 'fraux', name: '芙劳', nameEn: 'Fraux', plId: 'pl2800', hash: '646C3168', dlc: true, accent: '#96534c' },
  { slug: 'fediel', name: '菲迪埃尔', nameEn: 'Fediel', plId: 'pl2900', hash: '74DD4C79', dlc: true, accent: '#67546f' },
]

export const CHARACTER_ROSTER = Object.freeze(roster.map(character => Object.freeze({
  ...character,
  iconFile: `cmn_mini_s_${character.plId}.png`,
})))

export function characterByIdentity(name, hash = '') {
  const normalizedName = String(name || '').trim().toLocaleLowerCase('zh-CN')
  const normalizedHash = String(hash || '').trim().replace(/^0x/i, '').toUpperCase()
  return CHARACTER_ROSTER.find(character => (
    normalizedHash && character.hash === normalizedHash
  ) || [character.name, character.nameEn, character.slug, ...(character.aliases || [])]
    .some(value => value.toLocaleLowerCase('zh-CN') === normalizedName)) || CHARACTER_ROSTER[0]
}
