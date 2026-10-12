//go:build windows

package main

import (
	"os/exec"
	"syscall"
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
