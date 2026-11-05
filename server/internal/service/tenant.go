package service

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
	"jxc/server/internal/pkg/gormx"
)

// TenantService SaaS 租户：注册开户、套餐订阅、账单与额度。
// 租户表本身不带 tenant_id（平台级），业务表靠 ctx 携带的租户隔离。
type TenantService struct{ conn *gorm.DB }

func NewTenantService(db *gorm.DB) *TenantService { return &TenantService{conn: db} }

func (s *TenantService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

// TrialDays 试用天数
const TrialDays = 14

var tenantCodeRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,29}$`)

type RegisterReq struct {
	Name     string `json:"name" binding:"required"` // 企业名称
	Code     string `json:"code"`                    // 企业编码（登录用），留空自动生成
	Contact  string `json:"contact"`
	Phone    string `json:"phone"`
	Username string `json:"username" binding:"required"` // 管理员账号
	Password string `json:"password" binding:"required"`
	Template string `json:"template"` // 可选：注册即带货位与演示数据（hardware/convenience/fresh）
}

type RegisterResult struct {
	Tenant *model.Tenant `json:"tenant"`
	User   *model.User   `json:"user"`
}

// Register 开户：建租户 → 建角色 → 建管理员 → 初始化设置（可选写入演示数据）
func (s *TenantService) Register(ctx context.Context, req RegisterReq) (*RegisterResult, error) {
	name := strings.TrimSpace(req.Name)
	username := strings.TrimSpace(req.Username)
	if name == "" || username == "" || len(req.Password) < 6 {
		return nil, errcode.New(1001, "企业名称、管理员账号必填，密码至少 6 位")
	}
	code := strings.ToLower(strings.TrimSpace(req.Code))
	if code == "" {
		code = fmt.Sprintf("t%d%03d", time.Now().Year()%100, rand.Intn(1000))
	}
	if !tenantCodeRe.MatchString(code) {
		return nil, errcode.New(1001, "企业编码需 3-30 位小写字母/数字/短横线，且以字母或数字开头")
	}
	var dup int64
	// 平台级唯一：不能用租户上下文，显式按 code 查
	if err := s.conn.Model(&model.Tenant{}).Where("code = ?", code).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, errcode.ErrTenantCodeUsed
	}

	now := time.Now()
	expire := now.AddDate(0, 0, TrialDays)
	var plan model.Plan
	if err := s.conn.Where("code = ?", "trial").First(&plan).Error; err != nil {
		plan = model.DefaultPlans[0]
	}
	t := model.Tenant{
		Code: code, Name: name, Contact: req.Contact, Phone: req.Phone,
		PlanCode: "trial", Status: "active",
		TrialEnds: &expire, ExpireAt: &expire,
		MaxUsers: plan.MaxUsers,
	}
	// 开户过程放在一个事务里：中途失败（重名/密码哈希失败等）不留半截租户
	tctx := gormx.WithTenant(ctx, 0)
	var u model.User
	err := s.conn.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		// 之后的写操作切到新租户上下文，避免污染 0 号租户
		tctx = gormx.WithTenant(ctx, t.ID)
		tdb := tx.WithContext(tctx)

		roles := []model.Role{
			{Name: "管理员", Remark: "全部权限", Permissions: `["*"]`},
			{Name: "店员", Remark: "开单、盘点、看报表", Permissions: `["dashboard","sales","stock","reports"]`},
		}
		for i := range roles {
			roles[i].TenantID = t.ID
		}
		if err := tdb.Create(&roles).Error; err != nil {
			return err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u = model.User{
			Username: username, PasswordHash: string(hash),
			DisplayName: "管理员", RoleID: roles[0].ID, Status: "active",
		}
		u.TenantID = t.ID
		if err := tdb.Create(&u).Error; err != nil {
			return err
		}
		// 租户默认设置：店铺抬头用企业名称，试用起始时间作为授权计时起点
		for k, v := range map[string]string{
			"shop_name":                name,
			"shop_phone":               req.Phone,
			"first_run_at":             now.Format(time.RFC3339),
			"ticket_footer":            "谢谢惠顾，欢迎再次光临",
			"stocktake_review_enabled": "false",
		} {
			if err := model.SetSetting(tdb, k, v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 可选：注册即带行业演示数据（试用体验用）
	if req.Template != "" {
		demo := &DemoService{conn: s.conn}
		if _, err := demo.Reset(tctx, u.ID, req.Template); err != nil {
			return nil, err
		}
	}
	return &RegisterResult{Tenant: &t, User: &u}, nil
}

// ResolveByCode 按企业编码取租户（平台级查询，不隔离）
func (s *TenantService) ResolveByCode(code string) (*model.Tenant, error) {
	var t model.Tenant
	if err := s.conn.Where("code = ?", strings.ToLower(strings.TrimSpace(code))).First(&t).Error; err != nil {
		return nil, errcode.ErrTenantNotFound
	}
	return &t, nil
}

// Get 取当前租户
func (s *TenantService) Get(id int64) (*model.Tenant, error) {
	var t model.Tenant
	if err := s.conn.First(&t, id).Error; err != nil {
		return nil, errcode.ErrTenantNotFound
	}
	return &t, nil
}

type TenantView struct {
	ID           int64      `json:"id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Contact      string     `json:"contact"`
	Phone        string     `json:"phone"`
	Status       string     `json:"status"`
	PlanCode     string     `json:"plan_code"`
	PlanName     string     `json:"plan_name"`
	PriceMonthly int64      `json:"price_monthly"`
	Trial        bool       `json:"trial"`
	ExpireAt     *time.Time `json:"expire_at"`
	DaysLeft     int        `json:"days_left"`
	Expired      bool       `json:"expired"`
	MaxUsers     int        `json:"max_users"`
	UserCount    int64      `json:"user_count"`
	ProductCount int64      `json:"product_count"`
	MaxProducts  int        `json:"max_products"`
}

// Current 当前租户概览（含套餐、到期、用量）
func (s *TenantService) Current(ctx context.Context, tenantID int64) (*TenantView, error) {
	t, err := s.Get(tenantID)
	if err != nil {
		return nil, err
	}
	var plan model.Plan
	_ = s.conn.Where("code = ?", t.PlanCode).First(&plan).Error
	v := &TenantView{
		ID: t.ID, Code: t.Code, Name: t.Name, Contact: t.Contact, Phone: t.Phone,
		Status: t.Status, PlanCode: t.PlanCode, PlanName: plan.Name,
		PriceMonthly: plan.PriceMonthly, MaxProducts: plan.MaxProducts,
		Trial: t.PlanCode == "trial", ExpireAt: t.ExpireAt, MaxUsers: t.MaxUsers,
	}
	if v.PlanName == "" {
		v.PlanName = t.PlanCode
	}
	if t.ExpireAt != nil {
		left := int(math.Ceil(time.Until(*t.ExpireAt).Hours() / 24))
		if left < 0 {
			left = 0
		}
		v.DaysLeft = left
		v.Expired = time.Now().After(*t.ExpireAt)
	}
	// 用量统计走租户上下文
	s.db(ctx).Model(&model.User{}).Where("status = ?", "active").Count(&v.UserCount)
	s.db(ctx).Model(&model.Product{}).Count(&v.ProductCount)
	return v, nil
}

// Plans 套餐列表（平台维护，租户只读）
func (s *TenantService) Plans() ([]model.Plan, error) {
	var list []model.Plan
	if err := s.conn.Where("status = ?", "active").Order("sort asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Bills 本租户账单
func (s *TenantService) Bills(ctx context.Context, tenantID int64) ([]model.TenantBill, error) {
	var list []model.TenantBill
	if err := s.db(gormx.WithTenant(ctx, tenantID)).Order("id desc").Limit(100).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Subscribe 续费/升级：记录账单并把到期时间往后推。
// 当前版本为线下/演示支付（直接置为已支付），接微信/支付宝时在此处改为待支付 + 回调。
func (s *TenantService) Subscribe(ctx context.Context, tenantID int64, planCode string, months int, operator string) (*model.TenantBill, error) {
	if months < 1 {
		months = 1
	}
	if months > 24 {
		months = 24
	}
	var plan model.Plan
	if err := s.conn.Where("code = ? AND status = ?", planCode, "active").First(&plan).Error; err != nil {
		return nil, errcode.ErrPlanNotFound
	}
	t, err := s.Get(tenantID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	start := now
	if t.ExpireAt != nil && t.ExpireAt.After(now) {
		start = *t.ExpireAt
	}
	end := start.AddDate(0, months, 0)
	amount := plan.PriceMonthly * int64(months)

	// 升级/续费后同步套餐额度
	t.PlanCode = plan.Code
	t.MaxUsers = plan.MaxUsers
	t.ExpireAt = &end
	t.Status = "active"
	if t.TrialEnds == nil {
		t.TrialEnds = &now
	}
	if err := s.conn.Save(t).Error; err != nil {
		return nil, err
	}

	bill := model.TenantBill{
		BillNo:   "ZD" + now.Format("20060102150405") + fmt.Sprintf("%02d", rand.Intn(100)),
		PlanCode: plan.Code, PlanName: plan.Name, Months: months, Amount: amount,
		Status: "paid", PeriodStart: start, PeriodEnd: end,
		Operator: operator, Remark: "在线续费（演示支付，已标记为已支付）",
	}
	bill.TenantID = tenantID
	if err := s.db(gormx.WithTenant(ctx, tenantID)).Create(&bill).Error; err != nil {
		return nil, err
	}
	return &bill, nil
}

// SeedTenantIndexes 见 model.MigrateTenantSchema；此方法仅用于保证迁移在租户检查前执行。
func (s *TenantService) SeedTenantIndexes() error { return model.MigrateTenantSchema(s.conn) }
