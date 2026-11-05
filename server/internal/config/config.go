package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port      string
	DBPath    string
	JWTSecret string
	// TrustedProxies 允许携带 X-Forwarded-For / X-Forwarded-Proto 的代理地址（IP 或 CIDR）。
	// 反向代理（Caddy/Nginx）与应用同机部署时保持默认回环地址即可；代理在别的机器或
	// 容器内时用 JXC_TRUSTED_PROXIES 追加，例如 "127.0.0.1,172.17.0.1"。
	// 设为 "none" 表示不信任任何代理（客户端 IP 一律取 TCP 对端）。
	TrustedProxies []string
}

func Load() *Config {
	dbPath := getEnv("JXC_DB_PATH", "data/jxc.db")
	return &Config{
		Port:           getEnv("JXC_PORT", "8080"),
		DBPath:         dbPath,
		JWTSecret:      LoadJWTSecret(dbPath),
		TrustedProxies: ParseProxies(getEnv("JXC_TRUSTED_PROXIES", "127.0.0.1,::1")),
	}
}

// LoadJWTSecret 取 JWT 签名密钥，优先级：环境变量 > 数据目录下的 .jwt_secret > 新生成。
// 生成后落盘到数据库同目录（权限 0600），重启不会让已登录用户掉线；
// 多实例部署请统一用 JXC_JWT_SECRET，否则各实例签发的令牌互不认。
func LoadJWTSecret(dbPath string) string {
	if v := strings.TrimSpace(os.Getenv("JXC_JWT_SECRET")); v != "" {
		return v
	}
	dir := filepath.Dir(dbPath)
	if dir == "" {
		dir = "."
	}
	secretFile := filepath.Join(dir, ".jwt_secret")
	if b, err := os.ReadFile(secretFile); err == nil {
		if s := strings.TrimSpace(string(b)); len(s) >= 32 {
			return s
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// 极端情况下退回固定值并告警：能用但令牌可预测，必须让运维看到
		log.Printf("警告: 生成随机 JWT 密钥失败（%v），已退回临时密钥，请显式设置 JXC_JWT_SECRET", err)
		return "insecure-fallback-please-set-JXC_JWT_SECRET"
	}
	secret := hex.EncodeToString(buf)
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if err := os.WriteFile(secretFile, []byte(secret), 0o600); err != nil {
			log.Printf("警告: 无法写入 %s（%v），本次启动使用临时密钥，重启后需重新登录", secretFile, err)
		}
	}
	return secret
}

// ParseProxies 把逗号分隔的代理地址转为切片；"none"/空串返回空切片（不信任任何代理）。
func ParseProxies(raw string) []string {
	out := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" || strings.EqualFold(p, "none") {
			continue
		}
		out = append(out, p)
	}
	return out
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
