package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/license"
	"jxc/server/internal/pkg/query"
	"jxc/server/internal/pkg/response"
	"jxc/server/internal/service"
)

type SystemHandler struct {
	svc *service.SystemService
	lic *license.Manager
}

func NewSystemHandler(svc *service.SystemService, lic *license.Manager) *SystemHandler {
	return &SystemHandler{svc: svc, lic: lic}
}

// ---- 用户 ----

func (h *SystemHandler) ListUsers(c *gin.Context) {
	rows, err := h.svc.ListUsers(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *SystemHandler) CreateUser(c *gin.Context) {
	var req service.UserCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "用户名≥2位、密码≥6位、姓名和角色必填"))
		return
	}
	if err := h.svc.CreateUser(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *SystemHandler) UpdateUser(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req service.UserUpdateReq
	_ = c.ShouldBindJSON(&req)
	if err := h.svc.UpdateUser(c.Request.Context(), id, c.GetInt64("user_id"), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *SystemHandler) ResetPassword(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.New(1001, "密码必填"))
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), id, req.Password); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 角色 ----

func (h *SystemHandler) ListRoles(c *gin.Context) {
	rows, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *SystemHandler) SaveRole(c *gin.Context) {
	var req service.RoleSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.SaveRole(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *SystemHandler) DeleteRole(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.svc.DeleteRole(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 改密 / 设置 / 备份 / 日志 ----

func (h *SystemHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), c.GetInt64("user_id"), req.OldPassword, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *SystemHandler) GetSettings(c *gin.Context) {
	response.OK(c, h.svc.GetSettings(c.Request.Context()))
}

func (h *SystemHandler) SaveSettings(c *gin.Context) {
	var req service.SettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	if err := h.svc.SaveSettings(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, h.svc.GetSettings(c.Request.Context()))
}

func (h *SystemHandler) ListBackups(c *gin.Context) {
	rows, err := h.svc.ListBackups(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rows)
}

func (h *SystemHandler) CreateBackup(c *gin.Context) {
	b, err := h.svc.Backup(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, b)
}

// DownloadBackup 下载备份文件
func (h *SystemHandler) DownloadBackup(c *gin.Context) {
	name := c.Param("name")
	// 防目录穿越
	if filepath.Base(name) != name {
		response.Fail(c, errcode.ErrParam)
		return
	}
	path := filepath.Join("data/backups", name)
	if _, err := os.Stat(path); err != nil {
		response.Fail(c, errcode.ErrNotFound)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.Data(http.StatusOK, "application/octet-stream", []byte{})
	c.File(path)
}

// ---- 授权（M5） ----

// GetLicense 授权状态（试用/已授权/过期 + 机器码）
func (h *SystemHandler) GetLicense(c *gin.Context) {
	st := h.lic.Status(h.svc.FirstRun(c.Request.Context()))
	response.OK(c, st)
}

// UploadLicense 导入授权文件（license.key），校验通过后落盘并即时生效
func (h *SystemHandler) UploadLicense(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errcode.New(1001, "请选择 license.key 文件"))
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		response.Fail(c, errcode.ErrParam)
		return
	}
	var info license.Info
	if err := json.Unmarshal(data, &info); err != nil {
		response.Fail(c, errcode.New(1001, "授权文件格式错误"))
		return
	}
	if err := info.Verify(); err != nil {
		response.Fail(c, errcode.New(1001, "授权无效: "+err.Error()))
		return
	}
	if err := h.lic.EnsureDir(); err != nil {
		response.Fail(c, err)
		return
	}
	if err := os.WriteFile("data/license.key", data, 0644); err != nil {
		response.Fail(c, errcode.New(5000, "保存授权文件失败"))
		return
	}
	if err := h.lic.Reload(); err != nil {
		response.Fail(c, errcode.New(1001, err.Error()))
		return
	}
	response.OK(c, h.lic.Status(h.svc.FirstRun(c.Request.Context())))
}

func (h *SystemHandler) ListAuditLogs(c *gin.Context) {
	page, pageSize := query.Paging(c)
	list, total, err := h.svc.ListAuditLogs(c.Request.Context(), service.AuditListReq{
		Username: c.Query("username"),
		Action:   c.Query("action"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total)
}
