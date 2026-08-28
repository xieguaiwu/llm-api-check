//go:build linux || darwin

// tty_unix.go — Unix 平台 TTY 探测：ioctl termios 判定（ModeCharDevice 会把
// /dev/null 误判为终端，momus P1-2 回归）。ioctl 请求号按平台拆到
// tty_linux.go（TCGETS）/ tty_darwin.go（TIOCGETA）。
package main

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

// isTTY stdin 是否为真实终端。
// 缓冲区取 128 字节（大于 linux/darwin 的 struct termios，防止内核越界写）。
func isTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	var termios [128]byte
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), ioctlGetTermios, uintptr(unsafe.Pointer(&termios[0])))
	return errno == 0
}
