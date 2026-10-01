import { StrictMode, useCallback, useEffect, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import useUrlStateSync from '@/hooks/useUrlStateSync'
import { JavDetailNavigationContext } from '@/hooks/javDetailNavigation'
import { StudioDetailNavigationContext } from '@/hooks/studioDetailNavigation'
import JavGrid from '@/components/JavGrid'
import JavDetailRoute from '@/components/JavDetailRoute'
import JavStudioDetailModal from '@/components/JavStudioDetailModal'
import { useStore } from '@/store'
import { buildUrlFromState, parseUrlState } from '@/utils/urlState'
import '@/index.css'

const studio = {
  id: 2,
  name: 'Test studio',
  code_prefixes: [{ prefix: 'ABC', work_count: 3 }],
  series: [],
}
const item = {
  id: 1,
  code: 'ABC-001',
  title: 'Test JAV',
  studio,
  series: { id: 3, name: 'Test series' },
  idols: [{ id: 4, name: 'Test actress' }],
  tags: Array.from({ length: 80 }, (_, i) => ({ id: i + 1, name: `Tag ${i + 1}` })),
  videos: [],
  sample_images: ['not_found'],
}
useStore.setState({ javItems: location.search.includes('jav_detail=') ? [] : [item] })
window.javDetailRequests = 0
window.fixtureLoadId = Math.random()
window.fetch = async (url) => {
  if (String(url) === '/jav/items/1') {
    window.javDetailRequests++
    return Response.json(item)
  }
  if (String(url) === '/jav/studios/2') return Response.json(studio)
  if (String(url).includes('/sample-images')) return Response.json({ sample_images: [] })
  return new Response('Not found', { status: 404 })
}
function Fixture() {
  const [state, setState] = useState(() => parseUrlState(location.search, { defaultView: 'jav' }))
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
    route.navigateFromJavDetail(() =>
      setState((current) => ({ ...current, jav: { ...current.jav, ...filters } }))
    )
  const actions = {
    buildJavUrl: (options) =>
      buildUrlFromState({ ...state, jav: { ...state.jav, ...options } }, location.pathname),
    onStudioClick: (value) => select({ studioId: value.id }),
    onSeriesClick: (value) => select({ seriesId: value.id }),
    onIdolClick: (value) => select({ idolIds: [value.id] }),
    onTagClick: (value) => select({ tagIds: [value.id] }),
  }
  return (
    <StudioDetailNavigationContext.Provider value={route.openStudioDetail}>
      <JavDetailNavigationContext.Provider value={route.openJavDetail}>
        <div style={{ minHeight: 2600 }}>
          <div style={{ width: 400, marginTop: 500 }}>
            {!route.javDetailId && !state.jav.studioId && !state.jav.seriesId ? (
              <JavGrid items={[item]} {...actions} />
            ) : null}
          </div>
          {route.javDetailId ? (
            <JavDetailRoute
              key={route.javDetailId}
              {...actions}
              itemId={route.javDetailId}
              initialItem={route.javDetailItem}
              initialState={route.javDetailState}
              onLoaded={route.cacheJavDetail}
              onStateChange={route.saveJavDetailState}
              onClose={route.closeJavDetail}
            />
          ) : null}
          {route.studioDetailId ? (
            <JavStudioDetailModal
              key={route.studioDetailKey}
              studioId={route.studioDetailId}
              initialItem={route.studioDetailItem}
              initialState={route.studioDetailState}
              onLoaded={route.cacheStudioDetail}
              onStateChange={route.saveStudioDetailState}
              onClose={route.closeStudioDetail}
              onSelectStudio={actions.onStudioClick}
              buildJavUrl={actions.buildJavUrl}
            />
          ) : null}
          <button id="nested-studio" onClick={() => route.openStudioDetail(studio)}>
            Open studio
          </button>
        </div>
      </JavDetailNavigationContext.Provider>
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
