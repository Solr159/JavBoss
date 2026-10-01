import { createServer } from 'vite'

// Load application modules with the same aliases as the frontend, without a browser.
export async function loadModules(t, paths) {
  const server = await createServer({
    server: { middlewareMode: true },
    appType: 'custom',
    logLevel: 'silent',
  })
  t.after(() => server.close())
  return Promise.all(paths.map((path) => server.ssrLoadModule(`/src/${path}`)))
}
