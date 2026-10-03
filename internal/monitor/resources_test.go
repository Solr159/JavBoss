package monitor

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/process"
)

func TestSamplerRatesCacheAndResume(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	s := NewSampler()
	s.startedAt = now
	s.now = func() time.Time { return now }
	calls := 0
	s.read = func(context.Context, string) reading {
		calls++
		seconds := float64(calls * 6)
		return reading{counters: counters{
			processCPU: &seconds,
			systemCPU:  &cpu.TimesStat{User: float64(calls * 2), Idle: float64(calls * 6), Guest: 999},
			io:         &process.IOCountersStat{ReadBytes: uint64(calls * 300), WriteBytes: uint64(calls * 600)},
			diskIO:     &diskCounters{readBytes: uint64(calls * 30), writeBytes: uint64(calls * 90)},
		}}
	}
	first, err := s.Sample(context.Background(), "/data", false)
	if err != nil || first.Process.CPUPercent != nil || first.SystemCPUPercent != nil || first.Process.ReadBytesPerSecond != nil || first.Process.DiskReadBytesPerSecond != nil || first.Process.DiskWriteBytesPerSecond != nil {
		t.Fatalf("first sample must not invent rates: %+v, %v", first, err)
	}
	// Simultaneous viewers reuse the same reading.
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = s.Sample(context.Background(), "/data", false) }()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("read calls=%d, want 1", calls)
	}
	now = now.Add(3 * time.Second)
	next, err := s.Sample(context.Background(), "/data", false)
	if err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string]struct {
		got  *float64
		want float64
	}{
		"process CPU":     {next.Process.CPUPercent, 200},
		"system CPU":      {next.SystemCPUPercent, 25},
		"read rate":       {next.Process.ReadBytesPerSecond, 100},
		"write rate":      {next.Process.WriteBytesPerSecond, 200},
		"disk read rate":  {next.Process.DiskReadBytesPerSecond, 10},
		"disk write rate": {next.Process.DiskWriteBytesPerSecond, 30},
	} {
		if pair.got == nil || math.Abs(*pair.got-pair.want) > 0.001 {
			t.Errorf("%s: got %v, want %v", name, pair.got, pair.want)
		}
	}
	if next.SampleSeconds != 3 || next.Process.UptimeSeconds != 3 {
		t.Fatalf("wrong sample duration: %+v", next)
	}
	now = now.Add(time.Minute)
	resumed, _ := s.Sample(context.Background(), "/data", false)
	if resumed.Process.CPUPercent != nil || resumed.SampleSeconds != 0 || resumed.Process.DiskReadBytesPerSecond != nil || resumed.Process.DiskWriteBytesPerSecond != nil {
		t.Fatal("resume must reset rates")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Sample(ctx, "/data", false); err != context.Canceled {
		t.Fatalf("canceled request: %v", err)
	}
	if calls != 3 {
		t.Fatalf("canceled request collected metrics: %d", calls)
	}
	_, _ = s.Sample(context.Background(), "/other", true)
	if calls != 4 {
		t.Fatal("changed scope must invalidate cache")
	}
}

func TestDiskIORates(t *testing.T) {
	zero, readRate, writeRate := 0.0, 1024.0, 2048.0
	for _, tc := range []struct {
		name                string
		before, after       *diskCounters
		wantRead, wantWrite *float64
	}{
		{"storage traffic", &diskCounters{100, 200}, &diskCounters{3172, 6344}, &readRate, &writeRate},
		{"cached reads and idle disk", &diskCounters{100, 200}, &diskCounters{100, 200}, &zero, &zero},
		{"unsupported", nil, nil, nil, nil},
		{"failed collection", &diskCounters{100, 200}, nil, nil, nil},
		{"first supported sample", nil, &diskCounters{100, 200}, nil, nil},
		{"counter reset", &diskCounters{100, 200}, &diskCounters{50, 100}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := reading{counters: counters{diskIO: tc.before, io: &process.IOCountersStat{ReadBytes: 1000, WriteBytes: 2000}}}
			after := reading{counters: counters{diskIO: tc.after, io: &process.IOCountersStat{ReadBytes: 13000, WriteBytes: 26000}}}
			applyRates(&after, before, 3)
			for name, pair := range map[string]struct{ got, want *float64 }{
				"read":  {after.snapshot.Process.DiskReadBytesPerSecond, tc.wantRead},
				"write": {after.snapshot.Process.DiskWriteBytesPerSecond, tc.wantWrite},
			} {
				if (pair.got == nil) != (pair.want == nil) || (pair.got != nil && *pair.got != *pair.want) {
					t.Errorf("%s rate: got %v, want %v", name, pair.got, pair.want)
				}
			}
			if *after.snapshot.Process.ReadBytesPerSecond != 4000 || *after.snapshot.Process.WriteBytesPerSecond != 8000 {
				t.Fatal("disk counters must not replace logical I/O counters")
			}
		})
	}
}

func TestRatesUnavailableAndCounterReset(t *testing.T) {
	oldCPU, newCPU := 10.0, 5.0
	previous := reading{counters: counters{processCPU: &oldCPU, systemCPU: &cpu.TimesStat{Idle: 10}, io: &process.IOCountersStat{ReadBytes: 100, WriteBytes: 100}}}
	for _, tc := range []struct {
		name string
		next reading
	}{
		{"unavailable", reading{}},
		{"counter reset", reading{counters: counters{processCPU: &newCPU, systemCPU: &cpu.TimesStat{Idle: 5}, io: &process.IOCountersStat{ReadBytes: 50, WriteBytes: 50}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			applyRates(&tc.next, previous, 3)
			p := tc.next.snapshot.Process
			if p.CPUPercent != nil || p.ReadBytesPerSecond != nil || p.WriteBytesPerSecond != nil || tc.next.snapshot.SystemCPUPercent != nil {
				t.Fatal("invalid counters must not produce rates")
			}
		})
	}
}

func TestCollectLocalResources(t *testing.T) {
	snapshot, err := NewSampler().Sample(context.Background(), t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Process.Goroutines <= 0 || snapshot.LogicalCPUs <= 0 || snapshot.SampledAt.IsZero() {
		t.Fatalf("missing runtime metrics: %+v", snapshot)
	}
	if snapshot.DataDisk == nil || snapshot.DataDisk.TotalBytes == 0 {
		t.Fatalf("missing local disk metrics: %+v", snapshot)
	}
	if _, err := json.Marshal(snapshot); err != nil {
		t.Fatalf("metrics must be valid JSON: %v", err)
	}
}
