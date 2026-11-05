// Package license 实现产品授权：机器码绑定 + Ed25519 签名授权文件 + 试用期。
// 公钥随产品二进制分发；私钥只保存在发布方手中，经由环境变量传给 cmd/genlicense。
package license

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PublicKey 产品签名公钥（base64），随二进制分发，可公开。
// 私钥不进仓库：签发时由 cmd/genlicense 从环境变量 JXC_LICENSE_KEY 读取；
// 自行部署/二次开发可用 `go run ./cmd/genkey` 生成自己的密钥对并替换本值。
var PublicKey = "eHNKDvEgKSwmbgtSUDQRMdLHPmUVdI1b1H//kKh2eGI="

const (
	TrialDays   = 30    // 未授权时的试用期
	ProductCode = "JXC" // 产品码
)

// Info 授权文件内容（license.key）
type Info struct {
	Product     string `json:"product"`
	Owner       string `json:"owner"`
	ExpiresAt   string `json:"expires_at"` // YYYY-MM-DD
	MachineCode string `json:"machine_code"`
	Sig         string `json:"sig"` // 对 canonical 的 Ed25519 签名（base64）
}

// canonical 参与签名的统一字符串
func (i *Info) canonical() string {
	return fmt.Sprintf("%s|%s|%s|%s", i.Product, i.Owner, i.ExpiresAt, i.MachineCode)
}

func (i *Info) parseExp() (time.Time, error) {
	return time.ParseInLocation("2006-01-02", i.ExpiresAt, time.Local)
}

// MachineCode 机器指纹：主机名 + 全部网卡 MAC，SHA256 后格式化为 XXXX-XXXX-XXXX-XXXX
func MachineCode() string {
	host, _ := os.Hostname()
	var macs []string
	if ifaces, err := net.Interfaces(); err == nil {
		for _, it := range ifaces {
			if len(it.HardwareAddr) > 0 {
				macs = append(macs, it.HardwareAddr.String())
			}
		}
	}
	sort.Strings(macs)
	sum := sha256.Sum256([]byte(host + "|" + strings.Join(macs, ",")))
	hex := fmt.Sprintf("%x", sum[:8])
	var b strings.Builder
	for i := 0; i < len(hex); i += 4 {
		if i > 0 {
			b.WriteByte('-')
		}
		b.WriteString(strings.ToUpper(hex[i : i+4]))
	}
	return b.String()
}

// Verify 校验签名与机器码（不校验有效期）
func (i *Info) Verify() error {
	if i.Product != ProductCode {
		return errors.New("产品码不符")
	}
	pub, err := base64.StdEncoding.DecodeString(PublicKey)
	if err != nil {
		return errors.New("内置公钥损坏")
	}
	sig, err := base64.StdEncoding.DecodeString(i.Sig)
	if err != nil {
		return errors.New("签名格式错误")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), []byte(i.canonical()), sig) {
		return errors.New("签名无效")
	}
	if i.MachineCode != MachineCode() {
		return errors.New("机器码不匹配，本授权仅限其他设备")
	}
	return nil
}

// Status 运行时授权状态
type Status struct {
	Mode        string `json:"mode"` // licensed | trial | expired
	Owner       string `json:"owner,omitempty"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	TrialLeft   int    `json:"trial_left"` // 试用剩余天数（licensed 时为 0）
	MachineCode string `json:"machine_code"`
}

// Manager 加载并持有授权状态。服务启动时 Load，上传新授权后 Reload。
type Manager struct {
	filePath string
	nowFn    func() time.Time
	info     *Info // 有效授权（nil = 无）
	loaded   bool
}

func NewManager(filePath string) *Manager {
	return &Manager{filePath: filePath, nowFn: time.Now}
}

// Load 读取授权文件；文件不存在或校验失败均视为无授权（返回的 error 仅记录用）
func (m *Manager) Load() error {
	m.info = nil
	m.loaded = true
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return nil // 无授权文件，走试用
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("授权文件格式错误: %w", err)
	}
	if err := info.Verify(); err != nil {
		return err
	}
	exp, err := info.parseExp()
	if err != nil {
		return fmt.Errorf("授权到期日格式错误: %w", err)
	}
	if m.nowFn().After(exp.Add(24 * time.Hour)) {
		return errors.New("授权已过期: " + info.ExpiresAt)
	}
	m.info = &info
	return nil
}

// Reload 重新加载（上传新授权后调用）
func (m *Manager) Reload() error { return m.Load() }

// Status 计算当前状态。firstRun 为试用期起算时间（settings.first_run_at，首次启动时写入）。
func (m *Manager) Status(firstRun time.Time) Status {
	st := Status{MachineCode: MachineCode()}
	if m.info != nil {
		st.Mode = "licensed"
		st.Owner = m.info.Owner
		st.ExpiresAt = m.info.ExpiresAt
		return st
	}
	// 无授权 → 试用
	left := TrialDays - int(m.nowFn().Sub(firstRun).Hours()/24)
	if left < 0 {
		left = 0
	}
	st.TrialLeft = left
	if left <= 0 {
		st.Mode = "expired"
	} else {
		st.Mode = "trial"
	}
	return st
}

// Expired 写操作放行判断
func (m *Manager) Expired(firstRun time.Time) bool {
	return m.Status(firstRun).Mode == "expired"
}

// EnsureDir 确保授权文件所在目录存在
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(filepath.Dir(m.filePath), 0755)
}
