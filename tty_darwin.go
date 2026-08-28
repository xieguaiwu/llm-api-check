//go:build darwin

// tty_darwin.go — Darwin 的 termios 请求号与 Linux 不同（TCGETS 未定义），
// 用 TIOCGETA；其余探测逻辑见 tty_unix.go。
package main

import "syscall"

const ioctlGetTermios = syscall.TIOCGETA
