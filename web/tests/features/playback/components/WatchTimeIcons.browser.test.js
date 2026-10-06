import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  '19-second fills remain larger than 16-second fills at fractional card positions',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-watch-time-raster-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await waitFor('window.testStore && document.querySelector(".jav-card")')
    await evaluate(`window.testStore.setState(state => ({
      javItems: state.javItems.map(item => ({...item, watched_ms: 16000}))
    }))`)
    await waitFor('document.querySelector(".watch-time-icons")')

    for (const scale of [1, 1.25, 2]) {
      await command('Emulation.setDeviceMetricsOverride', {
        width: 800,
        height: 600,
        deviceScaleFactor: scale,
        mobile: false,
      })
      // Copy the rendered icon to isolate rasterization from card content.
      // The first row is an unfilled baseline at the same fractional positions.
      await evaluate(`(() => {
        document.getElementById('watch-raster-samples')?.remove();
        const original = document.querySelector('.watch-time-icons > span');
        const sheet = document.createElement('div');
        sheet.id = 'watch-raster-samples';
        sheet.style.cssText = 'position:fixed;inset:0;background:white;z-index:99999';
        [0, 16000, 19000].forEach((ms, row) => {
          for (let column = 0; column < 8; column++) {
            const icon = original.cloneNode(true);
            Object.assign(icon.style, {
              position: 'absolute',
              left: (40 + column * 60 + column / 8) + 'px',
              top: (40 + row * 40 + column / 8) + 'px',
            });
            icon.querySelector('svg:last-child').style.clipPath =
              'inset(0 ' + (100 - ms / 1800000 * 100) + '% 0 0)';
            sheet.append(icon);
          }
        });
        document.body.append(sheet);
      })()`)
      await evaluate(
        'new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))'
      )
      const screenshot = await command('Page.captureScreenshot', { format: 'png' })
      assert.ok(screenshot.result?.data, JSON.stringify(screenshot.error))
      const samples = await evaluate(`(async () => {
        const image = new Image();
        image.src = ${JSON.stringify(`data:image/png;base64,${screenshot.result.data}`)};
        await image.decode();
        const canvas = document.createElement('canvas');
        canvas.width = image.width;
        canvas.height = image.height;
        const context = canvas.getContext('2d');
        context.drawImage(image, 0, 0);
        const pixels = context.getImageData(0, 0, image.width, image.height).data;
        const red = (x, y) => pixels[(y * image.width + x) * 4];
        return Array.from({length: 8}, (_, column) => {
          const x = Math.floor((40 + column * 60) * ${scale});
          const y = 40 * ${scale};
          return [1, 2].map(row => {
            let darkCoverage = 0;
            for (let px = x - 2; px < x + 20 * ${scale}; px++) {
              for (let py = y - 2; py < y + 20 * ${scale}; py++) {
                darkCoverage += red(px, py) - red(px, py + row * 40 * ${scale});
              }
            }
            return darkCoverage;
          });
        });
      })()`)
      const max16 = Math.max(...samples.map(([shorter]) => shorter))
      const min19 = Math.min(...samples.map(([, longer]) => longer))
      assert.ok(max16 > 0 && min19 > max16, `scale=${scale}; coverage=${JSON.stringify(samples)}`)
    }
  }
)
