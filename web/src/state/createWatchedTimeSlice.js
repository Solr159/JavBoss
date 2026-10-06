import { mergeWatchedTimeTotals } from '@/features/playback/watchedTimeState'

export function createWatchedTimeSlice({ set }) {
  return {
    watchedTimes: { videos: {}, javs: {} },
    updateWatchedTimes: (snapshot) =>
      set((state) => {
        const watchedTimes = mergeWatchedTimeTotals(state.watchedTimes, snapshot)
        if (watchedTimes === state.watchedTimes) return state
        return { watchedTimes, videos: state.videos, javItems: state.javItems }
      }),
  }
}
