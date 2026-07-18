import { useState } from 'react'
import VisibilityOffOutlinedIcon from '@mui/icons-material/VisibilityOffOutlined'
import VisibilityOutlinedIcon from '@mui/icons-material/VisibilityOutlined'

import { zh } from '@/utils/i18n'

export default function LoginPage({ onLogin, checkError = '', onRetry }) {
  const [password, setPassword] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
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
          <div>
            <label
              htmlFor="login-password"
              className="mb-1.5 block text-sm font-medium text-zinc-700"
            >
              {zh('密码', 'Password')}
            </label>
            <div className="relative">
              <input
                id="login-password"
                type={passwordVisible ? 'text' : 'password'}
                autoComplete="current-password"
                value={password}
                onChange={(event) => {
                  setPassword(event.target.value)
                  setError('')
                }}
                className="w-full rounded-xl border border-zinc-200 bg-white py-2.5 pl-3 pr-11 text-zinc-900 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              />
              <button
                type="button"
                onClick={() => setPasswordVisible((visible) => !visible)}
                className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center justify-center rounded-md p-1 text-zinc-400 hover:bg-zinc-100 hover:text-zinc-700"
                aria-label={
                  passwordVisible
                    ? zh('隐藏密码', 'Hide password')
                    : zh('显示密码', 'Show password')
                }
              >
                {passwordVisible ? (
                  <VisibilityOutlinedIcon fontSize="small" aria-hidden="true" />
                ) : (
                  <VisibilityOffOutlinedIcon fontSize="small" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>
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
                    '找到项目目录，进入 data 文件夹，在里面新建 password_reset.txt。',
                    'Find the project directory, open the data folder, and create password_reset.txt inside it.'
                  )}
                </li>
                <li>
                  {zh(
                    '在 password_reset.txt 中填入一个 6-20 个字符的新密码。',
                    'Enter a new 6-20 character password in password_reset.txt.'
                  )}
                </li>
                <li>
                  {zh(
                    '重新启动 JavBoss；密码会自动重置，旧登录全部失效，重置文件会被自动删除。',
                    'Restart JavBoss. The password is reset, old sessions are revoked, and the reset file is deleted automatically.'
                  )}
                </li>
              </ol>
            </div>
          ) : null}
        </div>

        <p className="mt-5 text-center text-xs text-zinc-400">
          {zh(
            '默认密码：admin，登陆后可在全局设置中修改',
            'Default password: admin. You can change it in Global Settings after signing in.'
          )}
        </p>
      </section>
    </main>
  )
}
