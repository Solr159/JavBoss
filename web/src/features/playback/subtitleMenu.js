// Extend the native menu so track selection, keyboard navigation and dismissal
// stay owned by Video.js. The extra action keeps CC available without tracks.
export function registerSubtitleMenu(videojs) {
  const SubsCapsButton = videojs.getComponent('SubsCapsButton')
  const MenuItem = videojs.getComponent('MenuItem')
  class FindSubtitleItem extends MenuItem {
    handleClick(event) {
      super.handleClick(event)
      this.player().trigger('javbossloadsubtitles')
    }
  }
  class SubtitleButton extends SubsCapsButton {
    createItems() {
      const items = super.createItems()
      const hasDownloaded = Array.from(this.player().textTracks()).some((track) =>
        track.id?.startsWith('javboss:download:')
      )
      return hasDownloaded
        ? items
        : [new FindSubtitleItem(this.player(), { label: '查找字幕', selectable: false }), ...items]
    }
  }
  videojs.registerComponent('JavBossSubtitleButton', SubtitleButton)
}
