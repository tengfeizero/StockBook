package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/license"
	"jxc/server/internal/pkg/response"
)

// LicenseGuard 单机版授权过期时拦截所有写操作；读操作与登录/授权导入放行，
// 保证过期后仍可查看数据与导入新授权。
// SaaS 租户（tenant_id > 0）的付费状态由订阅管理，不受本地授权文件约束。
func LicenseGuard(m *license.Manager, firstRun func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut &&
			c.Request.Method != http.MethodDelete && c.Request.Method != http.MethodPatch {
			c.Next()
			return
		}
		p := c.FullPath()
		if strings.HasPrefix(p, "/api/v1/auth/") || p == "/api/v1/system/license" ||
			strings.HasPrefix(p, "/api/v1/tenants/") {
			c.Next()
			return
		}
		if c.GetInt64("tenant_id") > 0 {
			c.Next()
			return
		}
		if m.Expired(firstRun()) {
			response.Fail(c, errcode.New(5001, "授权已过期：请在 系统设置→授权信息 导入新的授权文件"))
			c.Abort()
			return
		}
		c.Next()
	}
}
