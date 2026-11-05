package model

import (
	"time"

	"gorm.io/gorm"
)

// Warehouse 仓库（单仓模式，默认建"主仓库"）
type Warehouse struct {
	Base
	Name string `gorm:"size:50;index" json:"name"`
}

func (Warehouse) TableName() string { return "warehouses" }

// Inventory 库存汇总（product × warehouse 一行，对应 DATA-MODEL.md）
type Inventory struct {
	Base
	ProductID   int64 `gorm:"uniqueIndex:idx_inv_prod_wh" json:"product_id"`
	WarehouseID int64 `gorm:"uniqueIndex:idx_inv_prod_wh" json:"warehouse_id"`
	Quantity    int64 `gorm:"default:0" json:"quantity"`    // ×1000
	AvgCost     int64 `gorm:"default:0" json:"avg_cost"`    // 分
	TotalValue  int64 `gorm:"default:0" json:"total_value"` // 分
}

func (Inventory) TableName() string { return "inventories" }

// InventoryBatch 批次库存（仅启用批次商品）
type InventoryBatch struct {
	Base
	ProductID      int64      `gorm:"uniqueIndex:idx_batch_prod_wh_no" json:"product_id"`
	WarehouseID    int64      `gorm:"uniqueIndex:idx_batch_prod_wh_no" json:"warehouse_id"`
	BatchNo        string     `gorm:"size:50;uniqueIndex:idx_batch_prod_wh_no" json:"batch_no"`
	ProductionDate *time.Time `json:"production_date"`
	ExpireDate     *time.Time `json:"expire_date"`
	QuantityIn     int64      `gorm:"default:0" json:"quantity_in"`  // ×1000
	QuantityOut    int64      `gorm:"default:0" json:"quantity_out"` // ×1000
	UnitCost       int64      `json:"unit_cost"`                     // 分
}

func (InventoryBatch) TableName() string { return "inventory_batches" }

// LocationInventory 库位库存（product × warehouse × location 一行）
// location_id = 0 表示「未分配」——存量库存升级时整体挂在这里，待用移库单上架。
// 不变式：同一 (product, warehouse) 的库位数量之和 == inventories.quantity。
// 成本仍由 inventories 在仓库级维护，库位只拆数量。
type LocationInventory struct {
	Base
	ProductID   int64 `gorm:"uniqueIndex:idx_locinv_prod_wh_loc" json:"product_id"`
	WarehouseID int64 `gorm:"uniqueIndex:idx_locinv_prod_wh_loc" json:"warehouse_id"`
	LocationID  int64 `gorm:"uniqueIndex:idx_locinv_prod_wh_loc" json:"location_id"` // 0=未分配
	Quantity    int64 `gorm:"default:0" json:"quantity"`                             // ×1000
}

func (LocationInventory) TableName() string { return "location_inventories" }

// InventoryMovement 库存流水（只追加，不修改——对账依据）
type InventoryMovement struct {
	ID             int64          `gorm:"primaryKey" json:"id"`
	TenantID       int64          `gorm:"index;default:0" json:"tenant_id"`
	DocumentID     *int64         `gorm:"index" json:"document_id"`
	DocumentItemID *int64         `json:"document_item_id"`
	ProductID      int64          `gorm:"index" json:"product_id"`
	WarehouseID    int64          `json:"warehouse_id"`
	BatchID        *int64         `json:"batch_id"`
	Direction      string         `gorm:"size:10" json:"direction"`           // in / out
	LocationID     int64          `gorm:"index;default:0" json:"location_id"` // 库位（0=未分配）
	Quantity       int64          `json:"quantity"`                           // 一律正数，×1000
	UnitCost       int64          `json:"unit_cost"`                          // 分
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (InventoryMovement) TableName() string { return "inventory_movements" }

// BackfillLocationStock 存量库存首次回填（v0.12 升级库位数量账时执行一次）
// 库位表为空而汇总表有数据 → 把每个 (商品,仓库) 的现有数量整体挂到「未分配」库位，
// 保证 Σ库位数量 == 仓库总量。货实际在哪个货架系统无从得知，不做任何猜测性摊派。
func BackfillLocationStock(db *gorm.DB) (int, error) {
	var locCount, invCount int64
	if err := db.Model(&LocationInventory{}).Count(&locCount).Error; err != nil {
		return 0, err
	}
	if locCount > 0 {
		return 0, nil
	}
	if err := db.Model(&Inventory{}).Count(&invCount).Error; err != nil {
		return 0, err
	}
	if invCount == 0 {
		return 0, nil
	}
	var invs []Inventory
	if err := db.Find(&invs).Error; err != nil {
		return 0, err
	}
	rows := make([]LocationInventory, 0, len(invs))
	for _, iv := range invs {
		if iv.Quantity == 0 {
			continue
		}
		rows = append(rows, LocationInventory{
			ProductID: iv.ProductID, WarehouseID: iv.WarehouseID, LocationID: 0, Quantity: iv.Quantity,
		})
	}
	if len(rows) == 0 {
		return 0, nil
	}
	if err := db.Create(&rows).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

// GetMainWarehouse 取主仓库，不存在则创建
func GetMainWarehouse(db *gorm.DB) (*Warehouse, error) {
	var w Warehouse
	err := db.Where("name = ?", "主仓库").First(&w).Error
	if err == nil {
		return &w, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	w = Warehouse{Name: "主仓库"}
	if err := db.Create(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}
