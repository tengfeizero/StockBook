package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

type OpeningService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *OpeningService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewOpeningService(db *gorm.DB) *OpeningService { return &OpeningService{conn: db} }

// OpeningStockRow 已建账商品行（列表展示）
type OpeningStockRow struct {
	ProductID      int64  `json:"product_id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Unit           string `json:"unit"`
	IsBatchEnabled bool   `json:"is_batch_enabled"`
	Quantity       int64  `json:"quantity"` // ×1000
	AvgCost        int64  `json:"avg_cost"` // 分
}

// ListStocks 已建账商品列表（小店商品量级小，全量返回）
func (s *OpeningService) ListStocks(ctx context.Context) ([]OpeningStockRow, error) {
	var products []model.Product
	if err := s.db(ctx).Select("id, code, name, unit, is_batch_enabled").Find(&products).Error; err != nil {
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
	rows := make([]OpeningStockRow, 0, len(products))
	for _, p := range products {
		r := OpeningStockRow{ProductID: p.ID, Code: p.Code, Name: p.Name, Unit: p.Unit, IsBatchEnabled: p.IsBatchEnabled}
		if iv, ok := invMap[p.ID]; ok {
			r.Quantity, r.AvgCost = iv.Quantity, iv.AvgCost
		}
		rows = append(rows, r)
	}
	return rows, nil
}

type OpeningReq struct {
	ProductID  int64  `json:"product_id" binding:"required"`
	Quantity   int64  `json:"quantity"`    // ×1000
	UnitCost   int64  `json:"unit_cost"`   // 分
	BatchNo    string `json:"batch_no"`    // 批次商品必填，默认"期初"
	LocationID int64  `json:"location_id"` // 库位（可空=未分配，v0.12 库位数量账）
}

// Save 期初建账：设置库存 + 写入流水（保持对账不变式）+ 批次商品写批次
func (s *OpeningService) Save(ctx context.Context, rows []OpeningReq) error {
	if len(rows) == 0 {
		return errcode.ErrParam
	}
	wh, err := model.GetMainWarehouse(s.db(ctx))
	if err != nil {
		return err
	}
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			var p model.Product
			if err := tx.First(&p, r.ProductID).Error; err != nil {
				return errcode.New(4004, fmt.Sprintf("商品 %d 不存在", r.ProductID))
			}
			var inv model.Inventory
			err := tx.Where("product_id = ? AND warehouse_id = ?", p.ID, wh.ID).First(&inv).Error
			if err == nil && inv.Quantity != 0 {
				return errcode.New(4005, fmt.Sprintf("商品「%s」已建账，库存调整请用其他出入库单", p.Name))
			}
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			inv.ProductID, inv.WarehouseID = p.ID, wh.ID
			inv.Quantity, inv.AvgCost = r.Quantity, r.UnitCost
			inv.TotalValue = r.Quantity * r.UnitCost / 1000 // 数量3位小数，先乘后除避免截断
			if err := tx.Save(&inv).Error; err != nil {
				return err
			}

			var batchID *int64
			if p.IsBatchEnabled {
				no := r.BatchNo
				if no == "" {
					no = "期初"
				}
				var b model.InventoryBatch
				err := tx.Where("product_id = ? AND warehouse_id = ? AND batch_no = ?", p.ID, wh.ID, no).First(&b).Error
				if err == gorm.ErrRecordNotFound {
					b = model.InventoryBatch{ProductID: p.ID, WarehouseID: wh.ID, BatchNo: no}
					if err := tx.Create(&b).Error; err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
				b.QuantityIn, b.UnitCost = r.Quantity, r.UnitCost
				if err := tx.Save(&b).Error; err != nil {
					return err
				}
				batchID = &b.ID
			}

			// 库位数量账：期初一并登记库位（未选则为未分配）
			if r.Quantity > 0 {
				if err := changeLocQty(tx, p.ID, wh.ID, r.LocationID, r.Quantity, false); err != nil {
					return err
				}
			}

			m := model.InventoryMovement{
				ProductID: p.ID, WarehouseID: wh.ID, BatchID: batchID,
				LocationID: r.LocationID,
				Direction:  "in", Quantity: r.Quantity, UnitCost: r.UnitCost,
			}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
