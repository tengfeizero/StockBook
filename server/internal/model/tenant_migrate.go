package model

import (
	"gorm.io/gorm"
)

// tenantIndexes 多租户改造：原先「全库唯一」的业务编码改为「租户内唯一」。
// 不改的话第二个租户建同名商品/用户/单号会直接撞唯一约束。
var tenantIndexes = []struct {
	old, table, cols, name string
}{
	{"idx_users_username", "users", "tenant_id, username", "uk_users_tenant_username"},
	{"idx_roles_name", "roles", "tenant_id, name", "uk_roles_tenant_name"},
	{"idx_products_code", "products", "tenant_id, code", "uk_products_tenant_code"},
	{"idx_suppliers_code", "suppliers", "tenant_id, code", "uk_suppliers_tenant_code"},
	{"idx_customers_code", "customers", "tenant_id, code", "uk_customers_tenant_code"},
	{"idx_warehouses_name", "warehouses", "tenant_id, name", "uk_warehouses_tenant_name"},
	{"idx_documents_doc_no", "documents", "tenant_id, doc_no", "uk_documents_tenant_doc_no"},
	{"idx_payments_doc_no", "payments", "tenant_id, doc_no", "uk_payments_tenant_doc_no"},
}

// MigrateTenantSchema 幂等迁移：
//  1. 业务编码唯一约束下沉到租户维度
//  2. 系统设置从全库一张表改为 (tenant_id, key)，旧数据归入租户 0（单机版）
//
// 必须在 AutoMigrate 之后调用（依赖 tenant_settings 已建表）。
func MigrateTenantSchema(db *gorm.DB) error {
	for _, ix := range tenantIndexes {
		if err := db.Exec("DROP INDEX IF EXISTS " + ix.old).Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS " + ix.name +
			" ON " + ix.table + "(" + ix.cols + ")").Error; err != nil {
			return err
		}
	}

	// 旧 settings 表 → tenant_settings（仅当旧表存在且尚未迁移）
	var legacy int64
	if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='settings'").Scan(&legacy).Error; err != nil {
		return err
	}
	if legacy == 0 {
		return nil
	}
	if err := db.Exec("INSERT OR IGNORE INTO tenant_settings(tenant_id, `key`, value) SELECT 0, `key`, value FROM settings").Error; err != nil {
		return err
	}
	// 备份旧表而不是删除，便于回滚
	return db.Exec("ALTER TABLE settings RENAME TO settings_legacy_v019").Error
}
