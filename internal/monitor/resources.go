// Package monitor collects resource usage on demand, without a background worker.
package monitor

import (
	"context"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	minSampleInterval = 2 * time.Second
	maxSampleInterval = 15 * time.Second
)

type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type Disk struct {
	Path        string  `json:"path"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type Process struct {
	CPUPercent              *float64 `json:"cpu_percent"`
	RSSBytes                *uint64  `json:"rss_bytes"`
	ReadBytesPerSecond      *float64 `json:"read_bytes_per_second"`
	WriteBytesPerSecond     *float64 `json:"write_bytes_per_second"`
	DiskReadBytesPerSecond  *float64 `json:"disk_read_bytes_per_second"`
	DiskWriteBytesPerSecond *float64 `json:"disk_write_bytes_per_second"`
	Goroutines              int      `json:"goroutines"`
	UptimeSeconds           int64    `json:"uptime_seconds"`
}

type Snapshot struct {
	SampledAt        time.Time `json:"sampled_at"`
	SampleSeconds    float64   `json:"sample_seconds"`
	OS               string    `json:"os"`
	LogicalCPUs      int       `json:"logical_cpus"`
	Container        bool      `json:"container"`
	Process          Process   `json:"process"`
	SystemCPUPercent *float64  `json:"system_cpu_percent"`
	SystemMemory     *Memory   `json:"system_memory"`
	DataDisk         *Disk     `json:"data_disk"`
	Unavailable      []string  `json:"unavailable"`
}

type counters struct {
	processCPU *float64
	systemCPU  *cpu.TimesStat
	io         *process.IOCountersStat
	diskIO     *diskCounters
}

// These counters represent storage I/O, separately from logical read/write I/O.
// A nil pointer means unavailable; zero counters are valid on a supported OS.
type diskCounters struct {
	readBytes  uint64
	writeBytes uint64
}

type reading struct {
	snapshot Snapshot
	counters counters
}

// Sampler shares a short-lived snapshot across viewers. The previous counters
// are discarded for rates after a long gap, so reopening starts a fresh interval.
type Sampler struct {
	mu        sync.Mutex
	previous  reading
	path      string
	container bool
	startedAt time.Time
	now       func() time.Time
	read      func(context.Context, string) reading
}

func NewSampler() *Sampler {
	return &Sampler{startedAt: time.Now(), now: time.Now, read: collect}
}

func (s *Sampler) Sample(ctx context.Context, dataPath string, container bool) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	now := s.now()
	elapsed := now.Sub(s.previous.snapshot.SampledAt)
	sameScope := s.path == dataPath && s.container == container
	if sameScope && !s.previous.snapshot.SampledAt.IsZero() && elapsed >= 0 && elapsed < minSampleInterval {
		return s.previous.snapshot, nil
	}
	next := s.read(ctx, dataPath)
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	next.snapshot.SampledAt = now
	next.snapshot.OS = runtime.GOOS
	next.snapshot.LogicalCPUs = runtime.NumCPU()
	next.snapshot.Container = container
	next.snapshot.Process.Goroutines = runtime.NumGoroutine()
	next.snapshot.Process.UptimeSeconds = int64(now.Sub(s.startedAt).Seconds())
	if next.snapshot.Unavailable == nil {
		next.snapshot.Unavailable = []string{}
	}
	if sameScope && elapsed >= minSampleInterval && elapsed <= maxSampleInterval {
		next.snapshot.SampleSeconds = elapsed.Seconds()
		applyRates(&next, s.previous, elapsed.Seconds())
	}
	s.previous, s.path, s.container = next, dataPath, container
	return next.snapshot, nil
}

func collect(ctx context.Context, dataPath string) reading {
	var result reading
	missing := func(field string) { result.snapshot.Unavailable = append(result.snapshot.Unavailable, field) }
	if times, err := cpu.TimesWithContext(ctx, false); err == nil && len(times) > 0 {
		result.counters.systemCPU = &times[0]
	} else {
		missing("system_cpu")
	}
	if memory, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		result.snapshot.SystemMemory = &Memory{TotalBytes: memory.Total, UsedBytes: memory.Used, AvailableBytes: memory.Available, UsedPercent: memory.UsedPercent}
	} else {
		missing("system_memory")
	}
	if usage, err := disk.UsageWithContext(ctx, dataPath); err == nil {
		result.snapshot.DataDisk = &Disk{Path: dataPath, TotalBytes: usage.Total, UsedBytes: usage.Used, FreeBytes: usage.Free, UsedPercent: usage.UsedPercent}
	} else {
		missing("data_disk")
	}
	p, err := process.NewProcessWithContext(ctx, int32(os.Getpid()))
	if err != nil {
		missing("process_cpu")
		missing("process_memory")
		missing("process_io")
		missing("process_disk_io")
		return result
	}
	if times, err := p.TimesWithContext(ctx); err == nil {
		seconds := times.User + times.System
		result.counters.processCPU = &seconds
	} else {
		missing("process_cpu")
	}
	if memory, err := p.MemoryInfoWithContext(ctx); err == nil {
		result.snapshot.Process.RSSBytes = &memory.RSS
	} else {
		missing("process_memory")
	}
	if io, err := p.IOCountersWithContext(ctx); err == nil {
		result.counters.io = io
		// gopsutil only supplies DiskReadBytes/DiskWriteBytes on Linux.
		// Other platforms' logical I/O must not be presented as disk I/O.
		if runtime.GOOS == "linux" {
			result.counters.diskIO = &diskCounters{readBytes: io.DiskReadBytes, writeBytes: io.DiskWriteBytes}
		}
	} else {
		missing("process_io")
	}
	if result.counters.diskIO == nil {
		missing("process_disk_io")
	}
	return result
}

func applyRates(next *reading, previous reading, seconds float64) {
	if seconds <= 0 {
		return
	}
	a, b := previous.counters, next.counters
	if a.processCPU != nil && b.processCPU != nil && *b.processCPU >= *a.processCPU {
		value := (*b.processCPU - *a.processCPU) / seconds * 100
		next.snapshot.Process.CPUPercent = &value
	}
	if a.systemCPU != nil && b.systemCPU != nil {
		oldTotal, oldIdle := cpuTotals(*a.systemCPU)
		newTotal, newIdle := cpuTotals(*b.systemCPU)
		if newTotal > oldTotal && newIdle >= oldIdle {
			value := math.Max(0, math.Min(100, (1-(newIdle-oldIdle)/(newTotal-oldTotal))*100))
			next.snapshot.SystemCPUPercent = &value
		}
	}
	if a.io != nil && b.io != nil {
		if b.io.ReadBytes >= a.io.ReadBytes {
			value := float64(b.io.ReadBytes-a.io.ReadBytes) / seconds
			next.snapshot.Process.ReadBytesPerSecond = &value
		}
		if b.io.WriteBytes >= a.io.WriteBytes {
			value := float64(b.io.WriteBytes-a.io.WriteBytes) / seconds
			next.snapshot.Process.WriteBytesPerSecond = &value
		}
	}
	if a.diskIO != nil && b.diskIO != nil {
		if b.diskIO.readBytes >= a.diskIO.readBytes {
			value := float64(b.diskIO.readBytes-a.diskIO.readBytes) / seconds
			next.snapshot.Process.DiskReadBytesPerSecond = &value
		}
		if b.diskIO.writeBytes >= a.diskIO.writeBytes {
			value := float64(b.diskIO.writeBytes-a.diskIO.writeBytes) / seconds
			next.snapshot.Process.DiskWriteBytesPerSecond = &value
		}
	}
}

func cpuTotals(t cpu.TimesStat) (total, idle float64) {
	// Guest time is already included in user/nice on Linux.
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal, t.Idle + t.Iowait
}
