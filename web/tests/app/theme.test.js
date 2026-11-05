import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import { normalizeTheme, resolveTheme, THEME_STORAGE_KEY } from '../../src/app/theme/preferences.js'

const bootstrap = await readFile(new URL('../../public/theme-init.js', import.meta.url), 'utf8')

for (const [saved, systemDark, expected] of [
  [null, true, 'light'],
  ['invalid', true, 'light'],
  ['light', true, 'light'],
  ['dark', false, 'dark'],
  ['system', true, 'dark'],
  ['system', false, 'light'],
]) {
  test(`first paint and React agree for ${saved} with system dark=${systemDark}`, () => {
    let darkClass = false
    const style = {}
    runInNewContext(bootstrap, {
      localStorage: {
        getItem: (key) => {
          assert.equal(key, THEME_STORAGE_KEY)
          return saved
        },
      },
      matchMedia: () => ({ matches: systemDark }),
      document: {
        documentElement: {
          classList: {
            toggle: (name, value) => {
              darkClass = value
            },
          },
          style,
        },
        querySelector: () => null,
      },
    })
    assert.equal(darkClass, expected === 'dark')
    assert.equal(style.colorScheme, expected)
    assert.equal(resolveTheme(normalizeTheme(saved), systemDark), expected)
  })
}

test('first paint falls back to light when browser storage is blocked', () => {
  const style = {}
  runInNewContext(bootstrap, {
    localStorage: {
      getItem: () => {
        throw new Error('blocked')
      },
    },
    document: {
      documentElement: { classList: { toggle: () => {} }, style },
      querySelector: () => null,
    },
  })
  assert.equal(style.colorScheme, 'light')
})
