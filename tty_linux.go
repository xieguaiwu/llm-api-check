//go:build linux

// tty_linux.go — Linux 的 termios 请求号。
package main

import "syscall"

const ioctlGetTermios = syscall.TCGETS
