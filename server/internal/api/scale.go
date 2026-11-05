package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type ScaleHandler struct{ svc *service.ScaleService }

func NewScaleHandler(svc *service.ScaleService) *ScaleHandler { return &ScaleHandler{svc: svc} }

func (h *ScaleHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *ScaleHandler) Create(c *gin.Context) {
	var req service.ScaleRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "参数格式错误"))
		return
	}
	if err := h.svc.Create(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ScaleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	var req service.ScaleRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "参数格式错误"))
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ScaleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// Parse 秤码解析（开单扫码 + 规则测试共用）
func (h *ScaleHandler) Parse(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "请提供条码"))
		return
	}
	res, err := h.svc.Parse(c.Request.Context(), req.Code)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}
