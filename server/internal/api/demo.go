package api

import (
	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

// DemoHandler 演示数据一键初始化
type DemoHandler struct{ svc *service.DemoService }

func NewDemoHandler(svc *service.DemoService) *DemoHandler { return &DemoHandler{svc: svc} }

type demoInitReq struct {
	Confirm  string `json:"confirm"`  // 必须传 "RESET"，防止误触清空数据
	Template string `json:"template"` // 行业模板：hardware（默认）/ convenience / fresh
}

// Templates 可选行业模板（登录页/系统管理页展示，公开接口）
func (h *DemoHandler) Templates(c *gin.Context) {
	response.OK(c, h.svc.Templates(c.Request.Context()))
}

// PublicInit 登录页「一键体验演示数据」（未登录可调用；带确认字防误触）
func (h *DemoHandler) PublicInit(c *gin.Context) {
	var req demoInitReq
	_ = c.ShouldBindJSON(&req)
	if req.Confirm != "RESET" {
		response.Fail(c, errcode.New(1001, "请传入确认字段以执行演示数据初始化"))
		return
	}
	// 已启用多租户（SaaS）时，公开入口会清空所有租户数据 —— 必须登录后在租户内重置
	if h.svc.HasTenants(c.Request.Context()) {
		response.Fail(c, errcode.New(1003, "已启用多租户，请在登录后于 系统管理→演示数据 中重置本企业数据"))
		return
	}
	sum, err := h.svc.Reset(c.Request.Context(), 0, req.Template)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sum)
}

// ResetAuthed 系统管理页「重置为演示数据」（需 system 权限；按当前操作者记审计）
func (h *DemoHandler) ResetAuthed(c *gin.Context) {
	var req demoInitReq
	_ = c.ShouldBindJSON(&req)
	if req.Confirm != "RESET" {
		response.Fail(c, errcode.New(1001, "请传入确认字段以执行演示数据初始化"))
		return
	}
	sum, err := h.svc.Reset(c.Request.Context(), c.GetInt64("user_id"), req.Template)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sum)
}
