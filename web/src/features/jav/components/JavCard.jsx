import { useState } from 'react'
import Tooltip from '@/shared/ui/Tooltip'
import CropRoundedIcon from '@mui/icons-material/CropRounded'
import JavCoverCropModal from '@/features/jav/components/JavCoverCropModal'
import useJavItemActions from '@/features/jav/hooks/useJavItemActions'
import { JavCoverImage, IdolTagList, JavTagList } from '@/features/jav/components/JavCardTags'
import { zh } from '@/utils/i18n'
import { Rating, Popper, IconButton } from '@mui/material'
import FavoriteRoundedIcon from '@mui/icons-material/FavoriteRounded'
import FavoriteBorderRoundedIcon from '@mui/icons-material/FavoriteBorderRounded'
import RemoveCircleOutlineRoundedIcon from '@mui/icons-material/RemoveCircleOutlineRounded'
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded'
import CheckBoxOutlineBlankRoundedIcon from '@mui/icons-material/CheckBoxOutlineBlankRounded'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import LinkRoundedIcon from '@mui/icons-material/LinkRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import SearchIcon from '@mui/icons-material/Search'
import PhotoLibraryOutlinedIcon from '@mui/icons-material/PhotoLibraryOutlined'
import { ReleaseIcon, DurationIcon } from '@/features/jav/components/JavMetadataIcons'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'
import CollectionsBookmarkOutlinedIcon from '@mui/icons-material/CollectionsBookmarkOutlined'
import { StudioCard } from '@/features/jav/components/JavStudioView'
import { SeriesCard } from '@/features/jav/components/JavSeriesView'
import { IdolCard } from '@/features/jav/components/JavIdolGrid'
import { MovieEdit } from '@mui/icons-material'
import VideoLibraryOutlinedIcon from '@mui/icons-material/VideoLibraryOutlined'
import { JavItemEditors } from '@/features/jav/components/JavItemEditors'
import WatchTimeIcons from '@/features/playback/components/WatchTimeIcons'

export default function JavCard(props) {
  const { portrait = false } = props
  const [coverCropEditorOpen, setCoverCropEditorOpen] = useState(false)
  const coverButtonSizeClassName = portrait ? 'h-7 w-7' : 'h-8 w-8'
  const coverSmallButtonSizeClassName = portrait ? 'h-6 w-6' : 'h-7 w-7'
  const favoriteRatingIconSize = portrait ? 18 : 21
  const model = useJavItemActions({
    ...props,
    showFullFavoriteRating: portrait ? false : props.showFullFavoriteRating,
  })
  const {
    checked,
    cover,
    item,
    handleOpenDetail,
    code,
    handlePlay,
    canPlay,
    setFavoriteRatingEditing,
    setFavoriteRatingPreview,
    favoriteRatingError,
    favoriteRatingPreview,
    hasFavoriteRatingTooltipValue,
    favoriteRatingTooltipValue,
    favoriteRatingSaving,
    favoriteRating,
    favoriteRatingWidth,
    handleFavoriteRatingChange,
    favoriteRatingEditing,
    onToggleSelect,

    selectionDisabled,
    externalLinks,
    handleExternalLinkClick,
    handleOpenCustomTags,
    favoriteCount,
    handleOpenJavFavorites,
    canOpen,
    handleOpenCoverPreview,
    handleOpenScreenshots,
    titleText,
    titleClampStyle,
    codeText,
    mainTitle,
    releaseText,
    durationText,
    studioText,
    buildStudioFilterHref,
    canFilterStudio,
    handleFilterLinkClick,
    onStudioClick,
    handleStudioHoverStart,
    scheduleHoverClose,
    hideStudio,
    hideSeries,
    seriesText,
    buildSeriesFilterHref,
    preferredSeries,
    canFilterSeries,
    onSeriesClick,
    handleSeriesHoverStart,

    previewStudio,
    studioHoverAnchorEl,
    clearHoverCloseTimer,
    onPrefixClick,
    onOpenStudioFavorites,
    onOpenSeriesFavorites,
    previewSeries,
    seriesHoverAnchorEl,
    hideIdols,

    idolTagMaxRows,
    preferChineseName,
    buildIdolFilterHref,
    onIdolClick,
    handleIdolHoverStart,
    previewIdol,
    idolHoverAnchorEl,
    onOpenFavorites,
    handleOpenIdolCoverEditor,
    handleOpenIdolEditor,
    handleOpenSeriesEditor,
    coverAspectPercent,
    showIdolWorkCount,
    hideTags,
    tags,
    tagMaxRows,
    buildTagFilterHref,
    onTagClick,
    hideActions,
    handleOpenEditor,
    handleOpenVideoManager,
  } = model
  const videoCount = Array.isArray(item?.videos) ? item.videos.length : 0
  const metadataTextClassName = portrait ? 'text-[11px] leading-4 tracking-tight' : 'text-xs'
  const fullDurationText = durationText || zh('时长未知', 'Unknown duration')
  const compactDurationText = item?.duration_min ? `${item.duration_min} min` : '—'
  const showStudio = !hideStudio && Boolean(studioText)
  const studioMetadata = showStudio ? (
    <span
      className={`flex min-w-0 flex-1 items-center overflow-hidden ${portrait ? 'gap-0.5' : 'gap-1'}`}
    >
      <Tooltip title={zh('片商', 'Studio')} arrow>
        <span className="inline-flex shrink-0">
          <VideocamOutlinedIcon
            sx={{ fontSize: portrait ? 12 : 16 }}
            className="shrink-0 text-sky-600"
          />
        </span>
      </Tooltip>
      <a
        href={buildStudioFilterHref(item.studio)}
        title={studioText}
        className={`block min-w-0 truncate text-left ${
          canFilterStudio ? 'cursor-pointer hover:text-blue-700 hover:underline' : ''
        }`}
        onClick={(event) =>
          handleFilterLinkClick(event, () => {
            if (canFilterStudio) onStudioClick(item.studio)
          })
        }
        onMouseEnter={(event) => handleStudioHoverStart(item.studio, event)}
        onMouseLeave={scheduleHoverClose}
      >
        {studioText}
      </a>
    </span>
  ) : null
  return (
    <>
      <div
        className={`jav-card flex flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg ${checked ? 'border-sky-400 ring-2 ring-sky-200' : ''}`}
        style={portrait ? { containerType: 'inline-size' } : undefined}
      >
        <div
          className="card-hover-scope group relative shrink-0 overflow-hidden bg-white"
          style={{ aspectRatio: portrait ? `100 / ${coverAspectPercent}` : '800 / 538' }}
        >
          {cover ? (
            <JavCoverImage
              src={cover}
              alt={item?.code || zh('JAV 封面', 'JAV cover')}
              portrait={portrait}
              cropPosition={item?.cover_crop_position}
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-600">
              {item?.code || zh('未知番号', 'Unknown code')}
            </div>
          )}
          <button
            type="button"
            className="card-modal-trigger absolute inset-0 z-[1] cursor-pointer"
            onClick={handleOpenDetail}
            aria-label={zh(`查看 ${code || 'JAV'} 详情`, `View ${code || 'JAV'} details`)}
          />
          <div className="card-hover-focus-visible pointer-events-none absolute inset-0 z-[2] flex items-center justify-center bg-black/0 text-white opacity-0 transition-opacity group-hover:opacity-100">
            <button
              onClick={handlePlay}
              disabled={!canPlay}
              className={`card-modal-trigger pointer-events-auto rounded-full ${portrait ? 'p-2.5' : 'p-3'} ${
                canPlay ? 'bg-black/60 hover:bg-black/80' : 'cursor-not-allowed bg-black/30'
              }`}
              aria-label={zh('播放', 'Play')}
              title={zh('播放', 'Play')}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className={portrait ? 'h-9 w-9' : 'h-10 w-10'}
              >
                <path d="M8 5v14l11-7z" />
              </svg>
            </button>
          </div>
          <div
            className="absolute left-2 top-2 z-10 flex items-center gap-1"
            onMouseLeave={() => {
              setFavoriteRatingEditing(false)
              setFavoriteRatingPreview(null)
            }}
            onBlur={(event) => {
              if (event.currentTarget.contains(event.relatedTarget)) return
              setFavoriteRatingEditing(false)
              setFavoriteRatingPreview(null)
            }}
          >
            <Tooltip
              title={
                favoriteRatingError ||
                (favoriteRatingPreview === 0
                  ? zh('清空喜爱度', 'Clear favorite rating')
                  : hasFavoriteRatingTooltipValue
                    ? zh(
                        `喜爱度：${favoriteRatingTooltipValue.toFixed(1)} 分`,
                        `Favorite rating: ${favoriteRatingTooltipValue.toFixed(1)}`
                      )
                    : zh('设置喜爱度评分', 'Set favorite rating'))
              }
              placement="top"
              arrow
            >
              <span
                role="group"
                aria-label={zh('喜爱度评分', 'Favorite rating')}
                className={`flex items-center rounded-full bg-black/70 py-0.5 shadow-lg shadow-black/50 transition-opacity ${portrait ? 'px-1' : 'px-1.5'} ${
                  favoriteRatingSaving
                    ? 'opacity-60'
                    : favoriteRating > 0
                      ? 'opacity-100'
                      : 'card-hover-focus-visible opacity-0 group-hover:opacity-100'
                }`}
              >
                <span
                  className="flex overflow-hidden transition-[width] duration-150"
                  style={{ width: favoriteRatingWidth * (favoriteRatingIconSize / 21) }}
                >
                  <Rating
                    name={`jav-favorite-rating-${item?.id || code || 'unknown'}`}
                    value={favoriteRating}
                    precision={0.5}
                    size="small"
                    icon={<FavoriteRoundedIcon fontSize="inherit" />}
                    emptyIcon={<FavoriteBorderRoundedIcon fontSize="inherit" />}
                    disabled={favoriteRatingSaving || !item?.id}
                    onChange={handleFavoriteRatingChange}
                    onClick={(event) => event.stopPropagation()}
                    onMouseDown={(event) => event.stopPropagation()}
                    onMouseEnter={() => setFavoriteRatingEditing(true)}
                    onFocus={() => setFavoriteRatingEditing(true)}
                    onChangeActive={(_, value) =>
                      setFavoriteRatingPreview(value >= 0.5 ? value : null)
                    }
                    sx={{
                      flexShrink: 0,
                      color: '#fbbf24',
                      fontSize: favoriteRatingIconSize,
                      '& .MuiRating-iconEmpty': {
                        color: 'rgba(255,255,255,0.85)',
                      },
                    }}
                  />
                </span>
                {favoriteRatingEditing && favoriteRating > 0 ? (
                  <button
                    type="button"
                    className={`ml-1 flex shrink-0 items-center justify-center rounded-full text-white transition hover:bg-white/20 ${portrait ? 'h-4 w-4' : 'h-5 w-5'}`}
                    disabled={favoriteRatingSaving || !item?.id}
                    aria-label={zh('清除喜爱度评分', 'Clear favorite rating')}
                    onMouseEnter={() => setFavoriteRatingPreview(0)}
                    onMouseLeave={() => setFavoriteRatingPreview(null)}
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={(event) => handleFavoriteRatingChange(event, 0)}
                  >
                    <RemoveCircleOutlineRoundedIcon sx={{ fontSize: portrait ? 12 : 15 }} />
                  </button>
                ) : null}
                {favoriteRating > 0 && !favoriteRatingEditing ? (
                  <span
                    className={`ml-1 shrink-0 font-semibold tabular-nums leading-none text-white ${portrait ? 'text-[11px]' : 'text-xs'}`}
                  >
                    {favoriteRating.toFixed(1)}
                  </span>
                ) : null}
              </span>
            </Tooltip>
            {onToggleSelect && Number(item?.id) > 0 ? (
              <Tooltip
                title={checked ? zh('取消选择', 'Deselect') : zh('选择', 'Select')}
                placement="top"
                arrow
              >
                <button
                  type="button"
                  role="checkbox"
                  aria-checked={checked}
                  aria-label={zh(`选择 ${code || item.title}`, `Select ${code || item.title}`)}
                  disabled={selectionDisabled}
                  onKeyDown={(event) => {
                    if (event.key === ' ' || event.key === 'Enter') event.stopPropagation()
                  }}
                  onClick={(event) => {
                    event.stopPropagation()
                    onToggleSelect(item)
                  }}
                  className={`card-hover-focus-visible flex ${coverSmallButtonSizeClassName} shrink-0 items-center justify-center rounded-full bg-black/65 shadow-lg shadow-black/40 transition hover:bg-black/80 disabled:cursor-not-allowed disabled:opacity-60 [@media(hover:none)]:opacity-100 ${
                    checked
                      ? 'text-sky-300 opacity-100'
                      : 'text-white opacity-0 group-hover:opacity-100'
                  }`}
                >
                  {checked ? (
                    <CheckBoxRoundedIcon sx={{ fontSize: portrait ? 16 : 18 }} />
                  ) : (
                    <CheckBoxOutlineBlankRoundedIcon sx={{ fontSize: portrait ? 16 : 18 }} />
                  )}
                </button>
              </Tooltip>
            ) : null}
          </div>
          {externalLinks.length > 0 ? (
            <div className="card-hover-focus-visible group/external-links absolute bottom-2 left-2 z-10 flex max-w-[calc(100%-1rem)] items-center opacity-0 transition-opacity group-hover:opacity-100">
              {portrait ? (
                <button
                  type="button"
                  aria-label={zh('外部链接', 'External links')}
                  className={`flex ${coverButtonSizeClassName} items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 transition hover:bg-black/85`}
                >
                  <LinkRoundedIcon sx={{ fontSize: 16 }} />
                </button>
              ) : null}
              <div
                className={
                  portrait
                    ? 'pointer-events-none invisible absolute bottom-full left-0 flex w-max items-center gap-1 pb-2 group-focus-within/external-links:pointer-events-auto group-focus-within/external-links:visible group-hover/external-links:pointer-events-auto group-hover/external-links:visible'
                    : 'flex items-center gap-1.5'
                }
              >
                {externalLinks.map((site) => (
                  <Tooltip
                    key={site.key}
                    title={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                    placement="top"
                    arrow
                  >
                    <a
                      href={site.href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={`flex ${coverSmallButtonSizeClassName} shrink-0 items-center justify-center rounded-full bg-black/70 shadow-lg shadow-black/60 transition hover:bg-black/85`}
                      aria-label={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                      onClick={(event) => handleExternalLinkClick(event, site)}
                    >
                      <img
                        src={site.icon}
                        alt={site.name}
                        className={`${portrait ? (site.key === 'javmenu' ? 'h-4 w-4' : 'h-3.5 w-3.5') : site.key === 'javmenu' ? 'h-5 w-5' : 'h-4 w-4'} ${site.loading ? 'animate-pulse' : ''}`}
                        loading="lazy"
                      />
                    </a>
                  </Tooltip>
                ))}
              </div>
            </div>
          ) : null}
          <button
            type="button"
            className={`card-hover-focus-visible absolute top-2 z-10 flex ${coverButtonSizeClassName} items-center justify-center rounded-full bg-black/65 text-white opacity-0 shadow-lg shadow-black/40 transition hover:bg-black/80 group-hover:opacity-100 ${portrait ? 'right-10' : 'right-12'}`}
            title={zh('编辑自定义标签', 'Edit custom tags')}
            aria-label={zh('编辑自定义标签', 'Edit custom tags')}
            onClick={handleOpenCustomTags}
          >
            <LocalOfferOutlinedIcon sx={{ fontSize: portrait ? 16 : 18 }} />
          </button>
          <button
            type="button"
            className={`card-hover-focus-visible absolute right-2 top-2 z-10 flex ${coverButtonSizeClassName} items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
              favoriteCount > 0
                ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
                : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-hover:opacity-100'
            }`}
            title={zh('加入作品收藏夹', 'Add to JAV favorite groups')}
            aria-label={zh('加入作品收藏夹', 'Add to JAV favorite groups')}
            onClick={handleOpenJavFavorites}
          >
            {favoriteCount > 0 ? (
              <StarRoundedIcon sx={{ fontSize: portrait ? 16 : 18 }} />
            ) : (
              <StarBorderRoundedIcon sx={{ fontSize: portrait ? 16 : 18 }} />
            )}
          </button>
          {cover || canOpen ? (
            <div
              className={`card-hover-focus-visible absolute bottom-2 right-2 z-10 flex items-center opacity-0 transition-opacity group-hover:opacity-100 ${portrait ? 'gap-1' : 'gap-2'}`}
            >
              {portrait && cover && item?.id ? (
                <button
                  type="button"
                  onClick={() => setCoverCropEditorOpen(true)}
                  title={zh('调整封面', 'Adjust cover')}
                  aria-label={zh('调整封面', 'Adjust cover')}
                  className={`flex ${coverButtonSizeClassName} items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85`}
                >
                  <CropRoundedIcon sx={{ fontSize: 16 }} />
                </button>
              ) : null}
              {cover ? (
                <button
                  type="button"
                  onClick={handleOpenCoverPreview}
                  title={zh('查看封面', 'View cover')}
                  aria-label={zh('查看封面', 'View cover')}
                  className={`flex ${coverButtonSizeClassName} items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85`}
                >
                  <SearchIcon
                    className={`text-white ${portrait ? 'h-4 w-4' : 'h-5 w-5'}`}
                    fontSize="inherit"
                  />
                </button>
              ) : null}
              <button
                type="button"
                onClick={handleOpenScreenshots}
                disabled={!canOpen}
                title={zh('查看截图', 'View screenshots')}
                aria-label={zh('查看截图', 'View screenshots')}
                className={`flex ${coverButtonSizeClassName} items-center justify-center rounded-full text-white shadow-lg shadow-black/60 ${
                  canOpen ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
                }`}
              >
                <PhotoLibraryOutlinedIcon
                  className={`text-white ${portrait ? 'h-4 w-4' : 'h-5 w-5'}`}
                  fontSize="inherit"
                />
              </button>
            </div>
          ) : null}
        </div>
        <div className={`flex flex-1 flex-col gap-2 py-3 ${portrait ? 'px-2' : 'px-3'}`}>
          <div
            className={`leading-tight ${portrait ? 'text-[13px]' : 'text-sm'}`}
            title={titleText}
            style={titleClampStyle}
          >
            {codeText ? <span className="font-semibold text-gray-800">{codeText}</span> : null}
            {codeText ? ' ' : null}
            <span className="font-medium text-gray-800">{mainTitle}</span>
          </div>
          <div
            className={`flex min-w-0 items-center text-gray-600 ${metadataTextClassName} ${portrait ? '-my-0.5 gap-px' : 'gap-2'}`}
          >
            <div
              className={`flex min-w-0 flex-1 flex-nowrap items-center overflow-hidden ${portrait ? 'gap-x-1.5' : 'gap-x-3'}`}
            >
              <span
                className={`inline-flex shrink-0 items-center ${portrait ? 'gap-px' : 'gap-1'}`}
              >
                <Tooltip title={zh('发行日期', 'Release date')} arrow>
                  <span className={`inline-flex ${portrait ? 'jav-card-release-icon' : ''}`}>
                    <ReleaseIcon compact={portrait} />
                  </span>
                </Tooltip>
                <span>{releaseText}</span>
              </span>
              <span
                className={`inline-flex shrink-0 items-center ${portrait ? 'gap-px' : 'gap-1'}`}
              >
                <Tooltip title={zh('时长', 'Duration')} arrow>
                  <span className={`inline-flex ${portrait ? 'jav-card-duration-icon' : ''}`}>
                    <DurationIcon compact={portrait} />
                  </span>
                </Tooltip>
                <span title={fullDurationText}>
                  {portrait ? compactDurationText : fullDurationText}
                </span>
              </span>
              {!portrait ? studioMetadata : null}
            </div>
            <WatchTimeIcons
              watchedMs={item?.watched_ms}
              entityType="javs"
              entityId={item?.id}
              compact={portrait}
            />
          </div>
          {(portrait && showStudio) || (!hideSeries && seriesText) ? (
            <div
              className={`flex min-w-0 items-center text-gray-600 ${metadataTextClassName} ${portrait ? '-my-0.5 gap-2' : 'gap-3'}`}
            >
              {portrait ? studioMetadata : null}
              {!hideSeries && seriesText ? (
                <div
                  className={`flex min-w-0 items-center ${portrait ? 'flex-1 gap-0.5' : 'gap-1'}`}
                >
                  <Tooltip title={zh('系列', 'Series')} arrow>
                    <span className="inline-flex shrink-0">
                      <CollectionsBookmarkOutlinedIcon
                        sx={{ fontSize: portrait ? 12 : 14 }}
                        className="shrink-0 text-emerald-600"
                      />
                    </span>
                  </Tooltip>
                  <a
                    href={buildSeriesFilterHref(preferredSeries)}
                    title={seriesText}
                    className={`min-w-0 text-left ${portrait ? 'truncate leading-4' : 'whitespace-normal break-words leading-snug'} ${
                      canFilterSeries ? 'cursor-pointer hover:text-blue-700 hover:underline' : ''
                    }`}
                    onClick={(event) =>
                      handleFilterLinkClick(event, () => {
                        if (canFilterSeries) onSeriesClick(preferredSeries)
                      })
                    }
                    onMouseEnter={(event) => handleSeriesHoverStart(preferredSeries, event)}
                    onMouseLeave={scheduleHoverClose}
                  >
                    {seriesText}
                  </a>
                </div>
              ) : null}
            </div>
          ) : null}
          <Popper
            open={Boolean(previewStudio && studioHoverAnchorEl)}
            anchorEl={studioHoverAnchorEl}
            placement="right-start"
            className="z-[1400]"
            modifiers={[
              {
                name: 'offset',
                options: {
                  offset: [10, 0],
                },
              },
            ]}
          >
            <div
              className="w-[320px]"
              onMouseEnter={clearHoverCloseTimer}
              onMouseLeave={scheduleHoverClose}
            >
              {previewStudio ? (
                <StudioCard
                  item={previewStudio}
                  href={buildStudioFilterHref(previewStudio)}
                  onSelectStudio={(studio) => onStudioClick?.(studio)}
                  onSelectSeries={(series) => onSeriesClick?.(series)}
                  onSelectPrefix={(prefix) => onPrefixClick?.(prefix)}
                  onOpenFavorites={onOpenStudioFavorites}
                  buildSeriesUrl={buildSeriesFilterHref}
                  onOpenSeriesFavorites={onOpenSeriesFavorites}
                />
              ) : null}
            </div>
          </Popper>
          <Popper
            open={Boolean(previewSeries && seriesHoverAnchorEl)}
            anchorEl={seriesHoverAnchorEl}
            placement="right-start"
            className="z-[1400]"
            modifiers={[
              {
                name: 'offset',
                options: {
                  offset: [10, 0],
                },
              },
            ]}
          >
            <div
              className="w-[260px]"
              onMouseEnter={clearHoverCloseTimer}
              onMouseLeave={scheduleHoverClose}
            >
              {previewSeries ? (
                <SeriesCard
                  item={previewSeries}
                  onOpenEditor={handleOpenSeriesEditor}
                  href={buildSeriesFilterHref(previewSeries)}
                  onSelectSeries={(series) => onSeriesClick?.(series)}
                  onSelectStudio={(studio) => onStudioClick?.(studio)}
                  onOpenFavorites={onOpenSeriesFavorites}
                />
              ) : null}
            </div>
          </Popper>
          {!hideIdols && Array.isArray(item?.idols) && item.idols.length > 0 && (
            <>
              <IdolTagList
                compact={portrait}
                idols={item.idols}
                maxRows={idolTagMaxRows}
                preferChineseName={preferChineseName}
                buildIdolFilterHref={buildIdolFilterHref}
                onIdolClick={onIdolClick}
                onFilterLinkClick={handleFilterLinkClick}
                onIdolHoverStart={handleIdolHoverStart}
                onIdolHoverEnd={scheduleHoverClose}
              />
              <Popper
                open={Boolean(previewIdol && idolHoverAnchorEl)}
                anchorEl={idolHoverAnchorEl}
                placement="right-start"
                className="z-[1400]"
                modifiers={[
                  {
                    name: 'offset',
                    options: {
                      offset: [10, 0],
                    },
                  },
                ]}
              >
                <div
                  className="w-[220px]"
                  onMouseEnter={clearHoverCloseTimer}
                  onMouseLeave={scheduleHoverClose}
                >
                  {previewIdol ? (
                    <IdolCard
                      item={previewIdol}
                      onSelectIdol={(idol) => onIdolClick?.(idol)}
                      onOpenFavorites={onOpenFavorites}
                      onOpenCoverEditor={handleOpenIdolCoverEditor}
                      onOpenEditor={handleOpenIdolEditor}
                      href={buildIdolFilterHref(previewIdol)}
                      coverAspectPercent={coverAspectPercent}
                      showWorkCount={showIdolWorkCount}
                      preferChineseName={preferChineseName}
                    />
                  ) : null}
                </div>
              </Popper>
            </>
          )}
          {!hideTags && tags.length > 0 && (
            <JavTagList
              compact={portrait}
              tags={tags}
              maxRows={tagMaxRows}
              buildTagFilterHref={buildTagFilterHref}
              onTagClick={onTagClick}
              onFilterLinkClick={handleFilterLinkClick}
            />
          )}
          {!hideActions ? (
            <div className={`flex flex-wrap items-center gap-2 ${portrait ? 'mt-auto' : ''}`}>
              <div className={`flex items-center ${portrait ? '-my-0.5 gap-1' : 'gap-2'}`}>
                <Tooltip title={zh('编辑 JAV', 'Edit JAV')}>
                  <IconButton
                    size="small"
                    onClick={handleOpenEditor}
                    aria-label={zh('编辑 JAV', 'Edit JAV')}
                    className={portrait ? undefined : 'h-6 w-6'}
                    sx={
                      portrait ? { width: 22, height: 22, padding: '2px', fontSize: 16 } : undefined
                    }
                  >
                    <MovieEdit fontSize="inherit" />
                  </IconButton>
                </Tooltip>
                <div className="flex items-center gap-0.5">
                  <Tooltip title={zh('视频管理', 'Manage videos')}>
                    <IconButton
                      size="small"
                      onClick={handleOpenVideoManager}
                      disabled={videoCount === 0}
                      aria-label={zh('视频管理', 'Manage videos')}
                      className={portrait ? undefined : 'h-6 w-6'}
                      sx={
                        portrait
                          ? { width: 22, height: 22, padding: '2px', fontSize: 16 }
                          : undefined
                      }
                    >
                      <VideoLibraryOutlinedIcon fontSize="inherit" />
                    </IconButton>
                  </Tooltip>
                  {videoCount > 1 ? (
                    <span className="text-xs text-gray-500">({videoCount})</span>
                  ) : null}
                </div>
              </div>
            </div>
          ) : null}
        </div>
      </div>
      <JavItemEditors {...model} />
      {coverCropEditorOpen ? (
        <JavCoverCropModal item={item} src={cover} onClose={() => setCoverCropEditorOpen(false)} />
      ) : null}
    </>
  )
}
