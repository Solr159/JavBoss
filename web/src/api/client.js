import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export const jsonHeaders = { 'Content-Type': 'application/json' }

export const authExpiredEvent = 'javboss:auth-expired'

export async function apiError(res) {
  const payload = await res.json().catch(() => ({}))
  return new Error(
    getErrorMessage(zh(String(payload.error_zh || ''), String(payload.error_en || '')))
  )
}

export async function apiFetch(input, init = {}) {
  const res = await fetch(input, init)
  if (res.status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new Event(authExpiredEvent))
  }
  return res
}

export async function parseJSONResponse(res) {
  const contentType = String(res.headers.get('content-type') || '').toLowerCase()
  if (!contentType.includes('json')) {
    throw new Error(
      zh(
        '服务端接口返回了网页，请确认后端已更新并重启',
        'The server returned a web page. Make sure the backend is updated and restarted.'
      )
    )
  }
  return res.json()
}
