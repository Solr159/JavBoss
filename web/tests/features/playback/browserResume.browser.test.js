import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'web player resumes on reopen/reload, isolates playlist items, and honors explicit seeks and disabled resume',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-browser-resume-test',
    })
    const installMedia = async () => {
      await waitFor(`document.querySelector('#root > button')`)
      await evaluate(`{
        const wav = new Uint8Array(44 + 8000 * 60).fill(128);
        const header = new DataView(wav.buffer);
        const text = (at, value) => [...value].forEach((letter, i) => wav[at + i] = letter.charCodeAt(0));
        text(0, 'RIFF'); header.setUint32(4, wav.length - 8, true); text(8, 'WAVEfmt ');
        header.setUint32(16, 16, true); header.setUint16(20, 1, true); header.setUint16(22, 1, true);
        header.setUint32(24, 8000, true); header.setUint32(28, 8000, true);
        header.setUint16(32, 1, true); header.setUint16(34, 8, true);
        text(36, 'data'); header.setUint32(40, wav.length - 44, true);
        const mediaURL = URL.createObjectURL(new Blob([wav], {type:'audio/wav'}));
        window.fetch = async (input) => {
          const url = new URL(input, location.origin);
          if (url.pathname.endsWith('/streams')) return Response.json({location_id: Number(url.pathname.split('/')[2]) + 10,
            preferred_kind:'direct', sources:[{kind:'direct',src:mediaURL,mime_type:'audio/wav'}]});
          return Response.json({session_id:'test'});
        };
      }`)
    }
    const player = `document.querySelector('.video-js')?.player`
    const ready = `${player}?.readyState() > 0 && !document.querySelector('[data-player-loading]')`
    const open = async (expected) => {
      await evaluate(`document.querySelector('#root > button').click()`)
      await waitFor(ready)
      await waitFor(`Math.abs(${player}.currentTime() - ${expected}) < 2`)
      await evaluate(`${player}.pause()`)
    }
    const seek = async (time) => {
      await evaluate(`${player}.pause(); ${player}.currentTime(${time})`)
      await waitFor(`Math.abs(${player}.currentTime() - ${time}) < 0.1`)
    }
    const close = async () => {
      await evaluate(`document.querySelector('button[aria-label="Close"]').click()`)
      await waitFor(`!${player}`)
    }
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
    await installMedia()
    await open(0)
    await seek(17)
    await close()
    assert.equal(await evaluate(`Number(localStorage.getItem('javboss.player.position.1.11'))`), 17)
    await open(17)
    await seek(23)
    await command('Page.reload')
    await installMedia()
    await open(23)
    await evaluate(`document.querySelector('[aria-label="Next video"]').click()`)
    await waitFor(
      `${ready} && document.querySelector('.player-window h2').textContent === 'Second video'`
    )
    assert.ok(await evaluate(`${player}.currentTime() < 2`))
    await seek(11)
    await evaluate(`document.querySelector('[aria-label="Previous video"]').click()`)
    await waitFor(`${ready} && ${player}.currentTime() >= 23`)
    await evaluate(`document.querySelector('[aria-label="Next video"]').click()`)
    await waitFor(`${ready} && ${player}.currentTime() >= 11 && ${player}.currentTime() < 13`)
    await evaluate(`${player}.pause(); ${player}.trigger('ended')`)
    await close()
    assert.equal(await evaluate(`localStorage.getItem('javboss.player.position.2.12')`), null)
    await open(0)
    await close()
    for (const [query, expected] of [
      ['resume=false', 0],
      ['start=7', 7],
      ['start=0', 0],
    ]) {
      await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html?${query}` })
      await installMedia()
      await open(expected)
      await close()
    }
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
