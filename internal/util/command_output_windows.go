package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// STARTF_FORCEOFFFEEDBACK belongs to STARTUPINFO.Flags, not CreationFlags.
// os/exec does not expose it through syscall.SysProcAttr.
const startfForceOffFeedback = 0x00000080

// BackgroundCombinedOutput runs a noninteractive tool without a console or
// Windows' startup busy cursor, including GUI tools such as MPV's image output.
// It inherits the current environment and directory and supplies EOF on stdin.
func BackgroundCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(cmd.Path)
	if err != nil {
		return nil, err
	}
	app, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	if err != nil {
		return nil, err
	}
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return nil, err
	}
	defer stdin.Close()
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	defer writer.Close()

	process, err := startBackgroundProcess(app, commandLine, stdin, writer)
	if err != nil {
		return nil, fmt.Errorf("start background tool %q: %w", name, err)
	}
	defer windows.CloseHandle(process.Process)
	windows.CloseHandle(process.Thread)
	// Drop the parent's pipe writer so reads reach EOF when the child exits.
	writer.Close()
	child, err := os.FindProcess(int(process.ProcessId))
	if err != nil {
		windows.TerminateProcess(process.Process, 1)
		windows.WaitForSingleObject(process.Process, windows.INFINITE)
		return nil, err
	}
	defer child.Release()

	finished := make(chan struct{})
	cancelDone := make(chan struct{})
	go func() {
		defer close(cancelDone)
		select {
		case <-ctx.Done():
			windows.TerminateProcess(process.Process, 1)
			// Also unblock reads if a descendant retained the output handle.
			reader.Close()
		case <-finished:
		}
	}()
	output, readErr := io.ReadAll(reader)
	state, waitErr := child.Wait()
	close(finished)
	<-cancelDone // The cancellation goroutine must stop before we close the handle.
	if err := ctx.Err(); err != nil {
		return output, err
	}
	if waitErr != nil {
		return output, waitErr
	}
	if !state.Success() {
		return output, &exec.ExitError{ProcessState: state}
	}
	return output, readErr
}

func startBackgroundProcess(app, commandLine *uint16, stdin, output *os.File) (*windows.ProcessInformation, error) {
	// Only pass these handles to the child. Screenshot workers run concurrently;
	// inheriting another worker's output pipe would prevent it from reaching EOF.
	handles := make([]windows.Handle, 2)
	current := windows.CurrentProcess()
	for i, file := range []*os.File{stdin, output} {
		if err := windows.DuplicateHandle(current, windows.Handle(file.Fd()), current, &handles[i], 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return nil, err
		}
		defer windows.CloseHandle(handles[i])
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attributes.Delete()
	if err := attributes.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST, unsafe.Pointer(&handles[0]), uintptr(len(handles))*unsafe.Sizeof(handles[0])); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Flags:      windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW | startfForceOffFeedback,
			ShowWindow: windows.SW_HIDE,
			StdInput:   handles[0],
			StdOutput:  handles[1],
			StdErr:     handles[1],
		},
		ProcThreadAttributeList: attributes.List(),
	}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var process windows.ProcessInformation
	err = windows.CreateProcess(app, commandLine, nil, nil, true,
		windows.CREATE_NO_WINDOW|windows.EXTENDED_STARTUPINFO_PRESENT,
		nil, nil, &startup.StartupInfo, &process)
	runtime.KeepAlive(handles)
	runtime.KeepAlive(stdin)
	runtime.KeepAlive(output)
	return &process, err
}
