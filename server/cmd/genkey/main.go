// genkey 生成一对 Ed25519 密钥，用于自建授权体系。
//
// 用法：
//
//	go run ./cmd/genkey
//
// 输出两行 base64：
//   - 公钥写入 internal/pkg/license/license.go 的 PublicKey（随产品分发，可公开）
//   - 私钥仅自己保管，签发授权时通过环境变量提供：
//     export HUOBEN_LICENSE_KEY='<私钥base64>'
//
// 注意：更换密钥后，此前签发的所有 license.key 立即失效，需要重新签发并发布新版程序。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Println("生成失败:", err)
		return
	}
	fmt.Println("PUBLIC_KEY=" + base64.StdEncoding.EncodeToString(pub))
	fmt.Println("PRIVATE_KEY=" + base64.StdEncoding.EncodeToString(priv))
	fmt.Println()
	fmt.Println("把 PUBLIC_KEY 填入 internal/pkg/license/license.go 的 PublicKey；")
	fmt.Println("PRIVATE_KEY 只保存在自己手里，签发时用环境变量 HUOBEN_LICENSE_KEY 传入。")
}
