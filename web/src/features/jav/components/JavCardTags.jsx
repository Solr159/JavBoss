import Tooltip from '@/shared/ui/Tooltip'
import { useState, useRef, useMemo, useEffect } from 'react'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import {
  normalizeIdolTagMaxRows,
  countFlexRows,
  normalizeJavTagMaxRows,
} from '@/features/jav/presentation'
import { getIdolDisplayName } from '@/utils/javIdol'
import { zh } from '@/utils/i18n'
import { isUserJavTag } from '@/constants/jav'
import { normalizeCoverCropPosition } from '@/features/jav/coverCrop'

export function JavCoverImage({ src, alt, portrait = false, cropPosition }) {
  return (
    <img
      src={src}
      alt={alt}
      className={`h-full w-full ${portrait ? 'object-cover object-right' : 'object-contain object-top'}`}
      style={
        portrait
          ? { objectPosition: `${normalizeCoverCropPosition(cropPosition) * 100}% center` }
          : undefined
      }
      loading="lazy"
    />
  )
}

export function TagCollapseToggleButton({
  expanded,
  count,
  title,
  expandedClassName,
  collapsedClassName,
  onToggle,
}) {
  const [tooltipOpen, setTooltipOpen] = useState(false)
  const [activeTooltipTitle, setActiveTooltipTitle] = useState(title)
  const className = expanded ? expandedClassName : collapsedClassName

  const button = (
    <button
      type="button"
      onClick={() => {
        setTooltipOpen(false)
        onToggle?.()
      }}
      aria-label={title}
      className={className}
    >
      {expanded ? (
        <ExpandLessIcon sx={{ fontSize: 15 }} />
      ) : (
        <>
          <span>{count}</span>
          <ExpandMoreIcon sx={{ fontSize: 15 }} />
        </>
      )}
    </button>
  )

  return (
    <Tooltip
      title={activeTooltipTitle}
      open={tooltipOpen}
      onOpen={() => {
        setActiveTooltipTitle(title)
        setTooltipOpen(true)
      }}
      onClose={() => setTooltipOpen(false)}
      TransitionProps={{ timeout: 0 }}
    >
      {button}
    </Tooltip>
  )
}

export function IdolTagList({
  idols,
  maxRows,
  compact = false,
  preferChineseName = false,
  buildIdolFilterHref,
  onIdolClick,
  onFilterLinkClick,
  onIdolHoverStart,
  onIdolHoverEnd,
}) {
  const tagSizeClassName = compact ? 'px-1.5 py-0.5 text-[11px] leading-4' : 'px-2 py-1 text-xs'
  const gapClassName = compact ? 'gap-0.5' : 'gap-1'
  const toggleSizeClassName = compact
    ? 'h-5 gap-0.5 px-1 text-[10px]'
    : 'h-6 gap-1 px-1.5 text-[11px]'
  const measureRef = useRef(null)
  const [expanded, setExpanded] = useState(false)
  const [overflowing, setOverflowing] = useState(false)
  const [visibleCount, setVisibleCount] = useState(idols.length)
  const rowLimit = normalizeIdolTagMaxRows(maxRows)
  const identity = useMemo(
    () =>
      (idols || [])
        .map(
          (idol) => `${idol?.id || idol?.name || ''}:${getIdolDisplayName(idol, preferChineseName)}`
        )
        .join('|'),
    [idols, preferChineseName]
  )

  useEffect(() => {
    setExpanded(false)
    setVisibleCount(idols.length)
  }, [compact, identity, idols.length, rowLimit])

  useEffect(() => {
    if (rowLimit <= 0) {
      setOverflowing(false)
      setVisibleCount(idols.length)
      return undefined
    }

    const measureList = measureRef.current
    if (!measureList) return undefined

    const measure = () => {
      const containerWidth = measureList.clientWidth
      const tagNodes = Array.from(measureList.querySelectorAll('[data-idol-tag-measure]'))
      const toggleNode = measureList.querySelector('[data-idol-toggle-measure]')

      if (containerWidth <= 0 || tagNodes.length === 0 || !toggleNode) {
        setOverflowing(false)
        setVisibleCount(idols.length)
        return
      }

      const tagWidths = tagNodes.map((node) => node.offsetWidth)
      const toggleWidth = toggleNode.offsetWidth
      const gap = Number.parseFloat(window.getComputedStyle(measureList).columnGap) || 0
      const fullRows = countFlexRows(tagWidths, 0, containerWidth, gap)
      const isOverflowing = fullRows > rowLimit
      setOverflowing(isOverflowing)
      if (!isOverflowing) {
        setVisibleCount(idols.length)
        return
      }

      let low = 0
      let high = tagWidths.length
      let best = 0
      while (low <= high) {
        const mid = Math.floor((low + high) / 2)
        const rows = countFlexRows(tagWidths.slice(0, mid), toggleWidth, containerWidth, gap)
        if (rows <= rowLimit) {
          best = mid
          low = mid + 1
        } else {
          high = mid - 1
        }
      }
      setVisibleCount(best)
    }

    measure()
    const resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null
    resizeObserver?.observe(measureList)
    window.addEventListener('resize', measure)
    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [compact, identity, idols.length, rowLimit])

  const showToggle = rowLimit > 0 && overflowing
  const renderedIdols = showToggle && !expanded ? idols.slice(0, visibleCount) : idols
  const toggleTitle = expanded
    ? zh('点击收回', 'Click to collapse')
    : zh(`共 ${idols.length} 位女优，点击展开`, `${idols.length} actresses total, click to expand`)

  return (
    <div className="relative">
      <div className={`flex min-w-0 flex-1 flex-wrap ${gapClassName}`}>
        {renderedIdols.map((idol) => (
          <a
            key={idol.id || idol.name}
            href={buildIdolFilterHref(idol)}
            className={`rounded-full bg-purple-100 font-medium text-purple-700 transition hover:bg-purple-200 ${tagSizeClassName}`}
            onMouseEnter={(event) => onIdolHoverStart(idol, event)}
            onMouseLeave={onIdolHoverEnd}
            onClick={(event) => onFilterLinkClick(event, () => onIdolClick?.(idol))}
          >
            {getIdolDisplayName(idol, preferChineseName)}
          </a>
        ))}
        {showToggle ? (
          <TagCollapseToggleButton
            expanded={expanded}
            count={idols.length}
            title={toggleTitle}
            expandedClassName={`inline-flex shrink-0 items-center justify-center rounded border border-gray-300 bg-gray-50 text-gray-600 shadow-sm transition hover:border-gray-400 hover:bg-gray-100 ${compact ? 'h-5 w-5' : 'h-6 w-6'}`}
            collapsedClassName={`inline-flex shrink-0 items-center rounded-md border border-purple-300 bg-white font-semibold text-purple-700 shadow-sm transition hover:border-purple-500 hover:bg-purple-50 ${toggleSizeClassName}`}
            onToggle={() => setExpanded((current) => !current)}
          />
        ) : null}
      </div>
      {rowLimit > 0 ? (
        <div
          ref={measureRef}
          aria-hidden="true"
          className={`pointer-events-none absolute inset-x-0 top-0 flex flex-wrap opacity-0 ${gapClassName}`}
        >
          {idols.map((idol) => (
            <span
              key={idol.id || idol.name}
              data-idol-tag-measure
              className={`rounded-full bg-purple-100 font-medium ${tagSizeClassName}`}
            >
              {getIdolDisplayName(idol, preferChineseName)}
            </span>
          ))}
          <span
            data-idol-toggle-measure
            className={`inline-flex shrink-0 items-center rounded-md border font-semibold ${toggleSizeClassName}`}
          >
            <span>{idols.length}</span>
            <ExpandMoreIcon sx={{ fontSize: 15 }} />
          </span>
        </div>
      ) : null}
    </div>
  )
}

export function JavTagList({
  tags,
  maxRows,
  compact = false,
  buildTagFilterHref,
  onTagClick,
  onFilterLinkClick,
}) {
  const tagSizeClassName = compact ? 'px-1.5 py-0.5 text-[11px] leading-4' : 'px-2 py-1 text-xs'
  const gapClassName = compact ? 'gap-0.5' : 'gap-1'
  const toggleSizeClassName = compact
    ? 'h-5 gap-0.5 px-1 text-[10px]'
    : 'h-6 gap-1 px-1.5 text-[11px]'
  const measureRef = useRef(null)
  const [expanded, setExpanded] = useState(false)
  const [overflowing, setOverflowing] = useState(false)
  const [visibleCount, setVisibleCount] = useState(tags.length)
  const rowLimit = normalizeJavTagMaxRows(maxRows)
  const identity = useMemo(
    () => (tags || []).map((tag) => tag?.id || tag?.name || '').join('|'),
    [tags]
  )

  useEffect(() => {
    setExpanded(false)
    setVisibleCount(tags.length)
  }, [compact, identity, tags.length, rowLimit])

  useEffect(() => {
    if (rowLimit <= 0) {
      setOverflowing(false)
      setVisibleCount(tags.length)
      return undefined
    }

    const measureList = measureRef.current
    if (!measureList) return undefined

    const measure = () => {
      const containerWidth = measureList.clientWidth
      const tagNodes = Array.from(measureList.querySelectorAll('[data-jav-tag-measure]'))
      const toggleNode = measureList.querySelector('[data-jav-tag-toggle-measure]')

      if (containerWidth <= 0 || tagNodes.length === 0 || !toggleNode) {
        setOverflowing(false)
        setVisibleCount(tags.length)
        return
      }

      const tagWidths = tagNodes.map((node) => node.offsetWidth)
      const toggleWidth = toggleNode.offsetWidth
      const gap = Number.parseFloat(window.getComputedStyle(measureList).columnGap) || 0
      const fullRows = countFlexRows(tagWidths, 0, containerWidth, gap)
      const isOverflowing = fullRows > rowLimit
      setOverflowing(isOverflowing)
      if (!isOverflowing) {
        setVisibleCount(tags.length)
        return
      }

      let low = 0
      let high = tagWidths.length
      let best = 0
      while (low <= high) {
        const mid = Math.floor((low + high) / 2)
        const rows = countFlexRows(tagWidths.slice(0, mid), toggleWidth, containerWidth, gap)
        if (rows <= rowLimit) {
          best = mid
          low = mid + 1
        } else {
          high = mid - 1
        }
      }
      setVisibleCount(best)
    }

    measure()
    const resizeObserver = typeof ResizeObserver === 'function' ? new ResizeObserver(measure) : null
    resizeObserver?.observe(measureList)
    window.addEventListener('resize', measure)
    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [compact, identity, rowLimit, tags.length])

  const showToggle = rowLimit > 0 && overflowing
  const renderedTags = showToggle && !expanded ? tags.slice(0, visibleCount) : tags
  const toggleTitle = expanded
    ? zh('点击收回', 'Click to collapse')
    : zh(`共 ${tags.length} 个标签，点击展开`, `${tags.length} tags total, click to expand`)

  return (
    <div className="relative">
      <div className={`flex min-w-0 flex-1 flex-wrap ${gapClassName}`}>
        {renderedTags.map((tag) => {
          const isUser = isUserJavTag(tag)
          const tagClass = isUser
            ? 'bg-emerald-500 hover:bg-emerald-600'
            : 'bg-orange-500 hover:bg-orange-600'
          return (
            <a
              key={`${tag.id || tag.name}-${tag.provider || 0}`}
              href={buildTagFilterHref(tag)}
              className={`rounded-full font-medium text-white transition ${tagClass} ${tagSizeClassName}`}
              onClick={(event) => onFilterLinkClick(event, () => onTagClick?.(tag))}
            >
              {tag.name}
            </a>
          )
        })}
        {showToggle ? (
          <TagCollapseToggleButton
            expanded={expanded}
            count={tags.length}
            title={toggleTitle}
            expandedClassName={`inline-flex shrink-0 items-center justify-center rounded border border-gray-300 bg-gray-50 text-gray-600 shadow-sm transition hover:border-gray-400 hover:bg-gray-100 ${compact ? 'h-5 w-5' : 'h-6 w-6'}`}
            collapsedClassName={`inline-flex shrink-0 items-center rounded-md border border-orange-300 bg-white font-semibold text-orange-700 shadow-sm transition hover:border-orange-500 hover:bg-orange-50 ${toggleSizeClassName}`}
            onToggle={() => setExpanded((current) => !current)}
          />
        ) : null}
      </div>
      {rowLimit > 0 ? (
        <div
          ref={measureRef}
          aria-hidden="true"
          className={`pointer-events-none absolute inset-x-0 top-0 flex flex-wrap opacity-0 ${gapClassName}`}
        >
          {tags.map((tag) => (
            <span
              key={`${tag.id || tag.name}-${tag.provider || 0}`}
              data-jav-tag-measure
              className={`rounded-full font-medium ${tagSizeClassName}`}
            >
              {tag.name}
            </span>
          ))}
          <span
            data-jav-tag-toggle-measure
            className={`inline-flex shrink-0 items-center rounded-md border font-semibold ${toggleSizeClassName}`}
          >
            <span>{tags.length}</span>
            <ExpandMoreIcon sx={{ fontSize: 15 }} />
          </span>
        </div>
      ) : null}
    </div>
  )
}
