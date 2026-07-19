import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'

import {
  authExpiredEvent,
  changePassword as changePasswordRequest,
  fetchAuthStatus,
  loginWithPassword,
  logoutSession,
} from '@/api'
import LoginPage from '@/components/LoginPage'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [checking, setChecking] = useState(true)
  const [authenticated, setAuthenticated] = useState(false)
  const [checkError, setCheckError] = useState('')

  const checkStatus = useCallback(async () => {
    setChecking(true)
    setCheckError('')
    try {
      const status = await fetchAuthStatus()
      setAuthenticated(Boolean(status?.authenticated))
    } catch (err) {
      setAuthenticated(false)
      setCheckError(getErrorMessage(err))
    } finally {
      setChecking(false)
    }
  }, [])

  useEffect(() => {
    checkStatus()
  }, [checkStatus])

  useEffect(() => {
    const handleExpired = () => setAuthenticated(false)
    window.addEventListener(authExpiredEvent, handleExpired)
    return () => window.removeEventListener(authExpiredEvent, handleExpired)
  }, [])

  const login = useCallback(async (password) => {
    await loginWithPassword(password)
    setAuthenticated(true)
    setCheckError('')
  }, [])

  const logout = useCallback(async () => {
    await logoutSession()
    setAuthenticated(false)
  }, [])

  const changePassword = useCallback(async (currentPassword, newPassword) => {
    await changePasswordRequest(currentPassword, newPassword)
  }, [])

  const value = useMemo(
    () => ({ authenticated, login, logout, changePassword }),
    [authenticated, changePassword, login, logout]
  )

  if (checking) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-zinc-100 text-sm text-zinc-500">
        {zh('正在检查登录状态…', 'Checking sign-in status...')}
      </div>
    )
  }

  if (!authenticated) {
    return <LoginPage onLogin={login} checkError={checkError} onRetry={checkStatus} />
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used within AuthProvider')
  return context
}
