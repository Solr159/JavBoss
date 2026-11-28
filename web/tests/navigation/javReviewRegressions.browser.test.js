import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../helpers/browser.js'

const tab = async (command, backwards = false) => {
  for (const type of ['keyDown', 'keyUp']) {
    await command('Input.dispatchKeyEvent', {
      type,
      key: 'Tab',
      code: 'Tab',
      windowsVirtualKeyCode: 9,
      modifiers: backwards ? 8 : 0,
    })
  }
}

test(
  'Tab and Shift+Tab traverse cover actions after rating focus',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-review-keyboard-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=jav` })
    await waitFor(`document.querySelector('.jav-card') && !window.testStore.getState().javLoading`)
    for (const value of [0, 3]) {
      await evaluate(
        `window.testStore.setState(state => ({javItems: state.javItems.map(item => ({...item, favorite_rating: ${value}}))}))`
      )
      await evaluate(
        `document.querySelector('.jav-card [aria-label="View ABC-001 details"]').focus()`
      )
      await tab(command)
      await waitFor(`document.querySelector('.jav-card-rating-editing')`)
      const visited = []
      for (let i = 0; i < 20; i++) {
        await tab(command)
        const label = await evaluate(`document.activeElement.getAttribute('aria-label')`)
        if (
          ['Select ABC-001', 'Edit JAV', 'Edit custom tags', 'Add to JAV favorite groups'].includes(
            label
          )
        ) {
          visited.push(label)
          await waitFor(`!document.querySelector('.jav-card-rating-editing')`)
          await waitFor(`getComputedStyle(document.activeElement).opacity === '1'`)
        }
        if (label === 'Add to JAV favorite groups') break
      }
      assert.deepEqual(visited, [
        'Select ABC-001',
        'Edit JAV',
        'Edit custom tags',
        'Add to JAV favorite groups',
      ])
      for (const expected of ['Edit custom tags', 'Edit JAV']) {
        await tab(command, true)
        assert.equal(await evaluate(`document.activeElement.getAttribute('aria-label')`), expected)
        await waitFor(`getComputedStyle(document.activeElement).opacity === '1'`)
      }
      for (let i = 0; i < 20; i++) {
        await tab(command, true)
        if (
          await evaluate(
            `Boolean(document.activeElement.closest('[aria-label="Favorite rating"]'))`
          )
        )
          break
      }
      assert.equal(
        await evaluate(`Boolean(document.activeElement.closest('[aria-label="Favorite rating"]'))`),
        true
      )
      await waitFor(`document.querySelector('.jav-card-rating-editing')`)
    }
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)

test(
  'relation link URLs match normal navigation with count and type filters active',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-review-links-test',
    })
    const fields = [
      'actor_counts',
      'movie_type',
      'solo',
      'idol_ids',
      'studio_id',
      'series_id',
      'tag_ids',
      'page',
    ]
    for (const [name, filter, id] of [
      ['Review Idol', 'idol_ids', '1'],
      ['Review Studio', 'studio_id', '2'],
      ['Review Series', 'series_id', '3'],
      ['Review Tag', 'tag_ids', '4'],
    ]) {
      await command('Page.navigate', {
        url: `${origin}/tests/fixtures/app.html?view=jav&actor_counts=2&movie_type=uncensored&solo=1`,
      })
      await waitFor(
        `document.querySelector('.jav-card') && !window.testStore.getState().javLoading`
      )
      await evaluate(
        `window.testStore.setState(state => ({javItems: state.javItems.map(item => ({...item, idols:[{id:1,name:'Review Idol'}],studio:{id:2,name:'Review Studio'},series:{id:3,name:'Review Series'},tags:[{id:4,name:'Review Tag',provider:3}]}))}))`
      )
      const link = `[...document.querySelectorAll('.jav-card a')].find(a => a.textContent.trim() === ${JSON.stringify(name)})`
      await waitFor(link)
      const href = await evaluate(`${link}.href`)
      const params = new URL(href).searchParams
      for (const field of ['actor_counts', 'movie_type', 'solo'])
        assert.equal(params.has(field), false, `${name}: ${field}`)
      assert.equal(params.get(filter), id)
      await evaluate(`${link}.click()`)
      await waitFor(
        `new URLSearchParams(location.search).get('${filter}') === '${id}' && !window.testStore.getState().javActorCounts.length && !window.testStore.getState().javMovieType`
      )
      const actual = new URL(await evaluate('location.href')).searchParams
      assert.deepEqual(
        fields.map((field) => actual.get(field)),
        fields.map((field) => params.get(field)),
        name
      )
    }
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
