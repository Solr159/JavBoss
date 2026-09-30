//go:build windows

package mpv

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PROC_THREAD_ATTRIBUTE_JOB_LIST (Windows 10+) assigns the process to the job
// during creation. Assigning it after cmd.Start leaves a race with parent exit.
const procThreadAttributeJobList = 0x0002000d

func startPlatformPlayer(cmd *exec.Cmd) (wait func() error, release func(), err error) {
	if cmd.Err != nil {
		return nil, nil, cmd.Err
	}
	// This launcher is deliberately limited to the standalone player commands
	// built in this package; it does not implement exec.Cmd's pipe machinery.
	if cmd.Process != nil || cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil || cmd.SysProcAttr != nil {
		return nil, nil, errors.New("unsupported player command attributes")
	}
	app, err := windows.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return nil, nil, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(cmd.Args))
	if err != nil {
		return nil, nil, err
	}
	var dir *uint16
	if cmd.Dir != "" {
		dir, err = windows.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			return nil, nil, err
		}
	}
	env := cmd.Environ()
	for _, value := range env {
		if strings.ContainsRune(value, 0) {
			return nil, nil, errors.New("invalid player environment")
		}
	}
	sort.Slice(env, func(i, j int) bool { return strings.ToUpper(env[i]) < strings.ToUpper(env[j]) })
	envBlock := utf16.Encode([]rune(strings.Join(env, "\x00") + "\x00\x00"))

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create player job: %w", err)
	}
	defer func() {
		if err != nil {
			_ = windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, nil, fmt.Errorf("configure player job: %w", err)
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, nil, err
	}
	defer attributes.Delete()
	if err = attributes.Update(procThreadAttributeJobList, unsafe.Pointer(&job), unsafe.Sizeof(job)); err != nil {
		return nil, nil, fmt.Errorf("assign player job at creation: %w", err)
	}
	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	var info windows.ProcessInformation
	// Neither the job nor any other parent handle is inherited. Only this mpv
	// and its descendants belong to the job; browsers and other players do not.
	if err = windows.CreateProcess(app, commandLine, nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW,
		&envBlock[0], dir, &startup.StartupInfo, &info); err != nil {
		return nil, nil, fmt.Errorf("create player process: %w", err)
	}
	defer windows.CloseHandle(info.Thread)
	defer windows.CloseHandle(info.Process)
	cmd.Process, err = os.FindProcess(int(info.ProcessId))
	if err != nil {
		return nil, nil, err
	}
	return func() error {
		state, err := cmd.Process.Wait()
		if err != nil {
			return err
		}
		if !state.Success() {
			return &exec.ExitError{ProcessState: state}
		}
		return nil
	}, func() { _ = windows.CloseHandle(job) }, nil
}
