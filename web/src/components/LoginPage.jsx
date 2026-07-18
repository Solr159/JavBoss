import { useState } from 'react'

import { zh } from '@/utils/i18n'

export default function LoginPage({ onLogin, checkError = '', onRetry }) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [recoveryOpen, setRecoveryOpen] = useState(false)

  const handleSubmit = async (event) => {
    event.preventDefault()
    if (!password) return
    setError('')
    setSubmitting(true)
    try {
      await onLogin(password)
    } catch (err) {
      setError(err.message || zh('登录失败', 'Login failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-gradient-to-br from-zinc-100 via-white to-zinc-200 px-4">
      <section className="w-full max-w-md rounded-[28px] border border-white/80 bg-white/90 p-8 shadow-2xl shadow-zinc-300/50 backdrop-blur">
        <div className="mb-8 text-center">
          <div className="text-3xl font-bold tracking-tight text-zinc-900">JavBoss</div>
          <p className="mt-2 text-sm text-zinc-500">
            {zh('请输入密码继续', 'Enter your password to continue')}
          </p>
        </div>

        {checkError ? (
          <div className="mb-4 rounded-xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700">
            <div>{checkError}</div>
            <button type="button" onClick={onRetry} className="mt-2 font-medium underline">
              {zh('重新连接', 'Retry')}
            </button>
          </div>
        ) : null}

        <form onSubmit={handleSubmit} className="space-y-4">
          <label className="block">
            <span className="mb-1.5 block text-sm font-medium text-zinc-700">
              {zh('密码', 'Password')}
            </span>
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => {
                setPassword(event.target.value)
                setError('')
              }}
              className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2.5 text-zinc-900 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
            />
          </label>
          {error ? <div className="text-sm text-red-600">{error}</div> : null}
          <button
            type="submit"
            disabled={submitting || !password}
            className="w-full rounded-xl bg-blue-600 px-4 py-2.5 font-medium text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? zh('登录中…', 'Signing in...') : zh('登录', 'Sign in')}
          </button>
        </form>

        <div className="mt-5 border-t border-zinc-200 pt-4">
          <button
            type="button"
            aria-expanded={recoveryOpen}
            aria-controls="password-recovery-help"
            onClick={() => setRecoveryOpen((value) => !value)}
            className="flex w-full items-center justify-between text-left text-sm font-medium text-zinc-600 hover:text-zinc-900"
          >
            <span>{zh('忘记密码？', 'Forgot your password?')}</span>
            <span aria-hidden="true" className="text-xs text-zinc-400">
              {recoveryOpen ? '▲' : '▼'}
            </span>
          </button>
          {recoveryOpen ? (
            <div
              id="password-recovery-help"
              className="mt-3 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900"
            >
              <ol className="list-decimal space-y-2 pl-5">
                <li>{zh('先停止 JavBoss。', 'Stop JavBoss first.')}</li>
                <li>
                  {zh(
                    '在数据目录新建 password_reset.txt，文件中只填写一个 6-20 个字符的新密码。',
                    'Create password_reset.txt in the data directory and put only a new 6-20 character password in it.'
                  )}
                </li>
                <li>
                  {zh(
                    '重新启动 JavBoss；密码会自动重置，旧登录全部失效，重置文件会被自动删除。',
                    'Restart JavBoss. The password is reset, old sessions are revoked, and the reset file is deleted automatically.'
                  )}
                </li>
              </ol>
              <p className="mt-3 text-xs text-amber-700">
                {zh(
                  '桌面版和 Docker Compose：项目目录/data/password_reset.txt。',
                  'Desktop and Docker Compose: project directory/data/password_reset.txt.'
                )}
              </p>
            </div>
          ) : null}
        </div>

        <p className="mt-5 text-center text-xs text-zinc-400">
          {zh(
            '默认密码：admin，登录后请及时修改',
            'Default password: admin. Change it after signing in.'
          )}
        </p>
      </section>
    </main>
  )
}
