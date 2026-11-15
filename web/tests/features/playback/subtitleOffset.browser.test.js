import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'subtitle offsets preserve milliseconds while buttons and arrow keys adjust by half a second',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t)
    await command('Page.navigate', { url: `${origin}/tests/fixtures/playerWindow.html` })
    await waitFor(`document.querySelector('#root > button')`)
    await evaluate(`{
      window.offsetSettings = {offset_ms: 125};
      window.offsetWrites = [];
      window.fetch = async (input, init = {}) => {
        if (String(input).endsWith('/subtitle-settings')) {
          if (init.method === 'PATCH') {
            window.offsetSettings = JSON.parse(init.body);
            window.offsetWrites.push(window.offsetSettings);
          }
          return Response.json(window.offsetSettings);
        }
        return Response.json({sources: []});
      };
      document.querySelector('#root > button').click();
    }`)
    const player = `document.querySelector('.video-js').player`
    const form = `document.querySelector('.player-subtitle-offset-settings')`
    const input = `${form}.querySelector('input')`
    await waitFor(`${player}`)
    const open = async () => {
      await evaluate(`${player}.getChild('textTrackSettings').open()`)
      await waitFor(`${form}?.querySelector('input:not(:disabled)')`)
    }
    const save = () => evaluate(`${form}.querySelector('button[type="submit"]').click()`)
    const setValue = (value) =>
      evaluate(`{
        const input = ${input};
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, ${JSON.stringify(value)});
        input.dispatchEvent(new Event('input', {bubbles: true}));
      }`)
    const clickStep = (direction) =>
      evaluate(`${form}.querySelector('button[aria-label="${direction} by 0.5 seconds"]').click()`)

    // Existing values from MPV must save without editing or rounding to 0.5s.
    for (const offset of [125, -125]) {
      await evaluate(`window.offsetSettings = {offset_ms: ${offset}}`)
      await open()
      assert.equal(await evaluate(`${input}.value`), String(offset / 1000))
      assert.equal(await evaluate(`${input}.validity.valid`), true)
      await save()
      await waitFor(`!${form}`)
      assert.equal(await evaluate(`window.offsetWrites.at(-1).offset_ms`), offset)
    }

    await evaluate(`window.offsetSettings = {offset_ms: 125}`)
    await open()
    await clickStep('Increase')
    assert.equal(await evaluate(`${input}.value`), '0.625')
    await clickStep('Decrease')
    assert.equal(await evaluate(`${input}.value`), '0.125')
    await evaluate(`${input}.focus()`)
    for (const [key, expected] of [
      ['ArrowDown', '-0.375'],
      ['ArrowUp', '0.125'],
    ]) {
      await command('Input.dispatchKeyEvent', { type: 'keyDown', key, code: key })
      await command('Input.dispatchKeyEvent', { type: 'keyUp', key, code: key })
      assert.equal(await evaluate(`${input}.value`), expected)
    }
    assert.equal(await evaluate(`window.offsetWrites.length`), 2)
    await setValue('-12.345')
    await save()
    await waitFor(`!${form}`)
    assert.equal(await evaluate(`window.offsetWrites.at(-1).offset_ms`), -12345)

    await open()
    for (const [value, direction, expected] of [
      ['3599.875', 'Increase', '3600'],
      ['-3599.875', 'Decrease', '-3600'],
    ]) {
      await setValue(value)
      await clickStep(direction)
      assert.equal(await evaluate(`${input}.value`), expected)
      await clickStep(direction)
      assert.equal(await evaluate(`${input}.value`), expected)
    }
    for (const [value, validity] of [
      ['', 'valueMissing'],
      ['3600.001', 'rangeOverflow'],
      ['-3600.001', 'rangeUnderflow'],
      ['0.0001', 'stepMismatch'],
    ]) {
      await setValue(value)
      await save()
      assert.equal(await evaluate(`${input}.validity.${validity}`), true)
      assert.equal(await evaluate(`window.offsetWrites.length`), 3)
    }
    assert.deepEqual(await evaluate(`window.appErrors`), [])
  }
)
