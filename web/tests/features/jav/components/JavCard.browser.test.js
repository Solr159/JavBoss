import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'JAV watch time icons show dark half-hour fill at the right of the metadata row',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-jav-watch-time-test',
    })
    await command('Emulation.setDeviceMetricsOverride', {
      width: 1280,
      height: 900,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    const indicator = `document.querySelector('.jav-card .watch-time-icons')`
    await waitFor('document.querySelector(".jav-card")')
    assert.equal(await evaluate(`Boolean(${indicator})`), false)
    assert.equal(
      await evaluate(`Boolean(document.querySelector('.jav-card > [role="img"]'))`),
      false
    )
    const clipPaths = `[...${indicator}.querySelectorAll('svg:last-child')].map(icon => getComputedStyle(icon).clipPath)`
    for (const [watchedMs, expected] of [
      [900000, [50, 100, 100]],
      [1800000, [0, 100, 100]],
      [2700000, [0, 50, 100]],
      [3600000, [0, 0, 100]],
      [4500000, [0, 0, 50]],
      [5400000, [0, 0, 0]],
      [12600000, [0, 0, 0]],
    ]) {
      await evaluate(`window.testStore.setState(state => ({
        javItems: state.javItems.map(item => ({...item, watched_ms: ${watchedMs}}))
      }))`)
      const expectedClips = expected.map((right) => `inset(0px ${right}% 0px 0px)`)
      await waitFor(
        `${indicator} && JSON.stringify(${clipPaths}) === '${JSON.stringify(expectedClips)}'`
      )
      assert.deepEqual(await evaluate(clipPaths), expectedClips)
      assert.equal(await evaluate(`${indicator}.children.length`), 3)
    }
    assert.ok(
      await evaluate(
        `[...${indicator}.querySelectorAll('svg')].every(icon => getComputedStyle(icon).opacity === '1')`
      )
    )
    assert.match(
      await evaluate(`${indicator}.getAttribute('aria-label')`),
      /3 (小时|h).*30 (分钟|min)/
    )
    // Keep all three icons visible at the right edge, including on narrow cards.
    await evaluate(`document.querySelector('.jav-card').style.width = '260px'`)
    const layout = await evaluate(`(() => {
      const indicator = ${indicator};
      const icons = indicator.getBoundingClientRect();
      const row = indicator.parentElement.getBoundingClientRect();
      return {right: icons.right, rowRight: row.right, top: icons.top, rowTop: row.top};
    })()`)
    assert.equal(layout.right, layout.rowRight)
    assert.equal(layout.top, layout.rowTop)
    await evaluate(`${indicator}.scrollIntoView({ block: 'center' })`)
    const point = await evaluate(`(() => {
      const rect = ${indicator}.getBoundingClientRect();
      return {x: rect.x + rect.width / 2, y: rect.y + rect.height / 2};
    })()`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
    await waitFor('document.querySelector(".MuiTooltip-tooltip")')
    assert.ok(
      await evaluate(
        `document.querySelector('.MuiTooltip-tooltip').textContent.includes(${indicator}.getAttribute('aria-label'))`
      )
    )
    assert.equal(
      await evaluate('getComputedStyle(document.querySelector(".MuiTooltip-tooltip")).transform'),
      'none'
    )
    await evaluate(`window.testStore.setState(state => ({
      javItems: state.javItems.map(item => ({...item, watched_ms: 0}))
    }))`)
    await waitFor(`!${indicator}`)
  }
)

test(
  'JAV cover actions hide after mouse-opened details close with Escape',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-jav-card-test',
    })
    await command('Emulation.setDeviceMetricsOverride', {
      width: 1280,
      height: 900,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    const cover = `document.querySelector('.jav-card .card-hover-scope')`
    const trigger = `${cover}.querySelector('button')`
    const modal = `document.querySelector('[aria-labelledby="jav-detail-title-1"]')`
    // Selection stays visible on devices reporting hover:none (including headless Chrome).
    const actions = `${cover}.querySelectorAll('.card-hover-focus-visible:not([role="checkbox"])')`
    const actionsHaveOpacity = (opacity) =>
      `[...${actions}].every(el => getComputedStyle(el).opacity === '${opacity}')`
    const pressKey = async (key, code, windowsVirtualKeyCode) => {
      await command('Input.dispatchKeyEvent', {
        type: 'keyDown',
        key,
        code,
        windowsVirtualKeyCode,
        text: key === 'Enter' ? '\r' : undefined,
      })
      await command('Input.dispatchKeyEvent', { type: 'keyUp', key, code, windowsVirtualKeyCode })
    }
    await waitFor(cover)
    await waitFor(trigger)
    const point = await evaluate(`(() => {
      const rect = ${trigger}.getBoundingClientRect();
      return {x: rect.x + rect.width / 4, y: rect.y + rect.height / 2};
    })()`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
    await waitFor(actionsHaveOpacity(1))
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
    await waitFor(modal)
    // Keep the pointer stationary throughout opening and closing the dialog.
    await pressKey('Escape', 'Escape', 27)
    await waitFor(`!${modal}`)
    await waitFor(`document.activeElement === ${trigger}`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 1200, y: 850 })
    await waitFor(`!${cover}.matches(':hover')`)
    await waitFor(actionsHaveOpacity(0))
    assert.equal(await evaluate(`document.activeElement === ${trigger}`), true)

    // Keyboard users can still reopen details and reach the hidden cover actions.
    await pressKey('Enter', 'Enter', 13)
    await waitFor(modal)
    await pressKey('Escape', 'Escape', 27)
    await waitFor(`!${modal} && document.activeElement === ${trigger}`)
    assert.equal(await evaluate(`${trigger}.matches(':focus-visible')`), true)
    assert.equal(await evaluate(`getComputedStyle(${trigger}).outlineStyle`), 'none')
    await pressKey('Tab', 'Tab', 9)
    await waitFor(
      `${cover}.contains(document.activeElement) && document.activeElement !== ${trigger}`
    )
    await waitFor(actionsHaveOpacity(1))
  }
)
