package service

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

type AuthService struct {
	conn *gorm.DB
}

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *AuthService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewAuthService(db *gorm.DB) *AuthService { return &AuthService{conn: db} }

// Login 登录：企业编码为空 = 单机版（0 号租户）；填写则在该企业内校验账号。
// 直接按 (tenant_id, username) 查询，不依赖 ctx 隔离——登录阶段还没有租户上下文。
func (s *AuthService) Login(ctx context.Context, tenantCode, username, password string) (*model.User, *model.Tenant, error) {
	var tenantID int64
	var tenant *model.Tenant
	code := strings.ToLower(strings.TrimSpace(tenantCode))
	if code != "" {
		var t model.Tenant
		if err := s.conn.Where("code = ?", code).First(&t).Error; err != nil {
			return nil, nil, errcode.ErrTenantNotFound
		}
		if t.Status != "active" {
			return nil, nil, errcode.ErrTenantDisabled
		}
		tenantID, tenant = t.ID, &t
	}

	var u model.User
	err := s.conn.Where("tenant_id = ? AND username = ?", tenantID, strings.TrimSpace(username)).First(&u).Error
	if err != nil {
		return nil, nil, errcode.ErrWrongCreds
	}
	if u.Status != "active" {
		return nil, nil, errcode.ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, nil, errcode.ErrWrongCreds
	}
	return &u, tenant, nil
}

func (s *AuthService) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	if err := s.db(ctx).First(&u, id).Error; err != nil {
		return nil, errcode.ErrNotFound
	}
	return &u, nil
}

// TenantOf 返回用户所属租户（0 号租户返回 nil，表示单机版）
func (s *AuthService) TenantOf(tenantID int64) *model.Tenant {
	if tenantID == 0 {
		return nil
	}
	var t model.Tenant
	if err := s.conn.First(&t, tenantID).Error; err != nil {
		return nil
	}
	return &t
}
