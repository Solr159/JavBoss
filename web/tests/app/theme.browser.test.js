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
    assert.equal(
      await evaluate(`getComputedStyle(document.querySelector('.skeuo-tag input')).accentColor`),
      'rgb(59, 130, 246)'
    )
    await evaluate(`document.querySelector('.skeuo-tag input').click()`)
    await waitFor(
      `getComputedStyle(document.querySelector('.skeuo-tag--selected')).borderTopColor === 'rgb(59, 130, 246)'`
    )
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
