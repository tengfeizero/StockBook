package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"jxc/server/internal/model"
)

type ImportService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *ImportService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewImportService(db *gorm.DB) *ImportService { return &ImportService{conn: db} }

// RowError 单行导入失败
type RowError struct {
	Row int    `json:"row"`
	Msg string `json:"msg"`
}

type ImportResult struct {
	Success int        `json:"success"`
	Skipped int        `json:"skipped"`
	Errors  []RowError `json:"errors"`
}

// ---------- 工具 ----------

// headerIndex 表头名 → 列号
func headerIndex(rows [][]string) map[string]int {
	if len(rows) == 0 {
		return nil
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

func cell(row []string, idx map[string]int, key string) string {
	i, ok := idx[key]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func parseFen(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if err != nil {
		return 0, fmt.Errorf("金额「%s」不是数字", s)
	}
	return int64(math.Round(f * 100)), nil
}

func parseQty1000(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if err != nil {
		return 0, fmt.Errorf("数量「%s」不是数字", s)
	}
	return int64(math.Round(f * 1000)), nil
}

func parseInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	if err != nil {
		return 0, fmt.Errorf("「%s」不是整数", s)
	}
	return n, nil
}

func readSheet(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("无法读取 Excel 文件")
	}
	defer f.Close()
	return f.GetRows(f.GetSheetName(0))
}

// ---------- 商品导入 ----------

func (s *ImportService) ImportProducts(ctx context.Context, r io.Reader) (*ImportResult, error) {
	rows, err := readSheet(r)
	if err != nil {
		return nil, err
	}
	idx := headerIndex(rows)
	if _, ok := idx["编码"]; !ok {
		return nil, fmt.Errorf("缺少「编码」列")
	}
	res := &ImportResult{}
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		code, name := cell(row, idx, "编码"), cell(row, idx, "名称")
		if code == "" && name == "" {
			continue // 空行
		}
		if code == "" || name == "" {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: "编码和名称必填"})
			continue
		}
		// 成本方式
		cm := cell(row, idx, "成本方式")
		cm = map[string]string{"加权平均": "weighted_avg", "先进先出": "fifo", "": "weighted_avg"}[cm]
		if cm == "" {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: fmt.Sprintf("成本方式「%s」无法识别", cell(row, idx, "成本方式"))})
			continue
		}
		batch := strings.Contains(cell(row, idx, "批次管理"), "是") || cm == "fifo"
		// 分类（不存在自动建）
		categoryID := int64(0)
		if cn := cell(row, idx, "分类"); cn != "" {
			var c model.Category
			if err := s.db(ctx).Where("name = ?", cn).First(&c).Error; err == gorm.ErrRecordNotFound {
				c = model.Category{Name: cn}
				if err := s.db(ctx).Create(&c).Error; err != nil {
					res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: "创建分类失败"})
					continue
				}
			} else if err != nil {
				res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: "查询分类失败"})
				continue
			}
			categoryID = c.ID
		}
		// 数值列
		shelfLife, err1 := parseInt(cell(row, idx, "保质期天数"))
		safeStock, err2 := parseQty1000(cell(row, idx, "安全库存"))
		if err1 != nil || err2 != nil {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: "保质期或安全库存格式错误"})
			continue
		}
		// 价位：表头以"价"结尾的列都作为价位（零售价/批发价/会员价…）
		type priceItem struct {
			Name string
			Fen  int64
		}
		var prices []priceItem
		priceErr := ""
		for h := range idx {
			if !strings.HasSuffix(h, "价") || h == "单价" {
				continue
			}
			fen, err := parseFen(cell(row, idx, h))
			if err != nil {
				priceErr = fmt.Sprintf("「%s」列金额格式错误", h)
				break
			}
			prices = append(prices, priceItem{Name: h, Fen: fen})
		}
		if priceErr != "" {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: priceErr})
			continue
		}
		// 编码重复 → 跳过
		var count int64
		s.db(ctx).Model(&model.Product{}).Where("code = ?", code).Count(&count)
		if count > 0 {
			res.Skipped++
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: fmt.Sprintf("编码「%s」已存在，已跳过", code)})
			continue
		}
		// PLU 重复 → 跳过（秤码会据此定位商品，必须唯一）
		if plu := cell(row, idx, "PLU"); plu != "" {
			var pluCount int64
			s.db(ctx).Model(&model.Product{}).Where("plu = ?", plu).Count(&pluCount)
			if pluCount > 0 {
				res.Skipped++
				res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: fmt.Sprintf("PLU「%s」已被其他商品使用，已跳过", plu)})
				continue
			}
		}
		// 事务建商品+价位
		err = s.db(ctx).Transaction(func(tx *gorm.DB) error {
			p := model.Product{
				Code: code, Name: name, Barcode: cell(row, idx, "条码"), PLU: cell(row, idx, "PLU"),
				CategoryID: categoryID, Unit: cell(row, idx, "单位"),
				CostMethod: cm, IsBatchEnabled: batch, ShelfLifeDays: shelfLife,
				SafeStock: safeStock, Status: "active",
			}
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
			defaultID := int64(0)
			for j, pr := range prices {
				pp := model.ProductPrice{ProductID: p.ID, Name: pr.Name, Price: pr.Fen, Sort: j}
				if err := tx.Create(&pp).Error; err != nil {
					return err
				}
				if j == 0 {
					defaultID = pp.ID
				}
			}
			return tx.Model(&model.Product{}).Where("id = ?", p.ID).Update("default_price_id", defaultID).Error
		})
		if err != nil {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: fmt.Sprintf("保存失败：%v", err)})
			continue
		}
		res.Success++
	}
	return res, nil
}

// ---------- 往来单位导入 ----------

func (s *ImportService) ImportPartners(ctx context.Context, kind string, r io.Reader) (*ImportResult, error) {
	if kind != "supplier" && kind != "customer" {
		return nil, fmt.Errorf("类型错误")
	}
	rows, err := readSheet(r)
	if err != nil {
		return nil, err
	}
	idx := headerIndex(rows)
	res := &ImportResult{}
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		code, name := cell(row, idx, "编码"), cell(row, idx, "名称")
		if code == "" && name == "" {
			continue
		}
		if code == "" || name == "" {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: "编码和名称必填"})
			continue
		}
		openingHeader := "期初应付"
		if kind == "customer" {
			openingHeader = "期初应收"
		}
		opening, err := parseFen(cell(row, idx, openingHeader))
		if err != nil {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: "期初金额格式错误"})
			continue
		}
		var count int64
		if kind == "supplier" {
			s.db(ctx).Model(&model.Supplier{}).Where("code = ?", code).Count(&count)
		} else {
			s.db(ctx).Model(&model.Customer{}).Where("code = ?", code).Count(&count)
		}
		if count > 0 {
			res.Skipped++
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: fmt.Sprintf("编码「%s」已存在，已跳过", code)})
			continue
		}
		if kind == "supplier" {
			err = s.db(ctx).Create(&model.Supplier{
				Code: code, Name: name, Contact: cell(row, idx, "联系人"),
				Phone: cell(row, idx, "电话"), Address: cell(row, idx, "地址"),
				OpeningPayable: opening, Remark: cell(row, idx, "备注"),
			}).Error
		} else {
			err = s.db(ctx).Create(&model.Customer{
				Code: code, Name: name, Contact: cell(row, idx, "联系人"),
				Phone: cell(row, idx, "电话"), Address: cell(row, idx, "地址"),
				OpeningReceivable: opening, Remark: cell(row, idx, "备注"),
			}).Error
		}
		if err != nil {
			res.Errors = append(res.Errors, RowError{Row: i + 1, Msg: fmt.Sprintf("保存失败：%v", err)})
			continue
		}
		res.Success++
	}
	return res, nil
}

// ---------- 模板下载 ----------

func (s *ImportService) Template(ctx context.Context, kind string) (*excelize.File, string, error) {
	f := excelize.NewFile()
	var headers []string
	var example []string
	switch kind {
	case "products":
		headers = []string{"编码", "名称", "分类", "单位", "条码", "PLU", "成本方式", "批次管理", "保质期天数", "安全库存", "零售价", "批发价", "会员价"}
		example = []string{"SP001", "散装自攻螺丝", "紧固件", "kg", "", "00001", "加权平均", "否", "0", "20", "12.6", "10.35", "11.25"}
		f.SetSheetName("Sheet1", "商品")
	case "supplier":
		headers = []string{"编码", "名称", "联系人", "电话", "地址", "期初应付", "备注"}
		example = []string{"GYS001", "华东饮料批发", "王经理", "13800000000", "嘉兴市南湖区", "500", ""}
		f.SetSheetName("Sheet1", "供应商")
	case "customer":
		headers = []string{"编码", "名称", "联系人", "电话", "地址", "期初应收", "备注"}
		example = []string{"KH001", "旺旺便利店", "李老板", "13900000000", "嘉兴市南湖区", "0", ""}
		f.SetSheetName("Sheet1", "客户")
	default:
		return nil, "", fmt.Errorf("类型错误")
	}
	sheet := f.GetSheetName(0)
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, v := range example {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, v)
	}
	return f, sheet, nil
}
