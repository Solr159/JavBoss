import { useEffect, useRef, useState } from 'react'

const retryDelays = [3000, 10000, 30000]

// Remount the retry state when the video or cover version changes.
export default function VideoThumbnail({ src, ...props }) {
  return <ThumbnailImage key={src} src={src} {...props} />
}

function ThumbnailImage({ src, onLoad, onError, style, alt, ...props }) {
  const [attempt, setAttempt] = useState(0)
  const [failed, setFailed] = useState(false)
  const timer = useRef(null)

  useEffect(() => () => clearTimeout(timer.current), [])

  const retrySrc = attempt
    ? `${src}${src.includes('?') ? '&' : '?'}thumbnail_retry=${attempt}`
    : src

  const handleError = (event) => {
    setFailed(true)
    if (timer.current !== null) return
    if (attempt < retryDelays.length) {
      timer.current = setTimeout(() => {
        timer.current = null
        setAttempt((current) => current + 1)
      }, retryDelays[attempt])
    } else {
      onError?.(event)
    }
  }

  return (
    <img
      {...props}
      src={retrySrc}
      alt={alt}
      style={{ ...style, ...(failed ? { visibility: 'hidden' } : {}) }}
      onError={handleError}
      onLoad={(event) => {
        clearTimeout(timer.current)
        timer.current = null
        setFailed(false)
        onLoad?.(event)
      }}
    />
  )
}
