// genlicense 签发产品授权文件（仅发布方使用，不随产品分发）。
//
// 用法：
//
//	export HUOBEN_LICENSE_KEY='<你的私钥 base64>'      # 或 -key-file 指定文件
//	go run ./cmd/genlicense -owner "某某商贸" -days 365 -code XXXX-XXXX-XXXX-XXXX -out license.key
//
// 机器码向客户索取：系统设置页「授权信息」卡片中展示，或让其运行 huoben.exe -machine-code 打印。
//
// 私钥不写入代码仓库：请用 cmd/genkey 生成密钥对，公钥填进 internal/pkg/license，
// 私钥只保存在发布方机器上（环境变量或权限 600 的文件）。
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"jxc/server/internal/pkg/license"
	"os"
	"strings"
	"time"
)

type info struct {
	Product     string `json:"product"`
	Owner       string `json:"owner"`
	ExpiresAt   string `json:"expires_at"`
	MachineCode string `json:"machine_code"`
	Sig         string `json:"sig"`
}

// loadPrivateKey 依次从 -key-file、环境变量 HUOBEN_LICENSE_KEY 读取私钥。
func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	var raw string
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取私钥文件失败: %w", err)
		}
		raw = strings.TrimSpace(string(b))
	} else {
		raw = strings.TrimSpace(os.Getenv("HUOBEN_LICENSE_KEY"))
	}
	if raw == "" {
		return nil, fmt.Errorf("未提供私钥：请设置环境变量 HUOBEN_LICENSE_KEY，或用 -key-file 指定（密钥对可用 go run ./cmd/genkey 生成）")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("私钥不是合法的 base64: %w", err)
	}
	if len(key) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("私钥长度 %d 不正确（应为 %d）", len(key), ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(key), nil
}

func main() {
	owner := flag.String("owner", "", "客户/商户名称（必填）")
	days := flag.Int("days", 365, "授权有效天数")
	code := flag.String("code", "", "客户机器码（必填，见系统设置页或 huoben.exe -machine-code）")
	out := flag.String("out", "license.key", "输出文件路径")
	keyFile := flag.String("key-file", "", "私钥文件（base64 一行）；留空则读环境变量 HUOBEN_LICENSE_KEY")
	flag.Parse()

	if *owner == "" || *code == "" {
		fmt.Println("缺少必填参数：-owner 与 -code")
		flag.Usage()
		os.Exit(1)
	}

	priv, err := loadPrivateKey(*keyFile)
	if err != nil {
		fmt.Println("签发失败:", err)
		os.Exit(1)
	}

	exp := time.Now().AddDate(0, 0, *days).Format("2006-01-02")
	inf := info{Product: license.ProductCode, Owner: *owner, ExpiresAt: exp, MachineCode: *code}
	canonical := fmt.Sprintf("%s|%s|%s|%s", inf.Product, inf.Owner, inf.ExpiresAt, inf.MachineCode)
	sig := ed25519.Sign(priv, []byte(canonical))
	inf.Sig = base64.StdEncoding.EncodeToString(sig)

	data, _ := json.MarshalIndent(inf, "", "  ")
	if err := os.WriteFile(*out, data, 0644); err != nil {
		fmt.Println("写入授权文件失败:", err)
		os.Exit(1)
	}
	fmt.Printf("授权已生成: %s\n  客户: %s\n  到期: %s\n  机器码: %s\n", *out, *owner, exp, *code)
}
