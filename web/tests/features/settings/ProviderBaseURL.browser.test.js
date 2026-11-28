import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

const EDIT_BUTTON = 'button[aria-label="Edit JavBus URL"]'
const DEFAULT_URL = 'https://www.javbus.com'

async function openProvidersPanel(t) {
  const browser = await openBrowser(t, {
    cacheDir: 'node_modules/.vite-provider-base-url-test',
  })
  await browser.command('Page.navigate', {
    url: `${browser.origin}/tests/fixtures/app.html?view=video`,
  })
  await browser.waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
  await browser.evaluate(`document.querySelector('aside button[aria-label="Settings"]').click()`)
  const dialog = `document.querySelector('[role="dialog"]')`
  await browser.waitFor(dialog)
  await browser.evaluate(
    `[...${dialog}.querySelectorAll('aside button')].find(button => button.textContent.includes('JAV Providers')).click()`
  )
  await browser.waitFor(`${dialog}.querySelector('${EDIT_BUTTON}')`)
  const card = `${dialog}.querySelector('button[aria-label="Check JavBus availability"]').closest('li')`
  return { ...browser, dialog, card }
}

test(
  'the provider address dialog pre-fills the active URL and supports saving, cancellation and reset',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const browser = await openProvidersPanel(t)
    const { dialog, card } = browser
    const editor = `document.querySelector('form[role="dialog"]')`
    const input = `${editor}.querySelector('input')`
    const alert = `${editor}.querySelector('[role="alert"]')`
    const openEditor = async () => {
      await browser.evaluate(`${card}.querySelector('${EDIT_BUTTON}').click()`)
      await browser.waitFor(editor)
    }
    const clickAction = (label) =>
      browser.evaluate(
        `[...${editor}.querySelectorAll('button')].find(button => button.textContent === '${label}').click()`
      )
    const typeAddress = (value) =>
      browser.evaluate(
        `Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(${input}, ${JSON.stringify(value)}); ${input}.dispatchEvent(new Event('input', { bubbles: true }))`
      )
    const typeAndSave = async (value) => {
      await typeAddress(value)
      await clickAction('Save')
    }

    // Only providers with a configurable origin expose an editor.
    assert.equal(
      await browser.evaluate(`${dialog}.querySelectorAll('li button[aria-label^="Edit "]').length`),
      1
    )
    assert.equal(await browser.evaluate(`Boolean(${editor})`), false)
    assert.equal(await browser.evaluate(`Boolean(${card}.querySelector('input'))`), false)

    await openEditor()
    assert.equal(
      await browser.evaluate(`${editor}.querySelector('h2').textContent`),
      'Edit JavBus URL'
    )
    assert.equal(await browser.evaluate(`${input}.value`), DEFAULT_URL)
    assert.equal(await browser.evaluate(`document.activeElement === ${input}`), true)

    await typeAddress('https://discarded.javbus.test')
    await clickAction('Cancel')
    await browser.waitFor(`!${editor}`)

    await openEditor()
    assert.equal(await browser.evaluate(`${input}.value`), DEFAULT_URL)
    await clickAction('Save')
    await browser.waitFor(`!${editor}`)
    assert.equal(
      await browser.evaluate(`window.requests.some(request => request.method === 'PUT')`),
      false
    )

    await openEditor()
    await typeAndSave('https://Mirror.JavBus.Test/')
    await browser.waitFor(`!${editor}`)
    await browser.waitFor(`${card}.textContent.includes('Custom')`)

    assert.deepEqual(
      await browser.evaluate(`window.requests.filter(request => request.method === 'PUT').at(-1)`),
      { url: '/jav/providers/1/base_url', method: 'PUT' }
    )
    assert.match(await browser.evaluate(`${card}.textContent`), /mirror\.javbus\.test/)

    // Origins with a path are rejected by the server and surfaced inline.
    await openEditor()
    assert.equal(await browser.evaluate(`${input}.value`), 'https://mirror.javbus.test')
    await typeAndSave('https://mirror.javbus.test/genre')
    await browser.waitFor(alert)
    assert.match(await browser.evaluate(`${alert}.textContent`), /Invalid provider URL/)
    assert.equal(await browser.evaluate(`${input}.value`), 'https://mirror.javbus.test/genre')

    // Restore defaults edits the draft. Cancel keeps the saved custom URL.
    await clickAction('Restore defaults')
    assert.equal(await browser.evaluate(`${input}.value`), DEFAULT_URL)
    assert.equal(await browser.evaluate(`Boolean(${alert})`), false)
    await clickAction('Cancel')
    await browser.waitFor(`!${editor}`)
    assert.match(await browser.evaluate(`${card}.textContent`), /mirror\.javbus\.test/)

    await openEditor()
    assert.equal(await browser.evaluate(`${input}.value`), 'https://mirror.javbus.test')
    await clickAction('Restore defaults')
    await clickAction('Save')
    await browser.waitFor(`!${editor}`)
    await browser.waitFor(`!${card}.textContent.includes('Custom')`)
    assert.match(await browser.evaluate(`${card}.textContent`), /www\.javbus\.com/)
    assert.deepEqual(await browser.evaluate('window.appErrors'), [])
  }
)
