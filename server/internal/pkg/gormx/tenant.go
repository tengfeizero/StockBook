// Package gormx 提供多租户基础设施。
//
// 约定：所有业务表含 tenant_id（0 = 默认租户/单机版）。
// 通过 context 传递当前租户：WithTenant(ctx, id) → GORM 插件自动
// 在 Create 时填充 tenant_id、在 Query/Update/Delete 时追加 tenant_id = ? 条件。
// context 中无租户信息（如系统内部任务）时完全不干预，行为与传统单库一致。
package gormx

import (
	"context"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ctxKey struct{}

// WithTenant 返回携带租户 ID 的新 context
func WithTenant(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, ctxKey{}, tenantID)
}

// TenantFrom 读取租户 ID；无信息时返回 0（false 表示 ctx 未携带）
func TenantFrom(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	v, ok := ctx.Value(ctxKey{}).(int64)
	return v, ok
}

// TenantPlugin GORM 插件：按 Statement.Context 自动做租户隔离
type TenantPlugin struct{}

func (TenantPlugin) Name() string { return "jxc:tenant" }

func (p TenantPlugin) Initialize(db *gorm.DB) error {
	hasField := func(tx *gorm.DB) bool {
		s := tx.Statement
		return s != nil && s.Schema != nil && s.Schema.FieldsByDBName["tenant_id"] != nil
	}
	fill := func(tx *gorm.DB) {
		tid, ok := TenantFrom(tx.Statement.Context)
		if !ok || tid == 0 || !hasField(tx) {
			return
		}
		s := tx.Statement
		f := s.Schema.FieldsByDBName["tenant_id"]
		setOne := func(v reflect.Value) {
			if v.Kind() == reflect.Ptr {
				if v.IsNil() {
					return
				}
				v = v.Elem()
			}
			if v.Kind() != reflect.Struct {
				return
			}
			if cur, zero := f.ValueOf(s.Context, v); zero || cur == int64(0) {
				_ = f.Set(s.Context, v, tid)
			}
		}
		// 批量插入时 ReflectValue 是切片，需逐元素填充（直接对 slice 取 Field 会 panic）
		switch s.ReflectValue.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < s.ReflectValue.Len(); i++ {
				setOne(s.ReflectValue.Index(i))
			}
		default:
			setOne(s.ReflectValue)
		}
	}
	scope := func(tx *gorm.DB) {
		tid, ok := TenantFrom(tx.Statement.Context)
		if !ok || !hasField(tx) {
			return
		}
		tx.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "tenant_id"}, Value: tid},
		}})
	}
	for _, cb := range []struct {
		name string
		fn   func(*gorm.DB)
	}{
		{"jxc:tenant_fill", fill},
		{"jxc:tenant_scope_query", scope},
	} {
		if err := db.Callback().Create().Before("gorm:create").Register(cb.name+"_create", cb.fn); err != nil {
			return err
		}
		if err := db.Callback().Query().Before("gorm:query").Register(cb.name+"_query", scope); err != nil {
			return err
		}
		if err := db.Callback().Update().Before("gorm:update").Register(cb.name+"_update", scope); err != nil {
			return err
		}
		if err := db.Callback().Delete().Before("gorm:delete").Register(cb.name+"_delete", scope); err != nil {
			return err
		}
	}
	return nil
}
