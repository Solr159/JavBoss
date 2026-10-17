import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

for (const scenario of ['unchanged source', 'changed source', 'manual correction']) {
  test(
    `JAV translation preview saves correctly with ${scenario}`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { origin, command, evaluate, waitFor } = await openBrowser(t)
      await command('Page.navigate', {
        url: `${origin}/tests/fixtures/javDetailNavigation.html?view=jav`,
      })
      await waitFor(`document.querySelector('[aria-label="Edit JAV"]')`)
      await evaluate(`{
        const originalFetch = window.fetch;
        window.fetch = (input, init) => {
          const path = new URL(input, location.origin).pathname;
          if (path.endsWith('/title-translation')) {
            window.translationPayload = JSON.parse(init.body);
            return Promise.resolve(Response.json({id:1,title:'Test JAV',zh_title:'雨天相遇'}));
          }
          if (['/jav/studios','/jav/series','/jav/idols/options'].includes(path))
            return Promise.resolve(Response.json({items:[],total:0}));
          if (path === '/jav/tags') return Promise.resolve(Response.json([]));
          if (path === '/jav/items/1' && init?.method === 'PUT') {
            window.savedPayload = JSON.parse(init.body);
            return Promise.resolve(Response.json({id:1,...window.savedPayload}));
          }
          return originalFetch(input, init);
        };
        window.testStore.setState({loadJavTags:()=>Promise.resolve()});
        window.setText = (selector, value) => {
          const input = document.querySelector(selector);
          Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(input,value);
          input.dispatchEvent(new Event('input',{bubbles:true}));
        };
        document.querySelector('[aria-label="Edit JAV"]').click();
      }`)
      await waitFor(`document.querySelector('#jav-title-1')`)
      await evaluate(
        `[...document.querySelectorAll('button')].find(b => b.textContent === 'Quick retranslation').click()`
      )
      await waitFor(`document.querySelector('#jav-zh-title-1').value === '雨天相遇'`)
      assert.equal(await evaluate('window.savedPayload'), undefined)
      assert.deepEqual(await evaluate('window.translationPayload'), {
        title: 'Test JAV',
        thinking: false,
      })
      if (scenario === 'manual correction') {
        await evaluate(`window.setText('#jav-zh-title-1', '人工校对译名')`)
      }
      if (scenario !== 'unchanged source') {
        await evaluate(`window.setText('#jav-title-1', 'Different source')`)
      }
      const expectedTitle = scenario === 'unchanged source' ? 'Test JAV' : 'Different source'
      const expectedChineseTitle =
        scenario === 'changed source'
          ? ''
          : scenario === 'manual correction'
            ? '人工校对译名'
            : '雨天相遇'
      assert.equal(
        await evaluate(`document.querySelector('#jav-zh-title-1').value`),
        expectedChineseTitle
      )
      await evaluate(
        `[...document.querySelectorAll('button')].find(b => b.textContent === 'Save').click()`
      )
      await waitFor('window.savedPayload')
      const payload = await evaluate('window.savedPayload')
      assert.equal(payload.title, expectedTitle)
      assert.equal(payload.zh_title || '', expectedChineseTitle)
    }
  )
}

test(
  'JAV quick retranslation displays provider details and preserves the draft on failure',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javDetailNavigation.html?view=jav`,
    })
    await waitFor(`document.querySelector('[aria-label="Edit JAV"]')`)
    await evaluate(`{
      const originalFetch = window.fetch;
      window.authExpired = false;
      window.addEventListener('javboss:auth-expired', () => { window.authExpired = true; });
      window.fetch = (input, init) => {
        const path = new URL(input, location.origin).pathname;
        if (path.endsWith('/title-translation')) {
          return Promise.resolve(Response.json({
            error_zh: '服务商账户余额已耗尽，请充值后重试 (HTTP 429 / credit_balance_exhausted): Add credits.',
            error_en: 'The provider account credit balance is exhausted; add credits and retry (HTTP 429 / credit_balance_exhausted): Add credits.',
          }, {status:429}));
        }
        if (['/jav/studios','/jav/series','/jav/idols/options'].includes(path))
          return Promise.resolve(Response.json({items:[],total:0}));
        if (path === '/jav/tags') return Promise.resolve(Response.json([]));
        return originalFetch(input, init);
      };
      window.testStore.setState({loadJavTags:()=>Promise.resolve()});
      document.querySelector('[aria-label="Edit JAV"]').click();
    }`)
    await waitFor(`document.querySelector('#jav-zh-title-1')`)
    await evaluate(`{
      const input = document.querySelector('#jav-zh-title-1');
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype,'value').set.call(input,'人工校对译名');
      input.dispatchEvent(new Event('input',{bubbles:true}));
    }`)
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Quick retranslation').click()`
    )
    await waitFor(`document.body.textContent.includes('credit_balance_exhausted')`)
    const text = await evaluate('document.body.textContent')
    assert.ok(text.includes('credit balance is exhausted'))
    assert.ok(text.includes('HTTP 429'))
    assert.ok(text.includes('Add credits.'))
    assert.equal(await evaluate(`document.querySelector('#jav-zh-title-1').value`), '人工校对译名')
    assert.equal(await evaluate('window.authExpired'), false)
  }
)

test(
  'quick and reasoning translation buttons respect model capabilities',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javDetailNavigation.html?view=jav`,
    })
    await waitFor(`document.querySelector('[aria-label="Edit JAV"]')`)
    await evaluate(`{
    const originalFetch=window.fetch;
    window.translationCalls=[];
    window.fetch=(input,init)=>{
      const path=new URL(input,location.origin).pathname;
      if(path.endsWith('/title-translation')) {
        window.translationCalls.push(JSON.parse(init.body));
        return Promise.resolve(Response.json({id:1,title:'Test JAV',zh_title:'测试译文'}));
      }
      if(['/jav/studios','/jav/series','/jav/idols/options'].includes(path)) return Promise.resolve(Response.json({items:[],total:0}));
      if(path==='/jav/tags') return Promise.resolve(Response.json([]));
      return originalFetch(input,init);
    };
    window.testStore.setState({loadJavTags:()=>Promise.resolve()});
    document.querySelector('[aria-label="Edit JAV"]').click();
  }`)
    await waitFor(`document.querySelector('#jav-title-1')`)
    const button = (label) =>
      `[...document.querySelectorAll('button')].find(b => b.textContent === '${label}')`
    for (const mode of ['none', 'always', 'toggle', 'unknown']) {
      await evaluate(
        `window.testStore.setState({config:{...window.testStore.getState().config,jav_title_translation_profiles:JSON.stringify({deepseek:{thinking:true}}),jav_title_translation_capabilities:JSON.stringify({reasoning:'${mode}'})}})`
      )
      assert.equal(
        await evaluate(`${button('Quick retranslation')}.disabled`),
        mode === 'always' || mode === 'unknown'
      )
      assert.equal(
        await evaluate(`${button('Retranslate with reasoning')}.disabled`),
        mode === 'unknown'
      )
      if (mode === 'unknown') continue
      if (mode !== 'always') {
        await evaluate(`${button('Quick retranslation')}.click()`)
        await waitFor(
          `Boolean(${button('Quick retranslation')}) && !${button('Quick retranslation')}.disabled`
        )
        assert.equal(await evaluate('window.translationCalls.at(-1).thinking'), false)
      }
      await evaluate(`${button('Retranslate with reasoning')}.click()`)
      await waitFor(
        `Boolean(${button('Retranslate with reasoning')}) && !${button('Retranslate with reasoning')}.disabled`
      )
      assert.equal(await evaluate('window.translationCalls.at(-1).thinking'), true)
    }
    assert.equal(await evaluate('window.translationCalls.length'), 5)
  }
)

test(
  'automatic translation preserves open editor fields and selection until saved',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javAutomaticTranslation.html` })
    await waitFor('window.translationRequests?.length === 1')
    await evaluate(`{
      window.testStore.getState().patchJavItem({id: 1, idols: [{id: 9, name: 'Original idol'}]});
      window.editTranslationItem(1);
    }`)
    await waitFor(`document.querySelector('#jav-title-1')`)
    await evaluate(`{
      window.setEditorValue = (selector, value) => {
        const input = document.querySelector(selector);
        const proto = input.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
        Object.getOwnPropertyDescriptor(proto, 'value').set.call(input, value);
        input.dispatchEvent(new Event('input', {bubbles: true}));
      };
      window.setEditorValue('#jav-title-1', 'Edited original');
      window.setEditorValue('#jav-zh-title-1', '人工校对');
      window.setEditorValue('input[type="number"]', '123');
      document.querySelector('[aria-label="Remove Original idol"]').click();
    }`)
    await evaluate('window.emitTranslation(0, [1])')
    await waitFor(`window.testStore.getState().javItems[0].zh_title === '译文 1'`)
    assert.equal(await evaluate(`document.querySelector('#jav-title-1').value`), 'Edited original')
    assert.equal(await evaluate(`document.querySelector('#jav-zh-title-1').value`), '人工校对')
    assert.equal(await evaluate(`document.querySelector('input[type="number"]').value`), '123')
    assert.equal(
      await evaluate(`Boolean(document.querySelector('[aria-label="Remove Original idol"]'))`),
      false
    )
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Save').click()`
    )
    await waitFor('window.savedEditorPayload')
    const payload = await evaluate('window.savedEditorPayload')
    assert.equal(payload.title, 'Edited original')
    assert.equal(payload.zh_title, '人工校对')
    assert.equal(payload.duration_min, 123)
    assert.deepEqual(payload.idol_ids, [])
  }
)

test(
  'editor keeps its opening baseline for saving and resets on reopening or switching items',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javAutomaticTranslation.html` })
    await waitFor('window.translationRequests?.length === 1')
    await evaluate('window.editTranslationItem(1)')
    await waitFor(`document.querySelector('#jav-zh-title-1')`)
    await evaluate('window.emitTranslation(0, [1])')
    await waitFor(`window.testStore.getState().javItems[0].zh_title === '译文 1'`)
    assert.equal(await evaluate(`document.querySelector('#jav-zh-title-1').value`), '')
    await evaluate(
      `[...document.querySelectorAll('button')].find(b => b.textContent === 'Save').click()`
    )
    await waitFor('window.savedEditorPayload')
    assert.equal(await evaluate(`Object.hasOwn(window.savedEditorPayload, 'zh_title')`), false)
    assert.equal(await evaluate(`window.testStore.getState().javItems[0].zh_title`), '译文 1')
    await evaluate('window.editTranslationItem(1)')
    await waitFor(`document.querySelector('#jav-zh-title-1')?.value === '译文 1'`)
    await evaluate('window.editTranslationItem(2)')
    await waitFor(`document.querySelector('#jav-title-2')?.value === 'Source 2'`)
    assert.equal(await evaluate(`document.querySelector('#jav-zh-title-2').value`), '')
    await evaluate('window.emitTranslation(0, [2])')
    await waitFor(`window.testStore.getState().javItems[1].zh_title === '译文 2'`)
    assert.equal(await evaluate(`document.querySelector('#jav-zh-title-2').value`), '')
  }
)
