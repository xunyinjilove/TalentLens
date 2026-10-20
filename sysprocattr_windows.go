//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// hideConsoleWindow 在 Windows 下彻底隐藏被拉起的子进程控制台（CMD黑框），实现静默无感后台运行
func hideConsoleWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
)

type winPoint struct {
	X int32
	Y int32
}

func startWindowNCDown(hitTest uintptr) {
	procReleaseCapture.Call()
	var pt winPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	lParam := uintptr((uint32(uint16(pt.Y)) << 16) | uint32(uint16(pt.X)))
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd != 0 {
		procSendMessageW.Call(hwnd, 0x00A1 /* WM_NCLBUTTONDOWN */, hitTest, lParam)
	}
}
