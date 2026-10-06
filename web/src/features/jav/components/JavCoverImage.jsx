import { useState } from 'react'

const portraitAspectRatio = 0.7
const landscapeMinRatio = 1.35
const landscapeMaxRatio = 1.65
const cropFraction = 10 / 21

export default function JavCoverImage({ src, alt, portraitMode = false }) {
  const [loadedImage, setLoadedImage] = useState(null)
  const aspectRatio = loadedImage?.src === src ? loadedImage.aspectRatio : 0
  const cropRight =
    portraitMode && aspectRatio >= landscapeMinRatio && aspectRatio <= landscapeMaxRatio
  const croppedAspectRatio = aspectRatio * cropFraction
  // Fit the exact right-hand slice without stretching it or cropping it again.
  const frameStyle = cropRight
    ? {
        width: `${Math.min(1, croppedAspectRatio / portraitAspectRatio) * 100}%`,
        height: `${Math.min(1, portraitAspectRatio / croppedAspectRatio) * 100}%`,
      }
    : { width: '100%', height: '100%' }

  return (
    <div className="flex h-full w-full justify-center overflow-hidden">
      <div className="relative overflow-hidden" style={frameStyle}>
        <img
          src={src}
          alt={alt}
          className="h-full w-full object-contain object-top"
          style={
            cropRight
              ? {
                  position: 'absolute',
                  right: 0,
                  width: `${100 / cropFraction}%`,
                  maxWidth: 'none',
                  objectFit: 'fill',
                }
              : undefined
          }
          loading="lazy"
          onLoad={(event) => {
            const { naturalWidth, naturalHeight } = event.currentTarget
            setLoadedImage({
              src,
              aspectRatio: naturalHeight > 0 ? naturalWidth / naturalHeight : 0,
            })
          }}
        />
      </div>
    </div>
  )
}
