import assert from 'node:assert/strict'
import test from 'node:test'
import { mkdir, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'portrait covers crop the exact right slice within tolerance and settings persist, cancel and reset',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const dimensions = {
      landscape: [1500, 1000],
      min: [1350, 1000],
      max: [1650, 1000],
      below: [1349, 1000],
      above: [1651, 1000],
      portrait: [260, 367],
      square: [1000, 1000],
    }
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-jav-portrait-test',
      plugins: [
        {
          name: 'jav-portrait-test-covers',
          configureServer(server) {
            server.middlewares.use((req, res, next) => {
              const match = req.url?.match(/^\/jav\/([^/]+)\/cover/)
              if (!match) return next()
              const size = dimensions[match[1]]
              if (!size) {
                res.statusCode = 404
                return res.end()
              }
              const [width, height] = size
              const cut = (width * 11) / 21
              res.setHeader('Content-Type', 'image/svg+xml')
              res.end(
                `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">
                  <rect width="${width}" height="${height}" fill="#2563eb"/>
                  <rect width="${cut}" height="${height}" fill="#f59e0b"/>
                  <text x="${cut / 2}" y="${height / 2}" text-anchor="middle" fill="white" font-size="80">LEFT 11/21</text>
                  <text x="${cut + (width - cut) / 2}" y="${height / 2}" text-anchor="middle" fill="white" font-size="80">RIGHT 10/21</text>
                </svg>`
              )
            })
          },
        },
      ],
    })
    await command('Emulation.setDeviceMetricsOverride', {
      width: 1280,
      height: 1100,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await command('Page.enable')
    const localeScript = await command('Page.addScriptToEvaluateOnNewDocument', {
      source: `
        Object.defineProperty(navigator, 'languages', { value: ['zh-CN'] });
        Object.defineProperty(navigator, 'language', { value: 'zh-CN' });
      `,
    })
    assert.ok(!localeScript.error, JSON.stringify(localeScript))
    await command('Page.navigate', { url: `${origin}/tests/fixtures/javPortrait.html` })
    await waitFor('window.openSettings && document.querySelectorAll(".jav-card").length === 8')
    await waitFor(
      'Array.from(document.querySelectorAll(".jav-card img")).every((img) => img.complete)'
    )
    const covers = () =>
      evaluate(`Array.from(document.querySelectorAll('.jav-card')).map((card) => {
        const cover = card.querySelector('.card-hover-scope');
        const img = cover.querySelector('img');
        const imageRect = img.getBoundingClientRect();
        const frameRect = img.parentElement.getBoundingClientRect();
        const box = cover.getBoundingClientRect();
        return {
          code: img.alt, ratio: box.width / box.height,
          fit: getComputedStyle(img).objectFit,
          visibleFraction: frameRect.width / imageRect.width,
          rightOffset: imageRect.right - frameRect.right,
          renderedRatio: imageRect.width / imageRect.height,
          sourceRatio: img.naturalWidth / img.naturalHeight,
          cropped: img.style.objectFit === 'fill'
        };
      })`)
    for (const cover of await covers()) {
      assert.ok(Math.abs(cover.ratio - 800 / 538) < 0.001)
      assert.equal(cover.fit, 'contain')
    }
    await evaluate('window.openSettings()')
    await waitFor('document.querySelector("[role=switch][aria-label=竖图模式]")')
    assert.equal(
      await evaluate(
        `document.querySelector('section').querySelector('[role=switch]').getAttribute('aria-label')`
      ),
      '竖图模式'
    )
    const clickButton = (label) =>
      evaluate(
        `Array.from(document.querySelectorAll('button')).find((button) => button.textContent.trim() === ${JSON.stringify(label)}).click()`
      )
    const toggleMode = () =>
      evaluate('document.querySelector("[role=switch][aria-label=竖图模式]").click()')
    await toggleMode()
    assert.equal(await evaluate('window.readConfig().jav_portrait_mode'), false)
    await evaluate('document.querySelector("[aria-label=关闭设置]").click()')
    await waitFor('!document.querySelector("[role=dialog]")')
    await evaluate('window.openSettings()')
    await waitFor('document.querySelector("[role=switch][aria-label=竖图模式]")')
    assert.equal(
      await evaluate(
        'document.querySelector("[role=switch][aria-label=竖图模式]").getAttribute("aria-checked")'
      ),
      'false'
    )
    await toggleMode()
    if (process.env.JAV_PORTRAIT_SCREENSHOTS) {
      await mkdir(process.env.JAV_PORTRAIT_SCREENSHOTS, { recursive: true })
      const capture = await command('Page.captureScreenshot', { format: 'png' })
      await writeFile(
        path.join(process.env.JAV_PORTRAIT_SCREENSHOTS, 'settings.png'),
        Buffer.from(capture.result.data, 'base64')
      )
    }
    await clickButton('保存')
    await waitFor(
      'window.readConfig().jav_portrait_mode && !document.querySelector("[role=dialog]")'
    )
    await waitFor('document.querySelector(".jav-card img").style.objectFit === "fill"')
    const enabled = await covers()
    for (const cover of enabled) {
      assert.ok(Math.abs(cover.ratio - 0.7) < 0.001, cover.code)
      const shouldCrop = ['landscape', 'min', 'max'].includes(cover.code)
      assert.equal(cover.cropped, shouldCrop, cover.code)
      if (shouldCrop) {
        assert.ok(Math.abs(cover.visibleFraction - 10 / 21) < 0.001, cover.code)
        assert.ok(Math.abs(cover.rightOffset) < 0.1, cover.code)
        assert.ok(Math.abs(cover.renderedRatio - cover.sourceRatio) < 0.001, cover.code)
      } else {
        assert.equal(cover.fit, 'contain', cover.code)
      }
    }
    if (process.env.JAV_PORTRAIT_SCREENSHOTS) {
      const capture = await command('Page.captureScreenshot', { format: 'png' })
      await writeFile(
        path.join(process.env.JAV_PORTRAIT_SCREENSHOTS, 'covers.png'),
        Buffer.from(capture.result.data, 'base64')
      )
    }
    await evaluate('window.reloadConfig()')
    await waitFor('window.readConfig().jav_portrait_mode')
    await evaluate('window.changeCoverSource()')
    await waitFor('document.querySelector(".jav-card img").naturalWidth === 260')
    assert.equal((await covers())[0].cropped, false)
    await evaluate('window.changeCoverSource()')
    await waitFor(
      'document.querySelector(".jav-card img").naturalWidth === 1500 && document.querySelector(".jav-card img").style.objectFit === "fill"'
    )
    await evaluate('window.openSettings()')
    await waitFor('document.querySelector("[role=switch][aria-label=竖图模式]")')
    await clickButton('恢复默认')
    assert.equal(
      await evaluate(
        'document.querySelector("[role=switch][aria-label=竖图模式]").getAttribute("aria-checked")'
      ),
      'false'
    )
    await clickButton('保存')
    await waitFor(
      '!window.readConfig().jav_portrait_mode && !document.querySelector("[role=dialog]")'
    )
    for (const cover of await covers()) {
      assert.ok(Math.abs(cover.ratio - 800 / 538) < 0.001)
      assert.equal(cover.cropped, false)
    }
    assert.equal(await evaluate('window.settingsError'), undefined)
  }
)
