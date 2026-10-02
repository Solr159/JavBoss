import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'prefix initials follow the censor filter and clear selections that become unavailable',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="JAV codes"]')`)
    await evaluate(`{
      const originalFetch = window.fetch;
      window.fetch = (input, init) => new URL(input, location.origin).pathname === '/jav/prefixes'
        ? Promise.resolve(Response.json([
            {prefix: 'ABC', is_uncensored: false, work_count: 1},
            {prefix: 'BBB', is_uncensored: true, work_count: 1},
            {prefix: 'CCC', is_uncensored: false, work_count: 1},
            {prefix: 'CCC', is_uncensored: true, work_count: 1},
            {prefix: '1pondo', is_uncensored: true, work_count: 1},
            {prefix: 'NNN', is_uncensored: null, work_count: 1}
          ]))
        : originalFetch(input, init);
      document.querySelector('aside button[aria-label="JAV codes"]').click();
    }`)

    const modal = `document.querySelector('[aria-labelledby="jav-prefix-modal-title"]')`
    const initials = `${modal}.querySelector('[aria-label="Filter by first code character"]')`
    const initial = (letter) =>
      `${initials}.querySelector('button[aria-label="Show codes starting with ${letter}"]')`
    const enabled = `Array.from(${initials}.querySelectorAll('button:not(:disabled)')).map(b => b.textContent).join('')`
    const selected = `${initials}.querySelector('button[aria-pressed="true"]')`
    const switchType = async (label, expected) => {
      await evaluate(`Array.from(${modal}.querySelectorAll('button'))
        .find(button => button.textContent === '${label}').click()`)
      await waitFor(`${enabled} === '${expected}'`)
    }

    await waitFor(`${modal} && ${enabled} === '1ABCN'`)
    await switchType('Censored', 'AC')
    assert.equal(await evaluate(`${initial('B')}.disabled`), true)
    assert.equal(await evaluate(`${initial('B')}.classList.contains('text-gray-300')`), true)
    await evaluate(`${initial('A')}.click()`)
    await waitFor(`${initial('A')}.getAttribute('aria-pressed') === 'true'`)

    await switchType('Uncensored', '1BC')
    await waitFor(`!${selected}`)
    assert.equal(await evaluate(`${initial('A')}.disabled`), true)
    assert.equal(await evaluate(`${initial('A')}.classList.contains('text-gray-300')`), true)
    assert.ok(await evaluate(`${modal}.querySelector('tbody').textContent.includes('BBB')`))
    assert.equal(
      await evaluate(`${modal}.querySelector('tbody').textContent.includes('ABC')`),
      false
    )

    await evaluate(`${initial('C')}.click()`)
    await waitFor(`${initial('C')}.getAttribute('aria-pressed') === 'true'`)
    await switchType('Censored', 'AC')
    assert.equal(await evaluate(`${initial('C')}.getAttribute('aria-pressed')`), 'true')
    await switchType('All', '1ABCN')
    assert.equal(await evaluate(`${initial('N')}.disabled`), false)
    assert.equal(await evaluate(`${initial('Z')}.disabled`), true)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
