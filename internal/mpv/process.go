package mpv

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"javboss/internal/common/logging"
)

const playerShutdownTimeout = 2 * time.Second

var playerProcesses = &playerProcessManager{}

type playerProcess struct {
	cmd     *exec.Cmd
	ipcPath string
	done    chan struct{}
}

type playerProcessManager struct {
	mu        sync.Mutex
	processes map[*playerProcess]struct{}
	closing   bool
	shutdown  sync.Once
}

// Shutdown closes every player launched by this instance, including independent
// windows and playlists. No new players can start once shutdown begins.
func Shutdown() {
	playerProcesses.close(playerShutdownTimeout)
}

func (m *playerProcessManager) start(cmd *exec.Cmd, ipcPath string) (*playerProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing {
		return nil, errors.New("player manager is shutting down")
	}
	wait, release, err := startPlatformPlayer(cmd)
	if err != nil {
		return nil, err
	}
	p := &playerProcess{cmd: cmd, ipcPath: ipcPath, done: make(chan struct{})}
	if m.processes == nil {
		m.processes = make(map[*playerProcess]struct{})
	}
	m.processes[p] = struct{}{}
	go func() {
		if err := wait(); err != nil {
			logging.Info("player process exited: %v", err)
		}
		release()
		if runtime.GOOS != "windows" && ipcPath != "" {
			_ = os.Remove(ipcPath)
		}
		m.mu.Lock()
		delete(m.processes, p)
		close(p.done)
		m.mu.Unlock()
	}()
	return p, nil
}

func (m *playerProcessManager) close(timeout time.Duration) {
	m.shutdown.Do(func() {
		m.mu.Lock()
		m.closing = true
		processes := make([]*playerProcess, 0, len(m.processes))
		for p := range m.processes {
			processes = append(processes, p)
		}
		m.mu.Unlock()
		var wg sync.WaitGroup
		for _, p := range processes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.close(timeout)
			}()
		}
		wg.Wait()
	})
}

func (p *playerProcess) running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *playerProcess) close(timeout time.Duration) {
	if !p.running() {
		return
	}
	// quit honors save-position-on-quit. Do not wait for an IPC response: mpv may
	// exit before replying, and an unresponsive player must not block shutdown.
	go func() {
		if p.ipcPath == "" {
			return
		}
		conn, err := dialMPVIPC(p.ipcPath, timeout)
		if err != nil {
			return
		}
		defer conn.Close()
		if c, ok := conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
			_ = c.SetWriteDeadline(time.Now().Add(timeout))
		}
		_, _ = io.WriteString(conn, "{\"command\":[\"quit\"]}\n")
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return
	case <-timer.C:
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}
