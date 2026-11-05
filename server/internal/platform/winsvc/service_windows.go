//go:build windows

package winsvc

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	// ServiceName 是注册到 SCM 的内部名（services.msc 里显示 DisplayName）
	ServiceName = "HUOBEN"
	DisplayName = "货本"
	Description = "货本 Web 服务（默认端口 8080）。数据保存在程序目录下的 data 子目录。"
)

// IsService 判断当前进程是否由 Windows 服务控制管理器启动。
func IsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

// Run 以服务方式运行；run 返回即服务退出，ctx 在收到停止/关机请求时取消。
func Run(run func(ctx context.Context) error) error {
	return svc.Run(ServiceName, &handler{run: run})
}

type handler struct {
	run func(ctx context.Context) error
}

// Execute 是 SCM 的调用入口：启动时把状态切到 Running，收到 Stop/Shutdown 时
// 取消 ctx 并等待业务优雅退出（关库、落盘），10 秒未退完则强制结束。
func (h *handler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case err := <-done:
			if err != nil {
				log.Printf("服务运行失败: %v", err)
				return true, 1
			}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case <-done:
				case <-time.After(10 * time.Second):
					log.Printf("服务停止超时（10 秒），强制退出")
				}
				return false, 0
			}
		}
	}
}

// Install 注册为开机自启的 Windows 服务（幂等：已存在则先停后删再建），并立即启动。
// 需要管理员权限。
func Install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}

	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("无法连接服务管理器，请以管理员身份运行：%w", err)
	}
	defer m.Disconnect()

	// 幂等重装：先停掉并删除同名旧服务，避免「服务已存在」失败
	if s, err := m.OpenService(ServiceName); err == nil {
		_, _ = s.Control(svc.Stop)
		time.Sleep(1500 * time.Millisecond)
		if err := s.Delete(); err != nil {
			_ = s.Close()
			return "", fmt.Errorf("删除旧服务失败：%w", err)
		}
		_ = s.Close()
		time.Sleep(1500 * time.Millisecond)
	}

	s, err := m.CreateService(ServiceName, exe, mgr.Config{
		DisplayName: DisplayName,
		Description: Description,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return "", fmt.Errorf("创建服务失败，请以管理员身份运行：%w", err)
	}
	defer s.Close()

	// 崩溃后自动重启：5s → 10s → 30s，一天无故障则失败计数清零
	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 5 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
	}, 86400)
	_ = s.SetRecoveryActionsOnNonCrashFailures(true)

	if err := s.Start(); err != nil {
		return "", fmt.Errorf("服务已注册但启动失败：%w", err)
	}

	return fmt.Sprintf("服务 %s（%s）已注册并启动\n程序：%s\n日志：%s\n数据：%s",
		ServiceName, DisplayName, exe,
		filepath.Join(filepath.Dir(exe), "data", "logs"),
		filepath.Join(filepath.Dir(exe), "data")), nil
}

// Uninstall 停止并删除服务；数据目录保留（避免误删账套）。
func Uninstall() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("无法连接服务管理器，请以管理员身份运行：%w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return "", fmt.Errorf("服务 %s 未安装", ServiceName)
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	time.Sleep(1500 * time.Millisecond)
	if err := s.Delete(); err != nil {
		return "", fmt.Errorf("删除服务失败（可能仍在停止中，稍后重试）：%w", err)
	}
	return "服务已卸载；data 目录（数据库与备份）仍保留，确认不需要可手动删除", nil
}

// Status 返回服务当前状态的可读描述。
func Status() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("无法连接服务管理器，请以管理员身份运行：%w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Sprintf("服务 %s 未安装（可用 huoben.exe -install 注册）", ServiceName), nil
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "服务：%s（%s）\n状态：%s\n", ServiceName, DisplayName, stateText(st.State))
	if cfg, err := s.Config(); err == nil {
		fmt.Fprintf(&b, "程序：%s\n启动类型：%s\n", cfg.BinaryPathName, startTypeText(cfg.StartType))
	}
	return b.String(), nil
}

func stateText(s svc.State) string {
	switch s {
	case svc.Running:
		return "运行中"
	case svc.Stopped:
		return "已停止"
	case svc.StartPending:
		return "正在启动"
	case svc.StopPending:
		return "正在停止"
	case svc.Paused:
		return "已暂停"
	default:
		return fmt.Sprintf("未知（%d）", s)
	}
}

func startTypeText(t uint32) string {
	switch t {
	case mgr.StartAutomatic:
		return "自动（开机启动）"
	case mgr.StartManual:
		return "手动"
	case mgr.StartDisabled:
		return "已禁用"
	default:
		return fmt.Sprintf("未知（%d）", t)
	}
}
