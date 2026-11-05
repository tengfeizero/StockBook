package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type LocationHandler struct{ svc *service.LocationService }

func NewLocationHandler(svc *service.LocationService) *LocationHandler {
	return &LocationHandler{svc: svc}
}

func (h *LocationHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// Alerts 库位容量/上下限预警（超出容量、已满、接近满、低于下限）
func (h *LocationHandler) Alerts(c *gin.Context) {
	list, err := h.svc.Alerts(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// Stock 库位库存（location_id 不传=全部，0=未分配）
func (h *LocationHandler) Stock(c *gin.Context) {
	locID := int64(-1)
	if v := c.Query("location_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			locID = n
		}
	}
	rows, err := h.svc.Stock(c.Request.Context(), locID, c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *LocationHandler) Create(c *gin.Context) {
	var req service.LocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "库位名称必填"))
		return
	}
	if err := h.svc.Create(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *LocationHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	var req service.LocationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "库位名称必填"))
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *LocationHandler) Delete(c *gin.Context) {
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
