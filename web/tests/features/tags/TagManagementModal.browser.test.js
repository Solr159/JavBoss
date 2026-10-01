import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'edit mode keeps delete actions visible and shows tooltips without reflowing row-end tags',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Tags (Video)"]')`)
    await evaluate(`{
      const originalFetch = window.fetch;
      window.fetch = (input, init) => new URL(input, location.origin).pathname === '/tags'
        ? Promise.resolve(Response.json(Array.from({length: 24}, (_, i) => ({
            id: i + 1, name: 'Example tag ' + (i + 1), count: 24 - i
          }))))
        : originalFetch(input, init);
      document.querySelector('aside button[aria-label="Tags (Video)"]').click();
    }`)
    await waitFor(
      `document.querySelectorAll('.tag-management-modal-list .skeuo-tag').length === 24`
    )
    await evaluate(`Array.from(document.querySelectorAll('[role="dialog"] button'))
      .find(button => button.textContent === 'Edit').click()`)
    await waitFor(`document.querySelector('.skeuo-tag--edit-mode')`)
    assert.equal(
      await evaluate(
        `document.querySelectorAll('.tag-management-modal-list button[aria-label="Delete tag"]').length`
      ),
      24,
      'all delete actions are visible before hovering'
    )

    for (const width of [1024, 768, 390]) {
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 0, y: 0 })
      await command('Emulation.setDeviceMetricsOverride', {
        width,
        height: 900,
        deviceScaleFactor: 1,
        mobile: false,
      })
      const rowEnds = await evaluate(`new Promise(resolve => requestAnimationFrame(() => {
        const list = document.querySelector('.tag-management-modal-list');
        list.scrollTop = 0;
        const bounds = list.getBoundingClientRect();
        const rows = new Map();
        Array.from(list.querySelectorAll('.skeuo-tag')).forEach((tag, index) => {
          const rect = tag.getBoundingClientRect();
          if (rect.top >= bounds.top && rect.bottom <= bounds.bottom) {
            rows.set(rect.top, {index, x: rect.left + rect.width / 2, y: rect.top + rect.height / 2});
          }
        });
        resolve(Array.from(rows.values()).slice(0, 2));
      }))`)
      assert.equal(rowEnds.length, 2, `multiple visible rows at width ${width}`)
      for (const { index, x, y } of rowEnds) {
        await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 0, y: 0 })
        const before =
          await evaluate(`Array.from(document.querySelectorAll('.tag-management-modal-list .skeuo-tag'))
          .map(tag => { const r = tag.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; })`)
        await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y })
        await waitFor(`Array.from(document.querySelectorAll('[role="tooltip"]')).some(tooltip =>
          tooltip.textContent === 'Click to rename: Example tag ${index + 1}')`)
        const samples = await evaluate(`new Promise(resolve => {
          const samples = [];
          const sample = () => {
            const tags = Array.from(document.querySelectorAll('.tag-management-modal-list .skeuo-tag'));
            const target = tags[${index}];
            samples.push({
              rects: tags.map(tag => { const r = tag.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; }),
              hovered: target.matches(':hover'),
              deleteVisible: Boolean(target.querySelector('button[aria-label="Delete tag"]'))
            });
            if (samples.length < 12) requestAnimationFrame(sample);
            else resolve(samples);
          };
          requestAnimationFrame(sample);
        })`)
        for (const sample of samples) {
          assert.deepEqual(sample.rects, before, `hover must not reflow tags at width ${width}`)
          assert.equal(sample.hovered, true, 'row-end tag remains under the pointer')
          assert.equal(sample.deleteVisible, true, 'delete action stays visible')
        }
      }
    }
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 0, y: 0 })
    assert.equal(
      await evaluate(
        `document.querySelectorAll('.tag-management-modal-list button[aria-label="Delete tag"]').length`
      ),
      24,
      'delete actions remain visible after leaving tags'
    )
    await evaluate(`Array.from(document.querySelectorAll('[role="dialog"] button'))
      .find(button => button.textContent === 'Exit edit').click()`)
    await waitFor(`!document.querySelector('.skeuo-tag--edit-mode')`)
    assert.equal(
      await evaluate(
        `document.querySelectorAll('.tag-management-modal-list button[aria-label="Delete tag"]').length`
      ),
      0,
      'delete actions are hidden outside edit mode'
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
