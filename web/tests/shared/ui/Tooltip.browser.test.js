import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

async function press(
  command,
  key,
  code = key,
  windowsVirtualKeyCode = key === 'Escape' ? 27 : key === 'Tab' ? 9 : 13,
  modifiers = 0
) {
  for (const type of ['keyDown', 'keyUp'])
    await command('Input.dispatchKeyEvent', {
      type,
      key,
      code,
      windowsVirtualKeyCode,
      modifiers,
      ...(key === 'Enter' && type === 'keyDown' ? { text: '\r', unmodifiedText: '\r' } : {}),
    })
}
async function pointAt(evaluate, command, selector, click = false) {
  const point = await evaluate(`(() => {
    const element = document.querySelector(${JSON.stringify(selector)});
    element.scrollIntoView({block:'center'});
    const rect = element.getBoundingClientRect();
    return {x:rect.x + rect.width / 2, y:rect.y + rect.height / 2};
  })()`)
  await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
  if (click)
    for (const type of ['mousePressed', 'mouseReleased'])
      await command('Input.dispatchMouseEvent', { type, ...point, button: 'left', clickCount: 1 })
}
async function assertNoTooltip(evaluate) {
  // Include MUI's focus enter timer and exit transition; a delayed reopen is also a failure.
  await evaluate('new Promise(resolve => setTimeout(resolve, 400))')
  assert.equal(await evaluate(`Boolean(document.querySelector('[role="tooltip"]'))`), false)
}

test(
  'JAV edit restores focus after Escape without reopening its tooltip',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/javDetailNavigation.html?view=jav`,
    })
    await waitFor(`document.querySelector('[aria-label="Edit JAV"]')`)
    await pointAt(evaluate, command, '[aria-label="Edit JAV"]', true)
    await waitFor(`document.querySelector('[aria-label="Edit JAV info"]')`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 5, y: 5 })
    await press(command, 'Escape')
    await waitFor(`!document.querySelector('[aria-label="Edit JAV info"]')`)
    assert.equal(await evaluate(`document.activeElement.getAttribute('aria-label')`), 'Edit JAV')
    await assertNoTooltip(evaluate)
    await pointAt(evaluate, command, '[aria-label="Edit JAV"]')
    await waitFor(`document.querySelector('[role="tooltip"]')?.textContent === 'Edit JAV'`)
    // The pointer may stay on the trigger while the overlay opens and closes.
    await pointAt(evaluate, command, '[aria-label="Edit JAV"]', true)
    await waitFor(`document.querySelector('[aria-label="Edit JAV info"]')`)
    await press(command, 'Escape')
    await waitFor(`!document.querySelector('[aria-label="Edit JAV info"]')`)
    await assertNoTooltip(evaluate)
  }
)

for (const [trigger, overlay] of [
  ['#editor-trigger', '[aria-label="Editor"]'],
  ['#menu-trigger', '[role="menu"]'],
]) {
  test(
    `${trigger} keeps restored focus silent and supports fresh keyboard and pointer navigation`,
    { skip: browserUnavailable, timeout: 60000 },
    async (t) => {
      const { origin, command, evaluate, waitFor } = await openBrowser(t)
      await command('Page.navigate', { url: `${origin}/tests/fixtures/tooltipFocus.html` })
      await waitFor(`document.querySelector('${trigger}')`)
      await pointAt(evaluate, command, trigger, true)
      await waitFor(`document.querySelector('${overlay}')`)
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 5, y: 5 })
      await press(command, 'Escape')
      await waitFor(`!document.querySelector('${overlay}')`)
      assert.equal(await evaluate(`document.activeElement.id`), trigger.slice(1))
      await assertNoTooltip(evaluate)
      // Tab away, then Shift+Tab back: keyboard users still get the tooltip.
      await press(command, 'Tab')
      await press(command, 'Tab', 'Tab', 9, 8)
      await waitFor(`document.querySelector('[role="tooltip"]')`)
      assert.equal(await evaluate(`document.activeElement.id`), trigger.slice(1))
      await press(command, 'Enter')
      await waitFor(`document.querySelector('${overlay}')`)
      await press(command, 'Escape')
      await waitFor(`!document.querySelector('${overlay}')`)
      await assertNoTooltip(evaluate)
      await pointAt(evaluate, command, trigger)
      await waitFor(`document.querySelector('[role="tooltip"]')`)
    }
  )
}

test(
  'video edit popover and scrape dialog do not reshow trigger tooltips on Escape',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(
      `window.testStore && document.querySelector('aside') && window.requests.some(request => request.url.startsWith('/videos?')) && !window.testStore.getState().loading`
    )
    await evaluate(
      `window.testStore.setState({videos:[{id:1,location_id:1,filename:'test.mp4',path:'test.mp4',tags:[]}],total:1})`
    )
    await waitFor(`document.querySelector('.video-card')`)
    for (const [label, overlay] of [
      ['Edit video', '.MuiPopover-root'],
      ['Scrape settings', '[aria-label="Scrape Settings"]'],
    ]) {
      await pointAt(evaluate, command, `[aria-label="${label}"]`, true)
      await waitFor(`document.querySelector('${overlay}')`)
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 5, y: 5 })
      await press(command, 'Escape')
      await waitFor(`!document.querySelector('${overlay}')`)
      assert.equal(await evaluate(`document.activeElement.getAttribute('aria-label')`), label)
      await assertNoTooltip(evaluate)
    }
  }
)
