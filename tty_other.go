//go:build !linux && !darwin

// tty_other.go — 其他平台（windows/freebsd 等）：不做 ioctl 探测，一律视为非
// TTY。缺失必填凭据时报错提示显式传入，属文档化的降级（绝不把 /dev/null 或
// 重定向输入误判成交互终端）。
package main

import "io"

func isTTY(r io.Reader) bool { return false }
