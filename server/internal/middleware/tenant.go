package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/gormx"
	"jxc/server/internal/pkg/response"
)

// TenantCtx 把 JWT 中的租户 ID 注入请求上下文。
// 服务层统一用 c.Request.Context() 取数据库句柄，GORM 租户插件据此自动
// 追加 tenant_id 条件 / 填充 tenant_id，实现租户间数据隔离。
// 单机版（TenantID=0）同样隔离到 0 号租户，杜绝越权看到 SaaS 租户的数据。
func TenantCtx() gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := c.GetInt64("tenant_id")
		c.Request = c.Request.WithContext(gormx.WithTenant(c.Request.Context(), tid))
		c.Next()
	}
}

// TenantGuard 订阅有效性校验：租户停用或订阅到期后禁止写操作（读仍可用，
// 便于导出数据）；登录、租户信息、续费相关接口不拦截，否则到期后无法自助续费。
// 0 号租户（单机版）不受订阅约束，由 LicenseGuard 管理。
func TenantGuard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 登录、租户信息、续费等接口始终放行（到期后要能自助续费/自证身份）
		p := c.FullPath()
		if strings.HasPrefix(p, "/api/v1/auth/") || strings.HasPrefix(p, "/api/v1/tenants/") {
			c.Next()
			return
		}
		tid := c.GetInt64("tenant_id")
		if tid == 0 {
			c.Next()
			return
		}
		var t model.Tenant
		if err := db.First(&t, tid).Error; err != nil {
			response.Fail(c, errcode.ErrTenantNotFound)
			c.Abort()
			return
		}
		// 停用：读写全禁（服务商强制断服）
		if t.Status != "active" {
			response.Fail(c, errcode.ErrTenantDisabled)
			c.Abort()
			return
		}
		// 到期：只读，便于导出数据；写操作需先续费
		readOnly := c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead
		if !readOnly && t.ExpireAt != nil && time.Now().After(*t.ExpireAt) {
			response.Fail(c, errcode.ErrTenantExpired)
			c.Abort()
			return
		}
		c.Next()
	}
}
