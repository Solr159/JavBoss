import { StrictMode, useCallback, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import useUrlStateSync from '@/hooks/useUrlStateSync'
import { StudioDetailNavigationContext } from '@/hooks/studioDetailNavigation'
import { StudioCard } from '@/components/JavStudioView'
import JavStudioDetailModal from '@/components/JavStudioDetailModal'
import JavFavoriteModal from '@/components/JavFavoriteModal'
import { buildUrlFromState, parseUrlState } from '@/utils/urlState'
import '@/index.css'

const studio = {
  id: 1,
  name: 'Test studio',
  aliases: ['Alias'],
  work_count: 100,
  code_prefixes: [
    { prefix: 'ABC', work_count: 5 },
    { prefix: 'XYZ', work_count: 12 },
  ],
  series: Array.from({ length: 60 }, (_, i) => ({
    id: i + 1,
    name: `Series ${i + 1}`,
    work_count: i + 1,
  })),
}
window.pendingStudioResponses = []
window.holdStudioResponses = false
window.releaseStudioResponses = (fail = false) => {
  window.holdStudioResponses = false
  for (const respond of window.pendingStudioResponses.splice(0)) respond(fail)
}
const originalFetch = window.fetch
window.fetch = (url, options) => {
  if (String(url) === '/jav/studios/1') {
    if (window.holdStudioResponses) {
      return new Promise((resolve) =>
        window.pendingStudioResponses.push((fail) =>
          resolve(
            fail
              ? new Response('Unavailable', { status: 503 })
              : Response.json({ ...studio, name: 'Updated studio' })
          )
        )
      )
    }
    return Promise.resolve(Response.json(studio))
  }
  if (String(url).startsWith('/jav/studios/'))
    return Promise.resolve(new Response('Not found', { status: 404 }))
  return originalFetch(url, options)
}

function Fixture() {
  const [state, setState] = useState(() =>
    parseUrlState(window.location.search, { defaultView: 'jav' })
  )
  const [favorite, setFavorite] = useState(null)
  const applyUrlState = useCallback((parsed) => setState(parsed), [])
  const route = useUrlStateSync({
    applyUrlState,
    currentUrlState: state,
    configLoaded: true,
    hydrated: true,
    initialViewMode: 'jav',
  })
  useEffect(() => route.schedulePendingScrollRestore(), [route])
  const select = (filters) =>
    route.navigateFromStudioDetail(() =>
      setState((current) => ({
        ...current,
        jav: { ...current.jav, tab: 'list', page: 1, ...filters },
      }))
    )
  return (
    <StudioDetailNavigationContext.Provider value={route.openStudioDetail}>
      <div style={{ minHeight: 2600 }}>
        <div id="route-state">
          {state.jav.tab}:{state.jav.seriesId || state.jav.prefix || state.jav.studioId || ''}
        </div>
        <div style={{ width: 300, marginTop: 500 }}>
          {!route.studioDetailId ? <StudioCard item={studio} /> : null}
        </div>
        {route.studioDetailId ? (
          <JavStudioDetailModal
            key={route.studioDetailKey}
            studioId={route.studioDetailId}
            initialItem={route.studioDetailItem}
            onLoaded={route.cacheStudioDetail}
            initialState={route.studioDetailState}
            onStateChange={route.saveStudioDetailState}
            onClose={route.closeStudioDetail}
            onSelectSeries={(series) => select({ seriesId: series.id })}
            onSelectStudio={(item) => select({ studioId: item.id })}
            onSelectPrefix={(prefix) => select({ prefix: prefix.prefix })}
            onOpenSeriesFavorites={setFavorite}
            buildJavUrl={(options) =>
              buildUrlFromState(
                { ...state, jav: { ...state.jav, ...options } },
                window.location.pathname
              )
            }
          />
        ) : null}
        <JavFavoriteModal
          open={Boolean(favorite)}
          idol={favorite}
          entityType="series"
          groups={[]}
          selectedIds={[]}
          onClose={() => setFavorite(null)}
          onSave={() => setFavorite(null)}
        />
      </div>
    </StudioDetailNavigationContext.Provider>
  )
}
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <BrowserRouter>
      <Fixture />
    </BrowserRouter>
  </StrictMode>
)
