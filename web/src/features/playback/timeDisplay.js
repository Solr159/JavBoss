export function registerTimeDisplay(videojs) {
  const CurrentTimeDisplay = videojs.getComponent('CurrentTimeDisplay')

  class PlayerCurrentTimeDisplay extends CurrentTimeDisplay {
    updateContent(event) {
      // Video.js 8 sends a SeekBar timeupdate after clearing pendingSeekTime.
      // Its time display formats that null as zero. Fall back to the native
      // current-time path, retaining valid pending targets (including zero).
      if (
        typeof event?.target?.pendingSeekTime === 'function' &&
        !Number.isFinite(event.target.pendingSeekTime())
      ) {
        super.updateContent()
        return
      }
      super.updateContent(event)
    }
  }

  videojs.registerComponent('JavBossCurrentTimeDisplay', PlayerCurrentTimeDisplay)
}
