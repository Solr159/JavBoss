import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'display settings cancel edits across all tabs without closing and keep default cursors',
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
      if (tab === 'Videos') return
      await evaluate(
        `[...${dialog}.querySelectorAll('[role="tab"]')].find(button => button.textContent === '${tab}').click()`
      )
      await waitFor(
        `${dialog}.querySelector('[role="tab"][aria-selected="true"]').textContent === '${tab}'`
      )
    }

    for (const [view, tabs] of [
      ['video', ['Videos']],
      ['jav', ['JAV', 'Idols', 'Studios', 'Series', 'Tags']],
    ]) {
      await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=${view}` })
      await waitFor(`document.querySelector('aside button[aria-label="Display"]')`)
      await openSettings()
      const savedValues = new Map()
      for (const tab of tabs) {
        await selectTab(tab)
        savedValues.set(tab, await evaluate(values))
        assert.deepEqual(
          await evaluate(`[...${footerButtons}].map(button => button.textContent)`),
          ['Edit'],
          tab
        )
        assert.ok(
          await evaluate(
            `[...${fields}.querySelectorAll(':disabled')].every(el => getComputedStyle(el).cursor === 'default')`
          ),
          tab
        )
        assert.ok(
          await evaluate(
            `[...${fields}.querySelectorAll('input, select, button')].every(el => el.matches(':disabled'))`
          ),
          tab
        )
        // Disabled switches and sort buttons must not change the draft on click.
        const before = await evaluate(`${fields}.innerHTML`)
        await evaluate(`${fields}.querySelectorAll('button').forEach(button => button.click())`)
        assert.equal(await evaluate(`${fields}.innerHTML`), before, tab)
      }

      const draftValues = new Map()
      for (const tab of tabs) {
        await selectTab(tab)
        assert.equal(await evaluate(`${fields}.disabled`), true, tab)
        await clickFooter('Edit')
        assert.deepEqual(
          await evaluate(`[...${footerButtons}].map(button => button.textContent)`),
          ['Restore defaults', 'Cancel', 'Save']
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
      await clickFooter('Cancel')
      await waitFor(`${dialog} && ${fields}.disabled`)
      if (view === 'jav') {
        assert.equal(
          await evaluate(
            `${dialog}.querySelector('[role="tab"][aria-selected="true"]').textContent`
          ),
          'Tags'
        )
      }
      assert.equal(
        await evaluate(`window.requests.some(r => r.url === '/config' && r.method === 'PATCH')`),
        false
      )
      assert.deepEqual(await evaluate(`[...${footerButtons}].map(button => button.textContent)`), [
        'Edit',
      ])
      for (const tab of tabs) {
        await selectTab(tab)
        if (tab !== tabs.at(-1)) {
          assert.equal(await evaluate(`${fields}.disabled`), false, tab)
          assert.deepEqual(await evaluate(values), draftValues.get(tab), tab)
          await clickFooter('Cancel')
        }
        assert.equal(await evaluate(`${fields}.disabled`), true)
        assert.deepEqual(await evaluate(values), savedValues.get(tab), tab)
      }
      // Editing again starts from the saved values, not the cancelled draft.
      await clickFooter('Edit')
      assert.deepEqual(await evaluate(values), savedValues.get(tabs.at(-1)))
      await clickFooter('Cancel')
      await evaluate(`${dialog}.querySelector('[aria-label="Close settings"]').click()`)
      await waitFor(`!${dialog}`)
      await openSettings()
      assert.equal(await evaluate(`${fields}.disabled`), true)
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
    await click('Edit')
    await setPageSize(40)
    await click('Idols')
    assert.equal(await evaluate(`${fields}.disabled`), true)
    await click('Edit')
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
    assert.equal(await evaluate(`${fields}.disabled`), true)
    assert.equal(await evaluate(`${input}.value`), '24')
    await click('Edit')
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
    assert.equal(await evaluate(`${fields}.disabled`), true)
    assert.equal(await evaluate(`${input}.value`), '48')
    await click('Edit')
    await setPageSize(72)
    await click('Cancel')
    assert.equal(await evaluate(`${input}.value`), '48')
    assert.equal(await evaluate('window.settingsSaves.length'), 3)
  }
)

test(
  'entering display editing preserves primary button focus and hover color without flashing',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-display-settings-test',
    })
    for (const view of ['jav', 'video']) {
      await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=${view}` })
      await waitFor(`document.querySelector('aside button[aria-label="Display"]')`)
      await evaluate(`document.querySelector('aside button[aria-label="Display"]').click()`)
      await waitFor(`document.querySelector('[role="dialog"] footer button')`)
      const point = await evaluate(`(() => {
        window.editButton = document.querySelector('[role="dialog"] footer button');
        const rect = window.editButton.getBoundingClientRect();
        return {x: rect.x + rect.width / 2, y: rect.y + rect.height / 2};
      })()`)
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
      await waitFor(`getComputedStyle(window.editButton).backgroundColor === 'rgb(29, 78, 216)'`)
      await command('Input.dispatchMouseEvent', {
        type: 'mousePressed',
        ...point,
        button: 'left',
        clickCount: 1,
      })
      await command('Input.dispatchMouseEvent', {
        type: 'mouseReleased',
        ...point,
        button: 'left',
        clickCount: 1,
      })
      const frames = await evaluate(`new Promise(resolve => {
        const frames = [];
        const sample = () => {
          const button = [...document.querySelectorAll('[role="dialog"] footer button')].find(button => button.textContent === 'Save');
          frames.push({color: getComputedStyle(button).backgroundColor, focused: document.activeElement === button});
          if (frames.length < 12) requestAnimationFrame(sample);
          else resolve(frames);
        };
        sample();
      })`)
      assert.ok(
        frames.every((frame) => frame.color === 'rgb(29, 78, 216)' && frame.focused),
        view
      )
      assert.equal(
        await evaluate(`window.requests.some(r => r.url === '/config' && r.method === 'PATCH')`),
        false
      )
    }
  }
)
