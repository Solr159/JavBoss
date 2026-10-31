import { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  LinearProgress,
  MenuItem,
  TextField,
} from '@mui/material'
import ArrowForwardRoundedIcon from '@mui/icons-material/ArrowForwardRounded'
import SystemUpdateAltRoundedIcon from '@mui/icons-material/SystemUpdateAltRounded'
import { fetchAppUpdate, isAppUpdateTargetReady, requestAppUpdate } from '@/features/settings/api'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'
import ReleaseNotes from '@/features/settings/components/ReleaseNotes'

export default function AppUpdateSettings({ clientMode = false }) {
  const [target, setTarget] = useState(clientMode ? 'local' : 'server')
  return (
    <div className="space-y-5">
      <h3 className="text-base font-semibold text-zinc-900">
        {zh('检查更新', 'Check for Updates')}
      </h3>
      {clientMode && (
        <TextField
          select
          fullWidth
          size="small"
          label={zh('更新目标', 'Update target')}
          value={target}
          onChange={(event) => setTarget(event.target.value)}
        >
          <MenuItem value="local">{zh('本机客户端', 'Local client')}</MenuItem>
          <MenuItem value="server">{zh('远程服务器', 'Remote server')}</MenuItem>
        </TextField>
      )}
      <UpdateTarget
        key={target}
        local={target === 'local'}
        remote={clientMode && target === 'server'}
      />
    </div>
  )
}

function UpdateTarget({ local, remote }) {
  const [status, setStatus] = useState(null)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [expectedVersion, setExpectedVersion] = useState('')
  const [reconnecting, setReconnecting] = useState(false)

  // The server owns downloads and restart. Polling only observes that process
  // and reconnects across the expected service interruption; closing this panel
  // does not cancel an update already confirmed by the user.
  useEffect(() => {
    const controller = new AbortController()
    let timer
    const refresh = async () => {
      try {
        const next = await fetchAppUpdate({ local, signal: controller.signal })
        if (controller.signal.aborted) return
        setStatus(next)
        setError('')
        setReconnecting(false)
        if (expectedVersion && next.current_version === expectedVersion) {
          window.location.reload()
          return
        }
        if (expectedVersion && next.phase === 'error') setExpectedVersion('')
      } catch (err) {
        if (controller.signal.aborted) return
        if (expectedVersion) {
          setReconnecting(true)
          // Do not depend on the old page successfully reading the new update
          // API forever. Once the target is reachable, load its current UI and
          // let the normal login flow handle any expired session.
          const ready = await isAppUpdateTargetReady({ local, signal: controller.signal }).catch(
            () => false
          )
          if (controller.signal.aborted) return
          if (ready) {
            window.location.reload()
            return
          }
        } else setError(getErrorMessage(err))
      }
      timer = window.setTimeout(refresh, 2000)
    }
    refresh()
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [local, expectedVersion])

  const action = async (name) => {
    setSubmitting(true)
    setError('')
    try {
      const next = await requestAppUpdate(name, { local })
      setStatus(next)
      if (name === 'install') {
        setExpectedVersion(next.latest_version)
        setConfirming(false)
      }
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  const busy = submitting || (status && !['idle', 'error'].includes(status.phase))
  const progress = status?.total_bytes
    ? Math.min(100, Math.round((status.downloaded_bytes / status.total_bytes) * 100))
    : 0
  const phaseText = {
    checking: zh('正在检查更新…', 'Checking for updates…'),
    downloading: status?.total_bytes
      ? zh(`正在下载更新 ${progress}%`, `Downloading update ${progress}%`)
      : zh('正在下载更新…', 'Downloading update…'),
    preparing: zh('正在准备更新…', 'Preparing update…'),
    restarting: zh('正在关闭并重新启动 JavBoss…', 'Closing and restarting JavBoss…'),
  }[status?.phase]
  const disabledText = {
    development: zh(
      '开发版不支持自动安装更新。',
      'Development builds do not support update installation.'
    ),
    container: zh(
      'Docker 版本请通过拉取新镜像并重建容器更新。',
      'Update Docker by pulling the new image and recreating the container.'
    ),
    platform: zh(
      '当前平台暂无自动更新安装包。',
      'No automatic update package is available for this platform.'
    ),
  }[status?.disabled_reason]

  return (
    <div className="space-y-4">
      <div className="space-y-2 text-sm text-zinc-700">
        <p>
          {zh('当前版本', 'Current version')}：{status?.current_version || '—'}
        </p>
        <p>
          {zh('最新版本', 'Latest version')}：{status?.latest_version || '—'}
        </p>
        {status?.checked_at > 0 && (
          <p>
            {zh('上次检查', 'Last checked')}：{new Date(status.checked_at).toLocaleString()}
          </p>
        )}
      </div>
      {disabledText && <Alert severity="info">{disabledText}</Alert>}
      {(error || status?.error) && <Alert severity="error">{error || status.error}</Alert>}
      {status?.last_result?.error && (
        <Alert severity="warning">
          {zh('上次更新未完成：', 'The previous update did not complete: ')}
          {status.last_result.error}
        </Alert>
      )}
      {(phaseText || reconnecting) && (
        <div className="space-y-2" role="status">
          <p className="text-sm text-zinc-600">
            {reconnecting
              ? zh('JavBoss 暂时断开，正在重新连接…', 'JavBoss is disconnected. Reconnecting…')
              : phaseText}
          </p>
          <LinearProgress
            variant={
              status?.phase === 'downloading' && status?.total_bytes > 0
                ? 'determinate'
                : 'indeterminate'
            }
            value={progress}
          />
        </div>
      )}
      {!busy && status?.checked_at > 0 && !status.available && !status.error && (
        <p className="text-sm text-zinc-500">
          {zh('没有可用的新版本。', 'No newer version is available.')}
        </p>
      )}
      <div className="flex flex-wrap gap-3">
        {reconnecting && (
          <Button variant="outlined" onClick={() => window.location.reload()}>
            {zh('刷新页面', 'Refresh page')}
          </Button>
        )}
        <Button variant="outlined" disabled={Boolean(busy)} onClick={() => action('check')}>
          {zh('检查更新', 'Check for updates')}
        </Button>
        {status?.available && (
          <Button
            variant="contained"
            disabled={Boolean(busy) || !status.can_install}
            onClick={() => setConfirming(true)}
          >
            {zh('更新并重启', 'Update and restart')}
          </Button>
        )}
        {status?.release_url && (
          <Button component="a" href={status.release_url} target="_blank" rel="noreferrer">
            {zh('查看发布页面', 'View release')}
          </Button>
        )}
      </div>
      {(status?.notes_html || status?.notes) && (
        <div className="rounded-xl border border-zinc-200 bg-white p-4">
          {status.latest_version && (
            <h4 className="mb-2 text-sm font-medium">{status.latest_version}</h4>
          )}
          <ReleaseNotes html={status.notes_html} text={status.notes} />
        </div>
      )}
      <Dialog
        open={confirming}
        onClose={() => !submitting && setConfirming(false)}
        fullWidth
        maxWidth="xs"
        aria-labelledby="app-update-confirm-title"
        aria-describedby="app-update-confirm-description"
        slotProps={{
          paper: {
            sx: { borderRadius: '20px', m: 2, width: 'calc(100% - 32px)' },
          },
        }}
      >
        <DialogTitle id="app-update-confirm-title" sx={{ px: 3, pt: 3, pb: 2.5 }}>
          <span className="flex items-center gap-3">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-50 text-blue-600">
              <SystemUpdateAltRoundedIcon />
            </span>
            <span className="text-lg font-semibold text-zinc-900">
              {zh('确认更新', 'Confirm update')}
            </span>
          </span>
        </DialogTitle>
        <DialogContent sx={{ px: 3, pb: 2.5 }}>
          <div className="mb-4 flex items-center gap-3 rounded-xl border border-zinc-200/70 bg-zinc-50 px-4 py-3">
            <div className="min-w-0 flex-1">
              <p className="text-xs text-zinc-500">{zh('当前版本', 'Current version')}</p>
              <p className="mt-1 break-all text-sm font-medium text-zinc-600">
                {status?.current_version || '—'}
              </p>
            </div>
            <ArrowForwardRoundedIcon className="shrink-0 text-zinc-400" fontSize="small" />
            <div className="min-w-0 flex-1 text-right">
              <p className="text-xs text-zinc-500">{zh('更新至', 'Update to')}</p>
              <p className="mt-1 break-all text-sm font-semibold text-blue-600">
                {status?.latest_version || '—'}
              </p>
            </div>
          </div>
          <Alert severity="warning" sx={{ borderRadius: '12px', alignItems: 'flex-start' }}>
            <DialogContentText
              id="app-update-confirm-description"
              sx={{ color: 'inherit', fontSize: 13, lineHeight: 1.7 }}
            >
              {remote
                ? zh(
                    '更新将关闭并重新启动远程服务器上的 JavBoss，正在进行的操作可能中断。是否继续？',
                    'This update will close and restart JavBoss on the remote server. Operations in progress may be interrupted. Continue?'
                  )
                : zh(
                    '更新将关闭并重新启动 JavBoss，正在进行的操作可能中断。是否继续？',
                    'This update will close and restart JavBoss. Operations in progress may be interrupted. Continue?'
                  )}
            </DialogContentText>
          </Alert>
          <DialogContentText sx={{ mt: 2, mb: 0, fontSize: 12, lineHeight: 1.7, color: '#71717a' }}>
            {zh(
              '更新包下载并校验成功后才会关闭程序。',
              'JavBoss will close only after the update has been downloaded and verified.'
            )}
          </DialogContentText>
        </DialogContent>
        <DialogActions
          sx={{
            px: 3,
            py: 2,
            gap: 1,
            borderTop: '1px solid #f4f4f5',
            '& > :not(style) ~ :not(style)': { ml: 0 },
            '& .MuiButton-root': {
              borderRadius: '10px',
              px: { xs: 1, sm: 2.5 },
              py: 1,
              textTransform: 'none',
              whiteSpace: 'nowrap',
              flex: { xs: 1, sm: 'initial' },
            },
          }}
        >
          <Button
            variant="outlined"
            disabled={submitting}
            onClick={() => setConfirming(false)}
            sx={{ color: '#52525b', borderColor: '#d4d4d8' }}
          >
            {zh('取消', 'Cancel')}
          </Button>
          <Button
            variant="contained"
            disableElevation
            disabled={submitting}
            onClick={() => action('install')}
          >
            {zh('继续更新', 'Continue update')}
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  )
}
