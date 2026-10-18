import { createServer } from 'vite'

// Load application modules with the same aliases as the frontend, without a browser.
export async function loadModules(t, paths) {
  const server = await createServer({
    plugins: [
      {
        name: 'unit-test-no-client-optimizer',
        configEnvironment(name, config) {
          if (name !== 'client') return
          // SSR-only tests need no browser bundles. Clear React's includes after
          // plugin configuration so cold runs do not start a new optimizer per test.
          config.optimizeDeps = { ...config.optimizeDeps, noDiscovery: true, include: [] }
        },
      },
    ],
    server: { middlewareMode: true },
    appType: 'custom',
    logLevel: 'silent',
  })
  t.after(() => server.close())
  return Promise.all(paths.map((path) => server.ssrLoadModule(`/src/${path}`)))
}
