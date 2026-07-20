const DEFAULT_MIN_CARD_WIDTH = 260
const DEFAULT_CARD_HEIGHT = 86
const DEFAULT_GAP = 9
const DEFAULT_OVERSCAN_ROWS = 1
const DEFAULT_MAX_COLUMNS = 6

function positiveNumber(value, fallback) {
  const numeric = Number(value)
  return Number.isFinite(numeric) && numeric > 0 ? numeric : fallback
}

export function resolveVirtualGridColumns(
  viewportWidth,
  minCardWidth = DEFAULT_MIN_CARD_WIDTH,
  gap = DEFAULT_GAP,
  maxColumns = DEFAULT_MAX_COLUMNS,
) {
  const width = Math.max(0, Number(viewportWidth) || 0)
  const cardWidth = positiveNumber(minCardWidth, DEFAULT_MIN_CARD_WIDTH)
  const safeGap = Math.max(0, Number(gap) || 0)
  const columnCap = Math.max(1, Math.floor(positiveNumber(maxColumns, DEFAULT_MAX_COLUMNS)))
  return Math.min(columnCap, Math.max(1, Math.floor((width + safeGap) / (cardWidth + safeGap))))
}

export function resolveVirtualGridWindow({
  itemCount = 0,
  viewportWidth = 0,
  viewportHeight = 0,
  scrollTop = 0,
  minCardWidth = DEFAULT_MIN_CARD_WIDTH,
  cardHeight = DEFAULT_CARD_HEIGHT,
  gap = DEFAULT_GAP,
  overscanRows = DEFAULT_OVERSCAN_ROWS,
  maxColumns = DEFAULT_MAX_COLUMNS,
} = {}) {
  const count = Math.max(0, Math.floor(Number(itemCount) || 0))
  const columns = resolveVirtualGridColumns(viewportWidth, minCardWidth, gap, maxColumns)
  if (!count) return { columns, startIndex: 0, endIndex: 0, offsetTop: 0, totalHeight: 0 }

  const safeCardHeight = positiveNumber(cardHeight, DEFAULT_CARD_HEIGHT)
  const safeGap = Math.max(0, Number(gap) || 0)
  const rowHeight = safeCardHeight + safeGap
  const totalRows = Math.ceil(count / columns)
  const totalHeight = totalRows * rowHeight - safeGap
  const height = Math.max(0, Number(viewportHeight) || 0)
  const maxScrollTop = Math.max(0, totalHeight - height)
  const clampedScrollTop = Math.min(Math.max(0, Number(scrollTop) || 0), maxScrollTop)
  const overscan = Math.max(0, Math.floor(Number(overscanRows) || 0))
  const firstVisibleRow = Math.floor(clampedScrollTop / rowHeight)
  const startRow = Math.max(0, firstVisibleRow - overscan)
  const visibleEndRow = Math.ceil((clampedScrollTop + height) / rowHeight)
  const endRow = Math.min(totalRows, visibleEndRow + overscan)

  return {
    columns,
    startIndex: startRow * columns,
    endIndex: Math.min(count, endRow * columns),
    offsetTop: startRow * rowHeight,
    totalHeight,
  }
}
