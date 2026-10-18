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
      `window.testStore.setState({config:{...window.testStore.getState().config,jav_series_translation_enabled:'false'}})`
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

const editorButton = 'button[aria-label="Edit series translation"]'
async function press(command, key, code, keyCode) {
  for (const type of ['keyDown', 'keyUp'])
    await command('Input.dispatchKeyEvent', {
      type,
      key,
      code,
      windowsVirtualKeyCode: keyCode,
      ...(type === 'keyDown' && key === 'Enter' ? { text: '\r' } : {}),
    })
}

test(
  'series edit buttons retain native Space and Enter activation without navigating the card',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javSeriesTranslation.html` })
    await waitFor(`document.querySelector('${editorButton}')`)
    for (const [key, code, keyCode] of [
      [' ', 'Space', 32],
      ['Enter', 'Enter', 13],
    ]) {
      await evaluate(`document.querySelector('${editorButton}').focus()`)
      await press(command, key, code, keyCode)
      await waitFor(`document.querySelector('textarea')`)
      assert.equal(await evaluate('window.seriesNavigations'), 0)
      await press(command, 'Escape', 'Escape', 27)
      await waitFor(`!document.querySelector('textarea')`)
    }
    await evaluate(`document.querySelector('${editorButton}').closest('a').focus()`)
    await press(command, ' ', 'Space', 32)
    assert.equal(await evaluate('window.seriesNavigations'), 1)
  }
)

for (const [owner, delayed] of [
  ['jav', false],
  ['studio', false],
  ['detail', false],
  ['jav', true],
]) {
  test(
    `${owner} series preview keeps saved corrections after reopening${delayed ? ' with a late preview response' : ''}`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { origin, command, evaluate, waitFor } = await openBrowser(t)
      await command('Page.navigate', {
        url: `${origin}/tests/fixtures/javSeriesTranslation.html?preview=${owner}`,
      })
      const trigger = `[...document.querySelectorAll('a,button')].find(element => element.textContent.trim() === '旧译名')`
      await waitFor(trigger)
      if (delayed) await evaluate('window.holdPreview = true')
      await evaluate(
        `window.previewTrigger = ${trigger}; window.previewTrigger.dispatchEvent(new MouseEvent('mouseover', {bubbles:true}))`
      )
      await waitFor(`document.querySelector('${editorButton}') && window.previewRequests === 1`)
      await evaluate(`document.querySelector('${editorButton}').click()`)
      await waitFor(`document.querySelector('textarea')?.value === '旧译名'`)
      if (owner === 'jav')
        assert.equal(await evaluate(`Boolean(document.querySelector('${editorButton}'))`), false)
      await evaluate(`{
        const input = document.querySelector('textarea');
        Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value').set.call(input, '校对后的译名');
        input.dispatchEvent(new Event('input', {bubbles:true}));
      }`)
      await evaluate(`document.querySelector('button[type="submit"]').click()`)
      await waitFor(`!document.querySelector('textarea') && window.savedTranslations.length === 1`)
      if (delayed) await evaluate('window.releasePreview()')
      await evaluate(
        `window.previewTrigger.dispatchEvent(new MouseEvent('mouseout', {bubbles:true, relatedTarget:document.body}))`
      )
      await waitFor(`!document.querySelector('${editorButton}')`)
      await evaluate(
        `window.previewTrigger.dispatchEvent(new MouseEvent('mouseover', {bubbles:true}))`
      )
      await waitFor(`document.querySelector('${editorButton}')`)
      await evaluate(`document.querySelector('${editorButton}').click()`)
      await waitFor(`document.querySelector('textarea')`)
      assert.equal(await evaluate(`document.querySelector('textarea').value`), '校对后的译名')
      assert.equal(await evaluate('window.previewRequests'), 1)
      // Saving again must not replace the correction with the original cached translation.
      await evaluate(`document.querySelector('button[type="submit"]').click()`)
      await waitFor(`window.savedTranslations.length === 2`)
      assert.equal(await evaluate('window.savedTranslations[1].zh_name'), '校对后的译名')
    }
  )
}

test(
  'JAV hover series editing closes the preview while the editor survives pointer movement and cancellation',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javSeriesTranslation.html?preview=jav`,
    })
    const trigger = `[...document.querySelectorAll('a')].find(element => element.textContent.trim() === '旧译名')`
    await waitFor(trigger)
    const pointAt = async (expression, click = false) => {
      const point = await evaluate(`(() => {
        const element = ${expression};
        element.scrollIntoView({block:'nearest'});
        const rect = element.getBoundingClientRect();
        return {x:rect.x + rect.width / 2, y:rect.y + rect.height / 2};
      })()`)
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
      if (click)
        for (const type of ['mousePressed', 'mouseReleased'])
          await command('Input.dispatchMouseEvent', {
            type,
            ...point,
            button: 'left',
            clickCount: 1,
          })
    }
    for (const closeWithEscape of [false, true]) {
      await pointAt(trigger)
      await waitFor(`document.querySelector('${editorButton}')`)
      await pointAt(`document.querySelector('${editorButton}')`, true)
      await waitFor(`document.querySelector('textarea')`)
      assert.equal(await evaluate(`Boolean(document.querySelector('${editorButton}'))`), false)
      await pointAt(`document.querySelector('textarea')`)
      // The preview's delayed mouse-leave cleanup must not unmount the editor.
      await evaluate('new Promise(resolve => setTimeout(resolve, 250))')
      assert.equal(await evaluate(`document.querySelector('textarea')?.value`), '旧译名')
      if (closeWithEscape) await press(command, 'Escape', 'Escape', 27)
      else
        await pointAt(
          `[...document.querySelectorAll('button')].find(button => button.textContent === 'Cancel')`,
          true
        )
      await waitFor(`!document.querySelector('textarea')`)
      assert.equal(await evaluate(`Boolean(document.querySelector('${editorButton}'))`), false)
    }
    assert.deepEqual(await evaluate('window.savedTranslations'), [])
    assert.equal(await evaluate('window.seriesNavigations'), 0)
  }
)
