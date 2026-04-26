// SPDX-License-Identifier: ISC
// SPDX-FileCopyrightText: 2026 Hajime Hoshi

package dialog

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	_MB_OK              = 0x00000000
	_MB_YESNO           = 0x00000004
	_MB_ICONHAND        = 0x00000010
	_MB_ICONASTERISK    = 0x00000040
	_MB_ICONERROR       = _MB_ICONHAND
	_MB_ICONINFORMATION = _MB_ICONASTERISK

	_IDYES = 6

	_OFN_OVERWRITEPROMPT = 0x00000002
	_OFN_NOCHANGEDIR     = 0x00000008
	_OFN_FILEMUSTEXIST   = 0x00001000

	_BIF_RETURNONLYFSDIRS = 0x00000001
	_BIF_NEWDIALOGSTYLE   = 0x00000040

	_WM_USER = 0x0400

	_TRUE = 1
)

const (
	_BFFM_INITIALIZED     = 1
	_BFFM_SELCHANGED      = 2
	_BFFM_VALIDATEFAILEDA = 3
	_BFFM_VALIDATEFAILEDW = 4
	_BFFM_SETSTATUSTEXTA  = (_WM_USER + 100)
	_BFFM_SETSTATUSTEXTW  = (_WM_USER + 104)
	_BFFM_ENABLEOK        = (_WM_USER + 101)
	_BFFM_SETSELECTIONA   = (_WM_USER + 102)
	_BFFM_SETSELECTIONW   = (_WM_USER + 103)
	_BFFM_SETOKTEXT       = (_WM_USER + 105)
	_BFFM_SETEXPANDED     = (_WM_USER + 106)
	_BFFM_SETSTATUSTEXT   = _BFFM_SETSTATUSTEXTW
	_BFFM_SETSELECTION    = _BFFM_SETSELECTIONW
	_BFFM_VALIDATEFAILED  = _BFFM_VALIDATEFAILEDW
)

type _OPENFILENAME struct {
	StructSize      uint32
	Owner           windows.HWND
	Instance        windows.Handle
	Filter          *uint16
	CustomFilter    *uint16
	MaxCustomFilter uint32
	FilterIndex     uint32
	File            *uint16
	MaxFile         uint32
	FileTitle       *uint16
	MaxFileTitle    uint32
	InitialDir      *uint16
	Title           *uint16
	Flags           uint32
	FileOffset      uint16
	FileExtension   uint16
	DefExt          *uint16
	CustData        uintptr
	FnHook          uintptr
	TemplateName    *uint16
	PvReserved      unsafe.Pointer
	DwReserved      uint32
	FlagsEx         uint32
}

type _BROWSEINFO struct {
	Owner        windows.HWND
	Root         *uint16
	DisplayName  *uint16
	Title        *uint16
	Flags        uint32
	CallbackFunc uintptr
	LParam       uintptr
	Image        int32
}

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procSendMessageW        = user32.NewProc("SendMessageW")
)

var (
	comdlg32 = windows.NewLazySystemDLL("comdlg32.dll")

	procCommDlgExtendedError = comdlg32.NewProc("CommDlgExtendedError")
	procGetOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW     = comdlg32.NewProc("GetSaveFileNameW")
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
)

func _CommDlgExtendedError() uint32 {
	r, _, _ := procCommDlgExtendedError.Call()
	return uint32(r)
}

func _GetForegroundWindow() windows.HWND {
	r, _, _ := procGetForegroundWindow.Call()
	return windows.HWND(r)
}

func _GetOpenFileName(ofn *_OPENFILENAME) bool {
	r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(ofn)))
	return r != 0
}

func _GetSaveFileName(ofn *_OPENFILENAME) bool {
	r, _, _ := procGetSaveFileNameW.Call(uintptr(unsafe.Pointer(ofn)))
	return r != 0
}

func _MessageBox(hwnd windows.HWND, text, caption string, flags uint32) (int32, error) {
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return 0, err
	}
	captionPtr, err := windows.UTF16PtrFromString(caption)
	if err != nil {
		return 0, err
	}
	r, _, err := procMessageBoxW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(captionPtr)),
		uintptr(flags),
	)
	if r == 0 {
		if err != nil && !errors.Is(err, windows.ERROR_SUCCESS) {
			return 0, fmt.Errorf("dialog: MessageBoxW failed: %w", err)
		}
		return 0, fmt.Errorf("dialog: MessageBoxW failed: returned 0")
	}
	return int32(r), nil
}

func _SendMessage(hwnd windows.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := procSendMessageW.Call(
		uintptr(hwnd),
		uintptr(msg),
		wParam,
		lParam,
	)
	return r
}

func _SHBrowseForFolder(bi *_BROWSEINFO) uintptr {
	r, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(bi)))
	return r
}

func _SHGetPathFromIDList(idl uintptr) (string, error) {
	buf := make([]uint16, windows.MAX_PATH)
	r, _, err := procSHGetPathFromIDListW.Call(idl, uintptr(unsafe.Pointer(&buf[0])))
	if r == 0 {
		if err != nil && !errors.Is(err, windows.ERROR_SUCCESS) {
			return "", fmt.Errorf("dialog: SHGetPathFromIDListW failed: %w", err)
		}
		return "", fmt.Errorf("dialog: SHGetPathFromIDListW failed: returned 0")
	}
	return windows.UTF16ToString(buf), nil
}
