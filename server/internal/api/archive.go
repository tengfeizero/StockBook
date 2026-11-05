package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/query"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

// 通用：路径参数 id
func pathID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errcode.ErrParam
	}
	return id, nil
}

type CategoryHandler struct{ svc *service.CategoryService }

func NewCategoryHandler(svc *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{svc: svc}
}

func (h *CategoryHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

func (h *CategoryHandler) Create(c *gin.Context) {
	var req service.CategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Create(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req service.CategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

type ProductHandler struct{ svc *service.ProductService }

func NewProductHandler(svc *service.ProductService) *ProductHandler { return &ProductHandler{svc: svc} }

func (h *ProductHandler) List(c *gin.Context) {
	page, pageSize := query.Paging(c)
	categoryID, _ := strconv.ParseInt(c.Query("category_id"), 10, 64)
	list, total, err := h.svc.List(c.Request.Context(), service.ProductListReq{
		Keyword:    c.Query("keyword"),
		CategoryID: categoryID,
		Status:     c.Query("status"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total)
}

func (h *ProductHandler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	p, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, p)
}

func (h *ProductHandler) Create(c *gin.Context) {
	var req service.ProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Create(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ProductHandler) Update(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req service.ProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *ProductHandler) Delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

type PartnerHandler struct{ svc *service.PartnerService }

func NewPartnerHandler(svc *service.PartnerService) *PartnerHandler { return &PartnerHandler{svc: svc} }

func (h *PartnerHandler) List(c *gin.Context) {
	page, pageSize := query.Paging(c)
	list, total, err := h.svc.List(c.Request.Context(), c.Param("kind"), c.Query("keyword"), page, pageSize)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total)
}

func (h *PartnerHandler) Create(c *gin.Context) {
	var req service.PartnerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Create(c.Request.Context(), c.Param("kind"), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *PartnerHandler) Update(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req service.PartnerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Update(c.Request.Context(), c.Param("kind"), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *PartnerHandler) Delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), c.Param("kind"), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
