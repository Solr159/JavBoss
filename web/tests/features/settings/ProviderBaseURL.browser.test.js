import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

const INPUT = 'input[aria-label="Custom JavBus URL"]'

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
  await browser.waitFor(`${dialog}.querySelector('${INPUT}')`)
  return { ...browser, dialog, card: `${dialog}.querySelector('${INPUT}').closest('li')` }
}

test(
  'a custom provider origin can be saved, rejected and reset from the source panel',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const browser = await openProvidersPanel(t)
    const { dialog, card } = browser
    const input = `${dialog}.querySelector('${INPUT}')`
    const clickButton = (label) =>
      browser.evaluate(
        `[...${card}.querySelectorAll('button')].find(button => button.textContent === '${label}').click()`
      )
    const typeAndSave = async (value) => {
      await browser.evaluate(
        `Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(${input}, ${JSON.stringify(value)}); ${input}.dispatchEvent(new Event('input', { bubbles: true }))`
      )
      await browser.evaluate(
        `${input}.closest('form').querySelector('button[type="submit"]').click()`
      )
    }

    // Only providers with a configurable origin expose an editor.
    assert.equal(
      await browser.evaluate(`${dialog}.querySelectorAll('input[aria-label^="Custom "]').length`),
      1
    )
    assert.equal(await browser.evaluate(`${input}.value`), '')
    assert.equal(await browser.evaluate(`${input}.placeholder`), 'https://www.javbus.com')

    await typeAndSave('https://Mirror.JavBus.Test/')
    await browser.waitFor(`${card}.textContent.includes('Custom')`)

    assert.deepEqual(
      await browser.evaluate(`window.requests.filter(request => request.method === 'PUT').at(-1)`),
      { url: '/jav/providers/1/base_url', method: 'PUT' }
    )
    assert.equal(await browser.evaluate(`${input}.value`), 'https://mirror.javbus.test')
    assert.match(await browser.evaluate(`${card}.textContent`), /mirror\.javbus\.test/)

    // Origins with a path are rejected by the server and surfaced inline.
    await typeAndSave('https://mirror.javbus.test/genre')
    await browser.waitFor(`${card}.querySelector('[role="alert"]')`)
    assert.match(
      await browser.evaluate(`${card}.querySelector('[role="alert"]').textContent`),
      /Invalid provider URL/
    )
    assert.equal(await browser.evaluate(`${input}.value`), 'https://mirror.javbus.test/genre')

    // Resetting restores the default origin and hides the custom badge.
    await clickButton('Default')
    await browser.waitFor(`!${card}.textContent.includes('Custom')`)
    assert.equal(await browser.evaluate(`${input}.value`), '')
    assert.match(await browser.evaluate(`${card}.textContent`), /www\.javbus\.com/)
    assert.deepEqual(await browser.evaluate('window.appErrors'), [])
  }
)
