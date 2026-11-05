package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"jxc/server/internal/pkg/errcode"
)

// 统一响应：{ code, msg, data }
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func Page(c *gin.Context, list any, total int64) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"list": list, "total": total}})
}

func Fail(c *gin.Context, err error) {
	var biz *errcode.BizError
	if errors.As(err, &biz) {
		c.JSON(http.StatusOK, gin.H{"code": biz.Code, "msg": biz.Msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 5000, "msg": "系统内部错误"})
}
