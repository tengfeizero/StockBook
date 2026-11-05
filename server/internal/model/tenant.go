package model

import (
	"time"

	"gorm.io/gorm"
)

// Tenant 租户（SaaS 版企业账套）。单机版固定使用 tenant_id = 0，不落该表。
// 注意：Tenant 自身不带 tenant_id 字段，租户插件对它是透明的。
type Tenant struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	Code      string         `gorm:"size:50;uniqueIndex" json:"code"` // 企业编码（登录时填写）
	Name      string         `gorm:"size:100;not null" json:"name"`
	Contact   string         `gorm:"size:50" json:"contact"`
	Phone     string         `gorm:"size:30" json:"phone"`
	PlanCode  string         `gorm:"size:30;default:trial" json:"plan_code"`
	Status    string         `gorm:"size:20;default:active" json:"status"` // active / suspended
	TrialEnds *time.Time     `json:"trial_ends_at"`
	ExpireAt  *time.Time     `json:"expire_at"` // 订阅到期时间（试用期同义）
	MaxUsers  int            `gorm:"default:3" json:"max_users"`
	Remark    string         `gorm:"size:200" json:"remark"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Tenant) TableName() string { return "tenants" }

// Plan 套餐（平台维护，租户只读）
type Plan struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	Code         string    `gorm:"size:30;uniqueIndex" json:"code"`
	Name         string    `gorm:"size:50" json:"name"`
	PriceMonthly int64     `json:"price_monthly"` // 分/月
	MaxUsers     int       `json:"max_users"`     // 0=不限
	MaxProducts  int       `json:"max_products"`  // 0=不限
	Features     string    `gorm:"size:1000" json:"features"`
	Sort         int       `gorm:"default:0" json:"sort"`
	Status       string    `gorm:"size:20;default:active" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Plan) TableName() string { return "plans" }

// TenantBill 订阅账单（租户自助续费/升级的流水，含租户 ID 由插件自动隔离）
type TenantBill struct {
	Base
	BillNo      string    `gorm:"size:40;index" json:"bill_no"`
	PlanCode    string    `gorm:"size:30" json:"plan_code"`
	PlanName    string    `gorm:"size:50" json:"plan_name"`
	Months      int       `json:"months"`
	Amount      int64     `json:"amount"`                             // 分
	Status      string    `gorm:"size:20;default:paid" json:"status"` // paid（当前为线下/演示支付）
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Operator    string    `gorm:"size:50" json:"operator"`
	Remark      string    `gorm:"size:200" json:"remark"`
}

func (TenantBill) TableName() string { return "tenant_bills" }

// 默认套餐（平台初始化时写入）
var DefaultPlans = []Plan{
	{Code: "trial", Name: "试用版", PriceMonthly: 0, MaxUsers: 2, MaxProducts: 100, Sort: 1,
		Features: "14 天全功能试用\n1 个仓库\n2 个用户"},
	{Code: "basic", Name: "标准版", PriceMonthly: 19900, MaxUsers: 5, MaxProducts: 3000, Sort: 2,
		Features: "进销存全流程\n不限单据量\n5 个用户\n邮件工单支持"},
	{Code: "pro", Name: "专业版", PriceMonthly: 49900, MaxUsers: 20, MaxProducts: 0, Sort: 3,
		Features: "含标准版全部功能\n20 个用户\n多仓位/批次/保质期\n报表与盘点审批\n专属客服"},
}

// SeedPlans 首次启动写入套餐（已存在则跳过，不覆盖平台改价）
func SeedPlans(db *gorm.DB) error {
	for _, p := range DefaultPlans {
		var cnt int64
		if err := db.Model(&Plan{}).Where("code = ?", p.Code).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			p := p
			if err := db.Create(&p).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
