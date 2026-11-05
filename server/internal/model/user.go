package model

import (
	"time"

	"gorm.io/gorm"
)

// Base 所有表的公共字段（对应 DATA-MODEL.md 约定）
type Base struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	TenantID  int64          `gorm:"index;default:0" json:"tenant_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Role 角色（M4：permissions 为模块码 JSON 数组，"*"=全部）
// 模块码：dashboard/archive/purchase/sales/finance/stock/reports/system
type Role struct {
	Base
	Name        string `gorm:"size:50;index" json:"name"`
	Remark      string `gorm:"size:200" json:"remark"`
	Permissions string `gorm:"size:500;default:\"[\\\"*\\\"]\"" json:"permissions"`
}

// AuditLog 操作日志（M4：中间件自动记录所有变更类请求）
type AuditLog struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	TenantID  int64     `gorm:"index;default:0" json:"tenant_id"`
	UserID    int64     `gorm:"index" json:"user_id"`
	Username  string    `gorm:"size:50" json:"username"`
	Action    string    `gorm:"size:20;index" json:"action"`  // POST/PUT/DELETE/LOGIN
	Target    string    `gorm:"size:200;index" json:"target"` // 请求路径
	Detail    string    `gorm:"size:500" json:"detail"`       // 请求体摘要（脱敏后）
	IP        string    `gorm:"size:50" json:"ip"`
	Result    string    `gorm:"size:20" json:"result"` // ok / fail
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// User 用户
type User struct {
	Base
	Username     string `gorm:"size:50;index" json:"username"`
	PasswordHash string `gorm:"size:100" json:"-"`
	DisplayName  string `gorm:"size:50" json:"display_name"`
	RoleID       int64  `json:"role_id"`
	Status       string `gorm:"size:20;default:active" json:"status"` // active / disabled
}

func (User) TableName() string { return "users" }

func (Role) TableName() string { return "roles" }
