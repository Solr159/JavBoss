import useResources from '@/features/settings/hooks/useResources'
import { zh } from '@/utils/i18n'

function bytes(value) {
  if (value == null || !Number.isFinite(value)) return '—'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const index = Math.min(units.length - 1, Math.floor(Math.log2(Math.max(1, value)) / 10))
  return `${(value / 1024 ** index).toFixed(index ? 1 : 0)} ${units[index]}`
}

function percent(value) {
  return value == null ? '—' : `${value.toFixed(1)}%`
}

function uptime(seconds) {
  if (seconds == null) return '—'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${hours}${zh(' 小时', 'h')} ${minutes}${zh(' 分钟', 'm')}`
}

function MetricCard({ title, value, detail, usage }) {
  return (
    <div className="min-w-0 rounded-2xl border border-zinc-200 bg-white p-4">
      <h4 className="text-sm font-medium text-zinc-500">{title}</h4>
      <div className="mt-2 text-2xl font-semibold tabular-nums tracking-tight text-zinc-900">
        {value}
      </div>
      {usage != null && (
        <div
          role="meter"
          aria-label={title}
          aria-valuenow={Math.round(usage)}
          aria-valuemin={0}
          aria-valuemax={100}
          className="mt-3 h-1.5 overflow-hidden rounded-full bg-zinc-100"
        >
          <div
            className={`h-full rounded-full transition-[width] ${usage >= 90 ? 'bg-amber-500' : 'bg-blue-500'}`}
            style={{ width: `${Math.max(0, Math.min(100, usage))}%` }}
          />
        </div>
      )}
      <p className="mt-2 text-xs leading-5 text-zinc-500">{detail}</p>
    </div>
  )
}

function IOCard({ title, read, write, detail }) {
  const rate = (value) => (value == null ? '—' : `${bytes(value)}/s`)
  return (
    <MetricCard
      title={title}
      value={
        <div className="flex flex-wrap gap-x-5 gap-y-2 text-lg">
          <span>
            {rate(read)}{' '}
            <span className="text-xs font-normal text-zinc-400">{zh('读', 'read')}</span>
          </span>
          <span>
            {rate(write)}{' '}
            <span className="text-xs font-normal text-zinc-400">{zh('写', 'write')}</span>
          </span>
        </div>
      }
      detail={detail}
    />
  )
}

export default function ResourceDashboard() {
  const { snapshot, error, refreshing } = useResources()
  const process = snapshot?.process
  const memory = snapshot?.system_memory
  const disk = snapshot?.data_disk
  return (
    <div className="space-y-5">
      <h3 className="text-base font-semibold text-zinc-900">
        {zh('资源监控', 'Resource Monitor')}
      </h3>
      <div
        className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-zinc-500"
        role="status"
      >
        <span className="inline-flex items-center gap-1.5">
          <span
            className={`h-1.5 w-1.5 rounded-full ${error ? 'bg-amber-500' : refreshing ? 'bg-emerald-500' : 'bg-zinc-400'}`}
          />
          {error
            ? zh('更新失败', 'Update failed')
            : refreshing
              ? zh('自动刷新', 'Auto refresh')
              : zh('已暂停', 'Paused')}
        </span>
        {snapshot && (
          <>
            <span>
              {snapshot.os} · {snapshot.logical_cpus} {zh('逻辑核', 'logical CPUs')}
            </span>
            <span>
              {zh('运行时间', 'Uptime')} {uptime(process.uptime_seconds)}
            </span>
            <span>
              {zh('更新于', 'Updated')} {new Date(snapshot.sampled_at).toLocaleTimeString()}
            </span>
          </>
        )}
      </div>
      {error && (
        <div role="alert" className="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800">
          {error}
          {snapshot && (
            <span className="ml-2">
              {zh('当前显示上次成功采样。', 'Showing the last successful sample.')}
            </span>
          )}
        </div>
      )}
      {!snapshot && !error && (
        <p className="text-sm text-zinc-500">
          {zh('正在读取资源信息…', 'Loading resource metrics…')}
        </p>
      )}
      {snapshot && (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <MetricCard
              title={zh('JavBoss CPU', 'JavBoss CPU')}
              value={percent(process.cpu_percent)}
              detail={zh(
                '100% 表示占满 1 个逻辑核，可超过 100%。',
                '100% means one full logical core; may exceed 100%.'
              )}
            />
            <MetricCard
              title={zh('JavBoss 内存', 'JavBoss memory')}
              value={bytes(process.rss_bytes)}
              detail={`${zh('常驻内存 RSS', 'Resident memory (RSS)')} · ${process.goroutines} goroutines`}
            />
            <IOCard
              title={zh('JavBoss 磁盘 I/O', 'JavBoss disk I/O')}
              read={process.disk_read_bytes_per_second}
              write={process.disk_write_bytes_per_second}
              detail={
                snapshot.unavailable.includes('process_disk_io')
                  ? zh(
                      '当前平台或权限下无法读取磁盘 I/O',
                      'Disk I/O unavailable on this platform or with current permissions'
                    )
                  : zh(
                      '存储读写速率；缓存命中的读取不计入。',
                      'Storage I/O rate; reads served from cache are excluded.'
                    )
              }
            />
            <IOCard
              title={zh('JavBoss 进程 I/O', 'JavBoss process I/O')}
              read={process.read_bytes_per_second}
              write={process.write_bytes_per_second}
              detail={zh(
                '统计进程读写量，包含缓存访问，不限于磁盘。',
                'Process reads and writes, including cache access; not limited to disk.'
              )}
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            <MetricCard
              title={zh('系统 CPU', 'System CPU')}
              value={percent(snapshot.system_cpu_percent)}
              usage={snapshot.system_cpu_percent}
              detail={zh(
                '全部逻辑核的整体利用率。',
                'Overall utilization across all logical CPUs.'
              )}
            />
            <MetricCard
              title={zh('系统内存', 'System memory')}
              value={percent(memory?.used_percent)}
              usage={memory?.used_percent}
              detail={
                memory
                  ? `${bytes(memory.used_bytes)} / ${bytes(memory.total_bytes)} · ${zh('可用', 'available')} ${bytes(memory.available_bytes)}`
                  : zh('当前平台无法读取', 'Unavailable on this platform')
              }
            />
            <MetricCard
              title={zh('数据磁盘', 'Data disk')}
              value={percent(disk?.used_percent)}
              usage={disk?.used_percent}
              detail={
                disk
                  ? `${zh('可用', 'Free')} ${bytes(disk.free_bytes)} / ${bytes(disk.total_bytes)}`
                  : zh('当前无法读取磁盘信息', 'Disk information unavailable')
              }
            />
          </div>
          <div className="space-y-1 text-xs leading-5 text-zinc-500">
            <p>
              {zh(
                '系统指标包含所有程序；— 表示待采样或不可用。',
                'System metrics include all applications; — means pending or unavailable.'
              )}
            </p>
            {snapshot.container && (
              <p>
                {zh(
                  '容器内显示可见系统资源，不代表容器配额。',
                  'Container metrics reflect visible system resources, not container quotas.'
                )}
              </p>
            )}
          </div>
        </>
      )}
    </div>
  )
}
