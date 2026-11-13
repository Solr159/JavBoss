import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'browser loads cache automatically with downloads off and supports manual downloads and offset editing',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
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
      window.subtitleRequests = [];
      window.holdSubtitleLookup = true;
      window.cachedSubtitles = false;
      window.subtitleSettings = {offset_ms: 0};
      window.settingsWrites = [];
      window.visibilityWrites = [];
      window.streamRequests = 0;
      window.fetch = async (input, init = {}) => {
        const url = new URL(input, location.origin);
        if (url.pathname.endsWith('/streams')) {
          window.streamRequests++;
          return Response.json({location_id:11,
            preferred_kind:'direct', sources:[{kind:'direct',src:mediaURL,mime_type:'audio/wav'}]});
        }
        if (url.pathname.endsWith('/subtitle-settings')) {
          if (init.method === 'PATCH') {
            if (window.holdSettingsSave) await new Promise(resolve => { window.finishSettingsSave = resolve; });
            if (window.settingsFailure) return Response.json({error:'Save failed'}, {status:500});
            const patch = JSON.parse(init.body);
            window.subtitleSettings = {...window.subtitleSettings, ...patch};
            if ('offset_ms' in patch) window.settingsWrites.push(patch);
            if ('track' in patch) window.visibilityWrites.push(patch.track);
          }
          return Response.json(window.subtitleSettings);
        }
        if (url.pathname.endsWith('/subtitles/resolve')) {
          const request = JSON.parse(init.body);
          window.subtitleRequests.push(request);
          if (window.holdSubtitleLookup) await new Promise(resolve => { window.finishSubtitleLookup = resolve; });
          if (url.pathname.includes('/2/')) return new Response(null, {status:204,headers:{'X-Subtitle-Status':request.manual ? 'not_found' : 'disabled'}});
          const source = window.localSubtitles && !request.manual && !request.download_only ? 'local' : 'test';
          if (source !== 'local' && !window.cachedSubtitles && (request.cache_only || (!request.manual && !window.freshDownload))) return new Response(null, {status:204,headers:{'X-Subtitle-Status':'disabled'}});
          if (source !== 'local') window.cachedSubtitles = true;
          const offset = window.subtitleSettings.offset_ms;
          const timing = offset < 0 ? '00:00:00.000 --> 00:00:57.500' : offset > 0 ? '00:00:30.500 --> 00:01:29.500' : '00:00:00.000 --> 00:00:59.000';
          return new Response('WEBVTT\\n\\n1\\n' + timing + '\\nSample subtitle\\n',
            {headers:{'Content-Type':'text/vtt','X-Subtitle-Language':'zh-CN','X-Subtitle-Source':source,'X-Subtitle-Key':source === 'local' ? 'local:video.zh-CN.srt' : 'download:zh-CN','X-Subtitle-Downloaded':String(window.freshDownload === true)}});
        }
        return Response.json({session_id:'test'});
      };
      document.querySelector('#root > button').click();
    }`)
    const player = `document.querySelector('.video-js').player`
    const subtitles = `Array.from(${player}.textTracks()).filter(t => t.kind === 'subtitles')`
    await waitFor(`${player}?.readyState() >= 1`)
    await waitFor(`window.subtitleRequests.length === 1`)
    assert.deepEqual(await evaluate(`window.subtitleRequests`), [
      { location_id: 11, format: 'vtt', manual: false },
    ])
    await waitFor(`window.finishSubtitleLookup`)
    assert.equal(
      await evaluate(`Boolean(document.querySelector('.player-subtitle-status'))`),
      false
    )
    await evaluate(`window.holdSubtitleLookup = false; window.finishSubtitleLookup()`)
    const pointAt = async (element) => {
      const point = await evaluate(`(() => {
        const rect = (${element}).getBoundingClientRect();
        return {x: rect.left + rect.width / 2, y: rect.top + rect.height / 2};
      })()`)
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
      return point
    }
    const click = async (element) => {
      const point = await pointAt(element)
      await command('Input.dispatchMouseEvent', {
        type: 'mousePressed',
        button: 'left',
        clickCount: 1,
        ...point,
      })
      await command('Input.dispatchMouseEvent', {
        type: 'mouseReleased',
        button: 'left',
        clickCount: 1,
        ...point,
      })
    }
    const loadSubtitle = async () => {
      const button = `document.querySelector('.vjs-subs-caps-button:not(.vjs-hidden) > button')`
      const item = `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('查找字幕'))`
      await waitFor(button)
      await click(button)
      await waitFor(`document.activeElement === (${item})`)
      for (let i = 0; i < 2; i++) {
        await pointAt(item)
        assert.equal(await evaluate(`(${item}).matches(':hover')`), true)
        assert.notEqual(
          await evaluate(`getComputedStyle(${item}).backgroundColor`),
          'rgba(0, 0, 0, 0)',
          'pointer focus must not suppress the load action hover highlight'
        )
        await pointAt(`document.querySelector('.vjs-subs-caps-button .vjs-texttrack-settings')`)
        assert.equal(
          await evaluate(`getComputedStyle(${item}).backgroundColor`),
          'rgba(0, 0, 0, 0)'
        )
      }
      await click(item)
    }
    const findItem = `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('查找字幕'))`
    const selectDownloaded = async () => {
      assert.equal(await evaluate(`Boolean(${findItem})`), false)
      await evaluate(`document.querySelector('.vjs-subs-caps-button > button').click()`)
      await evaluate(
        `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('已下载字幕')).click()`
      )
    }
    await evaluate(`window.holdSubtitleLookup = true; window.finishSubtitleLookup = null`)
    await loadSubtitle()
    await waitFor(
      `window.finishSubtitleLookup && document.querySelector('.player-subtitle-status')?.textContent.includes('Finding subtitles')`
    )
    await evaluate(`window.holdSubtitleLookup = false; window.finishSubtitleLookup()`)
    await waitFor(`${subtitles}.some(t => t.mode === 'showing' && t.cues?.length === 1)`)
    assert.equal(await evaluate(`Boolean(${findItem})`), false)
    assert.deepEqual(await evaluate(`window.subtitleRequests`), [
      { location_id: 11, format: 'vtt', manual: false },
      { location_id: 11, format: 'vtt', manual: true },
    ])
    await evaluate(`{
      ${player}.pause(); ${player}.currentTime(10);
    }`)
    assert.equal(
      await evaluate(
        `[...document.querySelectorAll('header button')].some(b => b.textContent === 'Subtitle offset')`
      ),
      false
    )
    const modal = `document.querySelector('.vjs-text-track-settings:not(.vjs-hidden)')`
    const form = `${modal}?.querySelector('.player-subtitle-offset-settings')`
    const openSettings = async () => {
      await evaluate(`document.querySelector('.vjs-subs-caps-button > button').click()`)
      await evaluate(
        `document.querySelector('.vjs-subs-caps-button .vjs-texttrack-settings').click()`
      )
      await waitFor(`${form}?.querySelector('input:not(:disabled)')`)
    }
    const setOffset = async (value) => {
      await evaluate(`{
        const input = ${form}.querySelector('input');
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, '${value}');
        input.dispatchEvent(new Event('input', {bubbles:true}));
      }`)
    }
    const saveOffset = () => evaluate(`${form}.querySelector('button[type="submit"]').click()`)
    const setFont = async (value) => {
      await evaluate(`{
        const font = ${modal}.querySelector('.vjs-font-percent select');
        font.value = '${value}'; font.dispatchEvent(new Event('change', {bubbles:true}));
      }`)
    }
    const savedStyle = `JSON.parse(localStorage.getItem('vjs-text-track-settings'))`
    await openSettings()
    assert.equal(await evaluate(`${modal}.textContent.includes('字体尺寸')`), true)
    assert.equal(await evaluate(`${form}.textContent.includes('Subtitle offset (seconds)')`), true)
    const checkControlsLayout = async () =>
      assert.equal(
        await evaluate(`(() => {
      const controls = ${modal}.querySelector('.vjs-track-settings-controls');
      const font = ${modal}.querySelector('.vjs-track-settings-font').getBoundingClientRect();
      const colors = ${modal}.querySelector('.vjs-track-settings-colors').getBoundingClientRect();
      const done = controls.querySelector('.vjs-done-button').getBoundingClientRect();
      const reset = controls.querySelector('.vjs-default-button').getBoundingClientRect();
      const dialog = ${modal}.getBoundingClientRect();
      const bounds = controls.getBoundingClientRect();
      return bounds.top >= Math.max(font.bottom, colors.bottom) && bounds.bottom <= ${form}.getBoundingClientRect().top && done.left > dialog.left + dialog.width / 2 && done.height === 28 && reset.height === 28 && !${form}.contains(controls);
    })()`),
        true
      )
    await checkControlsLayout()
    await evaluate(`window.holdSettingsSave = true`)
    await setOffset('30.5')
    await saveOffset()
    await waitFor(`window.finishSettingsSave && ${form}?.querySelector('button:disabled')`)
    await evaluate(`window.holdSettingsSave = false; window.finishSettingsSave()`)
    await waitFor(`!${modal} && ${subtitles}.some(t => t.cues?.[0]?.startTime === 30.5)`)
    assert.deepEqual(await evaluate(`window.settingsWrites`), [{ offset_ms: 30500 }])
    assert.equal(await evaluate(`window.streamRequests`), 1)
    assert.equal(await evaluate(`${player}.paused()`), true)
    assert.equal(await evaluate(`${player}.currentTime()`), 10)
    assert.equal(await evaluate(`${subtitles}.length`), 1)

    // Style controls never reset or save the per-video offset, including an unsaved draft.
    await openSettings()
    await setFont('2.00')
    await setOffset('99')
    await evaluate(`${modal}.querySelector('.vjs-default-button').click()`)
    assert.equal(await evaluate(`${modal}.querySelector('.vjs-font-percent select').value`), '1.00')
    assert.equal(await evaluate(`${form}.querySelector('input').value`), '99')
    assert.equal(await evaluate(`window.settingsWrites.length`), 1)
    await setFont('2.00')
    await evaluate(`${modal}.querySelector('.vjs-done-button').click()`)
    await waitFor(`!${modal}`)
    assert.equal(await evaluate(`${savedStyle}.fontPercent`), 2)
    assert.deepEqual(await evaluate(`window.settingsWrites`), [{ offset_ms: 30500 }])

    await command('Runtime.evaluate', {
      expression: `${player}.requestFullscreen()`,
      userGesture: true,
    })
    await waitFor(`document.fullscreenElement?.classList.contains('video-js')`)
    await openSettings()
    await waitFor(`${form}?.querySelector('input')?.value === '30.5'`)
    assert.equal(await evaluate(`document.fullscreenElement.contains(${form})`), true)
    await checkControlsLayout()
    await evaluate(`window.settingsFailure = true`)
    await setOffset('-1.5')
    await saveOffset()
    await waitFor(
      `${form}?.querySelector('[role="alert"]') && ${form}?.querySelector('button:not(:disabled)')`
    )
    assert.equal(await evaluate(`${form}.querySelector('input').value`), '-1.5')
    assert.equal(await evaluate(`window.settingsWrites.length`), 1)
    await evaluate(`window.settingsFailure = false`)
    await saveOffset()
    await waitFor(`!${modal} && ${subtitles}.some(t => t.cues?.[0]?.endTime === 57.5)`)
    assert.deepEqual(await evaluate(`window.settingsWrites`), [
      { offset_ms: 30500 },
      { offset_ms: -1500 },
    ])
    assert.equal(await evaluate(`window.streamRequests`), 1)
    assert.equal(await evaluate(`${player}.currentTime()`), 10)
    await openSettings()
    await setOffset('4000')
    await saveOffset()
    assert.equal(await evaluate(`${form}.querySelector('input').validity.rangeOverflow`), true)
    assert.equal(await evaluate(`window.settingsWrites.length`), 2)
    await evaluate(`${form}.querySelector('button[type="submit"]').focus()`)
    await command('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape' })
    await command('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape' })
    await waitFor(`!${modal}`)
    assert.equal(await evaluate(`Boolean(document.querySelector('.video-js'))`), true)
    await evaluate(`${player}.exitFullscreen()`)
    await waitFor(`!document.fullscreenElement`)
    await evaluate(`document.querySelector('button[aria-label="Next video"]').click()`)
    await waitFor(`window.streamRequests === 2 && ${subtitles}.length === 0`)
    await waitFor(`window.subtitleRequests.length === 5`)
    assert.equal(await evaluate(`window.subtitleRequests.at(-1).manual`), false)
    await loadSubtitle()
    await waitFor(`window.subtitleRequests.length === 6`)
    await waitFor(
      `document.querySelector('.player-subtitle-status')?.textContent.includes('No Chinese subtitles found')`
    )
    await waitFor(`${subtitles}.length === 0`)
    assert.equal(
      await evaluate(`${player}.getChild('textTrackSettings').getValues().fontPercent`),
      2
    )
    assert.deepEqual(await evaluate(`window.appErrors`), [])

    // Returning to the first video automatically loads the existing cached track.
    await evaluate(`document.querySelector('button[aria-label="Previous video"]').click()`)
    await waitFor(`${subtitles}.some(t => t.mode === 'showing' && t.cues?.length === 1)`)
    assert.equal(await evaluate(`window.subtitleRequests.length`), 7)
    assert.equal(await evaluate(`window.subtitleRequests.at(-1).manual`), false)

    // The native CC menu persists Off and restores it on returning to the video.
    await evaluate(`document.querySelector('.vjs-subs-caps-button > button').click()`)
    await evaluate(
      `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('字幕：隐藏')).click()`
    )
    await waitFor(`window.subtitleSettings.track === 'off'`)
    await evaluate(`document.querySelector('button[aria-label="Next video"]').click()`)
    await waitFor(`${subtitles}.length === 0`)
    await evaluate(`document.querySelector('button[aria-label="Previous video"]').click()`)
    await waitFor(`${subtitles}.some(t => t.mode === 'disabled' && t.cues?.length === 1)`)
    assert.equal(await evaluate(`window.visibilityWrites.at(-1)`), 'off')
    // Selecting the existing track enables it; Find remains hidden while off.
    await selectDownloaded()
    await waitFor(
      `${subtitles}.some(t => t.mode === 'showing') && window.subtitleSettings.track === 'download:zh-CN'`
    )

    // Fresh automatic downloads override the remembered Off preference.
    await evaluate(`{
      document.querySelector('.vjs-subs-caps-button > button').click();
      [...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('字幕：隐藏')).click();
    }`)
    await waitFor(`window.subtitleSettings.track === 'off'`)
    await evaluate(`{
      window.cachedSubtitles = false; window.freshDownload = true;
      document.querySelector('button[aria-label="Next video"]').click();
    }`)
    await waitFor(`${subtitles}.length === 0`)
    await evaluate(`document.querySelector('button[aria-label="Previous video"]').click()`)
    await waitFor(
      `${subtitles}.some(t => t.mode === 'showing' && t.cues?.length === 1) && window.subtitleSettings.track === 'download:zh-CN'`
    )
    assert.equal(await evaluate(`window.subtitleRequests.at(-1).manual`), false)

    // Local and downloaded tracks coexist and the native CC menu switches them.
    await evaluate(`{
      window.localSubtitles = true; window.freshDownload = false;
      document.querySelector('button[aria-label="Next video"]').click();
    }`)
    await waitFor(`${subtitles}.length === 0`)
    await evaluate(`document.querySelector('button[aria-label="Previous video"]').click()`)
    await waitFor(`${subtitles}.some(t => t.label.includes('同目录字幕') && t.cues?.length === 1)`)
    await waitFor(`${subtitles}.length === 2`)
    await selectDownloaded()
    await waitFor(
      `${subtitles}.length === 2 && ${subtitles}.some(t => t.label.includes('已下载字幕') && t.mode === 'showing')`
    )
    const menuLabels = `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item-text')].map(item => item.textContent)`
    assert.deepEqual(await evaluate(`${menuLabels}.slice(0, -1)`), [
      '字幕：同目录字幕',
      '字幕：已下载字幕',
      '字幕：隐藏',
    ])
    assert.equal(
      await evaluate(
        `document.querySelector('.vjs-subs-caps-button .vjs-menu-item:last-child').classList.contains('vjs-texttrack-settings')`
      ),
      true
    )
    // Native add/remove events rebuild labels as the number of downloads changes.
    await evaluate(`window.extraSubtitle = ${player}.addRemoteTextTrack({
      id: 'javboss:download:zh-TW', kind: 'subtitles', srclang: 'zh-TW', label: 'other.srt'
    }, false).track; undefined`)
    await waitFor(
      `${menuLabels}.includes('字幕：已下载字幕 · zh-CN') && ${menuLabels}.includes('字幕：已下载字幕 · zh-TW')`
    )
    await evaluate(`${player}.removeRemoteTextTrack(window.extraSubtitle)`)
    await waitFor(
      `${menuLabels}.includes('字幕：已下载字幕') && !${menuLabels}.includes('字幕：已下载字幕 · zh-CN')`
    )
    for (const label of ['同目录字幕', '已下载字幕']) {
      await evaluate(`document.querySelector('.vjs-subs-caps-button > button').click()`)
      await evaluate(
        `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('${label}')).click()`
      )
      await waitFor(
        `${subtitles}.filter(t => t.mode === 'showing').length === 1 && ${subtitles}.some(t => t.label.includes('${label}') && t.mode === 'showing')`
      )
    }
    assert.equal(await evaluate(`window.subtitleRequests.at(-1).cache_only`), true)
    const reopenFirst = async () => {
      await evaluate(`document.querySelector('button[aria-label="Next video"]').click()`)
      await waitFor(`${subtitles}.length === 0`)
      await evaluate(`document.querySelector('button[aria-label="Previous video"]').click()`)
      await waitFor(`${subtitles}.length === 2`)
    }
    for (const [label, key] of [
      ['同目录字幕', 'local:video.zh-CN.srt'],
      ['已下载字幕', 'download:zh-CN'],
    ]) {
      await evaluate(`document.querySelector('.vjs-subs-caps-button > button').click()`)
      await evaluate(
        `[...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('${label}')).click()`
      )
      await waitFor(`window.subtitleSettings.track === '${key}'`)
      await reopenFirst()
      await waitFor(`${subtitles}.some(t => t.label.includes('${label}') && t.mode === 'showing')`)
      assert.equal(await evaluate(`${subtitles}.filter(t => t.mode === 'showing').length`), 1)
    }
    // Reloading shifted subtitles keeps both tracks and the selected download.
    await openSettings()
    await setOffset('2')
    await saveOffset()
    await waitFor(
      `!${modal} && ${subtitles}.length === 2 && ${subtitles}.some(t => t.label.includes('已下载字幕') && t.mode === 'showing')`
    )
    await evaluate(`{
      document.querySelector('.vjs-subs-caps-button > button').click();
      [...document.querySelectorAll('.vjs-subs-caps-button .vjs-menu-item')].find(item => item.textContent.includes('字幕：隐藏')).click();
    }`)
    await waitFor(`window.subtitleSettings.track === 'off'`)
    await reopenFirst()
    await waitFor(`${subtitles}.every(t => t.mode === 'disabled')`)
    assert.equal(await evaluate(`window.subtitleSettings.track`), 'off')
    const beforeReload = await evaluate(`window.subtitleRequests.length`)
    await selectDownloaded()
    assert.equal(await evaluate(`${subtitles}.length`), 2)
    assert.equal(await evaluate(`window.subtitleRequests.length`), beforeReload)

    // Local subtitles alone do not hide Find. The action disappears as soon as
    // the additional downloaded track is added.
    await evaluate(`window.cachedSubtitles = false`)
    await evaluate(`document.querySelector('button[aria-label="Next video"]').click()`)
    await waitFor(`${subtitles}.length === 0`)
    await evaluate(`document.querySelector('button[aria-label="Previous video"]').click()`)
    await waitFor(`${subtitles}.length === 1 && ${subtitles}[0].label.includes('同目录字幕')`)
    await waitFor(`${findItem}`)
    await loadSubtitle()
    await waitFor(`${subtitles}.length === 2 && !(${findItem})`)

    // A fresh player after a page reload restores the global style.
    await reloadPage()
    await waitFor(`document.querySelector('#root > button')`)
    await evaluate(`document.querySelector('#root > button').click()`)
    await waitFor(`document.querySelector('.video-js')?.player`)
    assert.equal(
      await evaluate(`${player}.getChild('textTrackSettings').getValues().fontPercent`),
      2
    )
    await evaluate(`${player}.getChild('textTrackSettings').open()`)
    await waitFor(`${modal}`)
    await evaluate(
      `${modal}.querySelector('.vjs-default-button').click(); ${modal}.querySelector('.vjs-done-button').click()`
    )
    await waitFor(`!${modal}`)
    assert.deepEqual(
      await evaluate(savedStyle),
      await evaluate(`${player}.getChild('textTrackSettings').getValues()`)
    )
    assert.equal(await evaluate(`document.querySelector('.vjs-font-percent select').value`), '1.00')
    assert.deepEqual(await evaluate(`window.appErrors`), [])
  }
)
