import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'directory controls display Docker host paths without changing selection paths',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-directory-video-previews-test',
    })
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/app.html?view=video&directory_id=1&subdirectory=host/child`,
    })
    await waitFor(
      `window.testStore?.getState().config.video_page_size && window.testStore.getState().directories.length && !window.testStore.getState().loading`
    )
    await evaluate(`(() => {
      const originalFetch=window.fetch;
      window.fetch=(input,init) => {
        if(new URL(input,location.origin).pathname === '/videos/directories') return Promise.resolve(Response.json({
          directory_paths:{1:'/host/media/videos',2:'/hosted/archive',3:'/host'},
          items:[{directory_id:1,path:'.'},{directory_id:1,path:'host'},{directory_id:1,path:'host/child'},
            {directory_id:2,path:'.'},{directory_id:3,path:'.'},{directory_id:3,path:'child'}],
        }));
        return originalFetch(input,init);
      };
      window.testStore.setState({directories:[{id:1,path:'/host/media/videos',enabled:true}]});
    })()`)
    const nav = `document.querySelector('nav[aria-label="Current video directory"]')`
    const trigger = `document.querySelector('button[aria-label="Select subdirectory"]')`
    for (const [container, override, root] of [
      [true, null, '/media/videos'],
      [false, null, '/host/media/videos'],
      [true, false, '/host/media/videos'],
    ]) {
      await evaluate(`window.testStore.setState(state=>({config:{...state.config,runtime_container:${container},host_path_prefix_enabled:${override}}}));
        window.testStore.getState().setVideoDirectoryFilter({directory_id:1,path:'host/child'})`)
      await waitFor(
        `${nav}.querySelector('[aria-current="location"]').textContent === 'child' && !window.testStore.getState().loading`
      )
      assert.equal(await evaluate(`${nav}.querySelectorAll('button')[1].textContent`), root)
      assert.equal(await evaluate(`${nav}.querySelectorAll('button')[1].title`), root)
      assert.equal(
        await evaluate(`${nav}.querySelector('[aria-current="location"]').title`),
        `${root}/host/child`
      )
      assert.equal(await evaluate(`${trigger}.title`), `${root}/host/child`)
      await evaluate(`${trigger}.click()`)
      const top = `document.querySelector('[role="dialog"][aria-label="Video directories"]')`
      await waitFor(
        `${top}?.querySelector('button[aria-label="Select directory ${root}/host/child"]')`
      )
      assert.equal(
        await evaluate(
          `${top}.querySelector('button[aria-label="Select directory ${root}"]').previousElementSibling.textContent`
        ),
        root
      )
      assert.ok(
        await evaluate(
          `${top}.querySelector('button[aria-label="Select directory /hosted/archive"]') !== null`
        )
      )
      const mountRoot = container && override !== false ? '/' : '/host'
      assert.ok(
        await evaluate(
          `${top}.querySelector('button[aria-label="Select directory ${mountRoot}"]') !== null`
        )
      )
      await evaluate(
        `${top}.querySelector('button[aria-label="Select directory ${root}/host"]').click()`
      )
      await waitFor(
        `!${top} && window.testStore.getState().videoSubdirectory === 'host' && !window.testStore.getState().loading`
      )
      assert.equal(
        await evaluate('window.testStore.getState().directories[0].path'),
        '/host/media/videos'
      )
      assert.equal(
        await evaluate(`new URLSearchParams(location.search).get('subdirectory')`),
        'host'
      )
      await evaluate(`${nav}.querySelector('[aria-current="location"]').click()`)
      const bottom = `document.querySelector('[role="dialog"][aria-label="Choose video directory"]')`
      await waitFor(
        `${bottom}?.querySelector('button[aria-label="Select directory ${root}/host"]')`
      )
      assert.equal(
        await evaluate(`${bottom}.querySelector('[aria-pressed="true"]').title`),
        `${root}/host`
      )
      assert.equal(
        await evaluate(
          `${bottom}.querySelector('button[aria-label="Select directory ${root}"]').previousElementSibling.textContent`
        ),
        root
      )
      await evaluate(
        `${bottom}.querySelector('button[aria-label="Select directory ${root}"]').click()`
      )
      await waitFor(`!${bottom} && window.testStore.getState().videoSubdirectory === '.'`)
      assert.equal(await evaluate(`${trigger}.textContent`), root)
    }
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

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
        if(new URL(input,location.origin).pathname === '/videos/directories') return Promise.resolve(Response.json({directory_paths:{1:'/videos',2:'/archive'},items:[
          {directory_id:1,path:'folder',video_count:23,videos:Array.from({length:20},(_,i)=>'clip-'+String(i+1).padStart(2,'0')+'.mp4')},
          {directory_id:1,path:'.',video_count:1,videos:['root.mp4']},
          {directory_id:2,path:'.',video_count:1,videos:['other-root.mp4']},
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
      assert.deepEqual(
        await evaluate(`[...${files}].map(file=>file.textContent)`),
        Array.from({ length: 20 }, (_, i) => `clip-${String(i + 1).padStart(2, '0')}.mp4`)
      )
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
