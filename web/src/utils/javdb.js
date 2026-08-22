const ASSIST_TARGETS = new Set(['movie', 'idol', 'series', 'studio'])
const EXTENSION_ID = 'iikdjhkpjihfkehccfmkpkdmenmbaacn'
const EXTENSION_ORIGIN = `chrome-extension://${EXTENSION_ID}`
const EXTENSION_BRIDGE_URL = `${EXTENSION_ORIGIN}/bridge.html`
const MESSAGE_CONNECT = 'JAVBOSS_EXTENSION_CONNECT'
const MESSAGE_READY = 'JAVBOSS_EXTENSION_READY'
const BRIDGE_VERSION = 1

let bridgeElement = null
let bridgeReady = false

function newBridgeSessionId() {
  if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID()
  return `javboss-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

const bridgeSessionId = newBridgeSessionId()

function postBridgeConnect() {
  bridgeElement?.contentWindow?.postMessage(
    { type: MESSAGE_CONNECT, sessionId: bridgeSessionId },
    EXTENSION_ORIGIN
  )
}

function mountBridge() {
  if (bridgeElement || typeof document === 'undefined' || !document.body) return
  const iframe = document.createElement('iframe')
  iframe.src = EXTENSION_BRIDGE_URL
  iframe.title = 'JavBoss extension availability bridge'
  iframe.tabIndex = -1
  iframe.setAttribute('aria-hidden', 'true')
  iframe.style.display = 'none'
  iframe.addEventListener('load', () => {
    bridgeReady = false
    postBridgeConnect()
  })
  bridgeElement = iframe
  document.body.appendChild(iframe)
  const connectDelays = [0, 300, 1000]
  connectDelays.forEach((delay) => window.setTimeout(postBridgeConnect, delay))
}

function initializeBridge() {
  if (typeof window === 'undefined' || typeof document === 'undefined') return
  window.addEventListener('message', (event) => {
    if (event.origin !== EXTENSION_ORIGIN || event.source !== bridgeElement?.contentWindow) return
    const message = event.data
    if (
      message?.version === BRIDGE_VERSION &&
      message.type === MESSAGE_READY &&
      message.sessionId === bridgeSessionId
    ) {
      bridgeReady = true
    }
  })

  if (document.body) mountBridge()
  else document.addEventListener('DOMContentLoaded', mountBridge, { once: true })
}

initializeBridge()

export function isJavBossExtensionReady() {
  return bridgeReady
}

export function buildAssistedJavDBURL({ target, code, name = '' } = {}) {
  const cleanTarget = String(target || '').trim()
  const cleanCode = String(code || '').trim()
  const cleanName = String(name || '').trim()
  if (!ASSIST_TARGETS.has(cleanTarget) || !cleanCode) return ''

  const url = new URL('/search', 'https://javdb.com')
  url.searchParams.set('q', cleanCode)
  url.searchParams.set('f', 'all')

  const marker = new URLSearchParams()
  marker.set('javboss', 'direct')
  marker.set('target', cleanTarget)
  marker.set('code', cleanCode)
  if (cleanName) marker.set('name', cleanName)
  url.hash = marker.toString()
  return url.href
}

export function resolveJavDBOpenURL(fallbackURL, options, extensionReady) {
  const fallback = String(fallbackURL || '').trim()
  if (!fallback || !extensionReady) return fallback
  return buildAssistedJavDBURL(options) || fallback
}

export function openJavDBWithAssist(fallbackURL, options) {
  const url = resolveJavDBOpenURL(fallbackURL, options, isJavBossExtensionReady())
  if (!url) return false
  window.open(url, '_blank', 'noopener,noreferrer')
  return true
}
