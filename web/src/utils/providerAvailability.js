// Keep batch checks bounded and publish each result as soon as it arrives.
// Cancellation prevents both queued requests and stale updates after a proxy change.
export async function runProviderChecks(providers, check, onResult, signal) {
  let next = 0
  const worker = async () => {
    while (!signal.aborted && next < providers.length) {
      const provider = providers[next++]
      onResult(provider.id, { status: 'checking' })
      try {
        const result = await check(provider.id, { signal })
        if (!signal.aborted) onResult(provider.id, result)
      } catch (error) {
        if (!signal.aborted) {
          onResult(provider.id, { status: 'request_error', error })
        }
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(3, providers.length) }, worker))
}
