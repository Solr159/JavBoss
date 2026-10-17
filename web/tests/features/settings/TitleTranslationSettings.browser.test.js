import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'series translation switch and batch use series endpoints with the shared provider',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/titleTranslationSettings.html`,
    })
    await waitFor(`document.querySelector('#series-translation-auto-enabled')`)
    await evaluate(`document.querySelector('#series-translation-auto-enabled').click()`)
    await waitFor(`window.testStore.getState().config.jav_series_translation_enabled === true`)
    assert.deepEqual(
      await evaluate(`window.translationRequests.filter(r => r.path === '/config').at(-1).payload`),
      { jav_series_translation_enabled: true }
    )
    assert.equal(
      await evaluate(`window.testStore.getState().config.jav_title_translation_enabled`),
      'false'
    )
    const batch = `document.querySelector('[data-translation-batch="series"]')`
    await evaluate(`${batch}.querySelector('summary').click()`)
    await waitFor(`${batch}.textContent.includes('Translate 3 series names')`)
    await evaluate(
      `[...${batch}.querySelectorAll('button')].find(b => b.textContent === 'Start translation').click()`
    )
    await waitFor(
      `window.translationRequests.some(r => r.path === '/jav/series-translation/batch' && r.method === 'POST')`
    )
    assert.deepEqual(
      await evaluate(
        `window.translationRequests.find(r => r.path === '/jav/series-translation/batch' && r.method === 'POST').payload`
      ),
      { scope: 'missing', confirm_overwrite: false }
    )
    assert.equal(
      await evaluate(
        `window.translationRequests.some(r => r.path === '/jav/title-translation/batch' && r.method === 'POST')`
      ),
      false
    )
  }
)

test(
  'translation settings await the backend catalog and query capabilities only for model edits',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/titleTranslationSettings.html?deferConfig`,
    })
    await waitFor(`document.body?.textContent.includes('Loading translation settings')`)
    assert.equal(await evaluate(`document.querySelector('#title-translation-model')`), null)
    await evaluate('window.loadTranslationConfig()')
    await waitFor(`document.querySelector('#title-translation-model')`)
    await evaluate('new Promise(resolve => setTimeout(resolve, 600))')
    const modelRequests = `window.translationRequests.filter(r => r.path.endsWith('/models'))`
    const capabilityRequests = `${modelRequests}.filter(r => r.payload.discover === false)`
    assert.equal(await evaluate(`${modelRequests}.length`), 0)
    // Saved capabilities still render in read-only mode without querying the server.
    assert.ok(
      await evaluate(`Boolean(document.querySelector('[name="title-translation-thinking"]'))`)
    )
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Edit').click()`
    )
    await waitFor(`${modelRequests}.length === 2`)
    assert.equal(await evaluate(`${capabilityRequests}.length`), 1)
    await evaluate(`{
      document.querySelector('[data-translation-advanced] > summary').click();
      const customAddress = [...document.querySelectorAll('label')].find(l => l.textContent.includes('Use a custom address'));
      customAddress.querySelector('input').click();
    }`)
    await waitFor(`document.querySelector('input[placeholder="https://api.deepseek.com"]')`)
    await evaluate(`{
      const input = document.querySelector('input[placeholder="https://api.deepseek.com"]');
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, 'https://proxy.test');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }`)
    await waitFor(`${modelRequests}.length === 3`)
    assert.equal(await evaluate(`${capabilityRequests}.length`), 1)
    await evaluate(`{
      const input = document.querySelector('#title-translation-model');
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, 'another-model');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }`)
    await waitFor(`${capabilityRequests}.length === 2`)
    assert.deepEqual(await evaluate(`${capabilityRequests}.at(-1).payload.profile`), {
      model: 'another-model',
    })
  }
)

test(
  'provider settings require editing, cancel discards drafts and automatic translation saves independently',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/titleTranslationSettings.html`,
    })
    await waitFor(`document.querySelector('#title-translation-model')`)
    const button = (text) =>
      `[...document.querySelectorAll('button')].find(b => b.textContent === '${text}')`
    const disabled = `document.querySelector('#title-translation-model').matches(':disabled')`
    const requests = `window.translationRequests.filter(r => r.path === '/config')`
    assert.equal(await evaluate(disabled), true)
    assert.equal(await evaluate(`Boolean(${button('Save configuration')})`), false)
    assert.equal(await evaluate(`Boolean(${button('Cancel')})`), false)
    await evaluate(`${button('Edit')}.click()`)
    assert.equal(await evaluate(disabled), false)
    await evaluate(`{
      const input = document.querySelector('#title-translation-model');
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, 'unsaved-model');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    }`)
    await evaluate(
      `document.querySelector('[aria-label="Automatically translate missing titles"]').click()`
    )
    await waitFor(
      `!document.querySelector('[aria-label="Automatically translate missing titles"]').disabled`
    )
    assert.deepEqual(await evaluate(`${requests}.at(-1).payload`), {
      jav_title_translation_enabled: true,
    })
    assert.equal(
      await evaluate(`document.querySelector('#title-translation-model').value`),
      'unsaved-model'
    )
    await evaluate(`${button('Cancel')}.click()`)
    assert.equal(await evaluate(disabled), true)
    assert.equal(
      await evaluate(`document.querySelector('#title-translation-model').value`),
      'saved-model'
    )
    assert.equal(await evaluate(`${requests}.length`), 1)
    assert.equal(
      await evaluate(
        `document.querySelector('[aria-label="Automatically translate missing titles"]').checked`
      ),
      true
    )
    await evaluate(`${button('Edit')}.click()`)
    await evaluate(`window.configFailure = true; ${button('Save configuration')}.click()`)
    await waitFor(`document.body.textContent.includes('Save failed')`)
    assert.equal(await evaluate(disabled), false)
    await evaluate(`window.configFailure = false; ${button('Save configuration')}.click()`)
    await waitFor(disabled)
    assert.equal(await evaluate(`Boolean(${button('Edit')})`), true)
    assert.equal(
      await evaluate(`Object.hasOwn(${requests}.at(-1).payload, 'jav_title_translation_enabled')`),
      false
    )
    assert.equal(
      await evaluate(
        `document.querySelector('[aria-label="Automatically translate missing titles"]').checked`
      ),
      true
    )
  }
)

test(
  'display preference can be saved independently and restores to enabled',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/titleTranslationSettings.html`,
    })
    await waitFor(`document.querySelector('#title-translation-model')`)
    const click = (text) =>
      evaluate(
        `[...document.querySelectorAll('button')].find(b => b.textContent === '${text}').click()`
      )
    const toggle = `document.querySelector('[role="switch"][aria-label="Prefer Chinese titles"]')`
    await click('Display settings')
    await waitFor(toggle)
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
    await evaluate(`${toggle}.click()`)
    await click('Save')
    await waitFor(`!${toggle}`)
    const saved = await evaluate(
      `window.translationRequests.filter(r => r.path === '/config').at(-1).payload`
    )
    assert.equal(saved.jav_title_prefer_chinese, false)
    assert.equal(Object.hasOwn(saved, 'jav_title_translation_enabled'), false)
    assert.equal(
      await evaluate(`window.testStore.getState().config.jav_title_translation_enabled`),
      'false'
    )
    await click('Display settings')
    await waitFor(toggle)
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'false')
    await click('Restore defaults')
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
    await click('Save')
    await waitFor(`!${toggle}`)
    assert.equal(
      await evaluate(`window.testStore.getState().config.jav_title_prefer_chinese`),
      true
    )
  }
)

test(
  'model picker opens all models with a saved value, supports search, keyboard selection and manual IDs',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    const click = async (selector) => {
      const point = await evaluate(`(() => {
        const element = document.querySelector(${JSON.stringify(selector)});
        element.scrollIntoView({ block: 'center' });
        const rect = element.getBoundingClientRect();
        return { x: rect.x + rect.width / 2, y: rect.y + rect.height / 2 };
      })()`)
      for (const type of ['mousePressed', 'mouseReleased']) {
        await command('Input.dispatchMouseEvent', { type, ...point, button: 'left', clickCount: 1 })
      }
    }
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/titleTranslationSettings.html`,
    })
    await waitFor(`document.querySelector('#title-translation-model')`)
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Edit').click()`
    )
    await click('[aria-label="Choose model"]')
    await waitFor(`document.querySelectorAll('[role="option"]').length === 2`)
    assert.equal(
      await evaluate(`document.querySelector('#title-translation-model').value`),
      'saved-model'
    )
    await click('[role="option"]:nth-child(2)')
    await waitFor(`document.querySelector('#title-translation-model').value === 'another-model'`)
    assert.equal(await evaluate(`document.querySelector('[role="listbox"]')`), null)
    await click('#title-translation-model')
    await waitFor(`document.querySelectorAll('[role="option"]').length === 2`)
    await evaluate(`{
      window.typeModel = value => {
        const input = document.querySelector('#title-translation-model');
        input.focus();
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, value);
        input.dispatchEvent(new Event('input', { bubbles: true }));
      };
      window.typeModel('saved');
    }`)
    await waitFor(`document.querySelectorAll('[role="option"]').length === 1`)
    assert.equal(
      await evaluate(`document.querySelector('[role="option"]').textContent`),
      'saved-model'
    )
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'ArrowDown',
      code: 'ArrowDown',
      windowsVirtualKeyCode: 40,
    })
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'Enter',
      code: 'Enter',
      windowsVirtualKeyCode: 13,
    })
    await waitFor(`document.querySelector('#title-translation-model').value === 'saved-model'`)
    await evaluate(`window.typeModel('private-model')`)
    await waitFor(`document.body.textContent.includes('No matching models')`)
    await evaluate(`document.querySelector('input[type="password"]').focus()`)
    assert.equal(
      await evaluate(`document.querySelector('#title-translation-model').value`),
      'private-model'
    )
    assert.equal(
      await evaluate(`window.translationRequests.filter(r => r.path === '/config').length`),
      0
    )
    await click('[aria-label="Choose model"]')
    await waitFor(`document.querySelectorAll('[role="option"]').length === 2`)
  }
)

test(
  'translation settings keep drafts local, test without saving, and confirm batch overwrites',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/titleTranslationSettings.html`,
    })
    await waitFor(`document.querySelector('#title-translation-model')`)
    const button = (text) =>
      `[...document.querySelectorAll('button')].find(b=>b.textContent==='${text}')`
    const requests = (path) => `window.translationRequests.filter(r=>r.path==='${path}')`
    await evaluate(`{
    window.setInput=(selector,value)=>{
      const input=document.querySelector(selector);
      const prototype=input.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;
      Object.getOwnPropertyDescriptor(prototype,'value').set.call(input,value);
      input.dispatchEvent(new Event('input',{bubbles:true}));
    };
  }`)
    await evaluate(`${button('Edit')}.click()`)
    await evaluate(`window.setInput('#title-translation-model','draft-model')`)
    assert.equal(await evaluate(`${requests('/config')}.length`), 0)
    assert.equal(
      await evaluate(
        `document.querySelector('[data-translation-batch] select').matches(':disabled')`
      ),
      true
    )
    await evaluate(`document.querySelector('[data-translation-test] > summary').click()`)
    await evaluate(`window.testFailure=true; ${button('Test translation')}.click()`)
    await waitFor(`document.body.textContent.includes('credit_balance_exhausted')`)
    await evaluate(`window.testFailure=false; ${button('Test translation')}.click()`)
    await waitFor(`document.body.textContent.includes('雨天在车站重逢的两人')`)
    const payload = await evaluate(`${requests('/jav/title-translation/test')}.at(-1).payload`)
    assert.equal(payload.profile.model, 'draft-model')
    assert.equal(Object.hasOwn(payload.profile, 'api_key'), false)
    assert.equal(await evaluate(`${requests('/config')}.length`), 0)
    await evaluate(`document.querySelector('[data-translation-advanced] > summary').click()`)
    await evaluate(`${button('Restore default prompt')}.click()`)
    assert.equal(await evaluate(`document.querySelector('textarea').value`), 'Default prompt')
    await evaluate(`${button('Save configuration')}.click()`)
    await waitFor(
      `document.body.textContent.includes('Saved') && !document.querySelector('[data-translation-batch] select').matches(':disabled')`
    )
    const saved = await evaluate(`${requests('/config')}.at(-1).payload`)
    assert.equal(Object.hasOwn(saved, 'jav_title_prefer_chinese'), false)
    assert.equal(Object.hasOwn(saved, 'jav_title_translation_enabled'), false)
    assert.equal(saved.jav_title_translation_profiles.deepseek.model, 'draft-model')
    assert.equal(saved.jav_title_translation_prompt, 'Default prompt')
    await evaluate(`document.querySelector('[data-translation-batch] > summary').click()`)
    await waitFor(`document.body.textContent.includes('Translate 3 titles')`)
    await evaluate(`${button('Start translation')}.click()`)
    await waitFor(`${requests('/jav/title-translation/batch')}.some(r=>r.method==='POST')`)
    assert.equal(
      await evaluate(
        `${requests('/jav/title-translation/batch')}.filter(r=>r.method==='POST').at(-1).payload.scope`
      ),
      'missing'
    )
    await evaluate(`{
    const select=[...document.querySelectorAll('select')].find(s=>[...s.options].some(o=>o.value==='all'));
    select.value='all';select.dispatchEvent(new Event('change',{bubbles:true}));
  }`)
    await waitFor(`document.body.textContent.includes('Translate 10 titles; 7 already')`)
    assert.equal(await evaluate(`${button('Start translation')}.disabled`), true)
    assert.ok(await evaluate(`document.body.textContent.includes('including manual corrections')`))
    await evaluate(
      `[...document.querySelectorAll('label')].find(l=>l.textContent.includes('I confirm')).querySelector('input').click()`
    )
    await evaluate(`${button('Start translation')}.click()`)
    await waitFor(
      `${requests('/jav/title-translation/batch')}.filter(r=>r.method==='POST').length===2`
    )
    const batchPayload = await evaluate(
      `${requests('/jav/title-translation/batch')}.filter(r=>r.method==='POST').at(-1).payload`
    )
    assert.deepEqual(batchPayload, { scope: 'all', confirm_overwrite: true })
    assert.equal(await evaluate(`${requests('/config')}.length`), 1)
  }
)
