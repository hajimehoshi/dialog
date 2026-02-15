// SPDX-License-Identifier: ISC
// SPDX-FileCopyrightText: 2026 Hajime Hoshi

package dialog

import "golang.org/x/sys/windows"

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
)

func _GetForegroundWindow() windows.HWND {
	r, _, _ := procGetForegroundWindow.Call()
	return windows.HWND(r)
}
