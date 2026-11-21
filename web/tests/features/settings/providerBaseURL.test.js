import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../../helpers/modules.js'

test('provider base url updates are sent as PUT requests with the raw value', async (t) => {
  const [{ updateProviderBaseURL }] = await loadModules(t, ['features/settings/api.js'])
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    requests.push({ url, method: init.method, body: JSON.parse(init.body) })
    return Response.json({
      provider: 1,
      base_url: 'https://mirror.javbus.test',
      default_base_url: 'https://www.javbus.com',
      domain: 'mirror.javbus.test',
    })
  })

  const updated = await updateProviderBaseURL(1, ' https://mirror.javbus.test ')
  assert.deepEqual(requests, [
    {
      url: '/jav/providers/1/base_url',
      method: 'PUT',
      body: { base_url: ' https://mirror.javbus.test ' },
    },
  ])
  assert.equal(updated.default_base_url, 'https://www.javbus.com')
  assert.equal(updated.domain, 'mirror.javbus.test')

  await updateProviderBaseURL('avwiki', '')
  assert.deepEqual(requests.at(-1), {
    url: '/jav/providers/avwiki/base_url',
    method: 'PUT',
    body: { base_url: '' },
  })
})

test('an invalid provider base url surfaces the localized backend error', async (t) => {
  const [{ updateProviderBaseURL }] = await loadModules(t, ['features/settings/api.js'])
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({ error_zh: '数据源地址无效', error_en: 'Invalid provider URL' }, { status: 400 })
  )

  await assert.rejects(
    () => updateProviderBaseURL(1, 'https://mirror.javbus.test/genre'),
    /数据源地址无效|Invalid provider URL/
  )
})
