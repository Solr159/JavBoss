import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'directory tools preview, select and clean junk with confirmation and partial results',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/directoryCleanup.html` })
    const button = (label) =>
      `[...document.querySelectorAll('button')].find(b => b.textContent === ${JSON.stringify(label)})`
    await waitFor(`document.querySelector('button[aria-label="Tools"]')`)
    await evaluate(`document.querySelector('button[aria-label="Tools"]').click()`)
    await waitFor(`document.querySelector('#directory-process-cleanup')`)
    await evaluate(`document.querySelector('#directory-process-cleanup').click()`)
    await evaluate(`${button('Next')}.click()`)
    await waitFor(`document.querySelector('#directory-cleanup-title')`)
    assert.equal(await evaluate('window.cleanupRequests.length'), 0)
    assert.equal(
      await evaluate(`document.querySelector('input[aria-label="System junk"]').checked`),
      true
    )
    assert.equal(await evaluate(`${button('Clean selected files')}.disabled`), true)
    await evaluate(`window.holdPreview = true; ${button('Scan junk files')}.click()`)
    await waitFor('window.finishPreview')
    assert.deepEqual(await evaluate('window.cleanupRequests.at(-1).payload'), {
      include_misc: true,
      include_torrents: true,
      include_shortcuts: true,
      include_ad_images: true,
      include_downloads: false,
      include_system: true,
    })
    assert.equal(
      await evaluate(`document.querySelector('button[aria-label="Close"]').disabled`),
      true
    )
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'Escape',
      code: 'Escape',
      windowsVirtualKeyCode: 27,
    })
    assert.ok(await evaluate(`document.querySelector('#directory-cleanup-title') !== null`))
    await evaluate('window.finishPreview(); window.holdPreview = false')
    await waitFor(`document.querySelector('input[aria-label="ad.txt"]')`)
    assert.equal(
      await evaluate(`document.querySelector('input[aria-label="ad.txt"]').checked`),
      false
    )
    await evaluate(`document.querySelector('input[aria-label="ad.txt"]').click()`)
    await evaluate(`${button('Clean selected files')}.click()`)
    assert.equal(
      await evaluate(`window.cleanupRequests.filter(r => r.path.endsWith('/cleanup')).length`),
      0
    )
    await evaluate(
      `window.confirmCleanup = true; document.querySelector('input[aria-label="Select all"]').click()`
    )
    await evaluate(`${button('Clean selected files')}.click()`)
    await waitFor(`document.body.textContent.includes('Deleted 1 files')`)
    assert.deepEqual(await evaluate(`window.cleanupRequests.at(-1).payload`), {
      root_path: '/videos',
      files: [
        { path: 'ad.txt', size: 12, modified: 'snapshot-1', reason: 'misc' },
        { path: 'nested/Thumbs.db', size: 0, modified: 'snapshot-2', reason: 'system' },
        { path: 'ad.torrent', size: 24, modified: 'snapshot-3', reason: 'torrent' },
      ],
    })
    assert.equal(
      await evaluate(`document.querySelector('input[aria-label="ad.txt"]') === null`),
      true
    )
    assert.ok(await evaluate(`document.body.textContent.includes('File changed')`))
    assert.equal(await evaluate(`${button('Clean selected files')}.disabled`), true)
    await evaluate(`document.querySelector('input[aria-label="Download leftovers"]').click()`)
    await evaluate(`document.querySelector('input[aria-label="System junk"]').click()`)
    assert.equal(
      await evaluate(`document.body.textContent.includes('Possibly damaged MP4')`),
      false
    )
    assert.equal(await evaluate(`document.body.textContent.includes('review individually')`), false)
    assert.equal(
      await evaluate(`document.querySelector('input[aria-label="ad.torrent"]') === null`),
      true
    )
    await evaluate(`${button('Scan junk files')}.click()`)
    await waitFor(`document.querySelector('input[aria-label="unfinished.xltd"]')`)
    assert.equal(await evaluate('window.cleanupRequests.at(-1).payload.include_system'), false)
    assert.equal(
      await evaluate(`document.querySelector('input[aria-label="nested/Thumbs.db"]') === null`),
      true
    )
    await evaluate(`document.querySelector('input[aria-label="Select all"]').click()`)
    for (const name of ['ad.txt', 'ad.torrent', 'unfinished.xltd']) {
      assert.equal(
        await evaluate(`document.querySelector('input[aria-label="${name}"]').checked`),
        true
      )
    }
    await evaluate(`document.querySelector('input[aria-label="Select all"]').click()`)
    for (const name of ['ad.txt', 'ad.torrent', 'unfinished.xltd']) {
      assert.equal(
        await evaluate(`document.querySelector('input[aria-label="${name}"]').checked`),
        false
      )
    }
    assert.equal(await evaluate(`${button('Clean selected files')}.disabled`), true)
    await evaluate(`document.querySelector('input[aria-label="Select all"]').click()`)
    await evaluate(`${button('Clean selected files')}.click()`)
    await waitFor(`window.cleanupRequests.at(-1).path.endsWith('/cleanup')`)
    assert.deepEqual(
      await evaluate(`window.cleanupRequests.at(-1).payload.files.map(f => f.path)`),
      ['ad.txt', 'ad.torrent', 'unfinished.xltd']
    )
    await evaluate(`window.failCleanup = true; ${button('Scan again')}.click()`)
    await waitFor(`document.querySelector('[role="alert"]')?.textContent === 'Directory is busy'`)
    assert.equal(await evaluate(`${button('Clean selected files')}.disabled`), true)

    // A large result set must scroll inside the body without moving the header/footer.
    await evaluate(`window.failCleanup = false;
      window.extraCleanupFiles = Array.from({length: 100}, (_, index) => ({
        path: 'nested/long-directory-name-' + index + '/advertisement.txt',
        size: 12, modified: 'snapshot-extra-' + index, reason: 'misc'
      }));
      ${button('Scan junk files')}.click()`)
    await waitFor(
      `document.querySelector('input[aria-label="nested/long-directory-name-99/advertisement.txt"]')`
    )
    for (const [width, height] of [
      [1280, 720],
      [390, 640],
      [1024, 480],
    ]) {
      await command('Emulation.setDeviceMetricsOverride', {
        width,
        height,
        deviceScaleFactor: 1,
        mobile: false,
      })
      const layout = await evaluate(`(() => {
        const dialog = document.querySelector('[aria-labelledby="directory-cleanup-title"]');
        const close = dialog.querySelector('button[aria-label="Close"]');
        const header = dialog.querySelector('header');
        const footer = dialog.querySelector('footer');
        const bounds = dialog.getBoundingClientRect();
        const closeBounds = close.getBoundingClientRect();
        const scrollers = [...dialog.querySelectorAll('*')].filter(el =>
          ['auto', 'scroll'].includes(getComputedStyle(el).overflowY) && el.scrollHeight > el.clientHeight + 1);
        const before = [header.getBoundingClientRect().top, footer.getBoundingClientRect().bottom];
        scrollers[0].scrollTop = scrollers[0].scrollHeight;
        return {
          contained: bounds.top >= 15 && bounds.bottom <= innerHeight - 15 && bounds.left >= 15 && bounds.right <= innerWidth - 15,
          dialogOverflow: dialog.scrollHeight > dialog.clientHeight + 1,
          horizontalOverflow: dialog.scrollWidth > dialog.clientWidth + 1,
          scrollers: scrollers.length,
          scrolled: scrollers[0].scrollTop > 0,
          fixed: before[0] === header.getBoundingClientRect().top && before[1] === footer.getBoundingClientRect().bottom,
          closeAtTopRight: header.contains(close) && closeBounds.right > bounds.right - 50 && closeBounds.top < bounds.top + 30,
          footerClose: footer.querySelector('button[aria-label="Close"]') !== null,
        };
      })()`)
      assert.deepEqual(
        layout,
        {
          contained: true,
          dialogOverflow: false,
          horizontalOverflow: false,
          scrollers: 1,
          scrolled: true,
          fixed: true,
          closeAtTopRight: true,
          footerClose: false,
        },
        `${width}x${height}`
      )
    }
    await evaluate(`document.querySelector('button[aria-label="Close"]').click()`)
    await waitFor(`!document.querySelector('#directory-cleanup-title')`)
  }
)
