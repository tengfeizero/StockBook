package api

import (
	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/query"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type DocHandler struct{ svc *service.DocumentService }

func NewDocHandler(svc *service.DocumentService) *DocHandler { return &DocHandler{svc: svc} }

// docTypePerm 单据类型 → 权限模块（/docs 路由按模块放行，这里按单据类型细控）
var docTypePerm = map[string]string{
	"purchase_order": "purchase", "purchase_in": "purchase", "purchase_return": "purchase",
	"sales_out": "sales", "quotation": "sales", "sales_return": "sales",
	"other_in": "stock", "other_out": "stock", "stocktake": "stock", "transfer": "stock",
}

// canAccessDoc 校验当前用户是否有该单据类型的权限
func canAccessDoc(c *gin.Context, docType string) bool {
	need, ok := docTypePerm[docType]
	if !ok {
		return false
	}
	raw, _ := c.Get("user_perms")
	perms, _ := raw.([]string)
	for _, p := range perms {
		if p == "*" || p == need {
			return true
		}
	}
	return false
}

func (h *DocHandler) Create(c *gin.Context) {
	var req service.DocReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if !canAccessDoc(c, req.DocType) {
		response.Fail(c, errcode.ErrForbidden)
		return
	}
	userID := c.GetInt64("user_id")
	doc, err := h.svc.Create(c.Request.Context(), req, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": doc.ID, "doc_no": doc.DocNo, "status": doc.Status})
}

func (h *DocHandler) Confirm(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if !canAccessDoc(c, doc.DocType) {
		response.Fail(c, errcode.ErrForbidden)
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.Confirm(c.Request.Context(), id, userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *DocHandler) Void(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if !canAccessDoc(c, doc.DocType) {
		response.Fail(c, errcode.ErrForbidden)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.Void(c.Request.Context(), id, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// Review 盘点差异审批（v0.16）：仅持有 system 权限的主管可操作
func (h *DocHandler) Review(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if doc.DocType != "stocktake" {
		response.Fail(c, errcode.ErrForbidden)
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"` // approve / reject
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Action != "approve" && req.Action != "reject") {
		response.Fail(c, errcode.ErrParam)
		return
	}
	userID := c.GetInt64("user_id")
	if err := h.svc.Review(c.Request.Context(), id, req.Action == "approve", req.Note, userID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *DocHandler) List(c *gin.Context) {
	page, pageSize := query.Paging(c)
	list, total, err := h.svc.List(c.Request.Context(), service.DocListReq{
		DocType:  c.Query("doc_type"),
		Status:   c.Query("status"),
		Keyword:  c.Query("keyword"),
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total)
}

func (h *DocHandler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, doc)
}
