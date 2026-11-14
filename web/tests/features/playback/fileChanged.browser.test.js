import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'a changed file shows rescan guidance without starting playback or preview requests',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
    await waitFor(`document.querySelector('#root > button')`)
    await evaluate(`{
      window.playbackRequests = [];
      window.mediaPlayCalls = 0;
      HTMLMediaElement.prototype.play = function() {
        window.mediaPlayCalls++;
        return Promise.resolve();
      };
      window.fetch = async input => {
        window.playbackRequests.push(String(input));
        return Response.json({
          error_zh: '视频文件已修改，请重新扫描所在目录后再播放',
          error_en: 'The video file has changed. Rescan its directory before playing.'
        }, {status:409});
      };
      document.querySelector('#root > button').click();
    }`)
    await waitFor(
      `document.querySelector('[role="alert"]')?.textContent.includes('Rescan its directory before playing.')`
    )
    await waitFor(`!document.querySelector('[data-player-loading]')`)
    assert.equal(await evaluate(`window.mediaPlayCalls`), 0)
    assert.equal(await evaluate(`document.querySelector('.video-js').player.currentSrc()`), '')
    assert.equal(await evaluate(`document.querySelector('.video-js').player.controls()`), false)
    assert.equal(
      await evaluate(
        `window.playbackRequests.every(url => new URL(url, location.origin).pathname === '/videos/1/streams')`
      ),
      true
    )
    assert.deepEqual(await evaluate(`window.appErrors`), [])
  }
)
