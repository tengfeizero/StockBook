package service

import (
	"context"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

type CategoryService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *CategoryService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewCategoryService(db *gorm.DB) *CategoryService { return &CategoryService{conn: db} }

func (s *CategoryService) List(ctx context.Context, keyword string) ([]model.Category, error) {
	q := s.db(ctx).Model(&model.Category{})
	if keyword != "" {
		q = q.Where("name LIKE ?", "%"+keyword+"%")
	}
	var list []model.Category
	err := q.Order("sort, id").Find(&list).Error
	return list, err
}

type CategoryReq struct {
	Name     string `json:"name" binding:"required"`
	ParentID int64  `json:"parent_id"`
	Sort     int    `json:"sort"`
}

func (s *CategoryService) Create(ctx context.Context, req CategoryReq) error {
	var count int64
	s.db(ctx).Model(&model.Category{}).Where("name = ? AND parent_id = ?", req.Name, req.ParentID).Count(&count)
	if count > 0 {
		return errcode.New(4003, "同级分类名已存在")
	}
	return s.db(ctx).Create(&model.Category{Name: req.Name, ParentID: req.ParentID, Sort: req.Sort}).Error
}

func (s *CategoryService) Update(ctx context.Context, id int64, req CategoryReq) error {
	var c model.Category
	if err := s.db(ctx).First(&c, id).Error; err != nil {
		return errcode.ErrNotFound
	}
	var count int64
	s.db(ctx).Model(&model.Category{}).Where("name = ? AND parent_id = ? AND id <> ?", req.Name, req.ParentID, id).Count(&count)
	if count > 0 {
		return errcode.New(4003, "同级分类名已存在")
	}
	c.Name, c.ParentID, c.Sort = req.Name, req.ParentID, req.Sort
	return s.db(ctx).Save(&c).Error
}

func (s *CategoryService) Delete(ctx context.Context, id int64) error {
	var count int64
	s.db(ctx).Model(&model.Product{}).Where("category_id = ?", id).Count(&count)
	if count > 0 {
		return errcode.New(4004, "该分类下有商品，不能删除")
	}
	var child int64
	s.db(ctx).Model(&model.Category{}).Where("parent_id = ?", id).Count(&child)
	if child > 0 {
		return errcode.New(4004, "该分类下有子分类，不能删除")
	}
	return s.db(ctx).Delete(&model.Category{}, id).Error
}
