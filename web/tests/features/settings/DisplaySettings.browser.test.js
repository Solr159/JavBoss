import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'video directory bar defaults to visible and follows saved settings and restored defaults',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-display-settings-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    const bar = `document.querySelector('nav[aria-label="Current video directory"]')`
    const dialog = `document.querySelector('[role="dialog"]')`
    const toggle = `${dialog}.querySelector('[role="switch"][aria-label="Show bottom directory bar"]')`
    const openSettings = async () => {
      await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
      await waitFor(toggle)
    }
    const clickFooter = (label) =>
      evaluate(
        `[...${dialog}.querySelectorAll('footer button')].find(button => button.textContent === '${label}').click()`
      )
    await waitFor(`${bar} && window.testStore.getState().config.video_page_size`)
    await openSettings()
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
    await evaluate(`${toggle}.click()`)
    assert.ok(await evaluate(`Boolean(${bar})`), 'unsaved changes must not hide the bar')
    await evaluate(`${dialog}.querySelector('[aria-label="Close settings"]').click()`)
    await waitFor(`!${dialog}`)
    await openSettings()
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
    await evaluate(`${toggle}.click()`)
    await clickFooter('Save')
    await waitFor(`!${dialog} && !${bar}`)
    assert.equal(
      await evaluate('window.testStore.getState().config.video_show_directory_bar'),
      false
    )
    await evaluate(
      `window.testStore.setState({config:{}}); window.testStore.getState().loadConfig()`
    )
    await waitFor(`window.testStore.getState().config.video_page_size && !${bar}`)
    await openSettings()
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'false')
    await clickFooter('Restore defaults')
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
    assert.equal(await evaluate(`Boolean(${bar})`), false)
    await clickFooter('Save')
    await waitFor(`!${dialog} && ${bar}`)
    assert.equal(
      await evaluate('window.testStore.getState().config.video_show_directory_bar'),
      true
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test(
  'display settings are editable immediately and closing discards all unsaved drafts',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-display-settings-test',
    })
    const dialog = `document.querySelector('[role="dialog"]')`
    const fields = `${dialog}.querySelector('fieldset')`
    const footerButtons = `${dialog}.querySelectorAll('footer button')`
    const values = `[...${fields}.querySelectorAll('input, select, button[role="switch"]')].map(el => ({value: el.value, checked: el.checked, ariaChecked: el.getAttribute('aria-checked')}))`
    const clickFooter = async (label) =>
      evaluate(`[...${footerButtons}].find(button => button.textContent === '${label}').click()`)
    const openSettings = async () => {
      await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
      await waitFor(dialog)
    }
    const selectTab = async (tab) => {
      await evaluate(
        `[...${dialog}.querySelectorAll('[role="tab"]')].find(button => button.textContent === '${tab}').click()`
      )
      await waitFor(
        `${dialog}.querySelector('[role="tab"][aria-selected="true"]').textContent === '${tab}'`
      )
    }

    for (const [view, tabs] of [
      ['video', ['Videos', 'JAV', 'Idols', 'Studios', 'Series', 'Tags']],
      ['jav', ['Videos', 'JAV', 'Idols', 'Studios', 'Series', 'Tags']],
    ]) {
      await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=${view}` })
      await waitFor(`document.querySelector('aside button[aria-label="Display"]')`)
      await openSettings()
      assert.equal(await evaluate(`document.querySelectorAll('[role="dialog"]').length`), 1)
      assert.deepEqual(
        await evaluate(
          `[...${dialog}.querySelectorAll('[role="tab"]')].map(tab => tab.textContent)`
        ),
        ['Videos', 'JAV', 'Idols', 'Studios', 'Series', 'Tags']
      )
      assert.equal(
        await evaluate(`${dialog}.querySelector('[role="tab"][aria-selected="true"]').textContent`),
        view === 'video' ? 'Videos' : 'JAV'
      )
      const savedValues = new Map()
      for (const tab of tabs) {
        await selectTab(tab)
        savedValues.set(tab, await evaluate(values))
        assert.deepEqual(
          await evaluate(`[...${footerButtons}].map(button => button.textContent)`),
          ['Restore defaults', 'Save'],
          tab
        )
        assert.ok(
          await evaluate(
            `[...${fields}.querySelectorAll('input, select, button[role="switch"]')].every(el => !el.matches(':disabled'))`
          ),
          tab
        )
      }

      const draftValues = new Map()
      for (const tab of tabs) {
        await selectTab(tab)
        assert.equal(await evaluate(`${fields}.disabled`), false, tab)
        assert.deepEqual(
          await evaluate(`[...${footerButtons}].map(button => button.textContent)`),
          ['Restore defaults', 'Save']
        )
        assert.ok(
          await evaluate(
            `[...${fields}.querySelectorAll('input, select, button[role="switch"]')].every(el => !el.matches(':disabled'))`
          ),
          tab
        )
        const toggle = `${fields}.querySelector('button[role="switch"]')`
        if (await evaluate(`Boolean(${toggle})`)) {
          const checked = await evaluate(`${toggle}.getAttribute('aria-checked')`)
          await evaluate(`${toggle}.click()`)
          await waitFor(`${toggle}.getAttribute('aria-checked') !== '${checked}'`)
          await clickFooter('Restore defaults')
          await waitFor(`${toggle}.getAttribute('aria-checked') === 'false'`)
          await evaluate(`${toggle}.click()`)
          await waitFor(`${toggle}.getAttribute('aria-checked') === 'true'`)
        } else {
          await evaluate(`{
            const select = ${fields}.querySelector('select');
            select.value = 'simplified';
            select.dispatchEvent(new Event('change', {bubbles: true}));
          }`)
        }
        draftValues.set(tab, await evaluate(values))
      }
      assert.equal(
        await evaluate(`window.requests.some(r => r.url === '/config' && r.method === 'PATCH')`),
        false
      )
      for (const tab of tabs) {
        await selectTab(tab)
        assert.deepEqual(await evaluate(values), draftValues.get(tab), tab)
      }
      await evaluate(`${dialog}.querySelector('[aria-label="Close settings"]').click()`)
      await waitFor(`!${dialog}`)
      await openSettings()
      for (const tab of tabs) {
        await selectTab(tab)
        assert.equal(await evaluate(`${fields}.disabled`), false)
        assert.deepEqual(await evaluate(values), savedValues.get(tab), tab)
      }
      await evaluate(`${dialog}.querySelector('[aria-label="Close settings"]').click()`)
      await waitFor(`!${dialog}`)
    }
  }
)

test(
  'saving closes display settings and persists only the current tab, while failed saves stay editable',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-display-settings-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await waitFor(`document.querySelector('aside button[aria-label="Display"]')`)
    await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
    const dialog = `document.querySelector('[role="dialog"]')`
    const fields = `${dialog}.querySelector('fieldset')`
    const input = `${fields}.querySelector('input[type="number"]')`
    const click = (label) =>
      evaluate(
        `[...${dialog}.querySelectorAll('button')].find(button => button.textContent === '${label}').click()`
      )
    const setPageSize = (value) =>
      evaluate(`{
      const input = ${input};
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, '${value}');
      input.dispatchEvent(new Event('input', {bubbles: true}));
    }`)
    await waitFor(dialog)
    await evaluate(`{
      const originalFetch = window.fetch;
      window.settingsSaves = [];
      window.fetch = (url, init) => {
        if (url === '/config' && init?.method === 'PATCH') {
          window.settingsSaves.push(JSON.parse(init.body));
          if (window.failSettingsSave) return Promise.resolve(Response.json({error_en:'Save failed'}, {status:500}));
        }
        return originalFetch(url, init);
      };
    }`)
    await setPageSize(40)
    await click('Idols')
    assert.equal(await evaluate(`${fields}.disabled`), false)
    await setPageSize(48)
    await click('Save')
    await waitFor(`!${dialog} && window.testStore.getState().idolPageSize === 48`)
    assert.equal(await evaluate('window.testStore.getState().javPageSize'), 24)
    assert.deepEqual(await evaluate('Object.keys(window.settingsSaves[0]).sort()'), [
      'idol_page_size',
      'idol_sort',
      'idol_waterfall_default',
      'jav_idol_prefer_chinese_name',
    ])
    await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
    await waitFor(dialog)
    assert.equal(await evaluate(`${fields}.disabled`), false)
    assert.equal(await evaluate(`${input}.value`), '24')
    await setPageSize(40)
    await evaluate('window.failSettingsSave = true')
    await click('Save')
    await waitFor('window.settingsSaves.length === 2')
    await waitFor(`!${fields}.disabled`)
    assert.equal(await evaluate(`${input}.value`), '40')
    assert.equal(await evaluate('window.testStore.getState().javPageSize'), 24)
    await evaluate('window.failSettingsSave = false')
    await click('Save')
    await waitFor(`!${dialog} && window.testStore.getState().javPageSize === 40`)
    assert.equal(await evaluate('window.testStore.getState().idolPageSize'), 48)
    await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
    await waitFor(dialog)
    await click('Idols')
    assert.equal(await evaluate(`${fields}.disabled`), false)
    assert.equal(await evaluate(`${input}.value`), '48')
    await setPageSize(72)
    await evaluate(`${dialog}.querySelector('[aria-label="Close settings"]').click()`)
    await waitFor(`!${dialog}`)
    await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
    await waitFor(dialog)
    await click('Idols')
    assert.equal(await evaluate(`${input}.value`), '48')
    assert.equal(await evaluate('window.settingsSaves.length'), 3)
  }
)

test(
  'video and JAV tabs share one dialog but save only the selected tab',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-display-settings-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Display"]')`)
    const dialog = `document.querySelector('[aria-labelledby="display-settings-title"]')`
    const fields = `${dialog}.querySelector('fieldset')`
    const input = `${fields}.querySelector('input[type="number"]')`
    const click = (label) =>
      evaluate(
        `[...${dialog}.querySelectorAll('button')].find(button => button.textContent === '${label}').click()`
      )
    const setPageSize = (value) =>
      evaluate(`{
      const input = ${input};
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, '${value}');
      input.dispatchEvent(new Event('input', {bubbles: true}));
    }`)
    await evaluate(`{
      const originalFetch = window.fetch;
      window.savedSettings = [];
      window.fetch = (url, init) => {
        if (url === '/config' && init?.method === 'PATCH') window.savedSettings.push(JSON.parse(init.body));
        return originalFetch(url, init);
      };
    }`)
    for (const target of ['Videos', 'JAV']) {
      await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
      await waitFor(dialog)
      assert.equal(
        await evaluate(`${dialog}.querySelector('[role="tab"][aria-selected="true"]').textContent`),
        'Videos'
      )
      await setPageSize(target === 'Videos' ? 40 : 60)
      await click('JAV')
      assert.equal(await evaluate(`${fields}.disabled`), false)
      await setPageSize(48)
      await click('Videos')
      assert.equal(await evaluate(`${fields}.disabled`), false)
      assert.equal(await evaluate(`${input}.value`), target === 'Videos' ? '40' : '60')
      await click(target)
      await click('Save')
      await waitFor(`!${dialog}`)
      assert.equal(await evaluate('window.testStore.getState().pageSize'), 40)
      assert.equal(
        await evaluate('window.testStore.getState().javPageSize'),
        target === 'Videos' ? 24 : 48
      )
      assert.ok(
        await evaluate(
          `Object.keys(window.savedSettings.at(-1)).every(key => key.startsWith('${target === 'Videos' ? 'video_' : 'jav_'}'))`
        )
      )
      assert.equal(await evaluate('window.testStore.getState().viewMode'), 'video')
    }
  }
)
