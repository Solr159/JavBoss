import assert from 'node:assert/strict'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import test from 'node:test'
import { startChrome } from './chrome.js'

const fakeChrome = (source, options = {}) => ({
  executablePath: process.execPath,
  args: [
    '--input-type=commonjs',
    '-e',
    `const fs = require('node:fs');
     const path = require('node:path');
     const profile = process.argv.find(arg => arg.startsWith('--user-data-dir=')).split('=')[1];
     const portFile = path.join(profile, 'DevToolsActivePort');
     ${source}`,
    '--',
  ],
  ...options,
})

test('Chrome may take more than five seconds to publish a valid debugging port', async (t) => {
  const browser = await startChrome(
    t,
    fakeChrome(`
      fs.writeFileSync(portFile, '');
      setTimeout(() => fs.writeFileSync(portFile, '43210\\n/devtools/browser/test'), 5200);
      setInterval(() => {}, 1000);
    `)
  )
  assert.equal(browser.port, 43210)
  await browser.close()
})

test('Chrome launch errors include the executable and original OS error', async (t) => {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'javboss-chrome-missing-'))
  t.after(() => rm(directory, { recursive: true, force: true }))
  const executablePath = path.join(directory, 'missing-chrome')
  await assert.rejects(startChrome(t, { executablePath }), (error) => {
    assert.equal(error.cause.code, 'ENOENT')
    assert.ok(error.message.includes(executablePath))
    assert.match(error.message, /failed to launch/)
    return true
  })
})

test('Chrome exiting before startup reports its exit code and stderr', async (t) => {
  await assert.rejects(
    startChrome(
      t,
      fakeChrome(`process.stderr.write('required library is missing\\n', () => process.exit(23));`)
    ),
    (error) => {
      assert.match(error.message, /exited before the debugging port was ready/)
      assert.match(error.message, /Exit code: 23/)
      assert.match(error.message, /required library is missing/)
      return true
    }
  )
})

test(
  'Chrome terminated by a signal reports the signal',
  { skip: process.platform === 'win32' },
  async (t) => {
    await assert.rejects(
      startChrome(t, fakeChrome(`process.kill(process.pid, 'SIGTERM');`)),
      /signal: SIGTERM/
    )
  }
)

test('Chrome startup timeout includes stderr and cleans up the process and profile', async (t) => {
  const directory = await mkdtemp(path.join(os.tmpdir(), 'javboss-chrome-timeout-'))
  t.after(() => rm(directory, { recursive: true, force: true }))
  const stateFile = path.join(directory, 'state.json')
  await assert.rejects(
    startChrome(
      t,
      fakeChrome(
        `fs.writeFileSync(${JSON.stringify(stateFile)}, JSON.stringify({pid: process.pid, profile}));
         fs.writeFileSync(portFile, 'invalid-port');
         process.stderr.write('startup stalled\\n');
         setInterval(() => {}, 1000);`,
        { startupTimeoutMs: 1000 }
      )
    ),
    (error) => {
      assert.match(error.message, /timed out waiting for DevToolsActivePort \(1000ms limit\)/)
      assert.match(error.message, /startup stalled/)
      return true
    }
  )
  const { pid, profile } = JSON.parse(await readFile(stateFile, 'utf8'))
  assert.equal(existsSync(profile), false, 'temporary profile is removed')
  assert.throws(() => process.kill(pid, 0), { code: 'ESRCH' }, 'child process is stopped')
})
