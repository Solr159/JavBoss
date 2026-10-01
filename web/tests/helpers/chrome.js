import { spawn } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
export const chromePath = process.env.CHROME_BIN || '/usr/bin/google-chrome'

export async function startChrome(
  t,
  { executablePath = chromePath, args = [], startupTimeoutMs = 30000 } = {}
) {
  const profile = await mkdtemp(path.join(os.tmpdir(), 'javboss-browser-'))
  const startedAt = performance.now()
  const chrome = spawn(
    executablePath,
    [
      ...args,
      '--headless',
      '--no-sandbox',
      '--disable-gpu',
      '--remote-debugging-port=0',
      `--user-data-dir=${profile}`,
      '--no-first-run',
      '--no-default-browser-check',
      'about:blank',
    ],
    { stdio: ['ignore', 'ignore', 'pipe'] }
  )
  let stderr = ''
  let spawnError
  chrome.stderr.setEncoding('utf8')
  chrome.stderr.on('data', (chunk) => {
    // Keep diagnostics bounded, including after startup succeeds.
    stderr = (stderr + chunk).slice(-16384)
  })
  chrome.on('error', (error) => {
    spawnError = error
  })
  const closed = new Promise((resolve) => chrome.once('close', resolve))
  let cleanup
  const close = () => {
    cleanup ??= (async () => {
      let killTimer
      if (chrome.pid && chrome.exitCode === null && chrome.signalCode === null) {
        killTimer = setTimeout(() => chrome.kill('SIGKILL'), 2000)
        killTimer.unref()
        chrome.kill()
      }
      await closed
      clearTimeout(killTimer)
      await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 })
    })()
    return cleanup
  }
  t.after(close)

  const startupError = (reason, cause) =>
    new Error(
      `Chrome ${reason} after ${Math.round(performance.now() - startedAt)}ms\n` +
        `Executable: ${executablePath}\n` +
        `Exit code: ${chrome.exitCode}; signal: ${chrome.signalCode}\n` +
        `Chrome stderr (last 16384 characters):\n${stderr.trim() || '(empty)'}`,
      { cause }
    )

  try {
    while (performance.now() - startedAt < startupTimeoutMs) {
      if (t.signal?.aborted) throw startupError('startup cancelled', t.signal.reason)
      if (spawnError) throw startupError(`failed to launch: ${spawnError.message}`, spawnError)
      if (chrome.exitCode !== null || chrome.signalCode !== null) {
        await closed
        throw startupError('exited before the debugging port was ready')
      }
      try {
        const contents = await readFile(path.join(profile, 'DevToolsActivePort'), 'utf8')
        const port = Number(contents.split('\n')[0].trim())
        if (Number.isInteger(port) && port > 0 && port <= 65535) return { port, close }
      } catch (error) {
        if (error.code !== 'ENOENT') {
          throw startupError(`could not read DevToolsActivePort: ${error.message}`, error)
        }
      }
      await pause(50)
    }
    throw startupError(`timed out waiting for DevToolsActivePort (${startupTimeoutMs}ms limit)`)
  } catch (error) {
    await close()
    throw error
  }
}
