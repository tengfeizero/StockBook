package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/gormx"
)

// Audit 操作日志：记录登录及所有变更类（POST/PUT/DELETE）请求
// Detail 为请求体摘要（截断 500 字符，password 字段脱敏）
func Audit(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		isLogin := c.FullPath() == "/api/v1/auth/login" || c.Request.URL.Path == "/api/v1/auth/login"
		if method != "POST" && method != "PUT" && method != "DELETE" && !isLogin {
			c.Next()
			return
		}

		// 读取 body 并恢复，供后续 handler 使用
		var bodySnippet string
		if c.Request.Body != nil {
			raw, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
			bodySnippet = sanitize(string(raw))
			if len(bodySnippet) > 500 {
				bodySnippet = bodySnippet[:500]
			}
		}

		c.Next()

		action := method
		if isLogin {
			action = "LOGIN"
		}
		username, _ := c.Get("display_name")
		uid, _ := c.Get("user_id")
		uid64, _ := uid.(int64)
		result := "ok"
		if c.Writer.Status() >= 400 {
			result = "fail"
		}
		// 审计日志按租户归属：登录请求由 handler 在成功后写入 tenant_id
		entry := model.AuditLog{
			UserID:   uid64,
			Username: strOr(username, "-"),
			Action:   action,
			Target:   c.Request.URL.Path,
			Detail:   bodySnippet,
			IP:       c.ClientIP(),
			Result:   result,
		}
		tid := c.GetInt64("tenant_id")
		if tid == 0 {
			_ = db.Create(&entry).Error
			return
		}
		entry.TenantID = tid
		_ = db.WithContext(gormx.WithTenant(c.Request.Context(), tid)).Create(&entry).Error
	}
}

func strOr(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

// sanitize 脱敏 password 字段
func sanitize(s string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return strings.TrimSpace(s)
	}
	for _, k := range []string{"password", "old_password", "new_password"} {
		if _, ok := m[k]; ok {
			m[k] = "***"
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return s
	}
	return string(out)
}
