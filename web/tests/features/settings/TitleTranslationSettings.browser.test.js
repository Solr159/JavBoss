import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

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
    assert.equal(
      await evaluate(`document.querySelector('#translation-behavior-heading').textContent`),
      'Automatically translate missing titles'
    )
    assert.equal(await evaluate(`Boolean(${button('Review batch')})`), false)
    assert.equal(
      await evaluate(
        `document.body.textContent.includes('Uses saved settings; automatic translation can remain off.')`
      ),
      false
    )

    assert.equal(
      await evaluate(`document.querySelector('[data-translation-advanced]').open`),
      false
    )
    assert.equal(await evaluate(`document.querySelector('[data-translation-test]').open`), false)
    assert.equal(await evaluate(`document.querySelector('[data-translation-batch]').open`), false)
    assert.equal(
      await evaluate(`document.querySelector('input[placeholder="https://api.deepseek.com"]')`),
      null
    )
    assert.equal(
      await evaluate(`Boolean(document.querySelector('[aria-label="Prefer Chinese titles"]'))`),
      false
    )
    assert.equal(
      await evaluate(
        `document.querySelector('[aria-label="Automatically translate missing titles"]').checked`
      ),
      false
    )
    await evaluate(`{
    window.setInput=(selector,value)=>{
      const input=document.querySelector(selector);
      const prototype=input.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;
      Object.getOwnPropertyDescriptor(prototype,'value').set.call(input,value);
      input.dispatchEvent(new Event('input',{bubbles:true}));
    };
    document.querySelector('[aria-label="Automatically translate missing titles"]').click();
  }`)
    assert.equal(await evaluate(`${requests('/config')}.length`), 0)
    assert.equal(
      await evaluate(
        `document.querySelector('[data-translation-batch] select').matches(':disabled')`
      ),
      true
    )
    await evaluate(
      `document.querySelector('[aria-label="Automatically translate missing titles"]').click()`
    )
    await evaluate(`window.setInput('#title-translation-model','draft-model')`)
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
    await evaluate(`${button('Save settings')}.click()`)
    await waitFor(
      `document.body.textContent.includes('Saved') && !document.querySelector('[data-translation-batch] select').matches(':disabled')`
    )
    const saved = await evaluate(`${requests('/config')}.at(-1).payload`)
    assert.equal(Object.hasOwn(saved, 'jav_title_prefer_chinese'), false)
    assert.equal(saved.jav_title_translation_enabled, false)
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
