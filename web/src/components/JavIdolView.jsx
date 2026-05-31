import SettingsRoundedIcon from '@mui/icons-material/SettingsRounded'
import { IconButton } from '@mui/material'
import JavIdolGrid from '@/components/JavIdolGrid'
import Pagination from '@/components/Pagination'
import WaterfallLoader from '@/components/WaterfallLoader'
import { zh } from '@/utils/i18n'

export default function JavIdolView({
  page,
  lastPage,
  hasPrev,
  hasNext,
  loading,
  buildPageUrl,
  buildGroupUrl,
  buildIdolUrl,
  javMetadataLanguage,
  onFirst,
  onPrev,
  onGoToPage,
  onNext,
  onLast,
  items,
  favoriteGroups = [],
  selectedFavoriteGroupId = null,
  favoriteGroupsLoading = false,
  favoriteGroupsError = null,
  onSelectIdol,
  onFavoriteGroupSelect,
  onOpenFavorites,
  onOpenFavoriteManager,
  waterfallMode,
  onWaterfallModeChange,
  onLoadMore,
  loadingMore,
  hasMore,
}) {
  return (
    <>
      <div className="sticky-pagination mb-2.5">
        <div className="relative grid gap-3 md:grid-cols-[1fr_auto_1fr] md:items-center">
          <div />
          <div className="flex justify-center overflow-x-auto">
            <Pagination
              page={page}
              lastPage={lastPage}
              hasPrev={hasPrev}
              hasNext={hasNext}
              loading={loading}
              buildPageUrl={buildPageUrl}
              onFirst={onFirst}
              onPrev={onPrev}
              onGoToPage={onGoToPage}
              onNext={onNext}
              onLast={onLast}
              waterfallMode={waterfallMode}
              onWaterfallModeChange={onWaterfallModeChange}
            />
          </div>
          <div className="hidden md:block" />
        </div>
        <div className="mt-1 pl-2">
          <IdolFavoriteGroupRow
            groups={favoriteGroups}
            selectedGroupId={selectedFavoriteGroupId}
            loading={favoriteGroupsLoading}
            error={favoriteGroupsError}
            buildGroupUrl={buildGroupUrl}
            onSelect={onFavoriteGroupSelect}
            onOpenManager={onOpenFavoriteManager}
          />
        </div>
      </div>
      {loading ? (
        <div className="mt-4 flex min-h-[200px] items-center justify-center rounded border border-dashed border-gray-200 text-gray-500">
          {zh('加载中…', 'Loading...')}
        </div>
      ) : (
        <JavIdolGrid
          items={items}
          onSelectIdol={onSelectIdol}
          onOpenFavorites={onOpenFavorites}
          buildIdolUrl={buildIdolUrl}
          javMetadataLanguage={javMetadataLanguage}
        />
      )}
      <WaterfallLoader
        enabled={waterfallMode && !loading}
        hasMore={hasMore}
        loading={loadingMore}
        onLoadMore={onLoadMore}
      />
    </>
  )
}

function IdolFavoriteGroupRow({
  groups,
  selectedGroupId,
  error,
  buildGroupUrl,
  onSelect,
  onOpenManager,
}) {
  const list = Array.isArray(groups) ? groups : []
  const selected = Number(selectedGroupId) || null

  if (error && list.length === 0) {
    return <div className="text-xs text-red-600">{String(error)}</div>
  }

  return (
    <div className="flex max-w-full items-center gap-1.5">
      <div className="flex min-w-0 flex-1 flex-nowrap justify-start gap-1.5 overflow-x-auto overflow-y-hidden whitespace-nowrap pb-1">
        <FavoriteGroupLink
          active={!selected}
          href={buildGroupUrl?.(null)}
          onClick={() => onSelect?.(null)}
          label={zh('全部', 'All')}
        />
        {list.map((group) => {
          const id = Number(group?.id)
          if (!Number.isFinite(id) || id <= 0) return null
          const count = Number.isFinite(group?.count) ? group.count : 0
          return (
            <FavoriteGroupLink
              key={id}
              active={selected === id}
              href={buildGroupUrl?.(id)}
              onClick={() => onSelect?.(id)}
              label={group?.name || zh('未命名收藏夹', 'Untitled favorite group')}
              count={count}
            />
          )
        })}
        <IconButton
          type="button"
          size="small"
          onClick={onOpenManager}
          aria-label={zh('管理女优收藏夹', 'Manage idol favorites')}
          title={zh('管理女优收藏夹', 'Manage idol favorites')}
          sx={{
            width: 24,
            height: 24,
            flex: '0 0 auto',
            border: '1px solid rgb(229 231 235)',
            bgcolor: 'white',
            '&:hover': { bgcolor: 'rgb(249 250 251)' },
          }}
        >
          <SettingsRoundedIcon sx={{ fontSize: 15 }} />
        </IconButton>
      </div>
    </div>
  )
}

function FavoriteGroupLink({ active, href, onClick, label, count }) {
  return (
    <a
      href={href || '#'}
      onClick={(event) => {
        if (
          event.metaKey ||
          event.ctrlKey ||
          event.shiftKey ||
          event.altKey ||
          event.button !== 0
        ) {
          return
        }
        event.preventDefault()
        onClick?.()
      }}
      className={`inline-flex h-6 max-w-[12rem] items-center gap-1.5 rounded-full border px-2.5 text-xs transition-colors ${
        active
          ? 'border-blue-500 bg-blue-50 text-blue-700 shadow-sm'
          : 'border-gray-200 bg-white/90 text-gray-600 hover:border-gray-300 hover:bg-gray-50'
      }`}
      title={label}
    >
      <span className="truncate">{label}</span>
      {Number.isFinite(count) ? (
        <span
          className={`rounded-full px-1.5 text-[10px] leading-4 ${
            active ? 'bg-blue-100 text-blue-700' : 'bg-gray-100 text-gray-500'
          }`}
        >
          {count}
        </span>
      ) : null}
    </a>
  )
}
