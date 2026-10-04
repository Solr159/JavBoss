// One owner for a list's requests. The state adapter keeps this independent of React/Zustand.
export function createListResource({
  get,
  set,
  fields,
  query,
  key,
  fetcher,
  random = () => false,
  errorMessage = (error) => error.message,
  hasNextField,
  randomTotal = false,
  active = () => true,
  waterfall = () => false,
}) {
  let generation = 0
  let successfulKey = null
  // Invalidation makes the cache stale without discarding its loaded range.
  let itemsKey = null
  let pending = null
  let pendingMore = null
  let exhaustedKey = null

  const isCurrent = (request) => request.generation === generation && request.key === key(get())
  const invalidate = () => {
    generation += 1
    pending?.controller.abort()
    pendingMore?.controller.abort()
    pending = null
    pendingMore = null
    successfulKey = null
    exhaustedKey = null
    set({ [fields.loading]: false, [fields.loadingMore]: false })
  }

  const load = (options = {}) => {
    const state = get()
    const requestKey = key(state)
    if (!options.force && pending?.key === requestKey) return pending.promise
    if (!options.force && successfulKey === requestKey) return Promise.resolve()
    invalidate()
    const params = query(state)
    const sameQuery = itemsKey === requestKey
    const background = options.reconcile && (sameQuery || options.background)
    // Revalidate the remaining loaded range without replacing the grid with a spinner.
    if (options.reconcile && sameQuery && waterfall(state) && !random(state)) {
      params.limit = Math.max(1, state[fields.items]?.length || 0)
    }
    const request = { generation, key: requestKey, controller: new AbortController() }
    pending = request
    set({
      [fields.loading]: !background,
      [fields.loadingMore]: Boolean(background),
      [fields.error]: null,
    })
    request.promise = (async () => {
      try {
        const response = await fetcher({ ...params, signal: request.controller.signal })
        if (!isCurrent(request)) return
        const items = response.items || []
        const total = random(state) && randomTotal ? items.length : (response.total ?? 0)
        if (
          options.reconcile &&
          fields.page &&
          !random(state) &&
          params.offset > 0 &&
          params.offset >= total
        ) {
          const lastPage = Math.max(1, Math.ceil(total / query(state).limit))
          set({ [fields.page]: lastPage, [fields.total]: total })
          return load({ force: true, reconcile: true, background })
        }
        const patch = { [fields.items]: items, [fields.total]: total }
        if (hasNextField)
          patch[hasNextField] = !random(state) && params.offset + params.limit < total
        successfulKey = requestKey
        itemsKey = requestKey
        set(patch)
      } catch (error) {
        if (isCurrent(request) && !request.controller.signal.aborted) {
          set({ [fields.error]: errorMessage(error) })
        }
      } finally {
        if (pending === request) {
          pending = null
          set({ [fields.loading]: false, [fields.loadingMore]: false })
        }
      }
    })()
    return request.promise
  }

  const loadMore = () => {
    const state = get()
    const requestKey = key(state)
    // A background reconciliation must finish before choosing the next offset.
    if (pending?.key === requestKey) return pending.promise
    if (pendingMore?.key === requestKey) return pendingMore.promise
    if (
      state[fields.loading] ||
      random(state) ||
      requestKey !== itemsKey ||
      exhaustedKey === requestKey
    )
      return Promise.resolve()
    const params = query(state)
    const loaded = state[fields.items]?.length || 0
    const total = state[fields.total] || 0
    const offset = params.offset + loaded
    if (total > 0 && offset >= total) return Promise.resolve()
    const request = { generation, key: requestKey, controller: new AbortController() }
    pendingMore = request
    set({ [fields.loadingMore]: true, [fields.error]: null })
    request.promise = (async () => {
      try {
        const response = await fetcher({ ...params, offset, signal: request.controller.signal })
        if (!isCurrent(request)) return
        const items = response.items || []
        const nextTotal = response.total ?? total
        if (!items.length) exhaustedKey = requestKey
        const patch = {
          [fields.items]: [...(get()[fields.items] || []), ...items],
          [fields.total]: nextTotal,
        }
        if (hasNextField)
          patch[hasNextField] =
            items.length > 0 &&
            (nextTotal > 0 ? offset + items.length < nextTotal : items.length >= params.limit)
        set(patch)
      } catch (error) {
        if (isCurrent(request) && !request.controller.signal.aborted) {
          set({ [fields.error]: errorMessage(error) })
        }
      } finally {
        if (pendingMore === request) {
          pendingMore = null
          set({ [fields.loadingMore]: false })
        }
      }
    })()
    return request.promise
  }

  const reconcile = (remove = () => false) => {
    const state = get()
    const items = (state[fields.items] || []).filter((item) => !remove(item))
    const removed = (state[fields.items]?.length || 0) - items.length
    invalidate()
    set({
      [fields.items]: items,
      // Only loaded rows have known membership. The response supplies the filtered total.
      [fields.total]: Math.max(0, (state[fields.total] || 0) - removed),
    })
    return active(get()) ? load({ force: true, reconcile: true }) : Promise.resolve()
  }

  return { load, loadMore, invalidate, reconcile }
}
