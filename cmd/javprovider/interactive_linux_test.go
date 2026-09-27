//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"javboss/internal/jav"
)

func openTestTerminal(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(fd), "ptmx")
	t.Cleanup(func() { master.Close() })
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	if err := unix.IoctlSetWinsize(fd, unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80}); err != nil {
		t.Fatal(err)
	}
	return master, slave
}

func waitForTerminalText(t *testing.T, master *os.File, want ...string) string {
	t.Helper()
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var output strings.Builder
		buf := make([]byte, 1024)
		for {
			n, err := master.Read(buf)
			output.Write(buf[:n])
			remaining := output.String()
			matched := true
			for _, text := range want {
				_, suffix, ok := strings.Cut(remaining, text)
				if !ok {
					matched = false
					break
				}
				remaining = suffix
			}
			if matched {
				done <- result{text: output.String()}
				return
			}
			if err != nil {
				done <- result{err: err}
				return
			}
		}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		return got.text
	case <-time.After(3 * time.Second):
		t.Fatal("terminal prompt did not appear")
	}
	return ""
}

// Run the real form in a child process, as the CLI does. This also exercises
// process exit codes while the parent verifies restoration of the shared TTY.
func TestInteractiveProcess(t *testing.T) {
	if os.Getenv("JAVPROVIDER_TEST_PROCESS") != "1" {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmd := newCommand()
	calls := 0
	for i := range cmd.methods {
		method := cmd.methods[i].name
		cmd.methods[i].call = func(_ context.Context, provider jav.Provider, input string) (any, error) {
			calls++
			if method != os.Getenv("JAVPROVIDER_TEST_METHOD") || provider.String() != os.Getenv("JAVPROVIDER_TEST_PROVIDER") || input != os.Getenv("JAVPROVIDER_TEST_INPUT") {
				return nil, fmt.Errorf("unexpected lookup %s/%s/%q", method, provider, input)
			}
			return nil, nil
		}
	}
	err := cmd.run(ctx, nil, os.Stdin, os.Stdout, os.Stderr)
	if errors.Is(err, context.Canceled) {
		os.Exit(130)
	}
	if err != nil || calls != 1 {
		fmt.Fprintf(os.Stderr, "error=%v calls=%d\n", err, calls)
		os.Exit(1)
	}
	os.Exit(0)
}

func startInteractiveProcess(t *testing.T, slave *os.File, method, provider, input string) (*exec.Cmd, <-chan error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(executable, "-test.run=^TestInteractiveProcess$")
	child.Env = append(os.Environ(), "JAVPROVIDER_TEST_PROCESS=1", "JAVPROVIDER_TEST_METHOD="+method, "JAVPROVIDER_TEST_PROVIDER="+provider, "JAVPROVIDER_TEST_INPUT="+input, "GORACE=atexit_sleep_ms=0")
	child.Stdin, child.Stdout, child.Stderr = slave, slave, slave
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	t.Cleanup(func() { _ = child.Process.Kill() })
	return child, done
}

func finishInteractiveProcess(t *testing.T, child *exec.Cmd, done <-chan error, slave *os.File, before *term.State, wantExit int) {
	t.Helper()
	select {
	case <-done:
		if got := child.ProcessState.ExitCode(); got != wantExit {
			t.Fatalf("exit code=%d, want %d", got, wantExit)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("form process did not exit")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("terminal was not restored: %v", err)
	}
}

func TestInteractiveFormSelectsMethodThenProviderAndEditsInput(t *testing.T) {
	for _, tc := range []struct{ name, methodKeys, method, providerKeys, provider, keys, want string }{
		{"actress by code", "\r", "LookupActressByCode", "\r", "javdatabase", "AC\x1b[DB\x1b[CD\r", "ABCD"},
		{"actress by name", "\x1b[B\r", "LookupActressByJapaneseName", "\x1b[B\r", "minnanoav", "女优\x1b[D名\r", "女名优"},
		{"movie", "\x1b[B\x1b[B\r", "LookupJavByCode", "\r", "javbus", "ABC-001\r", "ABC-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			master, slave := openTestTerminal(t)
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			child, done := startInteractiveProcess(t, slave, tc.method, tc.provider, tc.want)
			first := waitForTerminalText(t, master, "请选择 method")
			if strings.Contains(first, "请选择 provider") {
				t.Fatal("provider shown before method selection")
			}
			if _, err := master.WriteString(tc.methodKeys); err != nil {
				t.Fatal(err)
			}
			second := waitForTerminalText(t, master, "请选择 provider", tc.provider)
			if tc.method != "LookupJavByCode" && strings.Contains(second, "javbus") {
				t.Fatalf("unsupported provider displayed: %q", second)
			}
			if _, err := master.WriteString(tc.providerKeys); err != nil {
				t.Fatal(err)
			}
			waitForTerminalText(t, master, "请输入")
			if _, err := master.WriteString(tc.keys); err != nil {
				t.Fatal(err)
			}
			finishInteractiveProcess(t, child, done, slave, before, 0)
		})
	}
}

func TestInteractiveProviderSelectionKeepsRowsInPlace(t *testing.T) {
	master, slave := openTestTerminal(t)
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	child, done := startInteractiveProcess(t, slave, "LookupJavByCode", "javbus", "ABC-001")
	waitForTerminalText(t, master, "请选择 method")
	master.WriteString("\x1b[B\x1b[B\r")
	waitForTerminalText(t, master, "请选择 provider", "> ", "javbus", "javdatabase")
	master.WriteString("\x1b[B")
	// Both rows must remain visible and keep their order as the marker moves.
	waitForTerminalText(t, master, "javbus", "> ", "javdatabase")
	master.WriteString("\x1b[A")
	waitForTerminalText(t, master, "> ", "javbus", "javdatabase")
	master.WriteString("\r")
	waitForTerminalText(t, master, "请输入番号")
	master.WriteString("ABC-001\r")
	finishInteractiveProcess(t, child, done, slave, before, 0)
}

func TestInteractiveFormRefreshesProvidersAfterChangingMethod(t *testing.T) {
	master, slave := openTestTerminal(t)
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	child, done := startInteractiveProcess(t, slave, "LookupActressByJapaneseName", "javmodel", "女优")
	waitForTerminalText(t, master, "请选择 method")
	master.WriteString("\r")
	waitForTerminalText(t, master, "请选择 provider", "javdatabase")
	master.WriteString("\x1b[Z") // Shift+Tab goes back to the method.
	waitForTerminalText(t, master, "请选择 method")
	master.WriteString("\x1b[B\r")
	waitForTerminalText(t, master, "请选择 provider", "javmodel", "minnanoav")
	master.WriteString("\r")
	waitForTerminalText(t, master, "请输入女优日文名")
	master.WriteString("女优\r")
	finishInteractiveProcess(t, child, done, slave, before, 0)
}

func TestInteractiveFormCancellationRestoresTerminal(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		t.Run(strconv.Itoa(stage), func(t *testing.T) {
			master, slave := openTestTerminal(t)
			before, err := term.GetState(int(slave.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			child, done := startInteractiveProcess(t, slave, "", "", "")
			waitForTerminalText(t, master, "请选择 method")
			if stage == 1 || stage == 2 {
				master.WriteString("\r")
				waitForTerminalText(t, master, "请选择 provider", "javdatabase")
			}
			if stage == 2 {
				master.WriteString("\r")
				waitForTerminalText(t, master, "请输入番号")
			}
			if stage == 3 {
				child.Process.Signal(os.Interrupt)
			} else {
				master.WriteString("\x03")
			}
			finishInteractiveProcess(t, child, done, slave, before, 130)
		})
	}
}
