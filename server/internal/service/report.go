package service

import (
	"context"
	"time"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// ReportService M3 报表：进销存汇总、销售毛利、首页看板
type ReportService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *ReportService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewReportService(db *gorm.DB) *ReportService { return &ReportService{conn: db} }

// ---------- 进销存汇总 ----------

type SummaryRow struct {
	ProductID  int64  `json:"product_id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	OpeningQty int64  `json:"opening_qty"` // 期初数量 ×1000
	OpeningVal int64  `json:"opening_val"` // 期初金额 分
	InQty      int64  `json:"in_qty"`
	InVal      int64  `json:"in_val"`
	OutQty     int64  `json:"out_qty"`
	OutVal     int64  `json:"out_val"` // 出库成本额
	ClosingQty int64  `json:"closing_qty"`
	ClosingVal int64  `json:"closing_val"`
}

// Summary 依据库存流水聚合（流水只追加 → 任意时间段可重放）。
// 期初 = 起始日之前全部流水；单据流水按单据日期归属，无单据流水（期初建账）按流水创建时间。
func (s *ReportService) Summary(ctx context.Context, dateFrom, dateTo string) ([]SummaryRow, error) {
	if dateFrom == "" || dateTo == "" {
		return nil, errcode.ErrParam
	}
	from, e1 := time.ParseInLocation("2006-01-02", dateFrom, time.Local)
	to, e2 := time.ParseInLocation("2006-01-02", dateTo, time.Local)
	if e1 != nil || e2 != nil {
		return nil, errcode.ErrParam
	}
	toEnd := to.AddDate(0, 0, 1)

	type mrow struct {
		ProductID int64
		Direction string
		Quantity  int64
		UnitCost  int64
		DocDate   *time.Time
		CreatedAt time.Time
	}
	var rows []mrow
	err := s.db(ctx).Table("inventory_movements").
		Select("inventory_movements.product_id, inventory_movements.direction, inventory_movements.quantity, inventory_movements.unit_cost, inventory_movements.created_at, documents.doc_date").
		Joins("LEFT JOIN documents ON documents.id = inventory_movements.document_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// 商品信息
	var products []model.Product
	if err := s.db(ctx).Find(&products).Error; err != nil {
		return nil, err
	}
	prodMap := map[int64]model.Product{}
	for _, p := range products {
		prodMap[p.ID] = p
	}

	summary := map[int64]*SummaryRow{}
	get := func(pid int64) *SummaryRow {
		if r, ok := summary[pid]; ok {
			return r
		}
		p := prodMap[pid]
		r := &SummaryRow{ProductID: pid, Code: p.Code, Name: p.Name, Unit: p.Unit}
		summary[pid] = r
		return r
	}

	for _, m := range rows {
		d := m.CreatedAt
		if m.DocDate != nil {
			d = *m.DocDate
		}
		r := get(m.ProductID)
		val := m.Quantity * m.UnitCost
		if d.Before(from) {
			// 期初
			if m.Direction == "in" {
				r.OpeningQty += m.Quantity
				r.OpeningVal += val
			} else {
				r.OpeningQty -= m.Quantity
				r.OpeningVal -= val
			}
		} else if d.Before(toEnd) {
			if m.Direction == "in" {
				r.InQty += m.Quantity
				r.InVal += val
			} else {
				r.OutQty += m.Quantity
				r.OutVal += val
			}
		}
		// toEnd 之后的不进本期，也不进期初
	}
	result := make([]SummaryRow, 0, len(summary))
	for _, r := range summary {
		r.ClosingQty = r.OpeningQty + r.InQty - r.OutQty
		r.ClosingVal = r.OpeningVal + r.InVal - r.OutVal
		// 无动过的商品不展示，减少噪音
		if r.OpeningQty == 0 && r.InQty == 0 && r.OutQty == 0 {
			continue
		}
		result = append(result, *r)
	}
	// 按编码排序
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Code < result[i].Code {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result, nil
}

// ---------- 销售毛利 ----------

type ProfitRow struct {
	DocID   int64  `json:"doc_id"`
	DocNo   string `json:"doc_no"`
	DocDate string `json:"doc_date"`
	Partner string `json:"partner"`
	DocType string `json:"doc_type"`
	Revenue int64  `json:"revenue"` // 销售出库 +，销售退货 −
	Cost    int64  `json:"cost"`    // 结转成本，退货为负
	Profit  int64  `json:"profit"`
}

type ProfitResult struct {
	Rows  []ProfitRow `json:"rows"`
	Total ProfitRow   `json:"total"`
}

func (s *ReportService) Profit(ctx context.Context, dateFrom, dateTo string) (*ProfitResult, error) {
	if dateFrom == "" || dateTo == "" {
		return nil, errcode.ErrParam
	}
	var docs []model.Document
	q := s.db(ctx).Preload("Items").
		Where("doc_type IN ? AND status = ?", []string{"sales_out", "sales_return"}, "confirmed").
		Where("doc_date >= ? AND doc_date < ?", dateFrom, dateTo+" 23:59:59").
		Order("id ASC")
	if err := q.Find(&docs).Error; err != nil {
		return nil, err
	}
	res := &ProfitResult{Rows: []ProfitRow{}}
	t := ProfitRow{DocNo: "合计"}
	for _, d := range docs {
		var revenue, cost int64
		sign := int64(1)
		if d.DocType == "sales_return" {
			sign = -1
		}
		for _, it := range d.Items {
			revenue += sign * it.Amount
			cost += sign * it.Cost * it.Quantity / 1000
		}
		row := ProfitRow{
			DocID: d.ID, DocNo: d.DocNo, DocDate: d.DocDate.Format("2006-01-02"),
			Partner: d.PartnerName, DocType: d.DocType,
			Revenue: revenue, Cost: cost, Profit: revenue - cost,
		}
		res.Rows = append(res.Rows, row)
		t.Revenue += revenue
		t.Cost += cost
		t.Profit += row.Profit
	}
	res.Total = t
	return res, nil
}

// ---------- 首页看板 ----------

type DashboardStats struct {
	TodaySales     int64          `json:"today_sales"`     // 今日销售额（分，扣退货）
	TodayProfit    int64          `json:"today_profit"`    // 今日毛利
	InventoryVal   int64          `json:"inventory_val"`   // 库存总值（分）
	Receivable     int64          `json:"receivable"`      // 应收合计（>0 部分）
	Payable        int64          `json:"payable"`         // 应付合计（>0 部分）
	TodayDocs      int64          `json:"today_docs"`      // 今日单据数（全部类型）
	LowStockCount  int64          `json:"low_stock_count"` // 低于安全库存商品数
	LowStock       []LowStockItem `json:"low_stock"`
	SalesTrend     []TrendPoint   `json:"sales_trend"`     // 近 14 天销售净额（扣退货）
	RecentDocs     []RecentDoc    `json:"recent_docs"`     // 最近 8 张单据
	LocationAlerts []LocationView `json:"location_alerts"` // 库位容量/上下限预警（v0.13）
}

type LowStockItem struct {
	ProductID int64  `json:"product_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Quantity  int64  `json:"quantity"`
	SafeStock int64  `json:"safe_stock"`
}

type TrendPoint struct {
	Date  string `json:"date"` // 2006-01-02
	Sales int64  `json:"sales"`
}

type RecentDoc struct {
	ID          int64  `json:"id"`
	DocNo       string `json:"doc_no"`
	DocType     string `json:"doc_type"`
	PartnerName string `json:"partner_name"`
	TotalAmount int64  `json:"total_amount"`
	Status      string `json:"status"`
	DocDate     string `json:"doc_date"`
}

func (s *ReportService) Dashboard(ctx context.Context) (*DashboardStats, error) {
	st := &DashboardStats{LowStock: []LowStockItem{}}
	today := time.Now().Format("2006-01-02")

	var docs []model.Document
	if err := s.db(ctx).Preload("Items").
		Where("doc_type IN ? AND status = ?", []string{"sales_out", "sales_return"}, "confirmed").
		Where("doc_date >= ? AND doc_date < ?", today, today+" 23:59:59").
		Find(&docs).Error; err != nil {
		return nil, err
	}
	for _, d := range docs {
		sign := int64(1)
		if d.DocType == "sales_return" {
			sign = -1
		}
		st.TodaySales += sign * d.TotalAmount
		for _, it := range d.Items {
			st.TodayProfit += sign * (it.Amount - it.Cost*it.Quantity/1000)
		}
	}

	// 库存总值 + 安全库存预警
	type invRow struct {
		ProductID int64
		Quantity  int64
		TotalVal  int64
		SafeStock int64
		Code      string
		Name      string
	}
	var invs []invRow
	err := s.db(ctx).Table("inventories").
		Select("inventories.product_id, inventories.quantity, inventories.total_value as total_val, products.safe_stock, products.code, products.name").
		Joins("JOIN products ON products.id = inventories.product_id").
		Where("inventories.quantity > 0 OR products.safe_stock > 0").
		Scan(&invs).Error
	if err != nil {
		return nil, err
	}
	for _, iv := range invs {
		st.InventoryVal += iv.TotalVal
		if iv.SafeStock > 0 && iv.Quantity < iv.SafeStock {
			st.LowStockCount++
			st.LowStock = append(st.LowStock, LowStockItem{
				ProductID: iv.ProductID, Code: iv.Code, Name: iv.Name,
				Quantity: iv.Quantity, SafeStock: iv.SafeStock,
			})
		}
	}

	// 应收/应付合计
	fin := &FinanceService{conn: s.db(ctx)}
	if balances, err := fin.Balances(ctx, "customer"); err == nil {
		for _, b := range balances {
			if b.Balance > 0 {
				st.Receivable += b.Balance
			}
		}
	}
	if balances, err := fin.Balances(ctx, "supplier"); err == nil {
		for _, b := range balances {
			if b.Balance > 0 {
				st.Payable += b.Balance
			}
		}
	}

	// 今日单据数（全部类型）
	s.db(ctx).Model(&model.Document{}).
		Where("doc_date >= ? AND doc_date < ?", today, today+" 23:59:59").
		Count(&st.TodayDocs)

	// 近 14 天销售净额（按日聚合，扣退货；补零保证连续曲线）
	st.SalesTrend = []TrendPoint{}
	type trendRow struct {
		Day    string
		Amount int64
	}
	var trs []trendRow
	start := time.Now().AddDate(0, 0, -13).Format("2006-01-02")
	s.db(ctx).Model(&model.Document{}).
		Select("date(doc_date) as day, SUM(CASE WHEN doc_type = 'sales_out' THEN total_amount ELSE -total_amount END) as amount").
		Where("doc_type IN ? AND status = ?", []string{"sales_out", "sales_return"}, "confirmed").
		Where("doc_date >= ?", start).
		Group("day").Scan(&trs)
	trendMap := map[string]int64{}
	for _, t := range trs {
		trendMap[t.Day] = t.Amount
	}
	for i := 13; i >= 0; i-- {
		d := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		st.SalesTrend = append(st.SalesTrend, TrendPoint{Date: d, Sales: trendMap[d]})
	}

	// 最近 8 张单据（含草稿/作废，展示整体动态）
	var recent []model.Document
	if err := s.db(ctx).Where("1 = 1").
		Order("doc_date desc, id desc").Limit(8).
		Find(&recent).Error; err == nil {
		st.RecentDocs = make([]RecentDoc, 0, len(recent))
		for _, d := range recent {
			st.RecentDocs = append(st.RecentDocs, RecentDoc{
				ID: d.ID, DocNo: d.DocNo, DocType: d.DocType, PartnerName: d.PartnerName,
				TotalAmount: d.TotalAmount, Status: d.Status,
				DocDate: d.DocDate.Format("2006-01-02"),
			})
		}
	}
	// 库位容量/上下限预警（v0.13）：货位满了/超出要提前知道，别等放不下才发现
	st.LocationAlerts = []LocationView{}
	if alerts, err := (&LocationService{conn: s.db(ctx)}).Alerts(ctx); err == nil {
		st.LocationAlerts = alerts
	}

	return st, nil
}
