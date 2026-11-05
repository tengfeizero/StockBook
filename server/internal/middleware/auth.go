package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
)

// JWTAuth 登录校验：解析 token，实时加载用户状态与角色权限注入上下文
// （权限每次请求从库读，角色调整即时生效，不受 7 天 token 有效期影响）
func JWTAuth(secret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token := strings.TrimPrefix(h, "Bearer ")
		if token == "" || token == h {
			response.Fail(c, errcode.ErrUnauth)
			c.Abort()
			return
		}
		claims, err := ParseToken(token, secret)
		if err != nil {
			response.Fail(c, errcode.ErrUnauth)
			c.Abort()
			return
		}
		// 实时校验用户状态 + 租户归属 + 取角色权限
		var u model.User
		if err := db.First(&u, claims.UserID).Error; err != nil || u.Status != "active" {
			response.Fail(c, errcode.ErrUserDisabled)
			c.Abort()
			return
		}
		// 令牌里的租户必须与用户实际所属租户一致（防止改包越权到其它企业）
		if u.TenantID != claims.TenantID {
			response.Fail(c, errcode.ErrUnauth)
			c.Abort()
			return
		}
		perms := []string{}
		if u.RoleID > 0 {
			var role model.Role
			if err := db.First(&role, u.RoleID).Error; err == nil {
				_ = json.Unmarshal([]byte(role.Permissions), &perms)
				c.Set("role_name", role.Name)
			}
		}
		c.Set("user_id", u.ID)
		c.Set("tenant_id", u.TenantID)
		c.Set("display_name", u.DisplayName)
		c.Set("user_perms", perms)
		c.Next()
	}
}

// RequirePerm 权限校验：拥有任一列出的模块码（或 "*"）即放行
func RequirePerm(anyOf ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, _ := c.Get("user_perms")
		perms, _ := raw.([]string)
		for _, p := range perms {
			if p == "*" {
				c.Next()
				return
			}
			for _, need := range anyOf {
				if p == need {
					c.Next()
					return
				}
			}
		}
		response.Fail(c, errcode.ErrForbidden)
		c.Abort()
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
