import { createListResource } from '@/query/createListResource'
import {
  videoQuery,
  javQuery,
  idolQuery,
  studioQuery,
  seriesQuery,
  listQueryKey,
} from '@/query/listQueries'
import { fetchVideos } from '@/features/video/api'
import { fetchJavs, fetchJavIdols, fetchJavStudios, fetchJavSeries } from '@/features/jav/api'
import { getErrorMessage } from '@/utils/errors'

export function createListResources({ get, set }) {
  const definitions = {
    video: {
      query: videoQuery,
      fetcher: fetchVideos,
      random: (state) => state.randomMode,
      hasNextField: 'hasNext',
      fields: {
        page: 'page',
        items: 'videos',
        total: 'total',
        loading: 'loading',
        loadingMore: 'videoLoadingMore',
        error: 'error',
      },
    },
    jav: {
      query: javQuery,
      fetcher: fetchJavs,
      randomTotal: true,
      random: (state) => state.javRandomMode,
    },
    idol: { query: idolQuery, fetcher: fetchJavIdols },
    studio: { query: studioQuery, fetcher: fetchJavStudios },
    series: { query: seriesQuery, fetcher: fetchJavSeries },
  }
  return Object.fromEntries(
    Object.entries(definitions).map(([name, definition]) => [
      name,
      createListResource({
        get,
        set,
        errorMessage: getErrorMessage,
        fields: {
          page: `${name}Page`,
          items: `${name}Items`,
          total: `${name}Total`,
          loading: `${name}Loading`,
          loadingMore: `${name}LoadingMore`,
          error: `${name}Error`,
        },
        key: (state) => listQueryKey(definition.query, state),
        active: (state) =>
          name === 'video'
            ? state.viewMode === 'video'
            : state.viewMode === 'jav' && state.javTab === (name === 'jav' ? 'list' : name),
        waterfall: (state) => Boolean(state.waterfallModes?.[name]),
        ...definition,
      }),
    ])
  )
}
