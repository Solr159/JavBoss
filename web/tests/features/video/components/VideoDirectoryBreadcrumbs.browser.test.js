import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../../helpers/browser.js'

test(
  'bottom directory path switches each level, shares the tree selection, and stays visible on narrow screens',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-video-directory-breadcrumbs-test',
    })
    const initialQuery = new URLSearchParams({
      view: 'video',
      directory_id: '1',
      subdirectory: '旅行/2026',
      search: 'clip',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?${initialQuery}` })
    const nav = `document.querySelector('nav[aria-label="Current video directory"]')`
    const menu = `document.querySelector('[role="menu"][aria-label="Choose video directory"]')`
    await waitFor(
      `${nav} && window.testStore && window.requests.some(r => r.url.startsWith('/videos?')) && !window.testStore.getState().loading`
    )
    await evaluate(`(() => {
      const originalFetch = window.fetch;
      window.directoryOptions = [
        {directory_id:1, directory_path:'/videos', path:'.'},
        {directory_id:1, directory_path:'/videos', path:'other'},
        {directory_id:1, directory_path:'/videos', path:'旅行'},
        {directory_id:1, directory_path:'/videos', path:'旅行/2025'},
        {directory_id:1, directory_path:'/videos', path:'旅行/2026'},
        {directory_id:1, directory_path:'/videos', path:'旅行/2026/deeper'},
        {directory_id:2, directory_path:'/archive', path:'.'},
        {directory_id:2, directory_path:'/archive', path:'旅行'},
      ];
      window.testStore.setState({directories: [{id:1,path:'/videos',enabled:true},{id:2,path:'/archive',enabled:true}]});
      window.fetch = async (input, init) => {
        if (new URL(input, location.origin).pathname === '/videos/directories') {
          if (window.failNextDirectoryRequest) {
            window.failNextDirectoryRequest = false;
            return Response.json({error_en:'Directory lookup failed'}, {status:503});
          }
          return Response.json({items:window.directoryOptions});
        }
        return originalFetch(input, init);
      };
    })()`)
    const labels = () => evaluate(`[...${nav}.querySelectorAll('button')].map(b => b.textContent)`)
    const openLevel = async (label) => {
      await evaluate(
        `[...${nav}.querySelectorAll('button')].find(b => b.textContent === ${JSON.stringify(label)}).click()`
      )
      await waitFor(menu)
      await waitFor(`!${menu}.textContent.includes('Loading...')`)
    }
    const select = async (label) => {
      await evaluate(
        `[...${menu}.querySelectorAll('[role="menuitem"]')].find(b => b.textContent === ${JSON.stringify(label)}).click()`
      )
      await waitFor(`!${menu}`)
    }
    const menuLabels = () =>
      evaluate(`[...${menu}.querySelectorAll('[role="menuitem"]')].map(b => b.textContent)`)
    assert.deepEqual(await labels(), ['All directories', '/videos', '旅行', '2026'])
    await openLevel('旅行')
    assert.deepEqual(await menuLabels(), ['View videos in this directory', '2025', '2026'])
    await select('2025')
    await waitFor(
      `new URLSearchParams(location.search).get('subdirectory') === '旅行/2025' && !window.testStore.getState().loading`
    )
    assert.deepEqual(await labels(), ['All directories', '/videos', '旅行', '2025'])
    assert.equal(
      await evaluate(`document.querySelector('button[aria-label="Select subdirectory"]').title`),
      '/videos/旅行/2025'
    )
    const query = new URL(
      await evaluate(`window.requests.filter(r => r.url.startsWith('/videos?')).at(-1).url`),
      origin
    ).searchParams
    assert.equal(query.get('directory_id'), '1')
    assert.equal(query.get('subdirectory'), '旅行/2025')
    assert.equal(query.get('search'), 'clip')
    assert.equal(query.get('offset'), '0')
    await openLevel('2025')
    assert.deepEqual(await menuLabels(), ['View videos in this directory', 'No subdirectories'])
    await command('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'Escape',
      code: 'Escape',
      windowsVirtualKeyCode: 27,
    })
    await command('Input.dispatchKeyEvent', {
      type: 'keyUp',
      key: 'Escape',
      code: 'Escape',
      windowsVirtualKeyCode: 27,
    })
    await waitFor(`!${menu}`)
    await openLevel('/videos')
    assert.deepEqual(await menuLabels(), ['View videos in this directory', 'other', '旅行'])
    await select('View videos in this directory')
    await waitFor(`window.testStore.getState().videoSubdirectory === '.'`)
    assert.deepEqual(await labels(), ['All directories', '/videos'])
    await evaluate('window.failNextDirectoryRequest = true')
    await openLevel('All directories')
    assert.deepEqual(await menuLabels(), ['View all videos', 'Directory lookup failed', 'Retry'])
    await evaluate(
      `[...${menu}.querySelectorAll('[role="menuitem"]')].find(b => b.textContent === 'Retry').click()`
    )
    await waitFor(`${menu}.textContent.includes('/archive')`)
    assert.deepEqual(await menuLabels(), ['View all videos', '/videos', '/archive'])
    await select('/archive')
    await waitFor(`window.testStore.getState().videoDirectoryId === 2`)
    await openLevel('/archive')
    assert.deepEqual(await menuLabels(), ['View videos in this directory', '旅行'])
    await select('旅行')
    await waitFor(`new URLSearchParams(location.search).get('subdirectory') === '旅行'`)
    await evaluate('history.back()')
    await waitFor(
      `window.testStore.getState().videoDirectoryId === 2 && window.testStore.getState().videoSubdirectory === '.'`
    )
    assert.deepEqual(await labels(), ['All directories', '/archive'])
    await openLevel('All directories')
    await select('View all videos')
    await waitFor(`!new URLSearchParams(location.search).has('directory_id')`)
    assert.deepEqual(await labels(), ['All directories'])

    // The top tree and URL restore update the same breadcrumb path.
    await evaluate(`document.querySelector('button[aria-label="Select subdirectory"]').click()`)
    await waitFor(
      `document.querySelector('summary[aria-label="Expand or collapse directory /videos"]')`
    )
    await evaluate(
      `document.querySelector('summary[aria-label="Expand or collapse directory /videos"]').click()`
    )
    await evaluate(
      `document.querySelector('button[aria-label="Select directory /videos/旅行"]').click()`
    )
    await waitFor(`new URLSearchParams(location.search).get('subdirectory') === '旅行'`)
    assert.deepEqual(await labels(), ['All directories', '/videos', '旅行'])
    await reloadPage()
    await waitFor(
      `${nav} && window.testStore?.getState().videoSubdirectory === '旅行' && window.testStore.getState().directories.length`
    )
    assert.deepEqual(await labels(), ['All directories', '/videos', '旅行'])

    await command('Emulation.setDeviceMetricsOverride', {
      width: 390,
      height: 800,
      deviceScaleFactor: 1,
      mobile: false,
    })
    await evaluate(
      `window.testStore.getState().setVideoDirectoryFilter({directory_id:1,path:'旅行/2026/a-long-directory-name/another-long-directory-name'})`
    )
    await waitFor(
      `!window.testStore.getState().loading && ${nav}.querySelector('[aria-current="location"]').textContent === 'another-long-directory-name'`
    )
    const bounds = await evaluate(`(() => {
      const bar=${nav}, list=bar.querySelector('ol'), current=bar.querySelector('[aria-current="location"]');
      const rect=bar.getBoundingClientRect(), selected=current.getBoundingClientRect();
      return {bottom:rect.bottom,height:document.documentElement.clientHeight,scroll:list.scrollLeft,currentRight:selected.right,barRight:rect.right};
    })()`)
    assert.ok(Math.abs(bounds.bottom - bounds.height) <= 1)
    assert.ok(bounds.scroll > 0)
    assert.ok(bounds.currentRight <= bounds.barRight + 1)
    await evaluate(
      `window.testStore.setState({videos:Array.from({length:20},(_,i)=>({id:i+1,location_id:i+1,filename:'test.mp4',path:'test.mp4',tags:[]})),total:20})`
    )
    await waitFor(`document.querySelectorAll('.video-card').length === 20`)
    await evaluate(`window.scrollTo(0,document.documentElement.scrollHeight)`)
    await waitFor(
      `document.querySelector('.video-card:last-child').getBoundingClientRect().bottom <= ${nav}.getBoundingClientRect().top`
    )
    await evaluate(`document.querySelector('aside button[aria-label="JAV"]').click()`)
    await waitFor(`!${nav}`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
