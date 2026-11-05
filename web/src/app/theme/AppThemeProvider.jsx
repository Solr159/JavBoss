import { createContext, useContext, useEffect, useLayoutEffect, useMemo, useState } from 'react'
import { createTheme, ThemeProvider } from '@mui/material'
import {
  applyTheme,
  normalizeTheme,
  readTheme,
  resolveTheme,
  SYSTEM_THEME_QUERY,
  THEME_STORAGE_KEY,
} from '@/app/theme/preferences'

const ThemeContext = createContext({ preference: 'light', setPreference: null })

export function useAppTheme() {
  return useContext(ThemeContext)
}

export default function AppThemeProvider({ children }) {
  const [preference, updatePreference] = useState(readTheme)
  const [systemDark, setSystemDark] = useState(() => matchMedia(SYSTEM_THEME_QUERY).matches)
  const mode = resolveTheme(preference, systemDark)

  useLayoutEffect(() => applyTheme(mode), [mode])

  useEffect(() => {
    const media = matchMedia(SYSTEM_THEME_QUERY)
    const onSystemChange = () => setSystemDark(media.matches)
    const onStorage = (event) => {
      if (event.key === THEME_STORAGE_KEY || event.key === null) updatePreference(readTheme())
    }
    media.addEventListener('change', onSystemChange)
    window.addEventListener('storage', onStorage)
    onSystemChange()
    return () => {
      media.removeEventListener('change', onSystemChange)
      window.removeEventListener('storage', onStorage)
    }
  }, [])

  const theme = useMemo(
    () =>
      createTheme({
        palette: {
          mode,
          ...(mode === 'dark'
            ? {
                background: { default: '#181818', paper: '#262626' },
                text: { primary: '#f2f2f2', secondary: '#a3a3a3' },
                divider: '#383838',
                primary: { main: '#3b82f6', dark: '#2563eb' },
                action: { hover: '#303030' },
              }
            : {}),
        },
        components: {
          ...(mode === 'dark'
            ? {
                MuiPaper: { styleOverrides: { root: { backgroundImage: 'none' } } },
                MuiAlert: {
                  styleOverrides: {
                    standardInfo: {
                      backgroundColor: 'var(--theme-blue-surface)',
                      color: 'var(--theme-blue-text)',
                      '& .MuiAlert-icon': { color: 'inherit', opacity: 1 },
                    },
                  },
                },
                MuiButton: {
                  styleOverrides: {
                    root: ({ ownerState }) =>
                      ownerState.color === 'primary' && ownerState.variant !== 'contained'
                        ? {
                            color: 'var(--theme-blue-text)',
                            ...(ownerState.variant === 'outlined'
                              ? { borderColor: 'var(--theme-blue-border)' }
                              : {}),
                            '&:hover': { backgroundColor: 'var(--theme-blue-surface)' },
                          }
                        : {},
                  },
                },
                MuiMenuItem: {
                  styleOverrides: {
                    root: {
                      '&.Mui-selected': {
                        backgroundColor: 'var(--theme-blue-surface)',
                        color: 'var(--theme-blue-text)',
                        '&:hover, &.Mui-focusVisible': {
                          backgroundColor: 'var(--theme-blue-hover)',
                        },
                      },
                    },
                  },
                },
              }
            : {}),
        },
      }),
    [mode]
  )
  const value = useMemo(
    () => ({
      preference,
      setPreference: (value) => {
        const next = normalizeTheme(value)
        updatePreference(next)
        try {
          localStorage.setItem(THEME_STORAGE_KEY, next)
          return true
        } catch {
          return false
        }
      },
    }),
    [preference]
  )

  return (
    <ThemeContext.Provider value={value}>
      <ThemeProvider theme={theme}>{children}</ThemeProvider>
    </ThemeContext.Provider>
  )
}
