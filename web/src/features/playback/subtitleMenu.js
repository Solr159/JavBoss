// Extend the native menu so track selection, keyboard navigation and dismissal
// stay owned by Video.js. The extra action keeps CC available without tracks.
export function registerSubtitleMenu(videojs) {
  const SubsCapsButton = videojs.getComponent('SubsCapsButton')
  const MenuItem = videojs.getComponent('MenuItem')
  const OffTextTrackMenuItem = videojs.getComponent('OffTextTrackMenuItem')
  const CaptionSettingsMenuItem = videojs.getComponent('CaptionSettingsMenuItem')
  class FindSubtitleItem extends MenuItem {
    handleClick(event) {
      super.handleClick(event)
      this.player().trigger('javbossloadsubtitles')
    }
  }
  class SubtitleButton extends SubsCapsButton {
    createItems() {
      const items = super.createItems()
      const tracks = Array.from(this.player().textTracks())
      const downloads = tracks.filter((track) => track.id?.startsWith('javboss:download:'))
      for (const item of items) {
        let label
        if (item instanceof OffTextTrackMenuItem) label = '字幕：隐藏'
        else if (tracks.includes(item.track)) {
          const track = item.track
          const downloaded = downloads.includes(track)
          const source = downloaded
            ? '已下载字幕'
            : track.id?.startsWith('javboss:local:')
              ? '同目录字幕'
              : '内嵌字幕'
          const language =
            track.language || (downloaded ? track.id.slice('javboss:download:'.length) : '')
          label = `字幕：${source}`
          const showLanguage = downloaded
            ? downloads.length > 1
            : !track.id?.startsWith('javboss:local:')
          if (showLanguage && language) label += ` · ${language}`
        }
        if (label) {
          // Native track labels can be read-only. Relabel the menu while keeping
          // Video.js selection handlers, accessibility text and caption icons.
          item.options({ label })
          item.$('.vjs-menu-item-text').firstChild.textContent = label
        }
      }
      const order = (item) =>
        item instanceof CaptionSettingsMenuItem ? 2 : item instanceof OffTextTrackMenuItem ? 1 : 0
      items.sort((a, b) => order(a) - order(b))
      return downloads.length
        ? items
        : [new FindSubtitleItem(this.player(), { label: '查找字幕', selectable: false }), ...items]
    }
  }
  videojs.registerComponent('JavBossSubtitleButton', SubtitleButton)
}
