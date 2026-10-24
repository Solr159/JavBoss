import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'studio visibility persists independently of series in both card orientations',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-jav-card-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await waitFor('document.querySelector(".jav-card")')
    await evaluate(
      `(async () => {
        const response = await fetch('/jav/items/1', {method: 'PUT', body: JSON.stringify({studio: {id: 1, name: 'Test Studio'}, series: {id: 1, name: 'Test Series'}})});
        window.testStore.getState().patchJavItem(await response.json());
      })()`
    )
    const studio = `document.querySelector('.jav-card a[title="Test Studio"]')`
    const series = `document.querySelector('.jav-card a[title="Test Series"]')`
    const toggle = `document.querySelector('[role="switch"][aria-label="Hide studio"]')`
    const openSettings = async () => {
      await evaluate(`document.querySelector('button[aria-label="Display"]').click()`)
      await waitFor(toggle)
    }
    const click = async (label) =>
      evaluate(
        `[...document.querySelectorAll('[role="dialog"] button')].find(button => button.textContent === '${label}').click()`
      )
    await waitFor(`${studio} && ${series}`)
    for (const portrait of [false, true]) {
      await evaluate(
        `window.testStore.setState(state => ({config: {...state.config, jav_card_portrait: ${portrait}}}))`
      )
      await openSettings()
      assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'false')
      await evaluate(`${toggle}.click()`)
      await click('Save')
      await waitFor(`!${toggle} && !${studio}`)
      assert.ok(await evaluate(`Boolean(${series})`))
      assert.equal(await evaluate('window.testStore.getState().config.jav_hide_studio'), true)
      await openSettings()
      assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
      await click('Restore defaults')
      await click('Cancel')
      await waitFor(`!${toggle}`)
      assert.equal(await evaluate(`Boolean(${studio})`), false)
      await openSettings()
      await click('Restore defaults')
      await click('Save')
      await waitFor(`!${toggle} && ${studio}`)
      assert.equal(await evaluate('window.testStore.getState().config.jav_hide_studio'), false)
    }
    await evaluate(
      `window.testStore.setState(state => ({config: {...state.config, jav_card_portrait: true}}))`
    )
    await waitFor(series)
    await evaluate(`${series}.parentElement.parentElement.dataset.testMetadataRow = 'true'`)
    await evaluate(
      `window.testStore.setState(state => ({config: {...state.config, jav_hide_studio: true, jav_hide_series: true}}))`
    )
    await waitFor(`!${studio} && !${series}`)
    assert.equal(
      await evaluate(`Boolean(document.querySelector('[data-test-metadata-row]'))`),
      false
    )
  }
)

test(
  'JAV portrait cards crop the right side and persist the display setting',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-jav-portrait-test',
      plugins: [
        {
          name: 'portrait-cover-fixture',
          configureServer(server) {
            server.middlewares.use('/jav/ABC-001/cover', (_request, response) => {
              response.setHeader('Content-Type', 'image/svg+xml')
              response.end(
                '<svg xmlns="http://www.w3.org/2000/svg" width="800" height="538"><path fill="red" d="M0 0h424v538H0z"/><path fill="blue" d="M424 0h376v538H424z"/></svg>'
              )
            })
          },
        },
      ],
    })
    await command('Emulation.setDeviceMetricsOverride', {
      width: 1280,
      height: 900,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    const frame = `document.querySelector('.jav-card .card-hover-scope')`
    const img = `${frame}.querySelector('img')`
    const select = `document.querySelector('select[aria-label="Cover display mode"]')`
    await waitFor(frame)
    await waitFor(`${img}?.naturalWidth === 800`)
    assert.equal(await evaluate(`getComputedStyle(${img}).objectFit`), 'cover')
    // An explicitly saved landscape preference still takes precedence over the default.
    await evaluate(
      `window.testStore.setState(state => ({config: {...state.config, jav_card_portrait: false}}))`
    )
    await waitFor(`getComputedStyle(${img}).objectFit === 'contain'`)
    const landscapeWidth = await evaluate(`${frame}.getBoundingClientRect().width`)
    assert.equal(await evaluate(`getComputedStyle(${img}).objectFit`), 'contain')
    const cropButton = `document.querySelector('.jav-card button[aria-label="Adjust cover"]')`
    assert.equal(await evaluate(`Boolean(${cropButton})`), false)
    const openSettings = async () => {
      await evaluate(`document.querySelector('button[aria-label="Display"]').click()`)
      await waitFor(select)
    }
    const clickDialogButton = async (text) => {
      await evaluate(
        `[...document.querySelectorAll('[role="dialog"] button')].find(button => button.textContent === '${text}').click()`
      )
    }
    await openSettings()
    assert.equal(await evaluate(`${select}.value`), 'landscape')
    await evaluate(
      `${select}.value = 'portrait'; ${select}.dispatchEvent(new Event('change', {bubbles: true}))`
    )
    await clickDialogButton('Save')
    await waitFor(`!${select} && getComputedStyle(${img}).objectFit === 'cover'`)
    assert.equal(await evaluate('window.testStore.getState().config.jav_card_portrait'), true)
    assert.equal(await evaluate(`getComputedStyle(${img}).objectPosition`), '100% 50%')
    const dimensions = await evaluate(`(() => {
      const rect = ${frame}.getBoundingClientRect();
      return {width: rect.width, height: rect.height};
    })()`)
    assert.ok(dimensions.width < landscapeWidth)
    assert.ok(Math.abs(dimensions.width / dimensions.height - 376 / 538) < 0.001)
    // The crop editor belongs immediately to the left of the full-cover preview.
    assert.equal(
      await evaluate(`${cropButton}.nextElementSibling.getAttribute('aria-label')`),
      'View cover'
    )
    const cropDialog = `document.querySelector('[role="dialog"][aria-label="Adjust cover"]')`
    const slider = `${cropDialog}.querySelector('input[type="range"]')`
    const openCrop = async () => {
      await evaluate(`${cropButton}.click()`)
      await waitFor(`${slider} && !${slider}.disabled`)
    }
    const setCrop = async (value) => {
      await evaluate(
        `Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(${slider}, '${value}'); ${slider}.dispatchEvent(new Event('input', {bubbles: true}))`
      )
      await waitFor(`${slider}.value === '${value}'`)
    }
    await openCrop()
    assert.equal(await evaluate(`${slider}.value`), '1')
    await setCrop(0)
    await clickDialogButton('Cancel')
    await waitFor(`!${cropDialog}`)
    assert.equal(await evaluate(`getComputedStyle(${img}).objectPosition`), '100% 50%')
    await openCrop()
    assert.equal(await evaluate(`${slider}.value`), '1')
    // Pointer movement can place the crop at the left edge.
    const point = await evaluate(`(() => {
      const rect = ${cropDialog}.querySelector('img').getBoundingClientRect();
      return {x: rect.left + 2, y: rect.top + rect.height / 2};
    })()`)
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
    await waitFor(`${slider}.value === '0'`)
    await setCrop(0.25)
    // A failed save leaves the draft available for retry.
    await evaluate(
      `window.cropFetch = window.fetch; window.fetch = (input, init) => init?.method === 'PUT' ? Promise.resolve(Response.json({error: 'Crop save failed'}, {status: 500})) : window.cropFetch(input, init)`
    )
    await clickDialogButton('Save')
    await waitFor(`${cropDialog}.querySelector('[role="alert"]')`)
    assert.equal(await evaluate(`${slider}.value`), '0.25')
    await evaluate('window.fetch = window.cropFetch')
    await clickDialogButton('Save')
    await waitFor(`!${cropDialog} && getComputedStyle(${img}).objectPosition === '25% 50%'`)
    assert.equal(
      await evaluate('window.testStore.getState().javItems[0].cover_crop_position'),
      0.25
    )
    assert.equal(
      await evaluate(
        `(async () => (await (await fetch('/jav/items/1')).json()).cover_crop_position)()`
      ),
      0.25
    )
    await openCrop()
    assert.equal(await evaluate(`${slider}.value`), '0.25')
    await clickDialogButton('Reset')
    await clickDialogButton('Save')
    await waitFor(`!${cropDialog} && getComputedStyle(${img}).objectPosition === '100% 50%'`)
    // The setting is restored when reopening; cancelling a change keeps portrait mode.
    await openSettings()
    assert.equal(await evaluate(`${select}.value`), 'portrait')
    await evaluate(
      `${select}.value = 'landscape'; ${select}.dispatchEvent(new Event('change', {bubbles: true}))`
    )
    await clickDialogButton('Cancel')
    await waitFor(`!${select}`)
    assert.equal(await evaluate(`getComputedStyle(${img}).objectFit`), 'cover')
    await openSettings()
    await evaluate(
      `${select}.value = 'landscape'; ${select}.dispatchEvent(new Event('change', {bubbles: true}))`
    )
    await clickDialogButton('Save')
    await waitFor(`!${select} && getComputedStyle(${img}).objectFit === 'contain'`)
    assert.equal(await evaluate('window.testStore.getState().config.jav_card_portrait'), false)
    assert.equal(await evaluate(`Boolean(${cropButton})`), false)
    assert.equal(await evaluate(`${frame}.getBoundingClientRect().width`), landscapeWidth)
    await openSettings()
    assert.equal(await evaluate(`${select}.value`), 'landscape')
    await clickDialogButton('Restore defaults')
    await waitFor(`${select}.value === 'portrait'`)
    await clickDialogButton('Save')
    await waitFor(`!${select} && getComputedStyle(${img}).objectFit === 'cover'`)
    assert.equal(await evaluate('window.testStore.getState().config.jav_card_portrait'), true)
  }
)

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
      return {right: icons.right, rowRight: row.right, centerY: icons.top + icons.height / 2, rowCenterY: row.top + row.height / 2};
    })()`)
    assert.equal(layout.right, layout.rowRight)
    assert.equal(layout.centerY, layout.rowCenterY)
    await evaluate(`${indicator}.scrollIntoView({ block: 'center' })`)
    const point = await evaluate(`(() => {
      const rect = ${indicator}.getBoundingClientRect();
      return {x: rect.x + rect.width / 2, y: rect.y + rect.height / 2};
    })()`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
    await waitFor('document.querySelector(".MuiTooltip-tooltip")')
    assert.equal(
      await evaluate(`document.querySelector('.MuiTooltip-tooltip').textContent`),
      await evaluate(`${indicator}.getAttribute('aria-label')`)
    )
    assert.equal(
      await evaluate('getComputedStyle(document.querySelector(".MuiTooltip-tooltip")).transform'),
      'none'
    )
    await evaluate(`window.testStore.setState(state => ({
      config: {...state.config, jav_watch_time_icon_minutes: 60},
      javItems: state.javItems.map(item => ({...item, watched_ms: 2700000}))
    }))`)
    await waitFor(
      `getComputedStyle(${indicator}.querySelector('svg:last-child')).clipPath === 'inset(0px 25% 0px 0px)'`
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
