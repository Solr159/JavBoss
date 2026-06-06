import { useMemo, useState } from 'react'
import PhotoCameraRoundedIcon from '@mui/icons-material/PhotoCameraRounded'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import { fetchJavIdolJavDBURL } from '@/api'
import JavIdolCoverModal, {
  IDOL_COVER_DEFAULT_CROP_LEFT,
  IDOL_COVER_VISIBLE_RATIO,
  idolCoverBackgroundPosition,
  normalizeIdolCoverCropLeft,
} from '@/components/JavIdolCoverModal'
import { isChineseLocale, zh } from '@/utils/i18n'

const RIGHT_PORTION = IDOL_COVER_VISIBLE_RATIO

export function getIdolCardLayoutProps() {
  const visibleRatio = Math.min(Math.max(RIGHT_PORTION, 0.01), 1)
  const bgWidthPercent = (1 / visibleRatio) * 100
  const originalWidth = 800
  const originalHeight = 538
  const coverAspectPercent = (originalHeight / (originalWidth * visibleRatio)) * 100

  return { bgWidthPercent, coverAspectPercent }
}

export default function JavIdolGrid({
  items,
  onSelectIdol,
  onOpenFavorites,
  buildIdolUrl,
  javMetadataLanguage,
  directoryIds = [],
}) {
  const { bgWidthPercent, coverAspectPercent } = getIdolCardLayoutProps()
  const [coverEditorItem, setCoverEditorItem] = useState(null)
  const [coverOverrides, setCoverOverrides] = useState(() => new Map())
  const displayItems = useMemo(() => {
    if (!Array.isArray(items)) return []
    return items.map((item) => {
      const id = Number(item?.id)
      const override = Number.isFinite(id) ? coverOverrides.get(id) : null
      return override ? { ...item, ...override } : item
    })
  }, [coverOverrides, items])

  const hasItems = displayItems.length > 0
  if (!hasItems) {
    return (
      <div className="flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
        {zh('暂无女优数据', 'No idol data')}
      </div>
    )
  }

  return (
    <>
      <div className="grid gap-3 bg-white sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8">
        {displayItems.map((item) => (
          <IdolCard
            key={item.id || item.name}
            item={item}
            onSelectIdol={onSelectIdol}
            onOpenFavorites={onOpenFavorites}
            onOpenCoverEditor={setCoverEditorItem}
            href={buildIdolUrl?.(item)}
            bgWidthPercent={bgWidthPercent}
            coverAspectPercent={coverAspectPercent}
            javMetadataLanguage={javMetadataLanguage}
          />
        ))}
      </div>
      <JavIdolCoverModal
        open={Boolean(coverEditorItem)}
        item={coverEditorItem}
        directoryIds={directoryIds}
        javMetadataLanguage={javMetadataLanguage}
        onClose={() => setCoverEditorItem(null)}
        onSaved={(updated) => {
          const id = Number(updated?.id)
          if (!Number.isFinite(id) || id <= 0) return
          setCoverOverrides((current) => {
            const next = new Map(current)
            next.set(id, updated)
            return next
          })
        }}
      />
    </>
  )
}

export function IdolCard({
  item,
  onSelectIdol,
  onOpenFavorites,
  onOpenCoverEditor,
  href,
  bgWidthPercent,
  coverAspectPercent,
  showWorkCount = true,
  javMetadataLanguage = 'zh',
}) {
  const chineseLocale = isChineseLocale()
  const coverCode = String(item?.cover_code || item?.sample_code || '').trim()
  const cover = coverCode ? `/jav/${encodeURIComponent(coverCode)}/cover` : null
  const coverCropLeft = normalizeIdolCoverCropLeft(
    item?.cover_crop_left ?? IDOL_COVER_DEFAULT_CROP_LEFT
  )
  const workCount = item?.work_count || 0
  const favoriteCount = Number(item?.favorite_count) || 0
  const name = item?.name || zh('未知女优', 'Unknown idol')
  const romanName = item?.roman_name || ''
  const japaneseName = item?.japanese_name || ''
  const chineseName = item?.chinese_name || ''
  const birthDate = formatBirthDateWithAge(item?.birth_date)
  const height = typeof item?.height_cm === 'number' ? `${item.height_cm}cm` : ''
  const bwh = formatBwh(item)
  const cup = formatCup(item?.cup)
  const sampleCode = String(item?.sample_code || coverCode || '').trim()
  const [javdbURL, setJavdbURL] = useState(String(item?.javdb_url || '').trim())
  const [javdbOpening, setJavdbOpening] = useState(false)
  const { primaryName, secondaryName } = buildDisplayNames({
    name,
    romanName,
    japaneseName,
    chineseName,
    chineseLocale,
    javMetadataLanguage,
  })
  const metaRows = buildMetaRows({ birthDate, height, bwh, cup, secondaryName })
  const canOpenJavDB = Boolean(javdbURL || (sampleCode && name))

  const handleClick = (e) => {
    const selection = window.getSelection?.()
    if (selection && String(selection).trim() !== '') {
      e.preventDefault()
      return
    }
    const isModified = e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0
    if (isModified) {
      return
    }
    e.preventDefault()
    onSelectIdol?.(item)
  }

  const handleOpenJavDB = async (event) => {
    event.preventDefault()
    event.stopPropagation()
    if (!canOpenJavDB || javdbOpening) return

    const popup = window.open('about:blank', '_blank')
    if (popup) {
      popup.opener = null
    }

    try {
      setJavdbOpening(true)
      let targetURL = javdbURL
      if (!targetURL) {
        targetURL = await fetchJavIdolJavDBURL({ code: sampleCode, name })
        setJavdbURL(targetURL)
      }
      if (!targetURL) {
        popup?.close()
        return
      }
      if (popup) {
        popup.location.replace(targetURL)
      } else {
        window.open(targetURL, '_blank', 'noopener,noreferrer')
      }
    } catch (error) {
      popup?.close()
      console.warn('open javdb idol failed', error)
    } finally {
      setJavdbOpening(false)
    }
  }

  const handleOpenFavorites = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenFavorites?.(item)
  }

  const handleOpenCoverEditor = (event) => {
    event.preventDefault()
    event.stopPropagation()
    onOpenCoverEditor?.(item)
  }

  return (
    <a
      href={href || '#'}
      className="group flex cursor-pointer flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg"
      onClick={handleClick}
      onKeyDown={(e) => {
        if (e.key === ' ') {
          e.preventDefault()
          onSelectIdol?.(item)
        }
      }}
    >
      <div
        className="relative w-full overflow-hidden bg-gray-100"
        style={{ paddingTop: `${coverAspectPercent}%` }} // 维持可见区域的原始纵横比，避免压扁
      >
        {cover ? (
          <div
            className="absolute inset-0"
            style={{
              backgroundImage: `url(${cover})`,
              backgroundSize: `${bgWidthPercent}% 100%`, // 根据 RIGHT_PORTION 自动计算
              backgroundPosition: idolCoverBackgroundPosition(coverCropLeft),
              backgroundRepeat: 'no-repeat',
            }}
            role="img"
            aria-label={primaryName}
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-600">
            {primaryName}
          </div>
        )}
        {showWorkCount && (
          <div className="absolute left-2 top-2 rounded bg-black/70 px-2 py-1 text-xs text-white">
            {zh(`作品 ${workCount}`, `${workCount} javs`)}
          </div>
        )}
        <button
          type="button"
          className={`absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
            favoriteCount > 0
              ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
              : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-focus-within:opacity-100 group-hover:opacity-100'
          }`}
          title={zh('加入女优收藏夹', 'Add to idol favorite groups')}
          aria-label={zh('加入女优收藏夹', 'Add to idol favorite groups')}
          onClick={handleOpenFavorites}
        >
          {favoriteCount > 0 ? (
            <StarRoundedIcon sx={{ fontSize: 18 }} />
          ) : (
            <StarBorderRoundedIcon sx={{ fontSize: 18 }} />
          )}
        </button>
        <button
          type="button"
          className={`absolute bottom-2 left-2 flex h-7 w-7 items-center justify-center rounded-full text-white opacity-0 shadow-lg shadow-black/60 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 ${
            canOpenJavDB ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
          }`}
          title={zh('在 JavDB 中打开女优详情', 'Open idol profile in JavDB')}
          aria-label={zh('在 JavDB 中打开女优详情', 'Open idol profile in JavDB')}
          disabled={!canOpenJavDB || javdbOpening}
          onClick={handleOpenJavDB}
        >
          <img
            src="/ico/javdb.png"
            alt="JavDB"
            className={`h-4 w-4 ${javdbOpening ? 'animate-pulse' : ''}`}
            loading="lazy"
          />
        </button>
        <button
          type="button"
          className="absolute bottom-2 right-2 flex h-7 w-7 items-center justify-center rounded-full bg-black/70 text-white opacity-0 shadow-lg shadow-black/60 transition-opacity hover:bg-black/85 group-focus-within:opacity-100 group-hover:opacity-100"
          title={zh('编辑女优封面', 'Edit idol cover')}
          aria-label={zh('编辑女优封面', 'Edit idol cover')}
          onClick={handleOpenCoverEditor}
        >
          <PhotoCameraRoundedIcon sx={{ fontSize: 16 }} />
        </button>
      </div>
      <div className="flex flex-1 flex-col gap-2 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-tight">{primaryName}</div>
        {metaRows.length > 0 ? (
          <div className="flex flex-col gap-1.5 text-[10px] text-gray-900">
            {metaRows.map((row) => (
              <div
                key={row.key}
                className={`flex flex-nowrap gap-1.5 overflow-hidden ${row.className || ''}`}
              >
                {row.items.map((meta) => (
                  <span key={meta.key} className="inline-flex items-center whitespace-nowrap">
                    {meta.label}
                  </span>
                ))}
              </div>
            ))}
          </div>
        ) : (
          <div className="text-xs text-gray-400">{zh('信息待补充', 'More info coming')}</div>
        )}
      </div>
    </a>
  )
}

function formatBirthDate(value) {
  if (!value) return ''
  if (typeof value === 'string') {
    return value.slice(0, 10)
  }
  if (value instanceof Date && !Number.isNaN(value.getTime())) {
    return value.toISOString().slice(0, 10)
  }
  return ''
}

function formatBirthDateWithAge(value) {
  const birthDate = formatBirthDate(value)
  if (!birthDate) return ''

  const age = calculateAge(birthDate)
  if (!Number.isFinite(age) || age < 0) {
    return birthDate
  }
  return zh(`${birthDate}（${age}岁）`, `${birthDate} (${age} years old)`)
}

function calculateAge(birthDate) {
  const date = new Date(`${birthDate}T00:00:00`)
  if (Number.isNaN(date.getTime())) return null

  const now = new Date()
  let age = now.getFullYear() - date.getFullYear()
  const monthDiff = now.getMonth() - date.getMonth()
  const dayDiff = now.getDate() - date.getDate()
  if (monthDiff < 0 || (monthDiff === 0 && dayDiff < 0)) {
    age -= 1
  }
  return age
}

function formatBwh(item) {
  const bust = item?.bust
  const waist = item?.waist
  const hips = item?.hips
  if (typeof bust === 'number' && typeof waist === 'number' && typeof hips === 'number') {
    return `B${bust}-W${waist}-H${hips}`
  }
  return ''
}

function formatCup(value) {
  if (typeof value !== 'number' || value <= 0) return ''
  const letter = String.fromCharCode(64 + value)
  return zh(`${letter}罩杯`, `${letter} cup`)
}

function buildMetaRows({ birthDate, height, bwh, cup, secondaryName }) {
  const rows = []
  if (secondaryName) {
    rows.push({
      key: 'row-1',
      className: 'font-semibold text-gray-950',
      items: [{ key: `secondary-${secondaryName}`, label: secondaryName }],
    })
  }
  if (birthDate) {
    rows.push({ key: 'row-2', items: [{ key: `birth-${birthDate}`, label: birthDate }] })
  }

  const rowTwo = []
  if (height) {
    rowTwo.push({ key: `height-${height}`, label: height })
  }
  if (bwh) {
    rowTwo.push({ key: `bwh-${bwh}`, label: bwh })
  }
  if (cup) {
    rowTwo.push({ key: `cup-${cup}`, label: cup })
  }
  if (rowTwo.length > 0) {
    rows.push({ key: 'row-3', items: rowTwo })
  }
  return rows
}

function buildDisplayNames({
  name,
  romanName,
  japaneseName,
  chineseName,
  chineseLocale,
  javMetadataLanguage,
}) {
  if (javMetadataLanguage === 'en') {
    const primaryName = firstNonEmpty(
      name,
      romanName,
      japaneseName,
      chineseName,
      zh('Unknown idol', 'Unknown idol')
    )
    return {
      primaryName,
      secondaryName: joinUniqueDisplayParts([japaneseName, chineseName], [primaryName]),
    }
  }

  if (chineseLocale) {
    const primaryName = firstNonEmpty(
      name,
      japaneseName,
      romanName,
      chineseName,
      zh('未知女优', 'Unknown idol')
    )
    return {
      primaryName,
      secondaryName: joinUniqueDisplayParts([romanName, chineseName], [primaryName]),
    }
  }
  const primaryName = firstNonEmpty(
    name,
    romanName,
    japaneseName,
    chineseName,
    zh('Unknown idol', 'Unknown idol')
  )
  return {
    primaryName,
    secondaryName: joinUniqueDisplayParts([japaneseName, chineseName], [primaryName]),
  }
}

function firstNonEmpty(...values) {
  for (const value of values) {
    const trimmed = String(value || '').trim()
    if (trimmed) return trimmed
  }
  return ''
}

function joinUniqueDisplayParts(values, exclude = []) {
  const excluded = new Set(exclude.map((value) => String(value || '').trim()).filter(Boolean))
  const seen = new Set()
  const parts = []
  for (const value of values) {
    const trimmed = String(value || '').trim()
    if (!trimmed || excluded.has(trimmed) || seen.has(trimmed)) {
      continue
    }
    seen.add(trimmed)
    parts.push(trimmed)
  }
  return parts.join(' · ')
}
