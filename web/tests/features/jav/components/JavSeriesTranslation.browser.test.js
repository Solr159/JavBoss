import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'series automatic translation has its own switch and preserves existing names',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javSeriesTranslation.html?automatic`,
    })
    await waitFor(`document.body.textContent.includes('自动系列译名')`)
    assert.deepEqual(await evaluate('window.translationRequests'), [[1]])
    assert.equal(await evaluate('window.testStore.getState().seriesItems[0].name'), '雨の日')
    assert.equal(await evaluate('window.testStore.getState().seriesItems[1].zh_name'), '人工译名')
    await evaluate(
      `window.testStore.setState({config:{...window.testStore.getState().config,jav_title_prefer_chinese:'false'}})`
    )
    await waitFor(
      `!document.body.textContent.includes('自动系列译名') && document.body.textContent.includes('雨の日')`
    )
  }
)

test(
  'series translation preview supports cancellation, manual correction and error recovery',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javSeriesTranslation.html` })
    await waitFor(`document.querySelector('[aria-label="Edit series translation"]')`)
    assert.deepEqual(await evaluate('window.translationRequests'), [])
    const clickButton = async (text) =>
      evaluate(
        `[...document.querySelectorAll('button')].find(button => button.textContent === ${JSON.stringify(text)}).click()`
      )
    await evaluate(`document.querySelector('[aria-label="Edit series translation"]').click()`)
    await waitFor(`document.querySelector('textarea')`)
    await clickButton('Quick retranslation')
    await waitFor(`document.querySelector('textarea').value === 'AI 系列译名'`)
    assert.deepEqual(await evaluate('window.previewPayload'), { name: '雨の日', thinking: false })
    assert.deepEqual(await evaluate('window.savedTranslations'), [])
    await clickButton('Cancel')
    await waitFor(`!document.querySelector('textarea')`)
    assert.equal(await evaluate('window.testStore.getState().seriesItems[0].zh_name'), '')
    await evaluate(`document.querySelector('[aria-label="Edit series translation"]').click()`)
    await waitFor(`document.querySelector('textarea')`)
    await evaluate(`{
      const input = document.querySelector('textarea');
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set.call(input, '人工系列译名');
      input.dispatchEvent(new Event('input', {bubbles:true}));
      window.translationFailure = true;
    }`)
    await clickButton('Quick retranslation')
    await waitFor(`document.querySelector('[role="alert"]')`)
    assert.equal(await evaluate('document.querySelector("textarea").value'), '人工系列译名')
    await clickButton('Save')
    await waitFor(
      `!document.querySelector('textarea') && document.body.textContent.includes('人工系列译名')`
    )
    assert.deepEqual(await evaluate('window.savedTranslations'), [
      { name: '雨の日', zh_name: '人工系列译名' },
    ])
  }
)
