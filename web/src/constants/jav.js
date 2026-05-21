export const JAV_PROVIDER_UNKNOWN = 0
export const JAV_PROVIDER_JAVBUS = 1
export const JAV_PROVIDER_JAVDATABASE = 2
export const JAV_PROVIDER_USER = 3

export const JAV_SORT_OPTIONS = [
  {
    base: 'recent',
    defaultValue: 'recent',
    ascValue: 'recent_asc',
    descValue: 'recent',
    label: ['加入时间', 'Added time'],
    asc: ['远→近', 'old→new'],
    desc: ['近→远', 'new→old'],
  },
  {
    base: 'code',
    defaultValue: 'code',
    ascValue: 'code',
    descValue: 'code_desc',
    label: ['番号', 'Code'],
    asc: ['A-Z', 'A-Z'],
    desc: ['Z-A', 'Z-A'],
  },
  {
    base: 'duration',
    defaultValue: 'duration',
    ascValue: 'duration_asc',
    descValue: 'duration',
    label: ['时长', 'Duration'],
    asc: ['短→长', 'short→long'],
    desc: ['长→短', 'long→short'],
  },
  {
    base: 'release',
    defaultValue: 'release',
    ascValue: 'release_asc',
    descValue: 'release',
    label: ['发行时间', 'Release date'],
    asc: ['远→近', 'old→new'],
    desc: ['近→远', 'new→old'],
  },
  {
    base: 'play_count',
    defaultValue: 'play_count',
    ascValue: 'play_count_asc',
    descValue: 'play_count',
    label: ['播放次数', 'Play count'],
    asc: ['少→多', 'low→high'],
    desc: ['多→少', 'high→low'],
  },
]

export const IDOL_SORT_OPTIONS = [
  {
    base: 'recent',
    defaultValue: 'recent',
    ascValue: 'recent_asc',
    descValue: 'recent',
    label: ['加入时间', 'Added time'],
    asc: ['远→近', 'old→new'],
    desc: ['近→远', 'new→old'],
  },
  {
    base: 'work',
    defaultValue: 'work',
    ascValue: 'work_asc',
    descValue: 'work',
    label: ['作品数量', 'Work count'],
    asc: ['少→多', 'low→high'],
    desc: ['多→少', 'high→low'],
  },
  {
    base: 'birth',
    defaultValue: 'birth',
    ascValue: 'birth',
    descValue: 'birth_asc',
    label: ['年龄', 'Age'],
    asc: ['小→大', 'young→old'],
    desc: ['大→小', 'old→young'],
  },
  {
    base: 'height',
    defaultValue: 'height',
    ascValue: 'height',
    descValue: 'height_desc',
    label: ['身高', 'Height'],
    asc: ['低→高', 'short→tall'],
    desc: ['高→低', 'tall→short'],
  },
  {
    base: 'bust',
    defaultValue: 'bust',
    ascValue: 'bust_asc',
    descValue: 'bust',
    label: ['胸围', 'Bust'],
    asc: ['小→大', 'small→large'],
    desc: ['大→小', 'large→small'],
  },
  {
    base: 'hips',
    defaultValue: 'hips',
    ascValue: 'hips_asc',
    descValue: 'hips',
    label: ['臀围', 'Hips'],
    asc: ['小→大', 'small→large'],
    desc: ['大→小', 'large→small'],
  },
  {
    base: 'waist',
    defaultValue: 'waist',
    ascValue: 'waist',
    descValue: 'waist_desc',
    label: ['腰围', 'Waist'],
    asc: ['小→大', 'small→large'],
    desc: ['大→小', 'large→small'],
  },
  {
    base: 'cup',
    defaultValue: 'cup',
    ascValue: 'cup_asc',
    descValue: 'cup',
    label: ['罩杯', 'Cup'],
    asc: ['小→大', 'small→large'],
    desc: ['大→小', 'large→small'],
  },
]

const buildSortMap = (options) => {
  const entries = new Map()
  for (const option of options) {
    entries.set(option.defaultValue, option)
    entries.set(option.ascValue, option)
    entries.set(option.descValue, option)
  }
  return entries
}

const javSortMap = buildSortMap(JAV_SORT_OPTIONS)
const idolSortMap = buildSortMap(IDOL_SORT_OPTIONS)

const normalizeFromOptions = (sort, fallback, optionsMap, aliases = {}) => {
  const key = String(sort || '')
    .trim()
    .toLowerCase()
  if (aliases[key]) return aliases[key]
  if (optionsMap.has(key)) return key
  return fallback
}

export function normalizeJavSort(sort, fallback = 'recent') {
  return normalizeFromOptions(sort, fallback, javSortMap, {
    recent_desc: 'recent',
    code_asc: 'code',
    duration_desc: 'duration',
    release_desc: 'release',
    play_count_desc: 'play_count',
  })
}

export function normalizeIdolSort(sort, fallback = 'work') {
  return normalizeFromOptions(sort, fallback, idolSortMap, {
    measurements: 'bust',
    measure: 'bust',
    bwh: 'bust',
    recent_desc: 'recent',
    added: 'recent',
    created: 'recent',
    created_at: 'recent',
    work_count: 'work',
    count: 'work',
    work_desc: 'work',
    birth_desc: 'birth',
    age: 'birth',
    age_asc: 'birth',
    age_desc: 'birth_asc',
    height_asc: 'height',
    bust_desc: 'bust',
    hip: 'hips',
    hips_desc: 'hips',
    waist_asc: 'waist',
    cup_desc: 'cup',
  })
}

export function findSortOption(options, sort) {
  const normalized = String(sort || '')
    .trim()
    .toLowerCase()
  return options.find(
    (option) =>
      option.defaultValue === normalized ||
      option.ascValue === normalized ||
      option.descValue === normalized
  )
}

export function getSortDirection(option, sort) {
  if (!option) return 'asc'
  return String(sort || '')
    .trim()
    .toLowerCase() === option.ascValue
    ? 'asc'
    : 'desc'
}

export function reverseSortValue(options, sort, fallback) {
  const option =
    findSortOption(options, sort) || options.find((item) => item.defaultValue === fallback)
  if (!option) return fallback
  return getSortDirection(option, sort) === 'asc' ? option.descValue : option.ascValue
}

export function sortLabel(option, sort, zh) {
  if (!option) return ''
  const dir = getSortDirection(option, sort)
  return zh(`${option.label[0]}：${option[dir][0]}`, `${option.label[1]}: ${option[dir][1]}`)
}

export function sortLabelParts(option, sort, zh) {
  if (!option) return { label: '', separator: '', direction: '' }
  const dir = getSortDirection(option, sort)
  return {
    label: zh(option.label[0], option.label[1]),
    separator: zh('：', ': '),
    direction: zh(option[dir][0], option[dir][1]),
  }
}

export function isUserJavTag(tag) {
  return Number(tag?.provider) === JAV_PROVIDER_USER
}
