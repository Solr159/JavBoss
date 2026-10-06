import { mergeWatchedTimeTotals } from '@/features/playback/watchedTimeState'

export function createWatchedTimeSlice({ set }) {
  return {
    watchedTimes: { videos: {}, javs: {} },
    updateWatchedTimes: (snapshot) =>
      set((state) => {
        const watchedTimes = mergeWatchedTimeTotals(state.watchedTimes, snapshot)
        if (watchedTimes === state.watchedTimes) return state
        // Live totals are independent of list snapshots; subscribers read them
        // without rewriting list objects or intercepting unrelated state updates.
        return { watchedTimes }
      }),
  }
}
