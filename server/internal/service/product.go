package service

import (
	"context"
	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

type ProductService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *ProductService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewProductService(db *gorm.DB) *ProductService { return &ProductService{conn: db} }

type ProductListReq struct {
	Keyword    string
	CategoryID int64
	Status     string
	Page       int
	PageSize   int
}

func (s *ProductService) List(ctx context.Context, req ProductListReq) ([]model.Product, int64, error) {
	q := s.db(ctx).Model(&model.Product{})
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		q = q.Where("name LIKE ? OR code LIKE ? OR barcode = ? OR plu = ?", kw, kw, req.Keyword, req.Keyword)
	}
	if req.CategoryID > 0 {
		q = q.Where("category_id = ?", req.CategoryID)
	}
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset, limit := (req.Page-1)*req.PageSize, req.PageSize
	var list []model.Product
	err := q.Preload("Prices").Order("id DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

type PriceReq struct {
	Name  string `json:"name" binding:"required"`
	Price int64  `json:"price"`
	Sort  int    `json:"sort"`
}

type ProductReq struct {
	Code              string     `json:"code" binding:"required"`
	Barcode           string     `json:"barcode"`
	PLU               string     `json:"plu"` // 秤码 PLU（散称商品在电子秤上的货号）
	Name              string     `json:"name" binding:"required"`
	CategoryID        int64      `json:"category_id"`
	Unit              string     `json:"unit"`
	CostMethod        string     `json:"cost_method" binding:"required"`
	IsBatchEnabled    bool       `json:"is_batch_enabled"`
	ShelfLifeDays     int        `json:"shelf_life_days"`
	SafeStock         int64      `json:"safe_stock"`
	Volume            int64      `json:"volume"` // 单件体积（升 ×1000，按基本单位）
	Weight            int64      `json:"weight"` // 单件重量（kg ×1000，按基本单位）
	Status            string     `json:"status"`
	Prices            []PriceReq `json:"prices"`
	DefaultPrice      string     `json:"default_price"`       // 默认价位名
	DefaultLocationID int64      `json:"default_location_id"` // 默认库位（0=不设）
}

func (r ProductReq) validate() error {
	if r.CostMethod != "weighted_avg" && r.CostMethod != "fifo" {
		return errcode.ErrCostMethod
	}
	if r.CostMethod == "fifo" && len(r.Prices) == 0 {
		return errcode.New(2004, "FIFO 商品至少需要一个售价")
	}
	return nil
}

// checkLocation 校验默认库位存在
func (s *ProductService) checkLocation(ctx context.Context, id int64) error {
	if id == 0 {
		return nil
	}
	var count int64
	s.db(ctx).Model(&model.Location{}).Where("id = ?", id).Count(&count)
	if count == 0 {
		return errcode.New(4004, "默认库位不存在")
	}
	return nil
}

// savePrices 事务内重建价位，并确定 default_price_id
func (s *ProductService) savePrices(ctx context.Context, tx *gorm.DB, productID int64, prices []PriceReq, defaultPrice string) error {
	if err := tx.Where("product_id = ?", productID).Delete(&model.ProductPrice{}).Error; err != nil {
		return err
	}
	var defaultID int64
	for i, p := range prices {
		pp := model.ProductPrice{ProductID: productID, Name: p.Name, Price: p.Price, Sort: i}
		if err := tx.Create(&pp).Error; err != nil {
			return err
		}
		if p.Name == defaultPrice || (defaultID == 0 && i == 0) {
			defaultID = pp.ID
		}
	}
	return tx.Model(&model.Product{}).Where("id = ?", productID).Update("default_price_id", defaultID).Error
}

func (s *ProductService) Create(ctx context.Context, req ProductReq) error {
	if err := req.validate(); err != nil {
		return err
	}
	// FIFO 商品强制启用批次（PRD §6.1/§6.2）
	if req.CostMethod == "fifo" {
		req.IsBatchEnabled = true
	}
	var count int64
	s.db(ctx).Model(&model.Product{}).Where("code = ?", req.Code).Count(&count)
	if count > 0 {
		return errcode.ErrCodeDup
	}
	if req.Barcode != "" {
		s.db(ctx).Model(&model.Product{}).Where("barcode = ?", req.Barcode).Count(&count)
		if count > 0 {
			return errcode.New(4005, "条码已被其他商品使用")
		}
	}
	if req.PLU != "" {
		s.db(ctx).Model(&model.Product{}).Where("plu = ?", req.PLU).Count(&count)
		if count > 0 {
			return errcode.New(4006, "PLU 码已被其他商品使用")
		}
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if err := s.checkLocation(ctx, req.DefaultLocationID); err != nil {
		return err
	}
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		p := model.Product{
			Code: req.Code, Barcode: req.Barcode, PLU: req.PLU, Name: req.Name,
			CategoryID: req.CategoryID, Unit: req.Unit,
			CostMethod: req.CostMethod, IsBatchEnabled: req.IsBatchEnabled,
			ShelfLifeDays: req.ShelfLifeDays, SafeStock: req.SafeStock, Status: req.Status,
			Volume: req.Volume, Weight: req.Weight,
			DefaultLocationID: req.DefaultLocationID,
		}
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
		return s.savePrices(ctx, tx, p.ID, req.Prices, req.DefaultPrice)
	})
}

func (s *ProductService) Update(ctx context.Context, id int64, req ProductReq) error {
	if err := req.validate(); err != nil {
		return err
	}
	var p model.Product
	if err := s.db(ctx).First(&p, id).Error; err != nil {
		return errcode.ErrNotFound
	}
	// 成本方式建后不可改（PRD §6.1：改则需库存清零）
	if p.CostMethod != req.CostMethod {
		return errcode.New(2005, "成本方式建立后不可修改")
	}
	if req.CostMethod == "fifo" {
		req.IsBatchEnabled = true
	}
	if err := s.checkLocation(ctx, req.DefaultLocationID); err != nil {
		return err
	}
	var count int64
	s.db(ctx).Model(&model.Product{}).Where("code = ? AND id <> ?", req.Code, id).Count(&count)
	if count > 0 {
		return errcode.ErrCodeDup
	}
	if req.PLU != "" {
		s.db(ctx).Model(&model.Product{}).Where("plu = ? AND id <> ?", req.PLU, id).Count(&count)
		if count > 0 {
			return errcode.New(4006, "PLU 码已被其他商品使用")
		}
	}
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		p.Code, p.Barcode, p.PLU, p.Name = req.Code, req.Barcode, req.PLU, req.Name
		p.CategoryID, p.Unit = req.CategoryID, req.Unit
		p.IsBatchEnabled, p.ShelfLifeDays, p.SafeStock, p.Status =
			req.IsBatchEnabled, req.ShelfLifeDays, req.SafeStock, req.Status
		p.Volume, p.Weight = req.Volume, req.Weight
		p.DefaultLocationID = req.DefaultLocationID
		if err := tx.Save(&p).Error; err != nil {
			return err
		}
		return s.savePrices(ctx, tx, p.ID, req.Prices, req.DefaultPrice)
	})
}

func (s *ProductService) Delete(ctx context.Context, id int64) error {
	var p model.Product
	if err := s.db(ctx).First(&p, id).Error; err != nil {
		return errcode.ErrNotFound
	}
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("product_id = ?", id).Delete(&model.ProductPrice{}).Error; err != nil {
			return err
		}
		return tx.Delete(&p).Error
	})
}

func (s *ProductService) GetByID(ctx context.Context, id int64) (*model.Product, error) {
	var p model.Product
	if err := s.db(ctx).Preload("Prices").First(&p, id).Error; err != nil {
		return nil, errcode.ErrNotFound
	}
	return &p, nil
}
