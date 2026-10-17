import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'automatic translation uses only the selected profile and allows custom services without a key',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javAutomaticTranslation.html?unconfigured`,
    })
    await waitFor('window.testStore && document.querySelector("output")')
    await evaluate(`{
      window.updateTranslationConfig = patch => window.testStore.setState({
        config: { ...window.testStore.getState().config, ...patch }
      });
      window.testStore.setState({ javItems: [{ id: 1, title: 'Source 1' }] });
    }`)
    await evaluate('new Promise(resolve => setTimeout(resolve, 100))')
    assert.equal(await evaluate('window.translationRequests.length'), 0)
    await evaluate(`window.updateTranslationConfig({
      jav_title_translation_profiles: JSON.stringify({
        deepseek: { model: 'no-key' },
        kimi: { model: 'configured-model', api_key_set: true }
      })
    })`)
    await evaluate('new Promise(resolve => setTimeout(resolve, 100))')
    assert.equal(await evaluate('window.translationRequests.length'), 0)
    await evaluate(`window.updateTranslationConfig({ jav_title_translation_provider: 'kimi' })`)
    await waitFor('window.translationRequests.length === 1')
    await evaluate('window.completeTranslation(0)')
    await waitFor(`document.querySelector('output').textContent === '1'`)
    await evaluate(`{
      window.updateTranslationConfig({
        jav_title_translation_provider: 'custom',
        jav_title_translation_profiles: JSON.stringify({ custom: { model: 'local-model' } })
      });
      window.testStore.setState({ javItems: [{ id: 2, title: 'Source 2' }] });
    }`)
    await waitFor('window.translationRequests.length === 2')
    assert.deepEqual(await evaluate('window.translationRequests[1].ids'), [2])
    await evaluate('window.completeTranslation(1)')
    await waitFor(`document.querySelector('output').textContent === '1'`)
  }
)

test(
  'streamed batches share requests across rerenders and remounts and display completed titles immediately',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javAutomaticTranslation.html` })
    await waitFor('window.translationRequests?.length === 1')
    assert.deepEqual(
      await evaluate('window.translationRequests[0].ids'),
      Array.from({ length: 20 }, (_, i) => i + 1)
    )
    await evaluate(
      `window.testStore.setState({javItems: [...window.testStore.getState().javItems]})`
    )
    await evaluate('window.emitTranslation(0, [3])')
    await waitFor(`document.querySelector('output').textContent === '2'`)
    assert.equal(await evaluate('window.testStore.getState().javItems[2].zh_title'), '译文 3')
    assert.equal(await evaluate('window.testStore.getState().javItems[0].zh_title'), undefined)
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Remount').click()`
    )
    await evaluate('window.emitTranslation(0, [1])')
    await waitFor(`document.querySelector('output').textContent === '3'`)
    assert.equal(await evaluate('window.translationRequests.length'), 1)
    await evaluate('window.completeTranslation(0)')
    await waitFor('window.translationRequests.length === 2')
    assert.deepEqual(await evaluate('window.translationRequests[1].ids'), [21, 22, 23, 24, 25])
    await evaluate('window.completeTranslation(1)')
    await waitFor(`document.querySelector('output').textContent === '26'`)
    assert.equal(await evaluate('window.translationRequests.length'), 2)
    assert.equal(
      await evaluate('window.testStore.getState().javItems.find(i => i.id === 26).zh_title'),
      '人工校对'
    )
  }
)

test(
  'interrupted translation streams retain completed titles and stop queued groups',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javAutomaticTranslation.html` })
    await waitFor('window.translationRequests?.length === 1')
    await evaluate('window.emitTranslation(0, [2])')
    await waitFor(`document.querySelector('output').textContent === '2'`)
    await evaluate('window.translationRequests[0].controller.close()')
    await waitFor(
      `document.querySelector('[role="alert"]')?.textContent.includes('connection interrupted')`
    )
    assert.equal(await evaluate('window.translationRequests.length'), 1)
    assert.equal(await evaluate('window.testStore.getState().javItems[1].zh_title'), '译文 2')
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Retry').click()`
    )
    await waitFor('window.translationRequests.length === 2')
    assert.equal(await evaluate('window.translationRequests[1].ids.includes(2)'), false)
  }
)

test(
  'partial batch failures retain success, stop subsequent requests and retry only missing titles',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javAutomaticTranslation.html` })
    await waitFor('window.translationRequests?.length === 1')
    await evaluate('window.completeTranslation(0, true)')
    await waitFor(
      `document.querySelector('[role="alert"]')?.textContent.includes('credit_balance_exhausted')`
    )
    assert.equal(await evaluate(`document.querySelector('output').textContent`), '2')
    assert.equal(await evaluate('window.translationRequests.length'), 1)
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Retry').click()`
    )
    await waitFor('window.translationRequests.length === 2')
    assert.deepEqual(
      await evaluate('window.translationRequests[1].ids'),
      Array.from({ length: 20 }, (_, i) => i + 2)
    )
    await evaluate('window.completeTranslation(1)')
    await waitFor('window.translationRequests.length === 3')
    assert.deepEqual(await evaluate('window.translationRequests[2].ids'), [22, 23, 24, 25])
    await evaluate('window.completeTranslation(2)')
    await waitFor(`document.querySelector('output').textContent === '26'`)
  }
)

test(
  'leaving a page prevents its remaining batches and ignores stale responses',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javAutomaticTranslation.html` })
    await waitFor('window.translationRequests?.length === 1')
    await evaluate(`window.testStore.setState({javItems: [{id: 101, title: 'New page'}]})`)
    await waitFor('window.translationRequests.length === 2')
    assert.deepEqual(await evaluate('window.translationRequests[1].ids'), [101])
    await evaluate('window.completeTranslation(0)')
    await evaluate('new Promise(resolve => setTimeout(resolve, 100))')
    assert.equal(await evaluate('window.translationRequests.length'), 2)
    assert.equal(await evaluate(`document.querySelector('output').textContent`), '0')
    await evaluate('window.completeTranslation(1)')
    await waitFor(`document.querySelector('output').textContent === '1'`)
  }
)
