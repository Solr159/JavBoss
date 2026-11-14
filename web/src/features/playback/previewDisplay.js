import { fetchVideoPreview } from '@/features/playback/api'

const INTERVAL = 3
const HOVER_DELAY = 100
const MAX_IMAGES = 48

export function registerPreviewDisplay(videojs) {
  const MouseTimeDisplay = videojs.getComponent('MouseTimeDisplay')

  class PreviewDisplay extends MouseTimeDisplay {
    constructor(player, options) {
      super(player, options)
      this.images = new Map()
      this.source = null
      this.bucket = null
      this.retryAfter = 0
      this.image = document.createElement('img')
      this.image.className = 'player-timeline-preview'
      this.image.alt = ''
      this.image.hidden = true
      this.el().appendChild(this.image)
      this.image.onload = () => {
        this.image.hidden = !this.source || this.images.get(this.bucket) !== this.image.src
      }
      this.image.onerror = () => {
        this.image.hidden = true
      }
      player.ready(() => {
        if (this.isDisposed()) return
        this.progress = player.getChild('controlBar').getChild('progressControl').el()
        this.on(this.progress, 'mouseleave', () => this.cancel())
      })
      this.on(player, ['userinactive', 'loadstart'], () => this.cancel())
    }

    setSource(source) {
      this.cancel()
      for (const url of this.images.values()) URL.revokeObjectURL(url)
      this.images.clear()
      this.source = source
      this.retryAfter = 0
    }

    cancel() {
      this.clearTimeout(this.timer)
      this.timer = null
      this.request?.abort()
      this.request = null
      this.bucket = null
      this.image.hidden = true
      this.image.removeAttribute('src')
    }

    update(rect, point) {
      super.update(rect, point)
      const duration = this.player().duration()
      // Video.js throttles mouse updates; a trailing update may arrive after leave.
      if (
        !this.source ||
        !this.progress?.matches(':hover') ||
        !Number.isFinite(duration) ||
        duration <= 0
      )
        return
      const time = Math.min(Math.max(0, point * duration), Math.max(0, duration - 0.001))
      const bucket = Math.floor(time / INTERVAL) * INTERVAL
      const width = Math.min(192, rect.width)
      const position = rect.width * point
      this.image.style.width = `${width}px`
      this.image.style.left = `${Math.max(0, Math.min(position - width / 2, rect.width - width)) - position}px`
      if (bucket === this.bucket) return
      this.cancel()
      this.bucket = bucket
      const cached = this.images.get(bucket)
      if (cached) {
        this.images.delete(bucket)
        this.images.set(bucket, cached)
        this.image.src = cached
        return
      }
      this.timer = this.setTimeout(() => this.load(bucket), HOVER_DELAY)
    }

    async load(bucket) {
      if (!this.source || Date.now() < this.retryAfter) {
        this.bucket = null
        return
      }
      const source = this.source
      const request = new AbortController()
      this.request = request
      try {
        const blob = await fetchVideoPreview(source.id, source.locationId, bucket, request.signal)
        if (request.signal.aborted || this.source !== source || this.bucket !== bucket) return
        const url = URL.createObjectURL(blob)
        this.images.set(bucket, url)
        if (this.images.size > MAX_IMAGES) {
          const oldest = this.images.keys().next().value
          URL.revokeObjectURL(this.images.get(oldest))
          this.images.delete(oldest)
        }
        this.image.src = url
      } catch {
        if (!request.signal.aborted && this.source === source) {
          this.retryAfter = Date.now() + 3000
          this.bucket = null
        }
      } finally {
        if (this.request === request) this.request = null
      }
    }

    dispose() {
      this.setSource(null)
      this.image.onload = null
      this.image.onerror = null
      super.dispose()
    }
  }

  videojs.registerComponent('JavBossPreviewDisplay', PreviewDisplay)
}
