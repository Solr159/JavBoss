import useJavItemActions from '@/features/jav/hooks/useJavItemActions'
import { JavCoverImage, IdolTagList, JavTagList } from '@/features/jav/components/JavCardTags'
import { zh } from '@/utils/i18n'
import { Tooltip, Rating, Popper, IconButton } from '@mui/material'
import FavoriteRoundedIcon from '@mui/icons-material/FavoriteRounded'
import FavoriteBorderRoundedIcon from '@mui/icons-material/FavoriteBorderRounded'
import RemoveCircleOutlineRoundedIcon from '@mui/icons-material/RemoveCircleOutlineRounded'
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded'
import CheckBoxOutlineBlankRoundedIcon from '@mui/icons-material/CheckBoxOutlineBlankRounded'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import SearchIcon from '@mui/icons-material/Search'
import PhotoLibraryOutlinedIcon from '@mui/icons-material/PhotoLibraryOutlined'
import PhotoCameraRoundedIcon from '@mui/icons-material/PhotoCameraRounded'
import { useEffect, useState } from 'react'
import { ReleaseIcon, DurationIcon } from '@/features/jav/components/JavMetadataIcons'
import VideocamOutlinedIcon from '@mui/icons-material/VideocamOutlined'
import CollectionsBookmarkOutlinedIcon from '@mui/icons-material/CollectionsBookmarkOutlined'
import { StudioCard } from '@/features/jav/components/JavStudioView'
import { SeriesCard } from '@/features/jav/components/JavSeriesView'
import { IdolCard } from '@/features/jav/components/JavIdolGrid'
import {
  JAV_COVER_DEFAULT_CROP_LEFT,
  normalizeJavCoverCropLeft,
  resolveJavCoverObjectPosition,
} from '@/utils/javCover'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import { MovieEdit } from '@mui/icons-material'
import VideoLibraryOutlinedIcon from '@mui/icons-material/VideoLibraryOutlined'
import { JavItemEditors } from '@/features/jav/components/JavItemEditors'

function CompactCoverImage({ src, alt, cropLeft }) {
  const [imageAspect, setImageAspect] = useState(0)
  useEffect(() => {
    setImageAspect(0)
  }, [src])

  return (
    <img
      src={src}
      alt={alt}
      className="absolute inset-0 h-full w-full select-none object-cover"
      style={{ objectPosition: `${resolveJavCoverObjectPosition(cropLeft, imageAspect)}% top` }}
      loading="lazy"
      draggable={false}
      onLoad={(event) => {
        const img = event.currentTarget
        if (img.naturalWidth > 0 && img.naturalHeight > 0) {
          setImageAspect(img.naturalWidth / img.naturalHeight)
        }
      }}
    />
  )
}

export default function JavCard(props) {
  const model = useJavItemActions(props)
  const compact = Boolean(props.compact)
  const coverCropLeft = normalizeJavCoverCropLeft(
    props.item?.cover_crop_left ?? JAV_COVER_DEFAULT_CROP_LEFT
  )
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
    handleOpenCoverCropEditor,
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
    coverAspectPercent,
    showIdolWorkCount,
    hideTags,
    tags,
    tagMaxRows,
    buildTagFilterHref,
    onTagClick,
    hideActions,
    openFileLabel,
    handleOpenFile,
    handleOpenEditor,
    handleOpenVideoManager,
  } = model
  return (
    <>
      <div
        className={`jav-card flex flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition hover:shadow-lg ${checked ? 'border-sky-400 ring-2 ring-sky-200' : ''}`}
      >
        <div
          className={`card-hover-scope group relative overflow-hidden bg-white ${
            compact ? '' : 'aspect-[800/538]'
          }`}
          style={compact ? { paddingTop: `${coverAspectPercent}%` } : undefined}
        >
          {cover ? (
            compact ? (
              <CompactCoverImage
                src={cover}
                alt={item?.code || zh('JAV 封面', 'JAV cover')}
                cropLeft={coverCropLeft}
              />
            ) : (
              <JavCoverImage src={cover} alt={item?.code || zh('JAV 封面', 'JAV cover')} />
            )
          ) : (
            <div
              className={`items-center justify-center bg-gradient-to-br from-gray-100 to-gray-200 text-lg font-semibold text-gray-600 ${
                compact ? 'absolute inset-0 flex' : 'flex h-full w-full'
              }`}
            >
              {item?.code || zh('未知番号', 'Unknown code')}
            </div>
          )}
          <button
            type="button"
            className="absolute inset-0 z-[1] cursor-pointer"
            onClick={handleOpenDetail}
            aria-label={zh(`查看 ${code || 'JAV'} 详情`, `View ${code || 'JAV'} details`)}
          />
          <div className="card-hover-focus-visible pointer-events-none absolute inset-0 z-[2] flex items-center justify-center bg-black/0 text-white opacity-0 transition-opacity group-hover:opacity-100">
            <button
              onClick={handlePlay}
              disabled={!canPlay}
              className={`pointer-events-auto rounded-full p-3 ${
                canPlay ? 'bg-black/60 hover:bg-black/80' : 'cursor-not-allowed bg-black/30'
              }`}
              aria-label={zh('播放', 'Play')}
              title={zh('播放', 'Play')}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="h-10 w-10"
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
                className={`flex items-center rounded-full bg-black/70 px-1.5 py-0.5 shadow-lg shadow-black/50 transition-opacity ${
                  favoriteRatingSaving
                    ? 'opacity-60'
                    : favoriteRating > 0
                      ? 'opacity-100'
                      : 'card-hover-focus-visible opacity-0 group-hover:opacity-100'
                }`}
              >
                <span
                  className="flex overflow-hidden transition-[width] duration-150"
                  style={{ width: favoriteRatingWidth }}
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
                      fontSize: 21,
                      '& .MuiRating-iconEmpty': {
                        color: 'rgba(255,255,255,0.85)',
                      },
                    }}
                  />
                </span>
                {favoriteRatingEditing && favoriteRating > 0 ? (
                  <button
                    type="button"
                    className="ml-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-white transition hover:bg-white/20"
                    disabled={favoriteRatingSaving || !item?.id}
                    aria-label={zh('清除喜爱度评分', 'Clear favorite rating')}
                    onMouseEnter={() => setFavoriteRatingPreview(0)}
                    onMouseLeave={() => setFavoriteRatingPreview(null)}
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={(event) => handleFavoriteRatingChange(event, 0)}
                  >
                    <RemoveCircleOutlineRoundedIcon sx={{ fontSize: 15 }} />
                  </button>
                ) : null}
                {favoriteRating > 0 && !favoriteRatingEditing ? (
                  <span className="ml-1 shrink-0 text-xs font-semibold tabular-nums leading-none text-white">
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
                  className={`card-hover-focus-visible flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-black/65 shadow-lg shadow-black/40 transition hover:bg-black/80 disabled:cursor-not-allowed disabled:opacity-60 [@media(hover:none)]:opacity-100 ${
                    checked
                      ? 'text-sky-300 opacity-100'
                      : 'text-white opacity-0 group-hover:opacity-100'
                  }`}
                >
                  {checked ? (
                    <CheckBoxRoundedIcon sx={{ fontSize: 18 }} />
                  ) : (
                    <CheckBoxOutlineBlankRoundedIcon sx={{ fontSize: 18 }} />
                  )}
                </button>
              </Tooltip>
            ) : null}
          </div>
          {!compact && externalLinks.length > 0 ? (
            <div className="card-hover-focus-visible absolute bottom-2 left-2 z-10 flex max-w-[calc(100%-1rem)] items-center gap-1.5 opacity-0 transition-opacity group-hover:opacity-100">
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
                    className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-black/70 shadow-lg shadow-black/60 transition hover:bg-black/85"
                    aria-label={zh(`在 ${site.name} 中打开`, `Open in ${site.name}`)}
                    onClick={(event) => handleExternalLinkClick(event, site)}
                  >
                    <img
                      src={site.icon}
                      alt={site.name}
                      className={`${site.key === 'javmenu' ? 'h-5 w-5' : 'h-4 w-4'} ${site.loading ? 'animate-pulse' : ''}`}
                      loading="lazy"
                    />
                  </a>
                </Tooltip>
              ))}
            </div>
          ) : null}
          <button
            type="button"
            className="card-hover-focus-visible absolute right-12 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full bg-black/65 text-white opacity-0 shadow-lg shadow-black/40 transition hover:bg-black/80 group-hover:opacity-100"
            title={zh('编辑自定义标签', 'Edit custom tags')}
            aria-label={zh('编辑自定义标签', 'Edit custom tags')}
            onClick={handleOpenCustomTags}
          >
            <LocalOfferOutlinedIcon sx={{ fontSize: 18 }} />
          </button>
          <button
            type="button"
            className={`card-hover-focus-visible absolute right-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
              favoriteCount > 0
                ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
                : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-hover:opacity-100'
            }`}
            title={zh('加入作品收藏夹', 'Add to JAV favorite groups')}
            aria-label={zh('加入作品收藏夹', 'Add to JAV favorite groups')}
            onClick={handleOpenJavFavorites}
          >
            {favoriteCount > 0 ? (
              <StarRoundedIcon sx={{ fontSize: 18 }} />
            ) : (
              <StarBorderRoundedIcon sx={{ fontSize: 18 }} />
            )}
          </button>
          {cover || canOpen ? (
            <div className="card-hover-focus-visible absolute bottom-2 right-2 z-10 flex items-center gap-2 opacity-0 transition-opacity group-hover:opacity-100">
              {compact && cover ? (
                <button
                  type="button"
                  onClick={handleOpenCoverCropEditor}
                  title={zh('编辑封面起点', 'Edit cover start')}
                  aria-label={zh('编辑封面起点', 'Edit cover start')}
                  className="flex h-8 w-8 items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85"
                >
                  <PhotoCameraRoundedIcon className="h-5 w-5 text-white" fontSize="inherit" />
                </button>
              ) : null}
              {cover ? (
                <button
                  type="button"
                  onClick={handleOpenCoverPreview}
                  title={zh('查看封面', 'View cover')}
                  aria-label={zh('查看封面', 'View cover')}
                  className="flex h-8 w-8 items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85"
                >
                  <SearchIcon className="h-5 w-5 text-white" fontSize="inherit" />
                </button>
              ) : null}
              <button
                type="button"
                onClick={handleOpenScreenshots}
                disabled={!canOpen}
                title={zh('查看截图', 'View screenshots')}
                aria-label={zh('查看截图', 'View screenshots')}
                className={`flex h-8 w-8 items-center justify-center rounded-full text-white shadow-lg shadow-black/60 ${
                  canOpen ? 'bg-black/70 hover:bg-black/85' : 'cursor-not-allowed bg-black/30'
                }`}
              >
                <PhotoLibraryOutlinedIcon className="h-5 w-5 text-white" fontSize="inherit" />
              </button>
            </div>
          ) : null}
        </div>
        <div className={`flex flex-1 flex-col ${compact ? 'gap-1 p-2' : 'gap-2 p-3'}`}>
          <div
            className={`leading-tight text-gray-800 ${compact ? 'truncate text-xs' : 'text-sm'}`}
            title={titleText}
            style={compact ? undefined : titleClampStyle}
          >
            {codeText ? <span className="font-semibold text-gray-800">{codeText}</span> : null}
            {codeText ? ' ' : null}
            <span className="font-medium text-gray-800">{mainTitle}</span>
          </div>
          <div
            className={`flex min-w-0 flex-nowrap items-center overflow-hidden text-gray-600 ${
              compact ? 'gap-x-2 text-[11px]' : 'gap-x-3 text-xs'
            }`}
          >
            <span className="inline-flex shrink-0 items-center gap-1">
              <Tooltip title={zh('发行日期', 'Release date')} arrow>
                <span className="inline-flex">
                  <ReleaseIcon />
                </span>
              </Tooltip>
              <span>{releaseText}</span>
            </span>
            <span className="inline-flex shrink-0 items-center gap-1">
              <Tooltip title={zh('时长', 'Duration')} arrow>
                <span className="inline-flex">
                  <DurationIcon />
                </span>
              </Tooltip>
              <span>{durationText || zh('时长未知', 'Unknown duration')}</span>
            </span>
            {!compact && studioText ? (
              <span className="flex min-w-0 flex-1 items-center gap-1 overflow-hidden">
                <Tooltip title={zh('片商', 'Studio')} arrow>
                  <span className="inline-flex">
                    <VideocamOutlinedIcon sx={{ fontSize: 16 }} className="shrink-0 text-sky-600" />
                  </span>
                </Tooltip>
                <a
                  href={buildStudioFilterHref(item.studio)}
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
            ) : null}
          </div>
          {!compact && !hideSeries && seriesText ? (
            <div className="flex min-w-0 items-center gap-1 text-xs text-gray-600">
              <Tooltip title={zh('系列', 'Series')} arrow>
                <span className="inline-flex">
                  <CollectionsBookmarkOutlinedIcon
                    sx={{ fontSize: 14 }}
                    className="shrink-0 text-emerald-600"
                  />
                </span>
              </Tooltip>
              <a
                href={buildSeriesFilterHref(preferredSeries)}
                className={`min-w-0 whitespace-normal break-words text-left leading-snug ${
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
          <Popper
            open={Boolean(!compact && previewStudio && studioHoverAnchorEl)}
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
            open={Boolean(!compact && previewSeries && seriesHoverAnchorEl)}
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
                  href={buildSeriesFilterHref(previewSeries)}
                  onSelectSeries={(series) => onSeriesClick?.(series)}
                  onSelectStudio={(studio) => onStudioClick?.(studio)}
                  onOpenFavorites={onOpenSeriesFavorites}
                />
              ) : null}
            </div>
          </Popper>
          {!compact && !hideIdols && Array.isArray(item?.idols) && item.idols.length > 0 && (
            <>
              <IdolTagList
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
                open={Boolean(!compact && previewIdol && idolHoverAnchorEl)}
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
          {!compact && !hideTags && tags.length > 0 && (
            <JavTagList
              tags={tags}
              maxRows={tagMaxRows}
              buildTagFilterHref={buildTagFilterHref}
              onTagClick={onTagClick}
              onFilterLinkClick={handleFilterLinkClick}
            />
          )}
          {!compact && !hideActions ? (
            <div className="flex flex-wrap items-center gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <Tooltip title={openFileLabel || zh('用默认程序打开', 'Open with default app')}>
                  <IconButton
                    size="small"
                    onClick={handleOpenFile}
                    disabled={!canOpen}
                    aria-label={openFileLabel || zh('打开文件', 'Open file')}
                    className="h-6 w-6"
                  >
                    <PlayArrowIcon fontSize="inherit" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={zh('编辑 JAV', 'Edit JAV')}>
                  <IconButton
                    size="small"
                    onClick={handleOpenEditor}
                    aria-label={zh('编辑 JAV', 'Edit JAV')}
                    className="h-6 w-6"
                  >
                    <MovieEdit fontSize="inherit" />
                  </IconButton>
                </Tooltip>
                <Tooltip title={zh('视频管理', 'Manage videos')}>
                  <IconButton
                    size="small"
                    onClick={handleOpenVideoManager}
                    disabled={!Array.isArray(item?.videos) || item.videos.length === 0}
                    aria-label={zh('视频管理', 'Manage videos')}
                    className="h-6 w-6"
                  >
                    <VideoLibraryOutlinedIcon fontSize="inherit" />
                  </IconButton>
                </Tooltip>
              </div>
              {Array.isArray(item?.videos) && item.videos.length > 1 && (
                <span className="text-xs text-gray-500">
                  {zh(`${item.videos.length} 个视频`, `${item.videos.length} video files`)}
                </span>
              )}
            </div>
          ) : null}
        </div>
      </div>
      <JavItemEditors {...model} />
    </>
  )
}
