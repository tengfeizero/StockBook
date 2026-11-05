package model

import (
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"jxc/server/internal/pkg/gormx"
)

// TenantOf 取数据库句柄绑定的租户 ID；未绑定时按默认租户 0 处理。
// 设置类数据必须显式按租户读写——不能依赖插件的 WHERE 追加，
// 否则单机版（租户 0）可能读到其它租户的同名键。
func TenantOf(db *gorm.DB) int64 {
	if db == nil || db.Statement == nil {
		return 0
	}
	if tid, ok := gormx.TenantFrom(db.Statement.Context); ok {
		return tid
	}
	return 0
}

// GetIntSetting 读整数设置，缺省用 def
func GetIntSetting(db *gorm.DB, key string, def int) int {
	v := GetSetting(db, key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// GetSetting 读设置，不存在返回空串
func GetSetting(db *gorm.DB, key string) string {
	var s Setting
	if err := db.Where("tenant_id = ? AND `key` = ?", TenantOf(db), key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// GetBoolSetting 读布尔设置，缺省用 def
func GetBoolSetting(db *gorm.DB, key string, def bool) bool {
	v := GetSetting(db, key)
	if v == "" {
		return def
	}
	return v == "true"
}

// SetSetting 写设置（按租户 upsert）
func SetSetting(db *gorm.DB, key, value string) error {
	s := Setting{TenantID: TenantOf(db), Key: key, Value: value}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": value}),
	}).Create(&s).Error
}

// GlobalSetting 读平台级设置（不含租户维度，用于授权/试用等单机场景）
func GlobalSetting(db *gorm.DB, key string) string {
	return GetSetting(db, key)
}
