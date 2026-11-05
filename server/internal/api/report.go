package api

import (
	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type ReportHandler struct {
	reportSvc *service.ReportService
	stockSvc  *service.StockService
}

func NewReportHandler(reportSvc *service.ReportService, stockSvc *service.StockService) *ReportHandler {
	return &ReportHandler{reportSvc: reportSvc, stockSvc: stockSvc}
}

func (h *ReportHandler) Summary(c *gin.Context) {
	from, to := c.Query("date_from"), c.Query("date_to")
	rows, err := h.reportSvc.Summary(c.Request.Context(), from, to)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *ReportHandler) Profit(c *gin.Context) {
	from, to := c.Query("date_from"), c.Query("date_to")
	res, err := h.reportSvc.Profit(c.Request.Context(), from, to)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, res)
}

func (h *ReportHandler) Dashboard(c *gin.Context) {
	st, err := h.reportSvc.Dashboard(c.Request.Context())
	if err != nil {
		response.Fail(c, errcode.New(5000, err.Error()))
		return
	}
	response.OK(c, st)
}

func (h *ReportHandler) Stocks(c *gin.Context) {
	rows, err := h.stockSvc.List(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *ReportHandler) Batches(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	rows, err := h.stockSvc.Batches(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}
