import { Slider } from '@mui/material'
import FavoriteBorderRoundedIcon from '@mui/icons-material/FavoriteBorderRounded'
import FavoriteRoundedIcon from '@mui/icons-material/FavoriteRounded'
import { zh } from '@/utils/i18n'

export default function FavoriteRatingFilter({
  enabled,
  min,
  max,
  onEnabledChange,
  onRangeChange,
  compact = false,
}) {
  const normalizedMin = Number.isFinite(Number(min)) ? Number(min) : 0.5
  const normalizedMax = Number.isFinite(Number(max)) ? Number(max) : 5
  const range = normalizedMin <= normalizedMax ? [normalizedMin, normalizedMax] : [0.5, 5]
  const formatValue = (value) => (Number.isInteger(value) ? String(value) : value.toFixed(1))
  const toggleLabel = enabled
    ? zh('关闭喜爱度筛选', 'Disable favorite rating filter')
    : zh('启用喜爱度筛选', 'Enable favorite rating filter')

  return (
    <div
      className={`favorite-rating-filter ${enabled ? 'favorite-rating-filter--active' : ''} ${compact ? 'favorite-rating-filter--compact' : ''}`}
    >
      <button
        type="button"
        className="favorite-rating-filter__toggle"
        onClick={() => onEnabledChange?.(!enabled)}
        title={toggleLabel}
        aria-label={toggleLabel}
        aria-pressed={Boolean(enabled)}
      >
        {enabled ? (
          <FavoriteRoundedIcon fontSize="inherit" />
        ) : (
          <FavoriteBorderRoundedIcon fontSize="inherit" />
        )}
      </button>
      <Slider
        value={range}
        onChange={(_, value) => {
          if (Array.isArray(value)) onRangeChange?.(value)
        }}
        min={0.5}
        max={5}
        step={0.5}
        disableSwap
        disabled={!enabled}
        getAriaLabel={(index) =>
          index === 0
            ? zh('最低喜爱度', 'Minimum favorite rating')
            : zh('最高喜爱度', 'Maximum favorite rating')
        }
        sx={{
          gridColumn: 3,
          width: '100%',
          minWidth: 0,
          alignSelf: 'center',
          transform: 'translateY(-1px)',
          p: 0,
          height: 3,
          '& .MuiSlider-rail, & .MuiSlider-track': { height: 3 },
          '& .MuiSlider-thumb': { width: 10, height: 10 },
          '& .MuiSlider-thumb::after': { width: 16, height: 16 },
        }}
      />
      <span className="favorite-rating-filter__value">
        {formatValue(range[0])}–{formatValue(range[1])}
      </span>
    </div>
  )
}
