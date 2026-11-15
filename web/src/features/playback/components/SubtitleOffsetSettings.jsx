import { useEffect, useId, useRef, useState } from 'react'
import KeyboardArrowUpRoundedIcon from '@mui/icons-material/KeyboardArrowUpRounded'
import KeyboardArrowDownRoundedIcon from '@mui/icons-material/KeyboardArrowDownRounded'
import { fetchVideoSubtitleSettings, updateVideoSubtitleSettings } from '@/features/playback/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function SubtitleOffsetSettings({ video, onSaved, onClose }) {
  const inputId = useId()
  const mountedRef = useRef(false)
  const inputRef = useRef(null)
  const [seconds, setSeconds] = useState('0')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    mountedRef.current = true
    const controller = new AbortController()
    setLoading(true)
    setError('')
    fetchVideoSubtitleSettings(video.id, controller.signal)
      .then((settings) => {
        if (!controller.signal.aborted) {
          setSeconds(String(settings.offset_ms / 1000))
          setLoading(false)
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(getErrorMessage(err))
      })
    return () => {
      mountedRef.current = false
      controller.abort()
    }
  }, [video.id])

  // Native spinner steps also constrain valid input. Keep millisecond validation
  // and use 500 native steps for each half-second adjustment.
  const adjustOffset = (direction) => {
    const input = inputRef.current
    if (!input || input.disabled) return
    input.stepUp(direction * 500)
    setSeconds(input.value)
  }

  const save = async (event) => {
    event.preventDefault()
    if (loading || saving) return
    setSaving(true)
    setError('')
    try {
      await updateVideoSubtitleSettings(video.id, Math.round(Number(seconds) * 1000))
      onSaved?.()
      if (!mountedRef.current) return
      onClose?.()
    } catch (err) {
      if (mountedRef.current) setError(getErrorMessage(err))
    } finally {
      if (mountedRef.current) setSaving(false)
    }
  }

  return (
    <form onSubmit={save} className="player-subtitle-offset-settings">
      <label htmlFor={inputId} className="mb-2 block font-semibold">
        {zh('字幕偏移（秒）', 'Subtitle offset (seconds)')}
      </label>
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-28">
          <input
            ref={inputRef}
            id={inputId}
            type="number"
            min="-3600"
            max="3600"
            step="0.001"
            required
            value={seconds}
            onChange={(event) => {
              setSeconds(event.target.value)
            }}
            onKeyDown={(event) => {
              if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
                event.preventDefault()
                adjustOffset(event.key === 'ArrowUp' ? 1 : -1)
              }
            }}
            disabled={loading || saving}
            className="player-subtitle-offset-input w-full rounded border border-white/40 bg-transparent py-1 pl-2 pr-7 text-white"
          />
          <div className="absolute inset-y-px right-px flex w-6 flex-col">
            <button
              type="button"
              aria-label={zh('增加 0.5 秒', 'Increase by 0.5 seconds')}
              disabled={loading || saving}
              onClick={() => adjustOffset(1)}
              className="player-subtitle-offset-step flex min-h-0 flex-1 items-center justify-center disabled:opacity-50"
            >
              <KeyboardArrowUpRoundedIcon sx={{ fontSize: 16 }} />
            </button>
            <button
              type="button"
              aria-label={zh('减少 0.5 秒', 'Decrease by 0.5 seconds')}
              disabled={loading || saving}
              onClick={() => adjustOffset(-1)}
              className="player-subtitle-offset-step flex min-h-0 flex-1 items-center justify-center disabled:opacity-50"
            >
              <KeyboardArrowDownRoundedIcon sx={{ fontSize: 16 }} />
            </button>
          </div>
        </div>
        <button type="submit" disabled={loading || saving} className="disabled:opacity-50">
          {saving ? zh('保存中…', 'Saving...') : zh('保存偏移', 'Save offset')}
        </button>
      </div>
      <p className="mt-2 text-xs leading-5 text-zinc-300">
        {zh(
          '正数延后字幕，负数提前字幕，默认为 0，修改后立即生效。',
          'Positive values delay subtitles; negative values advance them. Defaults to 0. Changes take effect immediately.'
        )}
      </p>
      {error ? (
        <p role="alert" className="mt-2 text-red-300">
          {error}
        </p>
      ) : null}
    </form>
  )
}
