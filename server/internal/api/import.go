package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type ImportHandler struct {
	svc        *service.ImportService
	openingSvc *service.OpeningService
}

func NewImportHandler(svc *service.ImportService, openingSvc *service.OpeningService) *ImportHandler {
	return &ImportHandler{svc: svc, openingSvc: openingSvc}
}

func (h *ImportHandler) file(c *gin.Context) (io.ReadCloser, bool) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return nil, false
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return nil, false
	}
	return f, true
}

func (h *ImportHandler) Products(c *gin.Context) {
	f, ok := h.file(c)
	if !ok {
		return
	}
	defer f.Close()
	res, err := h.svc.ImportProducts(c.Request.Context(), f)
	if err != nil {
		response.Fail(c, errcode.New(4006, err.Error()))
		return
	}
	response.OK(c, res)
}

func (h *ImportHandler) Partners(c *gin.Context) {
	f, ok := h.file(c)
	if !ok {
		return
	}
	defer f.Close()
	res, err := h.svc.ImportPartners(c.Request.Context(), c.Param("kind"), f)
	if err != nil {
		response.Fail(c, errcode.New(4006, err.Error()))
		return
	}
	response.OK(c, res)
}

func (h *ImportHandler) Template(c *gin.Context) {
	f, sheet, err := h.svc.Template(c.Request.Context(), c.Param("kind"))
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="import-template.xlsx"`)
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	buf, err := f.WriteToBuffer()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	_ = sheet
	c.Data(http.StatusOK, "application/octet-stream", buf.Bytes())
}

// ---------- 期初库存 ----------

type OpeningHandler struct{ svc *service.OpeningService }

func NewOpeningHandler(svc *service.OpeningService) *OpeningHandler { return &OpeningHandler{svc: svc} }

func (h *OpeningHandler) Stocks(c *gin.Context) {
	rows, err := h.svc.ListStocks(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

type openingSaveReq struct {
	Rows []service.OpeningReq `json:"rows" binding:"required,min=1"`
}

func (h *OpeningHandler) Save(c *gin.Context) {
	var req openingSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.Save(c.Request.Context(), req.Rows); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
