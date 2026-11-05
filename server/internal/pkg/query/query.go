package query

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// 分页参数（CONVENTIONS：page 从 1 起，page_size 默认 20）
func Paging(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return
}

func OffsetLimit(page, pageSize int) (int, int) {
	return (page - 1) * pageSize, pageSize
}
