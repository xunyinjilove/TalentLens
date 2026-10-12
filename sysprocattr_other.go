//go:build !windows

package main

import "os/exec"

// hideConsoleWindow 在非 Windows 系统（Linux/macOS）上无需设置隐藏控制台标志
func hideConsoleWindow(cmd *exec.Cmd) {
	// POSIX 系统下无 CMD 窗口概念，无需处理
}
