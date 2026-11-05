package repo

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"jxc/server/internal/model"
)

func FindUserByUsername(db *gorm.DB, username string) (*model.User, error) {
	var u model.User
	err := db.Where("username = ?", username).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// SeedAdmin 首次启动时创建默认角色（管理员/店员）和管理员 admin/admin123
func SeedAdmin(db *gorm.DB) error {
	var roleCount int64
	if err := db.Model(&model.Role{}).Count(&roleCount).Error; err != nil {
		return err
	}
	if roleCount == 0 {
		roles := []model.Role{
			{Name: "管理员", Remark: "全部权限", Permissions: `["*"]`},
			{Name: "店员", Remark: "开单、盘点、看报表", Permissions: `["dashboard","sales","stock","reports"]`},
		}
		if err := db.Create(&roles).Error; err != nil {
			return err
		}
	}
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		var adminRole model.Role
		if err := db.Where("name = ?", "管理员").First(&adminRole).Error; err != nil {
			return err
		}
		return db.Create(&model.User{
			Username:     "admin",
			PasswordHash: string(hash),
			DisplayName:  "管理员",
			RoleID:       adminRole.ID,
		}).Error
	}
	// 兼容升级：早期版本用户没有角色 → 统一挂"管理员"（只有单用户时安全）
	var orphan []model.User
	if err := db.Where("role_id = 0").Find(&orphan).Error; err != nil {
		return err
	}
	if len(orphan) > 0 {
		var adminRole model.Role
		if err := db.Where("name = ?", "管理员").First(&adminRole).Error; err != nil {
			return err
		}
		return db.Model(&model.User{}).Where("role_id = 0").Update("role_id", adminRole.ID).Error
	}
	return nil
}
