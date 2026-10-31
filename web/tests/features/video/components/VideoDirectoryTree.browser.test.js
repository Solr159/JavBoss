import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'both directory trees show noninteractive direct video previews and remaining counts',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-directory-video-previews-test',
    })
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/app.html?view=video&directory_id=1&subdirectory=folder`,
    })
    await waitFor(
      `window.testStore?.getState().directories.length && document.querySelector('nav[aria-label="Current video directory"]')`
    )
    await evaluate(`(() => {
      const originalFetch=window.fetch;
      window.fetch=(input,init) => {
        if(new URL(input,location.origin).pathname === '/videos/directories') return Promise.resolve(Response.json({items:[
          {directory_id:1,directory_path:'/videos',path:'folder',video_count:8,videos:Array.from({length:5},(_,i)=>'clip-0'+(i+1)+'.mp4')},
          {directory_id:1,directory_path:'/videos',path:'.',video_count:1,videos:['root.mp4']},
          {directory_id:2,directory_path:'/archive',path:'.',video_count:1,videos:['other-root.mp4']},
        ]}));
        return originalFetch(input,init);
      };
    })()`)
    for (const label of ['Video directories', 'Choose video directory']) {
      const popup = `document.querySelector('[role="dialog"][aria-label="${label}"]')`
      if (label === 'Video directories') {
        await evaluate(`document.querySelector('button[aria-label="Select subdirectory"]').click()`)
      } else {
        await evaluate(
          `document.querySelector('nav[aria-label="Current video directory"] [aria-current="location"]').click()`
        )
      }
      const branch = `${popup}?.querySelector('summary[aria-label="Expand or collapse directory /videos/folder"]')`
      await waitFor(branch)
      assert.equal(await evaluate(`${branch}.parentElement.open`), false)
      assert.ok(await evaluate(`${popup}.querySelector('li[title="root.mp4"]').checkVisibility()`))
      await evaluate(`${branch}.querySelector('span').click()`)
      await waitFor(`${branch}.parentElement.open`)
      const files = `${branch}.nextElementSibling.querySelectorAll('li[title]')`
      assert.deepEqual(await evaluate(`[...${files}].map(file=>file.textContent)`), [
        'clip-01.mp4',
        'clip-02.mp4',
        'clip-03.mp4',
        'clip-04.mp4',
        'clip-05.mp4',
      ])
      assert.ok(
        await evaluate(`${branch}.nextElementSibling.textContent.includes('3 more videos')`)
      )
      assert.ok(
        await evaluate(
          `[...${files}].every(file=>file.checkVisibility() && !file.querySelector('button,a,[tabindex]') && file.tabIndex === -1 && getComputedStyle(file).cursor === 'default')`
        )
      )
      await evaluate(`${branch}.nextElementSibling.querySelector('li').click()`)
      await waitFor(popup)
      assert.equal(await evaluate('window.testStore.getState().videoSubdirectory'), 'folder')
      assert.equal(await evaluate(`${branch}.parentElement.open`), true)
      assert.equal(
        await evaluate(`${popup}.querySelectorAll('li[title="clip-01.mp4"]').length`),
        1,
        'child previews must not be copied to ancestors'
      )
      for (const type of ['keyDown', 'keyUp']) {
        await command('Input.dispatchKeyEvent', {
          type,
          key: 'Escape',
          code: 'Escape',
          windowsVirtualKeyCode: 27,
        })
      }
      await waitFor(`!${popup}`)
    }
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
