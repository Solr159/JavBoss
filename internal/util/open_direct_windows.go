//go:build windows

package util

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const shellExecuteShowNormal = 1

var procShellExecuteW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteW")

func openFileDirect(path string) (bool, error) {
	operation, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return true, err
	}
	target, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true, err
	}
	ret, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		shellExecuteShowNormal,
	)
	if ret <= 32 {
		if callErr != windows.ERROR_SUCCESS {
			return true, fmt.Errorf("ShellExecuteW: %w", callErr)
		}
		return true, fmt.Errorf("ShellExecuteW returned %d", ret)
	}
	return true, nil
}
