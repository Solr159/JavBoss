export function createNavigationSlice({ set }) {
  return {
    waterfallModes: { video: false, jav: false, idol: false, studio: false, series: false },
    setWaterfallModes: (update) =>
      set((state) => ({ waterfallModes: update(state.waterfallModes) })),
    viewMode: 'video',
    javTab: 'list',
    clearLegacyDownloadTab: () => set({ javTab: 'list' }),
    setViewMode: (mode) => {
      if (mode !== 'video' && mode !== 'jav') return
      set({
        viewMode: mode,
        ...(mode === 'jav' ? { videoTempSort: '' } : { javTempSort: '', idolTempSort: '' }),
      })
    },
    setJavTab: (tab) => {
      if (
        tab !== 'list' &&
        tab !== 'idol' &&
        tab !== 'studio' &&
        tab !== 'series' &&
        tab !== 'download'
      )
        return
      set({ javTab: tab, javTempSort: '', idolTempSort: '' })
    },
  }
}
