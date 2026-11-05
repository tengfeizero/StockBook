package service

import (
	"context"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// PartnerService 供应商 / 客户共用（结构一致，仅表不同）
type PartnerService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *PartnerService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewPartnerService(db *gorm.DB) *PartnerService { return &PartnerService{conn: db} }

type PartnerReq struct {
	Code    string `json:"code" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Contact string `json:"contact"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	Opening int64  `json:"opening"` // 期初应收/应付，分
	Remark  string `json:"remark"`
}

func (s *PartnerService) List(ctx context.Context, kind, keyword string, page, pageSize int) (any, int64, error) {
	offset, limit := (page-1)*pageSize, pageSize
	switch kind {
	case "supplier":
		return listPartners[model.Supplier](s.db(ctx), keyword, offset, limit)
	case "customer":
		return listPartners[model.Customer](s.db(ctx), keyword, offset, limit)
	}
	return nil, 0, errcode.ErrParam
}

func listPartners[T any](db *gorm.DB, keyword string, offset, limit int) ([]T, int64, error) {
	q := db.Model(new(T))
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR code LIKE ?", kw, kw)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []T
	err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

func (s *PartnerService) Create(ctx context.Context, kind string, req PartnerReq) error {
	var count int64
	switch kind {
	case "supplier":
		s.db(ctx).Model(&model.Supplier{}).Where("code = ?", req.Code).Count(&count)
		if count > 0 {
			return errcode.ErrCodeDup
		}
		return s.db(ctx).Create(&model.Supplier{
			Code: req.Code, Name: req.Name, Contact: req.Contact,
			Phone: req.Phone, Address: req.Address, OpeningPayable: req.Opening, Remark: req.Remark,
		}).Error
	case "customer":
		s.db(ctx).Model(&model.Customer{}).Where("code = ?", req.Code).Count(&count)
		if count > 0 {
			return errcode.ErrCodeDup
		}
		return s.db(ctx).Create(&model.Customer{
			Code: req.Code, Name: req.Name, Contact: req.Contact,
			Phone: req.Phone, Address: req.Address, OpeningReceivable: req.Opening, Remark: req.Remark,
		}).Error
	}
	return errcode.ErrParam
}

func (s *PartnerService) Update(ctx context.Context, kind string, id int64, req PartnerReq) error {
	switch kind {
	case "supplier":
		var p model.Supplier
		if err := s.db(ctx).First(&p, id).Error; err != nil {
			return errcode.ErrNotFound
		}
		var count int64
		s.db(ctx).Model(&model.Supplier{}).Where("code = ? AND id <> ?", req.Code, id).Count(&count)
		if count > 0 {
			return errcode.ErrCodeDup
		}
		p.Code, p.Name, p.Contact, p.Phone, p.Address, p.OpeningPayable, p.Remark =
			req.Code, req.Name, req.Contact, req.Phone, req.Address, req.Opening, req.Remark
		return s.db(ctx).Save(&p).Error
	case "customer":
		var p model.Customer
		if err := s.db(ctx).First(&p, id).Error; err != nil {
			return errcode.ErrNotFound
		}
		var count int64
		s.db(ctx).Model(&model.Customer{}).Where("code = ? AND id <> ?", req.Code, id).Count(&count)
		if count > 0 {
			return errcode.ErrCodeDup
		}
		p.Code, p.Name, p.Contact, p.Phone, p.Address, p.OpeningReceivable, p.Remark =
			req.Code, req.Name, req.Contact, req.Phone, req.Address, req.Opening, req.Remark
		return s.db(ctx).Save(&p).Error
	}
	return errcode.ErrParam
}

func (s *PartnerService) Delete(ctx context.Context, kind string, id int64) error {
	switch kind {
	case "supplier":
		return s.db(ctx).Delete(&model.Supplier{}, id).Error
	case "customer":
		return s.db(ctx).Delete(&model.Customer{}, id).Error
	}
	return errcode.ErrParam
}
