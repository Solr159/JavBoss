import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

const isDark = `document.documentElement.classList.contains('dark')`
const selector = `document.getElementById('appearance-theme')`
const options = { skip: browserUnavailable, timeout: 60000 }

async function openAppearance({ evaluate, waitFor }) {
  try {
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
  } catch (error) {
    console.error(await evaluate('({body: document.body.innerText, errors: window.appErrors})'))
    throw error
  }
  await evaluate(`document.querySelector('aside button[aria-label="Settings"]').click()`)
  await waitFor(`document.getElementById('global-settings-title')`)
  await evaluate(
    `Array.from(document.querySelectorAll('aside button')).find(b => b.textContent === 'Display & Interaction').click()`
  )
  await waitFor(selector)
}

async function chooseTheme({ evaluate, waitFor }, preference, dark) {
  await evaluate(`{
    const select = ${selector};
    select.value = ${JSON.stringify(preference)};
    select.dispatchEvent(new Event('change', {bubbles: true}));
  }`)
  await waitFor(`${isDark} === ${dark}`)
}

async function emulateTheme({ command }, value) {
  await command('Emulation.setEmulatedMedia', {
    features: [{ name: 'prefers-color-scheme', value }],
  })
}

test(
  'appearance persists, follows the system only when selected, and themes MUI portals',
  options,
  async (t) => {
    const browser = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-theme-test',
      chromeArgs: [
        '--blink-settings=primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4',
      ],
    })
    await browser.command('Page.enable')
    const { origin, command, evaluate, waitFor, reloadPage } = browser
    await emulateTheme(browser, 'dark')
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await openAppearance(browser)
    assert.equal(await evaluate(isDark), false, 'existing users keep the light theme')
    assert.equal(await evaluate(`${selector}.value`), 'light')
    await chooseTheme(browser, 'dark', true)
    assert.equal(await evaluate(`localStorage.getItem('javboss.theme')`), 'dark')
    assert.equal(
      await evaluate(`getComputedStyle(document.body).backgroundColor`),
      'rgb(24, 24, 24)'
    )
    assert.equal(await evaluate(`getComputedStyle(${selector}).backgroundColor`), 'rgb(38, 38, 38)')
    assert.equal(await evaluate(`getComputedStyle(${selector}).color`), 'rgb(242, 242, 242)')
    assert.equal(await evaluate(`getComputedStyle(document.documentElement).colorScheme`), 'dark')
    assert.equal(
      await evaluate(`document.querySelector('meta[name="theme-color"]').content`),
      '#181818'
    )
    await waitFor(
      `getComputedStyle(document.querySelector('.jav-card')).backgroundColor === 'rgb(38, 38, 38)'`
    )
    for (const navigation of ['.side-tabs', '.filter-topbar', '.sticky-pagination']) {
      assert.equal(
        await evaluate(`getComputedStyle(document.querySelector('${navigation}')).backgroundColor`),
        'rgb(32, 32, 32)'
      )
    }
    assert.equal(
      await evaluate(`getComputedStyle(document.querySelector('.side-tabs')).backgroundImage`),
      'none'
    )
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('.side-tabs__button--active'), '::before').backgroundColor`
      ),
      'rgb(59, 130, 246)'
    )
    assert.equal(
      await evaluate(`getComputedStyle(document.getElementById('appearance-theme-hint')).color`),
      'rgb(163, 163, 163)'
    )
    assert.equal(await evaluate(`getComputedStyle(${selector}).borderTopColor`), 'rgb(56, 56, 56)')

    await reloadPage()
    try {
      await waitFor(`document.querySelector('.jav-card')`)
    } catch (error) {
      console.error(
        await evaluate(
          '({body: document.body.innerText, errors: window.appErrors, requests: window.requests})'
        )
      )
      throw error
    }
    assert.equal(
      await evaluate('window.initialTheme'),
      'dark',
      'theme is applied before React renders'
    )
    await evaluate(`document.querySelector('[aria-label="Select ABC-001"]').click()`)
    await waitFor(`document.querySelector('button[aria-label="Selection menu"]')`)
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('button[aria-label="Selection menu"]')).color`
      ),
      'rgb(59, 130, 246)'
    )
    await evaluate(`document.querySelector('button[aria-label="Selection menu"]').click()`)
    await waitFor(`document.querySelector('[role="menu"]')`)
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('[role="menu"]').closest('.MuiPaper-root')).backgroundColor`
      ),
      'rgb(38, 38, 38)'
    )
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('[role="menu"]').closest('.MuiPaper-root')).color`
      ),
      'rgb(242, 242, 242)'
    )
    const menuPosition = await evaluate(`(() => {
      // The first item has keyboard focus; hover another item to check the pointer state.
      const rect = document.querySelectorAll('[role="menuitem"]')[1].getBoundingClientRect();
      return {x: rect.x + rect.width / 2, y: rect.y + rect.height / 2};
    })()`)
    await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...menuPosition })
    await waitFor(
      `getComputedStyle(document.querySelectorAll('[role="menuitem"]')[1]).backgroundColor === 'rgb(48, 48, 48)'`
    )
    await evaluate(`document.querySelector('[role="menuitem"]').click()`)
    await waitFor(`!document.querySelector('[role="menu"]')`)
    await openAppearance(browser)
    assert.equal(await evaluate(`${selector}.value`), 'dark')
    await emulateTheme(browser, 'light')
    assert.equal(await evaluate(isDark), true, 'explicit dark ignores a light system')
    await chooseTheme(browser, 'system', false)
    await emulateTheme(browser, 'dark')
    await waitFor(isDark)
    await reloadPage()
    await openAppearance(browser)
    assert.equal(await evaluate('window.initialTheme'), 'dark')
    assert.equal(await evaluate(`${selector}.value`), 'system')
    await emulateTheme(browser, 'light')
    await waitFor(`!${isDark}`)
    await chooseTheme(browser, 'light', false)
    await emulateTheme(browser, 'dark')
    assert.equal(await evaluate(isDark), false, 'explicit light ignores a dark system')
    assert.equal(
      await evaluate(`getComputedStyle(${selector}).backgroundColor`),
      'rgb(255, 255, 255)'
    )
    assert.equal(
      await evaluate(
        `window.requests.filter(r => r.url === '/config' && r.method !== 'GET').length`
      ),
      0
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test(
  'appearance synchronizes browser contexts and survives unavailable storage',
  options,
  async (t) => {
    const browser = await openBrowser(t, { cacheDir: 'node_modules/.vite-theme-test' })
    await browser.command('Page.enable')
    const { origin, command, evaluate, waitFor, reloadPage } = browser
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html` })
    await openAppearance(browser)
    // A different same-origin window produces a real storage event in the parent.
    await evaluate(`{
    const frame = document.createElement('iframe');
    frame.hidden = true;
    document.body.append(frame);
    frame.contentWindow.localStorage.setItem('javboss.theme', 'dark');
    window.storageFrame = frame;
  }`)
    await waitFor(`${isDark} && ${selector}.value === 'dark'`)
    await evaluate(`window.storageFrame.contentWindow.localStorage.clear()`)
    await waitFor(`!${isDark} && ${selector}.value === 'light'`)
    await evaluate(`localStorage.setItem('javboss.theme', 'invalid')`)
    await reloadPage()
    await openAppearance(browser)
    assert.equal(await evaluate('window.initialTheme'), 'light')
    assert.equal(await evaluate(`${selector}.value`), 'light')

    await command('Page.addScriptToEvaluateOnNewDocument', {
      source: `for (const method of ['getItem', 'setItem']) {
        const original = Storage.prototype[method];
        Storage.prototype[method] = function(key, ...args) {
          if (key === 'javboss.theme') throw new DOMException('Storage blocked', 'SecurityError');
          return original.call(this, key, ...args);
        };
      }`,
    })
    await reloadPage()
    await openAppearance(browser)
    assert.equal(await evaluate('window.initialTheme'), 'light')
    await chooseTheme(browser, 'dark', true)
    assert.match(
      await evaluate(`document.querySelector('[role="alert"]').textContent`),
      /could not save your theme/
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test('the saved theme also applies before sign-in', options, async (t) => {
  const browser = await openBrowser(t, { cacheDir: 'node_modules/.vite-theme-test' })
  await browser.command('Page.enable')
  const { origin, command, evaluate, waitFor } = browser
  await command('Page.addScriptToEvaluateOnNewDocument', {
    source: `localStorage.setItem('javboss.theme', 'dark')`,
  })
  await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?login` })
  await waitFor(`document.querySelector('input[type="password"]')`)
  assert.equal(await evaluate('window.initialTheme'), 'dark')
  assert.equal(
    await evaluate(
      `getComputedStyle(document.querySelector('input[type="password"]')).backgroundColor`
    ),
    'rgb(38, 38, 38)'
  )
  assert.equal(
    await evaluate(`getComputedStyle(document.querySelector('input[type="password"]')).color`),
    'rgb(242, 242, 242)'
  )
  assert.deepEqual(await evaluate('window.appErrors'), [])
})

test(
  'dark favorites use blue surfaces, tag types stay distinct, and sort indicators are readable',
  options,
  async (t) => {
    const browser = await openBrowser(t, { cacheDir: 'node_modules/.vite-theme-test' })
    const { origin, command, evaluate, waitFor } = browser
    await command('Page.enable')
    await command('Page.addScriptToEvaluateOnNewDocument', {
      source: `localStorage.setItem('javboss.theme', 'dark')`,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await waitFor(`document.querySelector('.pagination-sort-caret')`)
    const activeTab = `document.querySelector('.side-tabs__button--active')`
    await waitFor(`getComputedStyle(${activeTab}).backgroundColor === 'rgb(48, 48, 48)'`)
    assert.equal(
      await evaluate(`getComputedStyle(${activeTab}).backgroundColor`),
      'rgb(48, 48, 48)'
    )
    assert.equal(await evaluate(`getComputedStyle(${activeTab}).color`), 'rgb(242, 242, 242)')
    assert.equal(await evaluate(`getComputedStyle(${activeTab}).boxShadow`), 'none')
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('.pagination-sort-caret')).borderRightColor`
      ),
      'rgb(163, 163, 163)'
    )
    await evaluate(`(async () => {
    const originalFetch = window.fetch;
    window.fetch = (input, init) => {
      const path = new URL(input, location.origin).pathname;
      if (path === '/jav/jav-favorite-groups') return Promise.resolve(Response.json({items: [{id: 1, name: 'Sample favorite', count: 1}]}));
      if (path === '/jav/tags') return Promise.resolve(Response.json([
        {id: 1, name: 'Sample scraped tag', provider: 1, count: 12},
        {id: 2, name: 'Sample custom tag', provider: 3, count: 2}
      ]));
      return originalFetch(input, init);
    };
    await window.testStore.getState().loadJavFavoriteGroups('jav', {force: true});
    await window.testStore.getState().loadJavTags({force: true});
    document.querySelector('button[aria-label="Work favorites"]').click();
  })()`)
    const tile = `document.querySelector('button[aria-label="Edit favorite Sample favorite"]').parentElement`
    const folder = `${tile}.querySelector('a > span:nth-of-type(2)')`
    await waitFor(`document.querySelector('button[aria-label="Edit favorite Sample favorite"]')`)
    assert.equal(await evaluate(`getComputedStyle(${folder}).backgroundColor`), 'rgb(59, 130, 246)')
    assert.equal(await evaluate(`getComputedStyle(${folder}).backgroundImage`), 'none')
    assert.doesNotMatch(await evaluate(`getComputedStyle(${folder}).boxShadow`), /inset/)
    await evaluate(`${tile}.querySelector('a').click()`)
    await waitFor(`!document.querySelector('[role="dialog"][aria-label="Work favorites"]')`)
    await evaluate(`document.querySelector('button[aria-label="Work favorites"]').click()`)
    await waitFor(`document.querySelector('button[aria-label="Edit favorite Sample favorite"]')`)
    assert.equal(await evaluate(`getComputedStyle(${tile}).borderTopColor`), 'rgb(59, 130, 246)')
    assert.equal(await evaluate(`getComputedStyle(${tile}).backgroundColor`), 'rgb(48, 48, 48)')
    await evaluate(`document.querySelector('button[aria-label="Work favorites"]').click()`)
    const openTags = async () => {
      await evaluate(`document.querySelector('aside button[aria-label="Tags"]').click()`)
      await waitFor(
        `document.querySelector('.skeuo-tag--scraped') && document.querySelector('.skeuo-tag--user')`
      )
    }
    await openTags()
    for (const [type, background, border] of [
      ['scraped', 'rgb(249, 115, 22)', 'rgb(249, 115, 22)'],
      ['user', 'rgb(16, 185, 129)', 'rgb(16, 185, 129)'],
    ]) {
      const tag = `document.querySelector('.skeuo-tag--${type}')`
      assert.equal(await evaluate(`getComputedStyle(${tag}).backgroundColor`), background)
      assert.equal(await evaluate(`getComputedStyle(${tag}).borderTopColor`), border)
      assert.equal(
        await evaluate(`getComputedStyle(${tag}.querySelector('.skeuo-tag-label')).color`),
        'rgb(242, 242, 242)'
      )
      assert.equal(
        await evaluate(
          `getComputedStyle(${tag}.querySelector('.skeuo-tag-count')).backgroundColor`
        ),
        'rgb(48, 48, 48)'
      )
    }
    await evaluate(
      `Array.from(document.querySelectorAll('[role="dialog"] button')).find(b => b.textContent === 'Find by multiple tags').click()`
    )
    await waitFor(`document.querySelector('.skeuo-tag input')`)
    for (const [type, accent, background, border] of [
      ['scraped', 'rgb(249, 115, 22)', 'rgb(194, 65, 12)', 'rgb(253, 186, 116)'],
      ['user', 'rgb(16, 185, 129)', 'rgb(4, 120, 87)', 'rgb(110, 231, 183)'],
    ]) {
      const tag = `document.querySelector('.skeuo-tag--${type}')`
      assert.equal(
        await evaluate(`getComputedStyle(${tag}.querySelector('input')).accentColor`),
        accent
      )
      await evaluate(`${tag}.querySelector('input').click()`)
      await waitFor(`getComputedStyle(${tag}).backgroundColor === '${background}'`)
      assert.equal(await evaluate(`getComputedStyle(${tag}).borderTopColor`), border)
    }
    await evaluate(`document.querySelector('[aria-label="Close tag management"]').click()`)
    await evaluate(`window.testStore.getState().setJavSearchTerm('sample')`)
    await waitFor(`document.querySelector('.filter-chip')`)
    const chip = `document.querySelector('.filter-chip')`
    assert.equal(await evaluate(`getComputedStyle(${chip}).backgroundColor`), 'rgb(30, 58, 95)')
    assert.equal(await evaluate(`getComputedStyle(${chip}).borderTopColor`), 'rgb(59, 130, 246)')
    assert.equal(await evaluate(`getComputedStyle(${chip}).color`), 'rgb(147, 197, 253)')
    await evaluate(`${chip}.querySelector('.filter-chip__remove').click()`)
    await waitFor(`!document.querySelector('.filter-chip')`)
    await openAppearance(browser)
    await chooseTheme(browser, 'light', false)
    await evaluate(`document.querySelector('[aria-label="Close global settings"]').click()`)
    await openTags()
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('.skeuo-tag--scraped')).backgroundColor`
      ),
      'rgb(255, 237, 213)'
    )
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('.skeuo-tag--user')).backgroundColor`
      ),
      'rgb(209, 250, 229)'
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test(
  'dark player tabs, disabled translation controls and update notices stay readable',
  options,
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-theme-test',
    })
    await command('Page.enable')
    await command('Page.addScriptToEvaluateOnNewDocument', {
      source: `localStorage.setItem('javboss.theme', 'dark')`,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
    await evaluate(`{
    const originalFetch = window.fetch;
    window.fetch = (input, init) => new URL(input, location.origin).pathname === '/system/update'
      ? Promise.resolve(Response.json({phase: 'idle', disabled_reason: 'development'}))
      : originalFetch(input, init);
    window.testStore.setState(s => ({config: {...s.config,
      jav_title_translation_provider: 'deepseek',
      jav_title_translation_catalog: JSON.stringify([{id: 'deepseek', name: 'DeepSeek', protocol: 'chat'}]),
      jav_title_translation_profiles: JSON.stringify({deepseek: {model: 'saved-model', api_key_set: true}}),
    }}));
    document.querySelector('aside button[aria-label="Settings"]').click();
  }`)
    const button = (text) =>
      `Array.from(document.querySelectorAll('button')).find(b => b.textContent === '${text}')`
    await waitFor(button('Player'))
    await evaluate(`${button('Player')}.click()`)
    for (const label of ['Basic Settings', 'Web Player', 'Shortcuts']) {
      await waitFor(button(label))
      await evaluate(`${button(label)}.click()`)
      await waitFor(`getComputedStyle(${button(label)}).backgroundColor === 'rgb(30, 58, 95)'`)
      assert.equal(await evaluate(`getComputedStyle(${button(label)}).color`), 'rgb(147, 197, 253)')
    }
    await evaluate(`${button('AI Translation')}.click()`)
    const section = `document.querySelector('[aria-labelledby="translation-service-heading"]')`
    await waitFor(`${section}?.querySelector('input[type="password"]')`)
    for (const field of ['select', '#title-translation-model', 'input[type="password"]']) {
      const control = `${section}.querySelector('${field}')`
      assert.equal(await evaluate(`${control}.matches(':disabled')`), true)
      assert.equal(
        await evaluate(`getComputedStyle(${control}).backgroundColor`),
        'rgb(24, 24, 24)'
      )
    }
    await evaluate(`${section}.querySelector('header button').click()`)
    for (const field of ['select', '#title-translation-model', 'input[type="password"]']) {
      const control = `${section}.querySelector('${field}')`
      await waitFor(`!${control}.matches(':disabled')`)
      assert.equal(
        await evaluate(`getComputedStyle(${control}).backgroundColor`),
        'rgb(38, 38, 38)'
      )
    }
    await evaluate(`${button('Check for Updates')}.click()`)
    await waitFor(`document.querySelector('.MuiAlert-standardInfo')`)
    assert.equal(
      await evaluate(
        `getComputedStyle(document.querySelector('.MuiAlert-standardInfo')).backgroundColor`
      ),
      'rgb(30, 58, 95)'
    )
    assert.equal(
      await evaluate(`getComputedStyle(document.querySelector('.MuiAlert-icon')).color`),
      'rgb(147, 197, 253)'
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test(
  'dark card hover, favorite stars and idol cover save keep clear contrast',
  options,
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-theme-test',
      chromeArgs: [
        '--blink-settings=primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4',
      ],
    })
    await command('Page.enable')
    await command('Page.addScriptToEvaluateOnNewDocument', {
      source: `localStorage.setItem('javboss.theme', 'dark')`,
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(
      `window.testStore && document.querySelector('aside') && window.requests.some(r => r.url.startsWith('/videos?')) && !window.testStore.getState().loading`
    )
    await evaluate(
      `window.testStore.setState({videos: [{id: 1, location_id: 1, filename: 'Example.mp4', path: 'Example.mp4', tags: []}], total: 1})`
    )
    await waitFor(`document.querySelector('.video-card')`)
    const hover = async (selector) => {
      const point = await evaluate(
        `(() => {const el = document.querySelector('${selector}'); el.scrollIntoView({block: 'center'}); const r = el.getBoundingClientRect(); return {x: r.x + r.width / 2, y: r.y + r.height / 2}})()`
      )
      await command('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point })
    }
    await hover('.video-card')
    await waitFor(
      `getComputedStyle(document.querySelector('.video-card')).borderTopColor === 'rgb(56, 56, 56)'`
    )
    await evaluate(`document.querySelector('aside button[aria-label="JAV"]').click()`)
    await waitFor(`document.querySelector('.jav-card')`)
    await evaluate(
      `window.testStore.setState(s => ({javItems: s.javItems.map(item => ({...item, favorite_count: 1}))}))`
    )
    const star = `document.querySelector('[aria-label="Add to JAV favorite groups"]')`
    await waitFor(`getComputedStyle(${star}).color === 'rgb(69, 26, 3)'`)
    assert.equal(await evaluate(`getComputedStyle(${star}).backgroundColor`), 'rgb(251, 191, 36)')
    await evaluate(`{
    const originalFetch = window.fetch;
    window.coverSaved = false;
    window.fetch = (input, init) => {
      const path = new URL(input, location.origin).pathname;
      if (path === '/jav/idols') return Promise.resolve(Response.json({items: [{id: 1, name: 'Example idol', favorite_count: 1}], total: 1}));
      if (path.endsWith('/cover-options')) return Promise.resolve(Response.json({items: []}));
      if (path === '/jav/idols/1/cover') {window.coverSaved = true; return Promise.resolve(Response.json({id: 1, name: 'Example idol'}));}
      return originalFetch(input, init);
    };
    document.querySelector('aside button[aria-label="Idols"]').click();
  }`)
    await waitFor(`document.querySelector('[aria-label="Edit idol cover"]')`)
    const idolStar = `document.querySelector('[aria-label="Add to idol favorite groups"]')`
    assert.equal(await evaluate(`getComputedStyle(${idolStar}).color`), 'rgb(69, 26, 3)')
    await evaluate(`document.querySelector('[aria-label="Edit idol cover"]').click()`)
    const save = `Array.from(document.querySelectorAll('[aria-label="Idol cover"] button')).find(b => b.textContent === 'Save')`
    await waitFor(`${save} && !${save}.disabled`)
    assert.equal(await evaluate(`getComputedStyle(${save}).backgroundColor`), 'rgb(59, 130, 246)')
    await evaluate(`${save}.click()`)
    await waitFor(`window.coverSaved && !document.querySelector('[aria-label="Idol cover"]')`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
