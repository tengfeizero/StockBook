package api

import (
	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/query"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type FinanceHandler struct{ svc *service.FinanceService }

func NewFinanceHandler(svc *service.FinanceService) *FinanceHandler { return &FinanceHandler{svc: svc} }

func (h *FinanceHandler) CreatePayment(c *gin.Context) {
	var req service.PaymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	userID := c.GetInt64("user_id")
	p, err := h.svc.CreatePayment(c.Request.Context(), req, userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": p.ID, "doc_no": p.DocNo})
}

func (h *FinanceHandler) VoidPayment(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.VoidPayment(c.Request.Context(), id, req.Reason); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *FinanceHandler) ListPayments(c *gin.Context) {
	page, pageSize := query.Paging(c)
	list, total, err := h.svc.ListPayments(c.Request.Context(), service.PaymentListReq{
		Kind:      c.Query("kind"),
		PartnerID: c.GetInt64("partner_id"),
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total)
}

func (h *FinanceHandler) Balances(c *gin.Context) {
	kind := c.Query("kind")
	if kind != "customer" && kind != "supplier" {
		response.Fail(c, errcode.ErrParam)
		return
	}
	list, err := h.svc.Balances(c.Request.Context(), kind)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}
