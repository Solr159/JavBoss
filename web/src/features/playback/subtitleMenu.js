// Extend the native menu so track selection, keyboard navigation and dismissal
// stay owned by Video.js. The extra action keeps CC available without tracks.
export function registerSubtitleMenu(videojs) {
  const SubsCapsButton = videojs.getComponent('SubsCapsButton')
  const MenuItem = videojs.getComponent('MenuItem')
  const OffTextTrackMenuItem = videojs.getComponent('OffTextTrackMenuItem')
  const CaptionSettingsMenuItem = videojs.getComponent('CaptionSettingsMenuItem')
  const TextTrackSettings = videojs.getComponent('TextTrackSettings')
  class SubtitleSettings extends TextTrackSettings {
    setDefaults() {
      super.setDefaults()
      this.setValues({ backgroundOpacity: '0', windowOpacity: '0' })
    }
  }
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
        if (item instanceof OffTextTrackMenuItem) {
          label = '关闭字幕'
          item.selected(false)
          item.selectable = false
          item.el().setAttribute('role', 'menuitem')
          item.el().removeAttribute('aria-checked')
        } else if (tracks.includes(item.track)) {
          const track = item.track
          const downloaded = downloads.includes(track)
          const filename = track.label || `内嵌字幕 ${track.language || tracks.indexOf(track) + 1}`
          label = `${filename}（${downloaded ? '下载' : '本地'}）`
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
  videojs.registerComponent('JavBossTextTrackSettings', SubtitleSettings)
}
