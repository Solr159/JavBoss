import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'editor fields share dark colors, preserve layout, and distinguish focus, errors and disabled state',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-form-fields-test',
    })
    await command('Emulation.setFocusEmulationEnabled', { enabled: true })
    const setTheme = async (theme) => {
      await evaluate(`localStorage.setItem('javboss.theme', '${theme}');
        window.dispatchEvent(new StorageEvent('storage', {key: 'javboss.theme'}))`)
      await waitFor(`document.documentElement.style.colorScheme === '${theme}'`)
    }
    const boxes = `Array.from(document.querySelectorAll('.form-field, .form-field-group')).map(el => {
      const {width, height} = el.getBoundingClientRect(); return [width, height];
    })`
    for (const dialog of ['jav', 'idol', 'query', 'scrape']) {
      const previousTimeOrigin = await evaluate('performance.timeOrigin')
      await command('Page.navigate', {
        url: `${origin}/tests/fixtures/formFields.html?dialog=${dialog}`,
      })
      await waitFor(`performance.timeOrigin !== ${previousTimeOrigin}`)
      await waitFor(`document.querySelector('.form-field')`)
      await command('Emulation.setFocusEmulationEnabled', { enabled: true })
      await setTheme('light')
      const lightBoxes = await evaluate(boxes)
      await setTheme('dark')
      assert.deepEqual(await evaluate(boxes), lightBoxes, `${dialog}: theme preserves layout`)
      const fields =
        await evaluate(`Array.from(document.querySelectorAll('.form-field, .form-field-group'))
        .filter(el => !el.matches(':disabled')).map(el => {
          const style = getComputedStyle(el); return [style.backgroundColor, style.color];
        })`)
      assert.ok(fields.length > 0)
      for (const colors of fields) {
        assert.deepEqual(colors, ['rgb(24, 24, 24)', 'rgb(242, 242, 242)'], dialog)
      }
      const input = `document.querySelector('input.form-field:not(:disabled), textarea.form-field')`
      await command('Page.bringToFront')
      await command('Emulation.setFocusEmulationEnabled', { enabled: false })
      await command('Emulation.setFocusEmulationEnabled', { enabled: true })
      await evaluate(`${input}.focus()`)
      await waitFor(`document.activeElement === ${input}`)
      assert.equal(await evaluate(`${input}.matches(':focus')`), true, `${dialog}: field focused`)
      assert.equal(
        await evaluate(`getComputedStyle(${input}).borderColor`),
        'rgb(59, 130, 246)',
        dialog
      )
      const inlineFields =
        await evaluate(`Array.from(document.querySelectorAll('.form-field-input'))
        .map(el => getComputedStyle(el).backgroundColor)`)
      for (const color of inlineFields) assert.equal(color, 'rgba(0, 0, 0, 0)')
    }

    // Exercise real validation and saving state rather than assigning CSS classes in the test.
    const code = `document.querySelector('input[aria-label="Specified code"]')`
    await evaluate(`Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(${code}, 'BAD!');
      ${code}.dispatchEvent(new Event('input', {bubbles: true}))`)
    await waitFor(`${code}.getAttribute('aria-invalid') === 'true'`)
    await evaluate(`${code}.focus()`)
    assert.equal(await evaluate(`getComputedStyle(${code}).borderColor`), 'rgb(248, 113, 113)')
    assert.equal(
      await evaluate(`getComputedStyle(${code}).getPropertyValue('--tw-ring-color').trim()`),
      'rgb(248 113 113 / 0.25)'
    )
    await evaluate('window.setFixtureSaving(true)')
    await waitFor(`${code}.disabled`)
    assert.equal(await evaluate(`getComputedStyle(${code}).backgroundColor`), 'rgb(48, 48, 48)')
    assert.equal(await evaluate(`getComputedStyle(${code}).color`), 'rgb(163, 163, 163)')
    await setTheme('light')
    assert.equal(await evaluate(`getComputedStyle(${code}).backgroundColor`), 'rgb(249, 250, 251)')
  }
)
