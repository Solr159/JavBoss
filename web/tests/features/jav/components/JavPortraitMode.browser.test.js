import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'portrait mode saves, positions covers like idol cards, and resets to landscape',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      plugins: [
        {
          name: 'portrait-test-cover',
          configureServer(server) {
            server.middlewares.use((req, res, next) => {
              const code = req.url.match(
                /^\/jav\/(ABC-001|PORTRAIT-001|SQUARE-001|WIDE-001|NARROW-LANDSCAPE-001)\/cover/
              )
              if (!code) return next()
              const [width, height] = {
                'ABC-001': [800, 538],
                'PORTRAIT-001': [680, 1000],
                'SQUARE-001': [1000, 1000],
                'WIDE-001': [1650, 1000],
                'NARROW-LANDSCAPE-001': [1200, 1000],
              }[code[1]]
              res.setHeader('Content-Type', 'image/svg+xml')
              res.end(
                `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}"><path fill="red" d="M0 0h1650v1000H0z"/><path fill="blue" d="M424 0H1650v1000H424z"/></svg>`
              )
            })
          },
        },
      ],
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    const image = `document.querySelector('.jav-card img')`
    const frame = `document.querySelector('.jav-card .card-hover-scope')`
    const modal = `document.querySelector('[aria-labelledby="jav-display-settings-title"]')`
    const toggle = `${modal}.querySelector('[role="switch"][aria-label="Portrait cover mode"], [role="switch"][aria-label="竖图模式"]')`
    const display = `document.querySelector('aside button[aria-label="Display"], aside button[aria-label="显示"]')`
    const button = (text) => {
      const chinese = { Cancel: '取消', Save: '保存', 'Restore defaults': '恢复默认' }[text]
      return `Array.from(${modal}.querySelectorAll('button')).find(b => ['${text}', '${chinese}'].includes(b.textContent))`
    }
    await waitFor(`${image}?.naturalWidth === 800`)
    await evaluate(`${display}.click()`)
    await waitFor(toggle)
    assert.ok(
      ['Portrait cover mode', '竖图模式'].includes(
        await evaluate(
          `${modal}.querySelector('section').querySelector('[role="switch"]').getAttribute('aria-label')`
        )
      )
    )
    await evaluate(`${toggle}.click(); ${button('Cancel')}.click()`)
    await waitFor(`!${modal}`)
    assert.equal(
      await evaluate('Boolean(window.testStore.getState().config.jav_portrait_mode)'),
      false
    )
    await evaluate(`${display}.click()`)
    await waitFor(toggle)
    await evaluate(`${toggle}.click()`)
    await waitFor(`${toggle}.getAttribute('aria-checked') === 'true'`)
    await evaluate(`${button('Save')}.click()`)
    await waitFor(`!${modal} && ${image}?.classList.contains('max-w-none')`)
    const readGeometry = () =>
      evaluate(`(() => {
      const image = ${image}.getBoundingClientRect();
      const crop = ${image}.parentElement.getBoundingClientRect();
      const frame = ${frame}.getBoundingClientRect();
      return { frameRatio: frame.width / frame.height, imageRatio: image.width / image.height,
        cropFraction: crop.width / image.width, rightEdge: image.right - crop.right,
        cropLeft: (crop.left - image.left) / image.width,
        centerOffset: (image.left + image.right - crop.left - crop.right) / 2,
        heightDifference: image.height - frame.height };
    })()`)
    const geometry = await readGeometry()
    const frameRatio = 376 / 538
    assert.ok(Math.abs(geometry.frameRatio - frameRatio) < 0.001)
    assert.ok(Math.abs(geometry.imageRatio - 800 / 538) < 0.001)
    assert.ok(Math.abs(geometry.cropFraction - 0.47) < 0.001)
    assert.ok(Math.abs(geometry.cropLeft - 0.53) < 0.001)
    assert.ok(Math.abs(geometry.rightEdge) < 1 && Math.abs(geometry.heightDifference) < 1)
    for (const [code, width, height] of [
      ['PORTRAIT-001', 680, 1000],
      ['SQUARE-001', 1000, 1000],
      ['WIDE-001', 1650, 1000],
      ['NARROW-LANDSCAPE-001', 1200, 1000],
      ['ABC-001', 800, 538],
    ]) {
      await evaluate(
        `window.testStore.setState({ javItems: window.testStore.getState().javItems.map(item => ({ ...item, code: '${code}' })) })`
      )
      await waitFor(
        `${image}?.naturalWidth === ${width} && ${image}?.classList.contains('max-w-none')`
      )
      const current = await readGeometry()
      assert.ok(Math.abs(current.frameRatio - frameRatio) < 0.001)
      assert.ok(Math.abs(current.imageRatio - width / height) < 0.001)
      assert.ok(Math.abs(current.heightDifference) < 1)
      if (width / height <= frameRatio) {
        assert.ok(Math.abs(current.centerOffset) < 1)
        assert.ok(current.cropFraction >= 1)
      } else {
        const expectedLeft = Math.min(0.53, 1 - frameRatio / (width / height))
        assert.ok(Math.abs(current.cropLeft - expectedLeft) < 0.001)
        assert.ok(current.rightEdge >= -1)
      }
    }
    await evaluate(`${display}.click()`)
    await waitFor(toggle)
    assert.equal(await evaluate(`${toggle}.getAttribute('aria-checked')`), 'true')
    await evaluate(`${button('Restore defaults')}.click()`)
    await waitFor(`${toggle}.getAttribute('aria-checked') === 'false'`)
    await evaluate(`${button('Save')}.click()`)
    await waitFor(`!${modal} && !${image}?.classList.contains('max-w-none')`)
    assert.equal(await evaluate('window.testStore.getState().config.jav_portrait_mode'), false)
    assert.ok(
      Math.abs(
        (await evaluate(
          `(() => { const r = ${frame}.getBoundingClientRect(); return r.width / r.height })()`
        )) -
          800 / 538
      ) < 0.001
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
