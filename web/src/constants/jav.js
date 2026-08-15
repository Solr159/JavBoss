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
  {
    base: 'favorite_rating',
    defaultValue: 'favorite_rating',
    ascValue: 'favorite_rating_asc',
    descValue: 'favorite_rating',
    label: ['喜爱度', 'Favorite rating'],
    asc: ['低→高', 'low→high'],
    desc: ['高→低', 'high→low'],
  },
]

export const JAV_SORT_RULE_FILTERS = [
  { key: 'search', label: ['搜索', 'Search'] },
  { key: 'idol', label: ['女优', 'Idol'] },
  { key: 'tag', label: ['标签', 'Tag'] },
  { key: 'studio', label: ['片商', 'Studio'] },
  { key: 'series', label: ['系列', 'Series'] },
  { key: 'prefix', label: ['番号', 'Code'] },
  { key: 'solo', label: ['单体作品', 'Solo'] },
  { key: 'favorite_rating', label: ['喜爱度', 'Favorite rating'] },
  { key: 'favorite_group', label: ['收藏夹', 'Favorite group'] },
]

export const JAV_SORT_RULE_VERSION = 1
export const JAV_SORT_RULE_LIMIT = 50

const javSortRuleFilterKeys = new Set(JAV_SORT_RULE_FILTERS.map((item) => item.key))

export const IDOL_SORT_OPTIONS = [
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
    base: 'recent',
    defaultValue: 'recent',
    ascValue: 'recent_asc',
    descValue: 'recent',
    label: ['加入时间', 'Added time'],
    asc: ['远→近', 'old→new'],
    desc: ['近→远', 'new→old'],
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

export const IDOL_FAVORITE_ORDER_SORT = 'favorite_order'

export const IDOL_PROFILE_FILTER_DEFINITIONS = [
  { key: 'height', label: ['身高', 'Height'], min: 130, max: 190, step: 1, unit: 'cm' },
  { key: 'age', label: ['年龄', 'Age'], min: 18, max: 60, step: 1, unit: ['岁', 'y'] },
  { key: 'cup', label: ['罩杯', 'Cup'], min: 1, max: 11, step: 1, format: 'cup' },
  { key: 'bust', label: ['胸围', 'Bust'], min: 60, max: 130, step: 1, unit: 'cm' },
  { key: 'waist', label: ['腰围', 'Waist'], min: 45, max: 100, step: 1, unit: 'cm' },
  { key: 'hips', label: ['臀围', 'Hips'], min: 65, max: 130, step: 1, unit: 'cm' },
]

export function createDefaultIdolProfileFilters() {
  return Object.fromEntries(
    IDOL_PROFILE_FILTER_DEFINITIONS.map((definition) => [
      definition.key,
      { enabled: false, min: definition.min, max: definition.max },
    ])
  )
}

export function normalizeIdolProfileFilters(value) {
  const source = value && typeof value === 'object' ? value : {}
  return Object.fromEntries(
    IDOL_PROFILE_FILTER_DEFINITIONS.map((definition) => {
      const candidate = source[definition.key] || {}
      const rawMin = Number(candidate.min)
      const rawMax = Number(candidate.max)
      const min = Math.max(
        definition.min,
        Math.min(definition.max, Number.isFinite(rawMin) ? Math.round(rawMin) : definition.min)
      )
      const max = Math.max(
        min,
        Math.min(definition.max, Number.isFinite(rawMax) ? Math.round(rawMax) : definition.max)
      )
      return [definition.key, { enabled: Boolean(candidate.enabled), min, max }]
    })
  )
}

export function formatIdolProfileFilterRange(definition, value, translate = (cn) => cn) {
  const normalized = normalizeIdolProfileFilters({ [definition.key]: value })[definition.key]
  const formatValue = (number) => {
    if (definition.format === 'cup') {
      return String.fromCharCode(64 + number)
    }
    const unit = Array.isArray(definition.unit)
      ? translate(definition.unit[0], definition.unit[1])
      : definition.unit || ''
    return `${number}${unit}`
  }
  return `${formatValue(normalized.min)}–${formatValue(normalized.max)}`
}

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
    favorite_rating_desc: 'favorite_rating',
  })
}

export function normalizeJavSortRules(value) {
  let parsed = value
  if (typeof value === 'string') {
    try {
      parsed = JSON.parse(value)
    } catch {
      return []
    }
  }
  const source = Array.isArray(parsed) ? parsed : parsed?.rules
  if (!Array.isArray(source)) return []

  const seenIds = new Set()
  const rules = []
  for (let index = 0; index < source.length && rules.length < JAV_SORT_RULE_LIMIT; index += 1) {
    const item = source[index]
    if (!item || typeof item !== 'object') continue
    const id = String(item.id || `rule-${index + 1}`).trim()
    const sort = normalizeJavSort(item.sort, '')
    const active = Array.from(
      new Set(
        (Array.isArray(item.active) ? item.active : [])
          .map((key) =>
            String(key || '')
              .trim()
              .toLowerCase()
          )
          .filter((key) => javSortRuleFilterKeys.has(key))
      )
    ).sort(
      (a, b) =>
        JAV_SORT_RULE_FILTERS.findIndex((item) => item.key === a) -
        JAV_SORT_RULE_FILTERS.findIndex((item) => item.key === b)
    )
    const mode = item.mode === 'any' ? 'any' : 'all'
    if (!id || seenIds.has(id) || !sort) continue
    seenIds.add(id)
    rules.push({ id, enabled: item.enabled !== false, mode, active, sort })
  }
  return rules
}

export function javSortRulesConfig(rules) {
  return {
    version: JAV_SORT_RULE_VERSION,
    rules: normalizeJavSortRules(rules),
  }
}

export function activeJavSortFilters(state) {
  const active = []
  if (String(state?.javSearchTerm || '').trim()) active.push('search')
  if ((state?.javIdolIds || []).length > 0) active.push('idol')
  if ((state?.javTags || []).length > 0) active.push('tag')
  if (state?.javStudioId !== null && state?.javStudioId !== undefined) active.push('studio')
  if (state?.javSeriesId !== null && state?.javSeriesId !== undefined && state.javSeriesId !== '') {
    active.push('series')
  }
  if (String(state?.javPrefix || '').trim()) active.push('prefix')
  if (state?.javSoloOnly) active.push('solo')
  if (state?.javFavoriteRatingEnabled) active.push('favorite_rating')
  if (Number(state?.javFavoriteGroupId) > 0) active.push('favorite_group')
  return active
}

export function resolveJavSort(state) {
  if (state?.javRandomMode) {
    return { sort: 'random', source: 'random', rule: null }
  }
  const temporary = normalizeJavSort(state?.javTempSort, '')
  if (temporary) {
    return { sort: temporary, source: 'temporary', rule: null }
  }

  const active = activeJavSortFilters(state)
  const activeSet = new Set(active)
  const rules = normalizeJavSortRules(state?.javSortRules)
  for (const rule of rules) {
    if (!rule.enabled) continue
    const matches =
      rule.active.length > 0 &&
      (rule.mode === 'any'
        ? rule.active.some((key) => activeSet.has(key))
        : rule.active.every((key) => activeSet.has(key)))
    if (matches) {
      return { sort: rule.sort, source: 'rule', rule }
    }
  }

  return { sort: normalizeJavSort(state?.javSort), source: 'default', rule: null }
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
