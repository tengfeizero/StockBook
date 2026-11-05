package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders 基础安全响应头。
// HTTPS（直连 TLS 或反向代理下发 X-Forwarded-Proto: https）时追加 HSTS：
// 浏览器会记住该域名一年内只走 HTTPS，因此只在确认是 HTTPS 的请求上下发，
// 避免单机 HTTP 部署被误锁在 https 上。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "same-origin")
		if IsHTTPS(c.Request) {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		c.Next()
	}
}

// IsHTTPS 判断请求是否经 HTTPS 到达：直连 TLS，或可信代理转发时带
// X-Forwarded-Proto: https。浏览器只在安全上下文才认可 HSTS，故该头被伪造无实际危害。
func IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
