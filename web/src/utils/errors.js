import { zh } from '@/utils/i18n'

export function fallbackErrorMessage() {
  return zh('操作失败，请稍后重试', 'Something went wrong. Please try again.')
}

export function getErrorMessage(error) {
  const message = error && typeof error === 'object' && 'message' in error ? error.message : error
  return String(message || '').trim() || fallbackErrorMessage()
}
