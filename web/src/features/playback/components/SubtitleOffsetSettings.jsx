import { useEffect, useId, useRef, useState } from 'react'
import { fetchVideoSubtitleSettings, updateVideoSubtitleSettings } from '@/features/playback/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function SubtitleOffsetSettings({ video, onSaved, onClose }) {
  const inputId = useId()
  const mountedRef = useRef(false)
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
        <input
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
          disabled={loading || saving}
          className="w-28 rounded border border-white/40 bg-transparent px-2 py-1 text-white"
        />
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
