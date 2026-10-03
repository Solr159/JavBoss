import assert from 'node:assert/strict'
import test from 'node:test'
import { writeFile } from 'node:fs/promises'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'resource dashboard shows metrics, recovers from errors and stops polling when inactive',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Emulation.setDeviceMetricsOverride', {
      width: 1280,
      height: 1000,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
    await evaluate(`{
      const originalFetch = window.fetch;
      window.resourceRequests = 0;
      window.resourceAborts = 0;
      window.resourceMode = 'ok';
      window.resourceWarmup = true;
      window.diskIOUnavailable = false;
      window.fetch = (input, init = {}) => {
        if (new URL(input, location.origin).pathname !== '/system/resources') return originalFetch(input, init);
        window.resourceRequests++;
        if (window.resourceMode === 'pending') return new Promise((resolve, reject) => {
          init.signal.addEventListener('abort', () => {
            window.resourceAborts++;
            reject(new DOMException('Aborted', 'AbortError'));
          }, { once: true });
        });
        if (window.resourceMode === 'error') return Promise.resolve(Response.json({error_en: 'Metrics unavailable'}, {status: 503}));
        return Promise.resolve(Response.json({
          sampled_at: new Date().toISOString(), sample_seconds: 3, os: 'linux', logical_cpus: 8, container: true,
          process: { cpu_percent: window.resourceWarmup ? null : 125.5, rss_bytes: 75497472, read_bytes_per_second: 1024, write_bytes_per_second: 2048, disk_read_bytes_per_second: window.resourceWarmup || window.diskIOUnavailable ? null : 0, disk_write_bytes_per_second: window.resourceWarmup || window.diskIOUnavailable ? null : 65536, goroutines: 32, uptime_seconds: 3660 },
          system_cpu_percent: 18.5,
          system_memory: {total_bytes: 17179869184, used_bytes: 4294967296, available_bytes: 12884901888, used_percent: 25},
          data_disk: {path: '/data/javboss', total_bytes: 1073741824000, used_bytes: 322122547200, free_bytes: 751619276800, used_percent: 30},
          unavailable: window.diskIOUnavailable ? ['process_disk_io'] : []
        }));
      };
      document.querySelector('aside button[aria-label="Settings"]').click();
    }`)
    const modal = `document.querySelector('[aria-labelledby="global-settings-title"]')`
    const button = (text) =>
      `Array.from(${modal}.querySelectorAll('button')).find(b => b.textContent.includes('${text}'))`
    await waitFor(modal)
    assert.equal(await evaluate('window.resourceRequests'), 0)
    await evaluate(`${button('Resource Monitor')}.click()`)
    await waitFor(`${modal}.textContent.includes('72.0 MiB')`)
    assert.equal(
      await evaluate(
        `${modal}.querySelector('svg[aria-label="JavBoss process CPU usage history"]') !== null`
      ),
      false
    )
    assert.equal(await evaluate(`${modal}.textContent.includes('Pause updates')`), false)
    assert.equal(
      await evaluate(`${modal}.textContent.includes('Live usage on the JavBoss server')`),
      false
    )
    assert.equal(await evaluate(`${modal}.textContent.includes('System disk I/O')`), false)
    const card = (title) =>
      `Array.from(${modal}.querySelectorAll('h4')).find(h => h.textContent === '${title}').parentElement`
    const diskCard = card('JavBoss disk I/O')
    const logicalCard = card('JavBoss process I/O')
    assert.ok(await evaluate(`${diskCard}.textContent.includes('—')`))
    await evaluate('window.resourceWarmup = false')
    await waitFor(`${modal}.textContent.includes('125.5%')`)
    assert.ok(
      await evaluate(
        `${diskCard}.textContent.includes('0 B/s') && ${diskCard}.textContent.includes('64.0 KiB/s')`
      )
    )
    assert.ok(
      await evaluate(
        `${logicalCard}.textContent.includes('1.0 KiB/s') && ${logicalCard}.textContent.includes('2.0 KiB/s')`
      )
    )
    assert.ok(await evaluate(`${modal}.textContent.includes('not container quotas')`))
    if (process.env.RESOURCE_SCREENSHOT) {
      const screenshot = await command('Page.captureScreenshot', { format: 'png' })
      await writeFile(
        process.env.RESOURCE_SCREENSHOT,
        Buffer.from(screenshot.result.data, 'base64')
      )
    }

    await evaluate('window.diskIOUnavailable = true')
    await waitFor(`${diskCard}.textContent.includes('Disk I/O unavailable')`)
    assert.ok(
      await evaluate(
        `${diskCard}.textContent.includes('—') && !${diskCard}.textContent.includes('0 B/s')`
      )
    )

    await evaluate(`window.resourceMode = 'error'`)
    await waitFor(`${modal}.textContent.includes('Metrics unavailable')`)
    assert.ok(await evaluate(`${modal}.textContent.includes('Showing the last successful sample')`))
    await evaluate(`window.resourceMode = 'ok'`)
    await waitFor(`!${modal}.querySelector('[role="alert"]')`)

    // Hide the document with a request in flight; it must be canceled, and no
    // further requests should start until the document becomes visible again.
    await evaluate(
      `window.resourceMode = 'pending'; window.beforePending = window.resourceRequests`
    )
    await waitFor('window.resourceRequests > window.beforePending')
    await evaluate(
      `Object.defineProperty(document, 'visibilityState', {configurable: true, value: 'hidden'}); document.dispatchEvent(new Event('visibilitychange'))`
    )
    await waitFor('window.resourceAborts === 1')
    const hiddenCount = await evaluate('window.resourceRequests')
    await evaluate('new Promise(resolve => setTimeout(resolve, 3200))')
    assert.equal(await evaluate('window.resourceRequests'), hiddenCount)
    await evaluate(
      `window.resourceMode = 'ok'; Object.defineProperty(document, 'visibilityState', {configurable: true, value: 'visible'}); document.dispatchEvent(new Event('visibilitychange'))`
    )
    await waitFor(`window.resourceRequests > ${hiddenCount}`)

    await command('Emulation.setDeviceMetricsOverride', {
      width: 390,
      height: 844,
      deviceScaleFactor: 1,
      mobile: true,
    })
    assert.ok(
      await evaluate(
        `${modal}.querySelector('section').scrollWidth <= ${modal}.querySelector('section').clientWidth`
      )
    )
    await evaluate(`${button('Display & Interaction')}.click()`)
    const switchedCount = await evaluate('window.resourceRequests')
    await evaluate('new Promise(resolve => setTimeout(resolve, 3200))')
    assert.equal(await evaluate('window.resourceRequests'), switchedCount)
    await evaluate(`${button('Resource Monitor')}.click()`)
    await waitFor(`window.resourceRequests > ${switchedCount}`)
    await evaluate(`${modal}.querySelector('[aria-label="Close global settings"]').click()`)
    await waitFor(`!${modal}`)
    const closedCount = await evaluate('window.resourceRequests')
    await evaluate('new Promise(resolve => setTimeout(resolve, 3200))')
    assert.equal(await evaluate('window.resourceRequests'), closedCount)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
