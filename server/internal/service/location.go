package service

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// LocationService 库位档案 + 库位数量账 + 容量/上下限预警
type LocationService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *LocationService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewLocationService(db *gorm.DB) *LocationService { return &LocationService{conn: db} }

// 库位状态
const (
	locNormal   = "normal"    // 正常
	locNearFull = "near_full" // 接近容量（≥ 预警阈值）
	locFull     = "full"      // 刚好用满
	locOver     = "over"      // 超出容量
	locUnder    = "under"     // 低于下限
	locEmpty    = "empty"     // 空置（设置了容量但没货）
)

// 容量计量方式（v0.17）：qty 数量合计 / volume 体积 / weight 重量
const (
	capModeQty    = "qty"
	capModeVolume = "volume"
	capModeWeight = "weight"
)

func validCapMode(m string) bool {
	return m == capModeQty || m == capModeVolume || m == capModeWeight
}

// capUnitText 计量方式展示单位（前端/错误消息共用口径）
func capUnitText(mode string) string {
	switch mode {
	case capModeVolume:
		return "L"
	case capModeWeight:
		return "kg"
	}
	return "件"
}

// usedExpr 库位占用的 SQL 表达式：按库位计量方式把数量换算到对应量纲
// 数量/体积/重量内部均 ×1000 存储；quantity×单件体积/1000 得到体积占用（×1000）
func usedExpr() string {
	return `SUM(CASE l.capacity_mode
		WHEN 'volume' THEN li.quantity * COALESCE(p.volume, 0) / 1000
		WHEN 'weight' THEN li.quantity * COALESCE(p.weight, 0) / 1000
		ELSE li.quantity END)`
}

// unknownExpr 该库位计量方式下「未登记体积/重量」的商品数（这些商品占用算 0，需提示补档案）
func unknownExpr() string {
	return `SUM(CASE
		WHEN l.capacity_mode = 'volume' AND COALESCE(p.volume, 0) = 0 THEN 1
		WHEN l.capacity_mode = 'weight' AND COALESCE(p.weight, 0) = 0 THEN 1
		ELSE 0 END)`
}

// LocationView 库位 + 实时占用（列表页与预警共用）
type LocationView struct {
	model.Location
	UsedQty     int64  `json:"used_qty"`     // 占用合计（×1000，量纲随 capacity_mode：数量/升/kg）
	UnknownSkus int    `json:"unknown_skus"` // 未登记体积/重量的在库商品数（volume/weight 模式下这些商品不计占用）
	UsedUnit    string `json:"used_unit"`    // 占用量纲：件 / L / kg
	SkuCount    int    `json:"sku_count"`    // 在库商品数
	UsageRate   int    `json:"usage_rate"`   // 占用率百分比；未设容量时为 -1
	Status      string `json:"status"`       // normal / near_full / full / over / under / empty
	StatusText  string `json:"status_text"`  // 中文状态（前端直接展示）
}

// normWarnRatio 阈值兜底：非法值按 90% 处理
func normWarnRatio(v int) int {
	if v <= 0 || v > 100 {
		return 90
	}
	return v
}

// buildView 计算单行视图（used 为该库位按计量方式的占用合计，skus 为在库商品数）
func buildView(loc model.Location, used int64, skus, unknown int) LocationView {
	v := LocationView{Location: loc, UsedQty: used, SkuCount: skus, UnknownSkus: unknown,
		UsedUnit: capUnitText(loc.CapacityMode), UsageRate: -1, Status: locNormal}
	ratio := normWarnRatio(loc.WarnRatio)
	v.WarnRatio = ratio
	if loc.Capacity > 0 {
		v.UsageRate = int(used * 100 / loc.Capacity)
		switch {
		case used > loc.Capacity:
			v.Status = locOver
		case used == loc.Capacity:
			v.Status = locFull
		case used*100 >= loc.Capacity*int64(ratio):
			v.Status = locNearFull
		case used == 0:
			v.Status = locEmpty
		}
	}
	if v.Status == locNormal && loc.MinQty > 0 && used < loc.MinQty {
		v.Status = locUnder
	}
	v.StatusText = locStatusText(v.Status)
	return v
}

func locStatusText(s string) string {
	switch s {
	case locOver:
		return "超出容量"
	case locFull:
		return "已满"
	case locNearFull:
		return "接近满"
	case locUnder:
		return "低于下限"
	case locEmpty:
		return "空置"
	}
	return "正常"
}

// usageMap 批量取库位占用：库位 ID → (按该库位计量方式的占用合计, 商品数, 未登记维度商品数)
type locUsage struct {
	LocationID int64
	Qty        int64
	Skus       int
	Unknown    int
}

func (s *LocationService) usageMap(ctx context.Context) (map[int64]locUsage, error) {
	var rows []locUsage
	err := s.db(ctx).Table("location_inventories AS li").
		Select(`li.location_id, ` + usedExpr() + ` AS qty, COUNT(1) AS skus, ` + unknownExpr() + ` AS unknown`).
		Joins("JOIN locations l ON l.id = li.location_id").
		Joins("JOIN products p ON p.id = li.product_id AND p.deleted_at IS NULL").
		Where("li.quantity <> 0 AND li.deleted_at IS NULL").
		Group("li.location_id").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[int64]locUsage, len(rows))
	for _, r := range rows {
		m[r.LocationID] = r
	}
	return m, nil
}

// List 库位列表（含占用与状态）。location_id=0 的「未分配」不建档案，
// 由接口单独返回汇总，不混在货架列表里。
func (s *LocationService) List(ctx context.Context) ([]LocationView, error) {
	var locs []model.Location
	if err := s.db(ctx).Order("sort, id").Find(&locs).Error; err != nil {
		return nil, err
	}
	usage, err := s.usageMap(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LocationView, 0, len(locs))
	for _, l := range locs {
		u := usage[l.ID]
		out = append(out, buildView(l, u.Qty, u.Skus, u.Unknown))
	}
	return out, nil
}

// Alerts 容量/上下限预警列表（超出容量、已满、接近满、低于下限）
func (s *LocationService) Alerts(ctx context.Context) ([]LocationView, error) {
	list, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LocationView, 0, 4)
	for _, v := range list {
		switch v.Status {
		case locOver, locFull, locNearFull, locUnder:
			out = append(out, v)
		}
	}
	return out, nil
}

// checkCapacity 目标库位容量校验（入库、移入共用），量纲随库位计量方式：
// qty=数量合计；volume/weight=Σ(数量×单件体积/重量)。商品未登记对应维度时不校验（占用算 0）。
// 默认只在库位页提示，不阻断作业；系统设置 location_block_overflow=true 时才拦截。
func checkCapacity(tx *gorm.DB, whID, locID, addQty, productID int64) error {
	if locID <= 0 || addQty <= 0 {
		return nil
	}
	var loc model.Location
	if err := tx.First(&loc, locID).Error; err != nil {
		return errcode.New(4004, "库位不存在")
	}
	if loc.Capacity <= 0 || !model.GetBoolSetting(tx, "location_block_overflow", false) {
		return nil
	}
	mode := loc.CapacityMode
	if mode == "" {
		mode = capModeQty
	}
	// 待入库存的量纲换算；volume/weight 模式下商品未登记维度则放行（无法核算）
	add := addQty
	if mode != capModeQty {
		var p model.Product
		if err := tx.Select("volume", "weight").First(&p, productID).Error; err != nil {
			return errcode.New(4004, "商品不存在")
		}
		dim := p.Volume
		if mode == capModeWeight {
			dim = p.Weight
		}
		if dim <= 0 {
			return nil
		}
		add = addQty * dim / 1000
	}
	var used int64
	if err := tx.Table("location_inventories AS li").
		Joins("JOIN products p ON p.id = li.product_id AND p.deleted_at IS NULL").
		Where("li.warehouse_id = ? AND li.location_id = ? AND li.quantity <> 0 AND li.deleted_at IS NULL", whID, locID).
		Select("COALESCE(" + usedExprOne(mode) + ", 0)").Scan(&used).Error; err != nil {
		return err
	}
	if used+add > loc.Capacity {
		u := capUnitText(mode)
		return errcode.New(2005, fmt.Sprintf("库位「%s」容量不足：本次增加 %s%s，已占用 %s%s，容量上限 %s%s。可改移其他库位，或到系统设置关闭「库位超容拦截」",
			loc.Name, qtyText(add), u, qtyText(used), u, qtyText(loc.Capacity), u))
	}
	return nil
}

// usedExprOne 单库位占用表达式（无 l 别名，模式由参数给定）
func usedExprOne(mode string) string {
	switch mode {
	case capModeVolume:
		return "SUM(li.quantity * COALESCE(p.volume, 0) / 1000)"
	case capModeWeight:
		return "SUM(li.quantity * COALESCE(p.weight, 0) / 1000)"
	}
	return "SUM(li.quantity)"
}

type LocationReq struct {
	Code         string `json:"code"`
	Name         string `json:"name" binding:"required"`
	Remark       string `json:"remark"`
	Sort         int    `json:"sort"`
	CapacityMode string `json:"capacity_mode"` // qty / volume / weight，空=qty
	Capacity     int64  `json:"capacity"`
	MinQty       int64  `json:"min_qty"`
	WarnRatio    int    `json:"warn_ratio"`
}

func (r *LocationReq) normalize() error {
	if r.CapacityMode == "" {
		r.CapacityMode = capModeQty
	}
	if !validCapMode(r.CapacityMode) {
		return errcode.New(1001, "容量计量方式仅支持 qty/volume/weight")
	}
	if r.Capacity < 0 {
		r.Capacity = 0
	}
	if r.MinQty < 0 {
		r.MinQty = 0
	}
	r.WarnRatio = normWarnRatio(r.WarnRatio)
	return nil
}

func (s *LocationService) Create(ctx context.Context, req LocationReq) error {
	var count int64
	s.db(ctx).Model(&model.Location{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		return errcode.New(4001, "库位名称已存在")
	}
	if err := req.normalize(); err != nil {
		return err
	}
	return s.db(ctx).Create(&model.Location{
		Code: req.Code, Name: req.Name, Remark: req.Remark, Sort: req.Sort,
		CapacityMode: req.CapacityMode,
		Capacity:     req.Capacity, MinQty: req.MinQty, WarnRatio: req.WarnRatio,
	}).Error
}

func (s *LocationService) Update(ctx context.Context, id int64, req LocationReq) error {
	var loc model.Location
	if err := s.db(ctx).First(&loc, id).Error; err != nil {
		return errcode.ErrNotFound
	}
	var count int64
	s.db(ctx).Model(&model.Location{}).Where("name = ? AND id <> ?", req.Name, id).Count(&count)
	if count > 0 {
		return errcode.New(4001, "库位名称已存在")
	}
	if err := req.normalize(); err != nil {
		return err
	}
	loc.Code, loc.Name, loc.Remark, loc.Sort = req.Code, req.Name, req.Remark, req.Sort
	loc.CapacityMode = req.CapacityMode
	loc.Capacity, loc.MinQty, loc.WarnRatio = req.Capacity, req.MinQty, req.WarnRatio
	return s.db(ctx).Save(&loc).Error
}

// LocationStockRow 库位库存行（v0.12 库位数量账）
type LocationStockRow struct {
	LocationID   int64  `json:"location_id"` // 0 = 未分配
	LocationName string `json:"location_name"`
	LocationCode string `json:"location_code"`
	ProductID    int64  `json:"product_id"`
	ProductCode  string `json:"product_code"`
	ProductName  string `json:"product_name"`
	Unit         string `json:"unit"`
	Barcode      string `json:"barcode"`
	PLU          string `json:"plu"`
	Quantity     int64  `json:"quantity"` // ×1000
}

// Stock 库位库存查询：locID=0 表示只看未分配；locID<0 表示不过滤（全部）
func (s *LocationService) Stock(ctx context.Context, locID int64, keyword string) ([]LocationStockRow, error) {
	q := s.db(ctx).Table("location_inventories AS li").
		Select(`li.location_id,
		        COALESCE(l.name, '未分配') AS location_name,
		        COALESCE(l.code, '') AS location_code,
		        li.product_id, p.code AS product_code, p.name AS product_name,
		        p.unit, p.barcode, p.plu, li.quantity`).
		Joins("LEFT JOIN locations l ON l.id = li.location_id").
		Joins("JOIN products p ON p.id = li.product_id").
		Where("p.deleted_at IS NULL AND li.quantity <> 0")
	if locID >= 0 {
		q = q.Where("li.location_id = ?", locID)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("p.code LIKE ? OR p.name LIKE ? OR p.barcode LIKE ? OR p.plu LIKE ?", kw, kw, kw, kw)
	}
	var rows []LocationStockRow
	err := q.Order("li.location_id, p.code").Scan(&rows).Error
	return rows, err
}

func (s *LocationService) Delete(ctx context.Context, id int64) error {
	// 有商品设为默认库位时禁止删除，避免开单自动带出失效
	var count int64
	s.db(ctx).Model(&model.Product{}).Where("default_location_id = ?", id).Count(&count)
	if count > 0 {
		return errcode.New(1003, "有商品使用该库位作为默认库位，请先在商品档案中调整")
	}
	// 库位还有库存时禁止删除（数量账要求货位有归属，先移库清空）
	s.db(ctx).Model(&model.LocationInventory{}).Where("location_id = ? AND quantity <> 0", id).Count(&count)
	if count > 0 {
		return errcode.New(1003, "该库位仍有库存，请先用移库单把货移到其他库位")
	}
	return s.db(ctx).Delete(&model.Location{}, id).Error
}
