import { parseTranslationConfig, titleLanguage } from '@/features/jav/titleTranslation'
import { zh } from '@/utils/i18n'

export const defaultRequest = {
  url: '{{base_url}}/chat/completions',
  method: 'POST',
  headers: { Authorization: 'Bearer {{api_key}}', 'Content-Type': 'application/json' },
  body: {
    model: '{{model}}',
    messages: [
      { role: 'system', content: '{{prompt}}' },
      { role: 'user', content: '{{title}}' },
    ],
    stream: false,
  },
  response_path: '/choices/0/message/content',
}

function profileDraft(profile = {}, provider = {}) {
  return {
    apiKey: '',
    hasApiKey: Boolean(profile.api_key_set),
    clearApiKey: false,
    baseUrl: profile.base_url || provider.base_url || '',
    protocol: profile.protocol || provider.protocol || 'chat',
    model: profile.model || '',
    customAddress: Boolean(
      profile.base_url &&
      profile.base_url.replace(/\/$/, '') !== provider.base_url?.replace(/\/$/, '')
    ),
    temperature: profile.temperature ?? '',
    thinking: Boolean(profile.thinking),
    effort: profile.effort || '',
    reasoning: profile.reasoning || 'none',
    request: JSON.stringify(profile.request || defaultRequest, null, 2),
    fastParams: JSON.stringify(profile.fast_params || {}, null, 2),
    thinkingParams: JSON.stringify(profile.thinking_params || {}, null, 2),
  }
}

export function createTitleTranslationDraft(config) {
  const suppliedCatalog = parseTranslationConfig(config?.jav_title_translation_catalog, [])
  const catalog = Array.isArray(suppliedCatalog) ? suppliedCatalog : []
  const provider = config?.jav_title_translation_provider || 'deepseek'
  const stored = parseTranslationConfig(config?.jav_title_translation_profiles)
  const profiles = Object.fromEntries(
    Object.entries(stored).map(([id, p]) => [
      id,
      profileDraft(
        p,
        catalog.find((item) => item.id === id)
      ),
    ])
  )
  return {
    jav_title_language: titleLanguage(config),
    defaultPrompt: config?.jav_title_translation_default_prompt || '',
    defaultPromptEn: config?.jav_title_translation_default_prompt_en || '',
    prompt: config?.jav_title_translation_prompt || '',
    promptEn: config?.jav_title_translation_prompt_en || '',
    catalog,
    provider,
    profiles,
    ...(profiles[provider] ||
      profileDraft(
        {},
        catalog.find((item) => item.id === provider)
      )),
  }
}

// Keep unfinished edits when switching providers; never reuse another provider's key.
export function changeTranslationDraft(current, patch) {
  if (!patch.provider || patch.provider === current.provider) return { ...current, ...patch }
  const {
    profiles,
    catalog,
    defaultPrompt,
    defaultPromptEn,
    prompt,
    promptEn,
    provider,
    ...profile
  } = current
  const nextProfiles = { ...profiles, [provider]: profile }
  return {
    profiles: nextProfiles,
    catalog,
    defaultPrompt,
    defaultPromptEn,
    prompt,
    promptEn,
    jav_title_language: current.jav_title_language,
    provider: patch.provider,
    ...(nextProfiles[patch.provider] ||
      profileDraft(
        {},
        catalog.find((item) => item.id === patch.provider)
      )),
  }
}

function parseParams(value) {
  const result = JSON.parse(value || '{}')
  if (!result || typeof result !== 'object' || Array.isArray(result)) {
    throw new Error(zh('自定义参数必须为 JSON 对象', 'Custom parameters must be JSON objects'))
  }
  return result
}

export function translationProfile(draft) {
  const profile = {
    api_key: draft.apiKey.trim() || (draft.clearApiKey ? '' : undefined),
    base_url: draft.baseUrl.trim(),
    protocol: draft.protocol,
    model: draft.model.trim(),
    temperature: draft.temperature === '' ? null : Number(draft.temperature),
    thinking: Boolean(draft.thinking),
    effort: draft.effort,
  }
  if (draft.provider === 'custom')
    Object.assign(profile, {
      reasoning: draft.reasoning,
      request: parseParams(draft.request),
      fast_params: parseParams(draft.fastParams),
      thinking_params: parseParams(draft.thinkingParams),
    })
  return profile
}

export function translationProfiles(draft) {
  const profiles = { ...draft.profiles, [draft.provider]: draft }
  return Object.fromEntries(
    Object.entries(profiles).map(([provider, profile]) => [
      provider,
      translationProfile({ ...profile, provider }),
    ])
  )
}
