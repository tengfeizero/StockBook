//go:build !windows

// Package winsvc 把程序注册为 Windows 服务；非 Windows 平台只提供占位实现，
// 保证同一份 main.go 能跨平台编译（Linux 用 deploy/huoben.service 交给 systemd）。
package winsvc

import (
	"context"
	"errors"
)

const (
	ServiceName = "HUOBEN"
	DisplayName = "货本"
	Description = "货本 Web 服务"
)

var errUnsupported = errors.New("Windows 服务模式仅支持 Windows；Linux 请用 deploy/huoben.service")

// IsService 非 Windows 平台恒为 false。
func IsService() bool { return false }

// Run 非 Windows 平台不可用。
func Run(run func(ctx context.Context) error) error { return errUnsupported }

// Install 非 Windows 平台不可用。
func Install() (string, error) { return "", errUnsupported }

// Uninstall 非 Windows 平台不可用。
func Uninstall() (string, error) { return "", errUnsupported }

// Status 非 Windows 平台不可用。
func Status() (string, error) { return "", errUnsupported }
