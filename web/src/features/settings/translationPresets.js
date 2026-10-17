import { defaultRequest } from '@/features/settings/titleTranslation'

export const translationPresets = {
  chat: { ...defaultRequest, body: { ...defaultRequest.body, max_tokens: 1024 } },
  responses: {
    url: '{{base_url}}/responses',
    method: 'POST',
    headers: defaultRequest.headers,
    body: {
      model: '{{model}}',
      instructions: '{{prompt}}',
      input: '{{title}}',
      max_output_tokens: 1024,
      store: false,
    },
    response_path: '/output/0/content/0/text',
  },
  anthropic: {
    url: '{{base_url}}/messages',
    method: 'POST',
    headers: {
      'x-api-key': '{{api_key}}',
      'anthropic-version': '2023-06-01',
      'Content-Type': 'application/json',
    },
    body: {
      model: '{{model}}',
      system: '{{prompt}}',
      messages: [{ role: 'user', content: '{{title}}' }],
      max_tokens: 1024,
    },
    response_path: '/content/0/text',
  },
  gemini: {
    url: '{{base_url}}/models/{{model}}:generateContent',
    method: 'POST',
    headers: { 'x-goog-api-key': '{{api_key}}', 'Content-Type': 'application/json' },
    body: {
      systemInstruction: { parts: [{ text: '{{prompt}}' }] },
      contents: [{ role: 'user', parts: [{ text: '{{title}}' }] }],
      generationConfig: { maxOutputTokens: 1024 },
    },
    response_path: '/candidates/0/content/parts/0/text',
  },
}

export const protocolNames = {
  chat: 'OpenAI Chat Completions',
  responses: 'OpenAI Responses',
  anthropic: 'Anthropic Messages',
  gemini: 'Gemini GenerateContent',
}
