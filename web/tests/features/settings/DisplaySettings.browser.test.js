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

      await clickFooter('Edit')
      assert.deepEqual(await evaluate(`[...${footerButtons}].map(button => button.textContent)`), [
        'Restore defaults',
        'Cancel',
        'Save',
      ])
      for (const tab of tabs) {
        await selectTab(tab)
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
