import { Fade, Tooltip } from '@mui/material'
import PlayCircleFilledIcon from '@mui/icons-material/PlayCircleFilled'
import { zh } from '@/utils/i18n'

export default function WatchTimeIcons({ watchedMs }) {
  const value = Number(watchedMs)
  const totalMs = Number.isFinite(value) && value > 0 ? value : 0
  if (totalMs === 0) return null
  const totalSeconds = Math.floor(totalMs / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor(totalSeconds / 60) % 60
  const seconds = totalSeconds % 60
  const timeText =
    [
      hours > 0 ? zh(`${hours} 小时`, `${hours} h`) : '',
      minutes > 0 ? zh(`${minutes} 分钟`, `${minutes} min`) : '',
      seconds > 0 ? zh(`${seconds} 秒`, `${seconds} s`) : '',
    ]
      .filter(Boolean)
      .join(' ') || zh('不足 1 秒', 'Less than 1 second')
  const label = zh(`已观看：${timeText}`, `Watched: ${timeText}`)

  return (
    <Tooltip
      title={
        <>
          {label}
          <br />
          {zh(
            '每个图标代表 30 分钟，深色填充表示已观看进度',
            'Each icon represents 30 minutes; the dark fill shows time watched'
          )}
        </>
      }
      placement="top"
      arrow
      slots={{ transition: Fade }}
    >
      <span
        role="img"
        aria-label={label}
        className="watch-time-icons inline-flex shrink-0 items-center gap-0.5 text-teal-600"
      >
        {[0, 1, 2].map((index) => {
          const progress = Math.min(1, Math.max(0, totalMs / 1800000 - index))
          return (
            <span
              key={index}
              aria-hidden="true"
              className="relative inline-flex h-4 w-4"
              // Rasterize partial fills separately so fractional card positions
              // don't round away their subpixel clip widths.
              style={{ willChange: progress > 0 && progress < 1 ? 'transform' : undefined }}
            >
              <PlayCircleFilledIcon
                viewBox="2 2 20 20"
                className="text-teal-100"
                sx={{ fontSize: 16 }}
              />
              <PlayCircleFilledIcon
                viewBox="2 2 20 20"
                className="absolute inset-0 text-teal-600"
                sx={{ fontSize: 16 }}
                style={{ clipPath: `inset(0 ${100 - progress * 100}% 0 0)` }}
              />
            </span>
          )
        })}
      </span>
    </Tooltip>
  )
}
