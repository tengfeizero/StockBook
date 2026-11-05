package api

import (
	"github.com/gin-gonic/gin"

	"jxc/server/internal/middleware"
	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type TenantHandler struct {
	svc     *service.TenantService
	authSvc *service.AuthService
	secret  string
}

func NewTenantHandler(svc *service.TenantService, authSvc *service.AuthService, secret string) *TenantHandler {
	return &TenantHandler{svc: svc, authSvc: authSvc, secret: secret}
}

// Register 公开注册：开户 + 建管理员 + 可选演示数据，成功即返回登录令牌
func (h *TenantHandler) Register(c *gin.Context) {
	var req service.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	res, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	token, err := middleware.MakeToken(res.User.ID, res.Tenant.ID, res.User.DisplayName, h.secret)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"token":        token,
		"tenant_code":  res.Tenant.Code,
		"tenant_name":  res.Tenant.Name,
		"display_name": res.User.DisplayName,
		"expire_at":    res.Tenant.ExpireAt,
	})
}

// Plans 套餐列表（公开，注册页展示）
func (h *TenantHandler) Plans(c *gin.Context) {
	list, err := h.svc.Plans()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// Current 当前租户概览（含到期天数与用量）
func (h *TenantHandler) Current(c *gin.Context) {
	tid := c.GetInt64("tenant_id")
	if tid == 0 {
		// 单机版：用授权信息代替订阅信息
		response.OK(c, gin.H{"standalone": true})
		return
	}
	v, err := h.svc.Current(c.Request.Context(), tid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, v)
}

// Bills 本租户账单
func (h *TenantHandler) Bills(c *gin.Context) {
	tid := c.GetInt64("tenant_id")
	if tid == 0 {
		response.OK(c, []model.TenantBill{})
		return
	}
	list, err := h.svc.Bills(c.Request.Context(), tid)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

type subscribeReq struct {
	PlanCode string `json:"plan_code" binding:"required"`
	Months   int    `json:"months"`
}

// Subscribe 续费/升级（需 system 权限；当前为演示支付，直接生效并记账单）
func (h *TenantHandler) Subscribe(c *gin.Context) {
	tid := c.GetInt64("tenant_id")
	if tid == 0 {
		response.Fail(c, errcode.New(1001, "单机版无需订阅，请使用授权文件"))
		return
	}
	var req subscribeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	name, _ := c.Get("display_name")
	bill, err := h.svc.Subscribe(c.Request.Context(), tid, req.PlanCode, req.Months, strOr(name))
	if err != nil {
		response.Fail(c, err)
		return
	}
	v, _ := h.svc.Current(c.Request.Context(), tid)
	response.OK(c, gin.H{"bill": bill, "tenant": v})
}

// PublicInfo 登录页展示用：按编码查企业名称（不暴露敏感信息）
func (h *TenantHandler) PublicInfo(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Fail(c, errcode.ErrParam)
		return
	}
	t, err := h.svc.ResolveByCode(code)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"code": t.Code, "name": t.Name, "status": t.Status})
}

func strOr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "-"
}
