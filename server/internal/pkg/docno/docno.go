package docno

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

var Prefixes = map[string]string{
	"purchase_order":  "CGDD", // 采购订单
	"purchase_in":     "CGRK", // 采购入库
	"sales_out":       "XSCK", // 销售出库
	"quotation":       "BJD",  // 报价单
	"purchase_return": "CGTH", // 采购退货
	"sales_return":    "XSTH", // 销售退货
	"stocktake":       "PD",   // 盘点
	"transfer":        "DB",   // 调拨
	"other_in":        "QTRK", // 其他入库
	"other_out":       "QTCK", // 其他出库
}

// Next 生成单号：前缀+日期+3位流水（业务层生成，不依赖数据库特性——PG 迁移安全）。
// 并发冲突由调用方在插入单据时重试（见 DocumentService.Create）。
func Next(db *gorm.DB, docType string, docDate time.Time) (string, error) {
	prefix, ok := Prefixes[docType]
	if !ok {
		return "", errcode.ErrParam
	}
	base := prefix + docDate.Format("20060102")
	var count int64
	if err := db.Model(&model.Document{}).Where("doc_no LIKE ?", base+"%").Count(&count).Error; err != nil {
		return "", err
	}
	// 找到第一个未占用的号
	for n := count + 1; n <= count+100; n++ {
		no := fmt.Sprintf("%s%03d", base, n)
		var dup int64
		if err := db.Model(&model.Document{}).Where("doc_no = ?", no).Count(&dup).Error; err != nil {
			return "", err
		}
		if dup == 0 {
			return no, nil
		}
	}
	return "", errcode.ErrDocNoDup
}
