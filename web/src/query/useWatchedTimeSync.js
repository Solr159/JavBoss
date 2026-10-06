import { useEffect } from 'react'
import { useStore } from '@/store'
import { fetchWatchedTimes } from '@/features/playback/api'
import { startWatchedTimeSync } from '@/query/watchedTimeSync'

export default function useWatchedTimeSync() {
  useEffect(() => startWatchedTimeSync({ store: useStore, fetchSnapshot: fetchWatchedTimes }), [])
}
