//go:build windows

package util

import (
	"fmt"
	"os"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	foDelete           = 0x0003
	fofAllowUndo       = 0x0040
	fofWantNukeWarning = 0x4000
)

var procSHFileOperationW = windows.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW")

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

func moveFileToTrash(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	from := utf16DoubleNull(path)
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofWantNukeWarning,
	}

	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("move file to recycle bin: SHFileOperationW returned %d", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("move file to recycle bin: operation aborted")
	}
	return nil
}

func utf16DoubleNull(s string) []uint16 {
	out := utf16.Encode([]rune(s))
	return append(out, 0, 0)
}
