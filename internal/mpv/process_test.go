package mpv

import (
	"bufio"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A real child process lets these tests detect leaked players and verify Wait
// completes, without requiring a GUI or an installed MPV binary.
func TestPlayerProcessHelper(t *testing.T) {
	path := os.Getenv("JAVBOSS_PLAYER_TEST_PATH")
	if path == "" {
		return
	}
	if err := os.WriteFile(path+".ready", nil, 0600); err != nil {
		os.Exit(2)
	}
	for {
		if _, err := os.Stat(path + ".quit"); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func playerHelperCommand(t *testing.T, path string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestPlayerProcessHelper$")
	cmd.Env = append(os.Environ(), "JAVBOSS_PLAYER_TEST_PATH="+path)
	return cmd
}

func waitForPlayerTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func TestPlayerManagerShutdown(t *testing.T) {
	for _, responsive := range []bool{true, false} {
		name := "forced"
		if responsive {
			name = "graceful"
		}
		t.Run(name, func(t *testing.T) {
			m := &playerProcessManager{}
			originalDial := dialMPVIPCOverride
			var requests sync.WaitGroup
			attempts := make(chan struct{}, 3)
			dialMPVIPCOverride = func(path string, _ time.Duration) (io.ReadWriteCloser, error) {
				attempts <- struct{}{}
				if !responsive {
					return nil, os.ErrNotExist
				}
				client, server := net.Pipe()
				requests.Add(1)
				go func() {
					defer requests.Done()
					defer server.Close()
					line, err := bufio.NewReader(server).ReadString('\n')
					if err == nil && line == "{\"command\":[\"quit\"]}\n" {
						_ = os.WriteFile(path+".quit", nil, 0600)
					}
				}()
				return client, nil
			}
			t.Cleanup(func() {
				m.close(time.Second)
				dialMPVIPCOverride = originalDial
			})

			// A player started outside the manager must survive its shutdown.
			unrelated := playerHelperCommand(t, filepath.Join(t.TempDir(), "unrelated"))
			if err := unrelated.Start(); err != nil {
				t.Fatal(err)
			}
			unrelatedDone := make(chan struct{})
			go func() { _ = unrelated.Wait(); close(unrelatedDone) }()
			t.Cleanup(func() { _ = unrelated.Process.Kill(); <-unrelatedDone })

			var players []*playerProcess
			for _, mode := range []string{"reusable", "independent", "playlist"} {
				path := filepath.Join(t.TempDir(), mode)
				p, err := m.start(playerHelperCommand(t, path), path)
				if err != nil {
					t.Fatal(err)
				}
				players = append(players, p)
				waitForPlayerTestFile(t, path+".ready")
			}
			timeout := 50 * time.Millisecond
			if responsive {
				timeout = 3 * time.Second
			}
			// Concurrent shutdown calls must all wait for cleanup to finish.
			var closers sync.WaitGroup
			for range 2 {
				closers.Add(1)
				go func() { defer closers.Done(); m.close(timeout) }()
			}
			closers.Wait()
			for range players {
				select {
				case <-attempts:
				case <-time.After(time.Second):
					t.Fatal("shutdown did not attempt IPC quit")
				}
			}
			requests.Wait()
			for _, p := range players {
				if p.running() {
					t.Fatal("player survived shutdown")
				}
				if responsive {
					if _, err := os.Stat(p.ipcPath + ".quit"); err != nil {
						t.Fatalf("player did not receive quit: %v", err)
					}
				}
			}
			select {
			case <-unrelatedDone:
				t.Fatal("shutdown terminated an unrelated process")
			default:
			}
			if _, err := m.start(playerHelperCommand(t, filepath.Join(t.TempDir(), "late")), ""); err == nil {
				t.Fatal("shutdown allowed a new player to start")
			}
			m.mu.Lock()
			remaining := len(m.processes)
			m.mu.Unlock()
			if remaining != 0 {
				t.Fatalf("registry retained %d exited players", remaining)
			}
		})
	}
}

func TestPlayerManagerReapsNaturalExit(t *testing.T) {
	m := &playerProcessManager{}
	t.Cleanup(func() { m.close(time.Millisecond) })
	path := filepath.Join(t.TempDir(), "natural-exit")
	p, err := m.start(playerHelperCommand(t, path), "")
	if err != nil {
		t.Fatal(err)
	}
	waitForPlayerTestFile(t, path+".ready")
	if err := os.WriteFile(path+".quit", nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("naturally exited player was not reaped")
	}
	m.mu.Lock()
	remaining := len(m.processes)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatal("naturally exited player is still registered")
	}
}
