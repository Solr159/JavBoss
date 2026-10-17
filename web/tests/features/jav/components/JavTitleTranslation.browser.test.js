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
  'retranslation restores separate quick and reasoning buttons with model capability limits',
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
        await evaluate(
          `Boolean(document.querySelector('[aria-label="Use another translation mode"]'))`
        ),
        false
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
