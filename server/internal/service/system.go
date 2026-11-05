package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// SystemService M4：用户/角色管理、改密、系统设置、数据库备份
type SystemService struct {
	conn   *gorm.DB
	dbPath string
}

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *SystemService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewSystemService(db *gorm.DB, dbPath string) *SystemService {
	return &SystemService{conn: db, dbPath: dbPath}
}

// ---------- 用户 ----------

type UserRow struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	RoleID      int64     `json:"role_id"`
	RoleName    string    `json:"role_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func (s *SystemService) ListUsers(ctx context.Context) ([]UserRow, error) {
	var users []model.User
	if err := s.db(ctx).Find(&users).Error; err != nil {
		return nil, err
	}
	var roles []model.Role
	if err := s.db(ctx).Find(&roles).Error; err != nil {
		return nil, err
	}
	roleMap := map[int64]string{}
	for _, r := range roles {
		roleMap[r.ID] = r.Name
	}
	rows := make([]UserRow, 0, len(users))
	for _, u := range users {
		rows = append(rows, UserRow{
			ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
			RoleID: u.RoleID, RoleName: roleMap[u.RoleID], Status: u.Status,
			CreatedAt: u.CreatedAt,
		})
	}
	return rows, nil
}

type UserCreateReq struct {
	Username    string `json:"username" binding:"required,min=2,max=50"`
	Password    string `json:"password" binding:"required,min=6"`
	DisplayName string `json:"display_name" binding:"required"`
	RoleID      int64  `json:"role_id" binding:"required"`
}

func (s *SystemService) CreateUser(ctx context.Context, req UserCreateReq) error {
	// 套餐用户数上限（SaaS 租户；单机版不限）
	if tid := model.TenantOf(s.db(ctx)); tid > 0 {
		var t model.Tenant
		if err := s.conn.First(&t, tid).Error; err == nil && t.MaxUsers > 0 {
			var used int64
			s.db(ctx).Model(&model.User{}).Count(&used)
			if int(used) >= t.MaxUsers {
				return errcode.ErrUserQuota
			}
		}
	}
	var count int64
	s.db(ctx).Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		return errcode.New(4001, "用户名已存在")
	}
	var role model.Role
	if err := s.db(ctx).First(&role, req.RoleID).Error; err != nil {
		return errcode.New(4004, "角色不存在")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db(ctx).Create(&model.User{
		Username: req.Username, PasswordHash: string(hash),
		DisplayName: req.DisplayName, RoleID: req.RoleID, Status: "active",
	}).Error
}

type UserUpdateReq struct {
	DisplayName string `json:"display_name"`
	RoleID      int64  `json:"role_id"`
	Status      string `json:"status"` // active / disabled
}

func (s *SystemService) UpdateUser(ctx context.Context, id, operatorID int64, req UserUpdateReq) error {
	var u model.User
	if err := s.db(ctx).First(&u, id).Error; err != nil {
		return errcode.ErrNotFound
	}
	// 管理员自我保护：不能停用/降级自己，避免把自己锁在门外
	if id == operatorID && (req.Status == "disabled" || (req.RoleID > 0 && req.RoleID != u.RoleID)) {
		return errcode.New(1003, "不能停用或变更自己的账号/角色")
	}
	updates := map[string]any{}
	if req.DisplayName != "" {
		updates["display_name"] = req.DisplayName
	}
	if req.RoleID > 0 {
		var role model.Role
		if err := s.db(ctx).First(&role, req.RoleID).Error; err != nil {
			return errcode.New(4004, "角色不存在")
		}
		updates["role_id"] = req.RoleID
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	return s.db(ctx).Model(&u).Updates(updates).Error
}

func (s *SystemService) ResetPassword(ctx context.Context, id int64, password string) error {
	if len(password) < 6 {
		return errcode.New(1001, "密码至少6位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db(ctx).Model(&model.User{}).Where("id = ?", id).Update("password_hash", string(hash)).Error
}

// ---------- 角色 ----------

type RoleRow struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Remark      string   `json:"remark"`
	Permissions []string `json:"permissions"`
	UserCount   int64    `json:"user_count"`
}

func (s *SystemService) ListRoles(ctx context.Context) ([]RoleRow, error) {
	var roles []model.Role
	if err := s.db(ctx).Find(&roles).Error; err != nil {
		return nil, err
	}
	type uc struct {
		RoleID int64
		Cnt    int64
	}
	var counts []uc
	s.db(ctx).Model(&model.User{}).Select("role_id, COUNT(*) as cnt").Group("role_id").Scan(&counts)
	countMap := map[int64]int64{}
	for _, c := range counts {
		countMap[c.RoleID] = c.Cnt
	}
	rows := make([]RoleRow, 0, len(roles))
	for _, r := range roles {
		perms := []string{}
		_ = json.Unmarshal([]byte(r.Permissions), &perms)
		rows = append(rows, RoleRow{
			ID: r.ID, Name: r.Name, Remark: r.Remark,
			Permissions: perms, UserCount: countMap[r.ID],
		})
	}
	return rows, nil
}

type RoleSaveReq struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name" binding:"required"`
	Remark      string   `json:"remark"`
	Permissions []string `json:"permissions" binding:"required"`
}

var validPerms = map[string]bool{
	"dashboard": true, "archive": true, "purchase": true, "sales": true,
	"finance": true, "stock": true, "reports": true, "system": true, "*": true,
}

func (s *SystemService) SaveRole(ctx context.Context, req RoleSaveReq) error {
	perms := []string{}
	for _, p := range req.Permissions {
		if !validPerms[p] {
			return errcode.New(1001, "未知权限码: "+p)
		}
		perms = append(perms, p)
	}
	if len(perms) == 0 {
		return errcode.New(1001, "至少选择一个权限")
	}
	raw, _ := json.Marshal(perms)
	if req.ID > 0 {
		var r model.Role
		if err := s.db(ctx).First(&r, req.ID).Error; err != nil {
			return errcode.ErrNotFound
		}
		r.Name, r.Remark, r.Permissions = req.Name, req.Remark, string(raw)
		return s.db(ctx).Save(&r).Error
	}
	var count int64
	s.db(ctx).Model(&model.Role{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		return errcode.New(4001, "角色名已存在")
	}
	return s.db(ctx).Create(&model.Role{Name: req.Name, Remark: req.Remark, Permissions: string(raw)}).Error
}

func (s *SystemService) DeleteRole(ctx context.Context, id int64) error {
	var count int64
	s.db(ctx).Model(&model.User{}).Where("role_id = ?", id).Count(&count)
	if count > 0 {
		return errcode.New(1003, "该角色下还有用户，不能删除")
	}
	return s.db(ctx).Delete(&model.Role{}, id).Error
}

// ---------- 改密 ----------

func (s *SystemService) ChangePassword(ctx context.Context, userID int64, oldPwd, newPwd string) error {
	var u model.User
	if err := s.db(ctx).First(&u, userID).Error; err != nil {
		return errcode.ErrNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(oldPwd)); err != nil {
		return errcode.ErrWrongCreds
	}
	if len(newPwd) < 6 {
		return errcode.New(1001, "新密码至少6位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db(ctx).Model(&u).Update("password_hash", string(hash)).Error
}

// ---------- 系统设置 ----------

type SettingsOut struct {
	AllowNegativeStock    bool `json:"allow_negative_stock"`
	ShelfLifeWarnDays     int  `json:"shelf_life_warn_days"`
	LocationBlockOverflow bool `json:"location_block_overflow"` // 库位超容时是否禁止入库/移入
	// 小票抬头（v0.14）：58mm 热敏小票打印时使用
	ShopName     string `json:"shop_name"`
	ShopPhone    string `json:"shop_phone"`
	ShopAddr     string `json:"shop_addr"`
	TicketFooter string `json:"ticket_footer"`
	// 盘点差异审批（v0.16）：差异超阈值的盘点单需主管确认后才调账
	StocktakeReviewEnabled bool `json:"stocktake_review_enabled"`
	StocktakeReviewPercent int  `json:"stocktake_review_percent"` // 差异率阈值（%，账面为 0 的行不参与）
	StocktakeReviewAmount  int  `json:"stocktake_review_amount"`  // 差异金额阈值（分），0=不按金额
	// 打印模板（v0.21）：Logo 与字段显隐（true=打印该字段）
	PrintLogo         string `json:"print_logo"`          // data URL（data:image/...），空=不打印
	TicketShowPrice   bool   `json:"ticket_show_price"`   // 小票：单价列
	TicketShowAmount  bool   `json:"ticket_show_amount"`  // 小票：金额列与合计（送货单可关）
	TicketShowPartner bool   `json:"ticket_show_partner"` // 小票：客户/供应商名
	TicketShowBatch   bool   `json:"ticket_show_batch"`   // 小票：批次号
	A4ShowPrice       bool   `json:"a4_show_price"`       // A4：单价列
	A4ShowAmount      bool   `json:"a4_show_amount"`      // A4：金额列与大写金额
	A4ShowSigns       bool   `json:"a4_show_signs"`       // A4：底部落款签字行
}

func (s *SystemService) GetSettings(ctx context.Context) *SettingsOut {
	return &SettingsOut{
		AllowNegativeStock:     model.GetBoolSetting(s.db(ctx), "allow_negative_stock", false),
		ShelfLifeWarnDays:      model.GetIntSetting(s.db(ctx), "shelf_life_warn_days", 7),
		LocationBlockOverflow:  model.GetBoolSetting(s.db(ctx), "location_block_overflow", false),
		ShopName:               model.GetSetting(s.db(ctx), "shop_name"),
		ShopPhone:              model.GetSetting(s.db(ctx), "shop_phone"),
		ShopAddr:               model.GetSetting(s.db(ctx), "shop_addr"),
		TicketFooter:           model.GetSetting(s.db(ctx), "ticket_footer"),
		StocktakeReviewEnabled: model.GetBoolSetting(s.db(ctx), "stocktake_review_enabled", false),
		StocktakeReviewPercent: model.GetIntSetting(s.db(ctx), "stocktake_review_percent", 10),
		StocktakeReviewAmount:  model.GetIntSetting(s.db(ctx), "stocktake_review_amount", 0),
		PrintLogo:              model.GetSetting(s.db(ctx), "print_logo"),
		TicketShowPrice:        model.GetBoolSetting(s.db(ctx), "ticket_show_price", true),
		TicketShowAmount:       model.GetBoolSetting(s.db(ctx), "ticket_show_amount", true),
		TicketShowPartner:      model.GetBoolSetting(s.db(ctx), "ticket_show_partner", true),
		TicketShowBatch:        model.GetBoolSetting(s.db(ctx), "ticket_show_batch", true),
		A4ShowPrice:            model.GetBoolSetting(s.db(ctx), "a4_show_price", true),
		A4ShowAmount:           model.GetBoolSetting(s.db(ctx), "a4_show_amount", true),
		A4ShowSigns:            model.GetBoolSetting(s.db(ctx), "a4_show_signs", true),
	}
}

type SettingsReq struct {
	AllowNegativeStock     *bool   `json:"allow_negative_stock"`
	ShelfLifeWarnDays      *int    `json:"shelf_life_warn_days"`
	LocationBlockOverflow  *bool   `json:"location_block_overflow"`
	ShopName               *string `json:"shop_name"`
	ShopPhone              *string `json:"shop_phone"`
	ShopAddr               *string `json:"shop_addr"`
	TicketFooter           *string `json:"ticket_footer"`
	StocktakeReviewEnabled *bool   `json:"stocktake_review_enabled"`
	StocktakeReviewPercent *int    `json:"stocktake_review_percent"`
	StocktakeReviewAmount  *int    `json:"stocktake_review_amount"`
	// 打印模板（v0.21）
	PrintLogo         *string `json:"print_logo"`
	TicketShowPrice   *bool   `json:"ticket_show_price"`
	TicketShowAmount  *bool   `json:"ticket_show_amount"`
	TicketShowPartner *bool   `json:"ticket_show_partner"`
	TicketShowBatch   *bool   `json:"ticket_show_batch"`
	A4ShowPrice       *bool   `json:"a4_show_price"`
	A4ShowAmount      *bool   `json:"a4_show_amount"`
	A4ShowSigns       *bool   `json:"a4_show_signs"`
}

func (s *SystemService) SaveSettings(ctx context.Context, req SettingsReq) error {
	if req.AllowNegativeStock != nil {
		v := "false"
		if *req.AllowNegativeStock {
			v = "true"
		}
		if err := model.SetSetting(s.db(ctx), "allow_negative_stock", v); err != nil {
			return err
		}
	}
	if req.LocationBlockOverflow != nil {
		v := "false"
		if *req.LocationBlockOverflow {
			v = "true"
		}
		if err := model.SetSetting(s.db(ctx), "location_block_overflow", v); err != nil {
			return err
		}
	}
	if req.ShelfLifeWarnDays != nil && *req.ShelfLifeWarnDays >= 0 {
		if err := model.SetSetting(s.db(ctx), "shelf_life_warn_days", fmt.Sprintf("%d", *req.ShelfLifeWarnDays)); err != nil {
			return err
		}
	}
	// 盘点差异审批阈值
	if req.StocktakeReviewEnabled != nil {
		v := "false"
		if *req.StocktakeReviewEnabled {
			v = "true"
		}
		if err := model.SetSetting(s.db(ctx), "stocktake_review_enabled", v); err != nil {
			return err
		}
	}
	if req.StocktakeReviewPercent != nil && *req.StocktakeReviewPercent >= 0 && *req.StocktakeReviewPercent <= 100 {
		if err := model.SetSetting(s.db(ctx), "stocktake_review_percent", fmt.Sprintf("%d", *req.StocktakeReviewPercent)); err != nil {
			return err
		}
	}
	if req.StocktakeReviewAmount != nil && *req.StocktakeReviewAmount >= 0 {
		if err := model.SetSetting(s.db(ctx), "stocktake_review_amount", fmt.Sprintf("%d", *req.StocktakeReviewAmount)); err != nil {
			return err
		}
	}
	// 打印模板（v0.21）：Logo 与字段显隐
	if req.PrintLogo != nil {
		v := strings.TrimSpace(*req.PrintLogo)
		if v != "" && !strings.HasPrefix(v, "data:image/") {
			return errcode.New(1001, "Logo 必须是图片 data URL")
		}
		if len(v) > 300000 { // base64 后约 220KB，足够 240px 宽的压缩图
			return errcode.New(1001, "Logo 图片过大，请使用 240px 宽以内的小图")
		}
		if err := model.SetSetting(s.db(ctx), "print_logo", v); err != nil {
			return err
		}
	}
	for key, v := range map[string]*bool{
		"ticket_show_price":   req.TicketShowPrice,
		"ticket_show_amount":  req.TicketShowAmount,
		"ticket_show_partner": req.TicketShowPartner,
		"ticket_show_batch":   req.TicketShowBatch,
		"a4_show_price":       req.A4ShowPrice,
		"a4_show_amount":      req.A4ShowAmount,
		"a4_show_signs":       req.A4ShowSigns,
	} {
		if v == nil {
			continue
		}
		b := "false"
		if *v {
			b = "true"
		}
		if err := model.SetSetting(s.db(ctx), key, b); err != nil {
			return err
		}
	}
	// 小票抬头：空字符串也允许写（表示不打印该行）
	for key, v := range map[string]*string{
		"shop_name":     req.ShopName,
		"shop_phone":    req.ShopPhone,
		"shop_addr":     req.ShopAddr,
		"ticket_footer": req.TicketFooter,
	} {
		if v == nil {
			continue
		}
		if len([]rune(*v)) > 60 {
			return errcode.New(1001, "小票抬头字段不超过 60 字")
		}
		if err := model.SetSetting(s.db(ctx), key, *v); err != nil {
			return err
		}
	}
	return nil
}

// ---------- 数据库备份 ----------

type BackupFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Time string `json:"time"`
}

func backupDir() string { return "data/backups" }

// Backup 用 SQLite VACUUM INTO 生成一致性快照
func (s *SystemService) Backup(ctx context.Context) (*BackupFile, error) {
	if err := os.MkdirAll(backupDir(), 0755); err != nil {
		return nil, err
	}
	name := "huoben-" + time.Now().Format("20060102-150405") + ".db"
	target := filepath.Join(backupDir(), name)
	if err := s.db(ctx).Exec("VACUUM INTO ?", target).Error; err != nil {
		return nil, errcode.New(5000, "备份失败: "+err.Error())
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	return &BackupFile{Name: name, Size: info.Size(), Time: info.ModTime().Format("2006-01-02 15:04:05")}, nil
}

// AutoBackup 每天首次启动时自动备份一次
func (s *SystemService) AutoBackup(ctx context.Context) {
	today := "huoben-" + time.Now().Format("20060102")
	entries, _ := os.ReadDir(backupDir())
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), today) {
			return // 今天已有备份
		}
	}
	if b, err := s.Backup(ctx); err == nil {
		fmt.Println("自动备份完成:", b.Name)
	}
}

func (s *SystemService) ListBackups(ctx context.Context) ([]BackupFile, error) {
	entries, err := os.ReadDir(backupDir())
	if err != nil {
		return []BackupFile{}, nil // 尚无备份目录
	}
	rows := []BackupFile{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rows = append(rows, BackupFile{
			Name: e.Name(), Size: info.Size(),
			Time: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name > rows[j].Name })
	return rows, nil
}

// ---------- 授权（M5） ----------

// FirstRun 首次启动时间（试用期起算点），首次调用时写入 settings
func (s *SystemService) FirstRun(ctx context.Context) time.Time {
	v := model.GetSetting(s.db(ctx), "first_run_at")
	if v == "" {
		t := time.Now()
		_ = model.SetSetting(s.db(ctx), "first_run_at", t.Format(time.RFC3339))
		return t
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t
	}
	return time.Now()
}

// ---------- 操作日志 ----------

type AuditListReq struct {
	Username string
	Action   string
	Page     int
	PageSize int
}

func (s *SystemService) ListAuditLogs(ctx context.Context, req AuditListReq) ([]model.AuditLog, int64, error) {
	q := s.db(ctx).Model(&model.AuditLog{})
	if req.Username != "" {
		q = q.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Action != "" {
		q = q.Where("action = ?", req.Action)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.AuditLog
	err := q.Order("id DESC").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&list).Error
	return list, total, err
}
