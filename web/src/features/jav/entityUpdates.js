export function mergeJavItem(items, updated) {
  if (!updated?.id || !Array.isArray(items)) return items
  return items.map((item) =>
    Number(item.id) === Number(updated.id) ? { ...item, ...updated } : item
  )
}

export function mergeJavIdol(items, updated) {
  if (!updated?.id || !Array.isArray(items)) return items
  return items.map((item) => {
    if (!item.idols?.some((idol) => Number(idol.id) === Number(updated.id))) return item
    return {
      ...item,
      idols: item.idols.map((idol) =>
        Number(idol.id) === Number(updated.id) ? { ...idol, ...updated } : idol
      ),
    }
  })
}
