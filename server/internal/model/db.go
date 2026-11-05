package model

import (
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动，免 CGO
	"gorm.io/gorm"
)

func InitDB(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	return gorm.Open(sqlite.Open(path), &gorm.Config{})
}

// AutoMigrate 按里程碑逐步追加模型；与 DATA-MODEL.md 保持一致
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Role{},
		&User{},
		&Category{},
		&Product{},
		&ProductPrice{},
		&Supplier{},
		&Customer{},
		&Location{},
		&ScaleRule{},
		&Warehouse{},
		&Inventory{},
		&LocationInventory{},
		&InventoryBatch{},
		&InventoryMovement{},
		&Document{},
		&DocumentItem{},
		&Payment{},
		&AuditLog{},
		&Setting{},
		&Tenant{},
		&Plan{},
		&TenantBill{},
	)
}
