package api

import (
	"github.com/gin-gonic/gin"

	"jxc/server/internal/middleware"
	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type AuthHandler struct {
	svc    *service.AuthService
	secret string
}

func NewAuthHandler(svc *service.AuthService, secret string) *AuthHandler {
	return &AuthHandler{svc: svc, secret: secret}
}

type loginReq struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	TenantCode string `json:"tenant_code"` // 企业编码：SaaS 版必填，单机版留空
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	u, tenant, err := h.svc.Login(c.Request.Context(), req.TenantCode, req.Username, req.Password)
	if err != nil {
		response.Fail(c, err)
		return
	}
	// 供审计中间件把这条登录记录归到对应租户
	c.Set("tenant_id", u.TenantID)
	token, err := middleware.MakeToken(u.ID, u.TenantID, u.DisplayName, h.secret)
	if err != nil {
		response.Fail(c, err)
		return
	}
	out := gin.H{
		"token":        token,
		"display_name": u.DisplayName,
		"tenant_id":    u.TenantID,
	}
	if tenant != nil {
		out["tenant_code"] = tenant.Code
		out["tenant_name"] = tenant.Name
		out["plan_code"] = tenant.PlanCode
		out["expire_at"] = tenant.ExpireAt
	}
	response.OK(c, out)
}

func (h *AuthHandler) Me(c *gin.Context) {
	uid := c.GetInt64("user_id")
	u, err := h.svc.GetByID(c.Request.Context(), uid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	// 权限与角色名由 JWT 中间件实时注入
	perms, _ := c.Get("user_perms")
	roleName, _ := c.Get("role_name")
	out := gin.H{
		"username":     u.Username,
		"display_name": u.DisplayName,
		"role_name":    roleName,
		"permissions":  perms,
		"tenant_id":    u.TenantID,
	}
	if t := h.svc.TenantOf(u.TenantID); t != nil {
		out["tenant"] = gin.H{"code": t.Code, "name": t.Name, "plan_code": t.PlanCode, "expire_at": t.ExpireAt}
	}
	response.OK(c, out)
}
