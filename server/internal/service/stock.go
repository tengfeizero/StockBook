package service

import (
	"context"
	"gorm.io/gorm"

	"jxc/server/internal/model"
)

// StockService M3 库存查询
type StockService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *StockService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewStockService(db *gorm.DB) *StockService { return &StockService{conn: db} }

type StockRow struct {
	ProductID    int64  `json:"product_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	CategoryName string `json:"category_name"`
	Unit         string `json:"unit"`
	CostMethod   string `json:"cost_method"`
	IsBatch      bool   `json:"is_batch"`
	Quantity     int64  `json:"quantity"`    // ×1000
	AvgCost      int64  `json:"avg_cost"`    // 分
	TotalValue   int64  `json:"total_value"` // 分
	SafeStock    int64  `json:"safe_stock"`
}

// List 全量库存（小店量级，全量返回；keyword 过滤编码/名称/条码/PLU）
func (s *StockService) List(ctx context.Context, keyword string) ([]StockRow, error) {
	pq := s.db(ctx).Model(&model.Product{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		pq = pq.Where("code LIKE ? OR name LIKE ? OR barcode LIKE ? OR plu LIKE ?", kw, kw, kw, kw)
	}
	var products []model.Product
	if err := pq.Find(&products).Error; err != nil {
		return nil, err
	}
	var invs []model.Inventory
	if err := s.db(ctx).Find(&invs).Error; err != nil {
		return nil, err
	}
	invMap := map[int64]model.Inventory{}
	for _, iv := range invs {
		invMap[iv.ProductID] = iv
	}
	// 分类名
	var cats []model.Category
	if err := s.db(ctx).Find(&cats).Error; err != nil {
		return nil, err
	}
	catMap := map[int64]string{}
	for _, c := range cats {
		catMap[c.ID] = c.Name
	}

	rows := make([]StockRow, 0, len(products))
	for _, p := range products {
		r := StockRow{
			ProductID: p.ID, Code: p.Code, Name: p.Name, Unit: p.Unit,
			CategoryName: catMap[p.CategoryID], CostMethod: p.CostMethod,
			IsBatch: p.IsBatchEnabled, SafeStock: p.SafeStock,
		}
		if iv, ok := invMap[p.ID]; ok {
			r.Quantity, r.AvgCost, r.TotalValue = iv.Quantity, iv.AvgCost, iv.TotalValue
		}
		rows = append(rows, r)
	}
	return rows, nil
}

type BatchRow struct {
	ID         int64   `json:"id"`
	BatchNo    string  `json:"batch_no"`
	ExpireDate *string `json:"expire_date"`
	Quantity   int64   `json:"quantity"`  // ×1000
	UnitCost   int64   `json:"unit_cost"` // 分
}

// Batches 某商品的批次明细（先进先出顺序）
func (s *StockService) Batches(ctx context.Context, productID int64) ([]BatchRow, error) {
	var bs []model.InventoryBatch
	if err := s.db(ctx).Where("product_id = ? AND quantity_in - quantity_out > 0", productID).
		Order("id ASC").Find(&bs).Error; err != nil {
		return nil, err
	}
	rows := make([]BatchRow, 0, len(bs))
	for _, b := range bs {
		r := BatchRow{ID: b.ID, BatchNo: b.BatchNo, Quantity: b.QuantityIn - b.QuantityOut, UnitCost: b.UnitCost}
		if b.ExpireDate != nil {
			d := b.ExpireDate.Format("2006-01-02")
			r.ExpireDate = &d
		}
		rows = append(rows, r)
	}
	return rows, nil
}
