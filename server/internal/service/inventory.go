package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// InventoryService 库存引擎：入库/出库/作废回冲，全部只追加流水保持对账不变式。
//
// 库存分两层：
//   - inventories          商品 × 仓库：数量 + 成本（均价/FIFO 结转都在这一层）
//   - location_inventories 商品 × 仓库 × 库位：只有数量，location_id=0 表示未分配
//
// 不变式：同一 (商品,仓库) 的 Σ库位数量 == inventories.quantity
type InventoryService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *InventoryService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewInventoryService(db *gorm.DB) *InventoryService { return &InventoryService{conn: db} }

// invOf 取库存行（不存在则返回零值行，不落库）
func invOf(tx *gorm.DB, productID, whID int64) (*model.Inventory, error) {
	var inv model.Inventory
	err := tx.Where("product_id = ? AND warehouse_id = ?", productID, whID).First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.Inventory{ProductID: productID, WarehouseID: whID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// locInvOf 取库位库存行（不存在则返回零值行，不落库）
func locInvOf(tx *gorm.DB, productID, whID, locID int64) (*model.LocationInventory, error) {
	var li model.LocationInventory
	err := tx.Where("product_id = ? AND warehouse_id = ? AND location_id = ?", productID, whID, locID).First(&li).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.LocationInventory{ProductID: productID, WarehouseID: whID, LocationID: locID}, nil
	}
	if err != nil {
		return nil, err
	}
	return &li, nil
}

// changeLocQty 库位数量增减（行不存在则创建）。allowNeg=false 时减到负数直接拦截——
// 库位账不允许负库存，否则"货在哪个货架"就失去意义。
func changeLocQty(tx *gorm.DB, productID, whID, locID, delta int64, allowNeg bool) error {
	li, err := locInvOf(tx, productID, whID, locID)
	if err != nil {
		return err
	}
	newQty := li.Quantity + delta
	if newQty < 0 && !allowNeg {
		return errcode.New(2004, fmt.Sprintf("库位库存不足：需要 %s，可用 %s", qtyText(-delta), qtyText(li.Quantity)))
	}
	li.Quantity = newQty
	if li.ID == 0 {
		return tx.Create(li).Error
	}
	return tx.Save(li).Error
}

// ensureBatch 取批次行（不存在则创建），batchNo 为空用"默认"
func ensureBatch(tx *gorm.DB, productID, whID int64, batchNo string) (*model.InventoryBatch, error) {
	if batchNo == "" {
		batchNo = "默认"
	}
	var b model.InventoryBatch
	err := tx.Where("product_id = ? AND warehouse_id = ? AND batch_no = ?", productID, whID, batchNo).First(&b).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		b = model.InventoryBatch{ProductID: productID, WarehouseID: whID, BatchNo: batchNo}
		if err := tx.Create(&b).Error; err != nil {
			return nil, err
		}
		return &b, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func addMovement(tx *gorm.DB, productID, whID, locID int64, batchID *int64, direction string, qty, unitCost int64, docID *int64) error {
	m := model.InventoryMovement{
		ProductID: productID, WarehouseID: whID, LocationID: locID, BatchID: batchID,
		Direction: direction, Quantity: qty, UnitCost: unitCost,
	}
	if docID != nil {
		m.DocumentID = docID
	}
	return tx.Create(&m).Error
}

// locAlloc 库位分配量
type locAlloc struct {
	LocID int64
	Qty   int64
}

// batchAlloc 批次分配量（非批次商品 BatchID=nil）
type batchAlloc struct {
	BatchID *int64
	Qty     int64
	Cost    int64
}

// planLocationOut 计算库位扣减方案。
//   - 指定库位：只扣该库位，不足直接拦截（库位账恒禁负）
//   - 未指定：商品默认库位优先，其余按可用数量降序补足；仍不足且允许负库存时，
//     差额挂到「未分配」库位，用于暴露账实差异
func planLocationOut(tx *gorm.DB, whID, locID int64, p *model.Product, qty int64, allowNegative bool) ([]locAlloc, error) {
	if locID > 0 {
		li, err := locInvOf(tx, p.ID, whID, locID)
		if err != nil {
			return nil, err
		}
		if li.Quantity < qty {
			name := fmt.Sprintf("库位 #%d", locID)
			var loc model.Location
			if e := tx.First(&loc, locID).Error; e == nil {
				name = "库位「" + loc.Name + "」"
			}
			return nil, errcode.New(2004, fmt.Sprintf("%s库存不足：出库 %s，可用 %s", name, qtyText(qty), qtyText(li.Quantity)))
		}
		return []locAlloc{{locID, qty}}, nil
	}

	var rows []model.LocationInventory
	if err := tx.Where("product_id = ? AND warehouse_id = ? AND quantity > 0", p.ID, whID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		di, dj := rows[i].LocationID == p.DefaultLocationID, rows[j].LocationID == p.DefaultLocationID
		if di != dj {
			return di
		}
		return rows[i].Quantity > rows[j].Quantity
	})

	need := qty
	allocs := make([]locAlloc, 0, len(rows)+1)
	for _, r := range rows {
		if need <= 0 {
			break
		}
		take := r.Quantity
		if need < take {
			take = need
		}
		allocs = append(allocs, locAlloc{r.LocationID, take})
		need -= take
	}
	if need > 0 {
		if !allowNegative {
			return nil, errcode.ErrStockNotEnough
		}
		allocs = append(allocs, locAlloc{0, need})
	}
	if len(allocs) == 0 {
		return []locAlloc{{0, qty}}, nil
	}
	return allocs, nil
}

// emitOutMovements 把批次序列与库位序列按量切分对齐，逐段写流水（两个维度各自独立累加）
func emitOutMovements(tx *gorm.DB, p *model.Product, whID int64, bAllocs []batchAlloc, lAllocs []locAlloc, docID *int64) error {
	if len(bAllocs) == 0 || len(lAllocs) == 0 {
		return nil
	}
	i, j := 0, 0
	remB, remL := bAllocs[0].Qty, lAllocs[0].Qty
	for i < len(bAllocs) && j < len(lAllocs) {
		take := remB
		if remL < take {
			take = remL
		}
		if take > 0 {
			if err := addMovement(tx, p.ID, whID, lAllocs[j].LocID, bAllocs[i].BatchID,
				"out", take, bAllocs[i].Cost, docID); err != nil {
				return err
			}
		}
		remB -= take
		remL -= take
		if remB == 0 {
			i++
			if i < len(bAllocs) {
				remB = bAllocs[i].Qty
			}
		}
		if remL == 0 {
			j++
			if j < len(lAllocs) {
				remL = lAllocs[j].Qty
			}
		}
	}
	return nil
}

// ApplyIn 入库：加权平均商品重算均价；批次商品写批次；库位数量同步增加
func (s *InventoryService) ApplyIn(ctx context.Context, tx *gorm.DB, whID, locID int64, p *model.Product, qty, unitCost int64, batchNo string, docID *int64) error {
	if qty <= 0 {
		return errcode.ErrParam
	}
	var batchID *int64
	if p.IsBatchEnabled {
		b, err := ensureBatch(tx, p.ID, whID, batchNo)
		if err != nil {
			return err
		}
		b.QuantityIn += qty
		b.UnitCost = unitCost
		if err := tx.Save(b).Error; err != nil {
			return err
		}
		batchID = &b.ID
	}
	inv, err := invOf(tx, p.ID, whID)
	if err != nil {
		return err
	}
	newQty := inv.Quantity + qty
	if p.CostMethod == "weighted_avg" {
		if inv.Quantity > 0 {
			inv.AvgCost = (inv.Quantity*inv.AvgCost + qty*unitCost) / newQty
		} else {
			inv.AvgCost = unitCost
		}
	} else if inv.Quantity <= 0 {
		// FIFO 商品库存为零时，均价兜底为本次入库成本
		inv.AvgCost = unitCost
	}
	inv.Quantity = newQty
	inv.TotalValue = newQty * inv.AvgCost / 1000
	if err := tx.Save(inv).Error; err != nil {
		return err
	}
	if err := changeLocQty(tx, p.ID, whID, locID, qty, false); err != nil {
		return err
	}
	return addMovement(tx, p.ID, whID, locID, batchID, "in", qty, unitCost, docID)
}

// ApplyOut 出库：FIFO 按批次先进先出分配；加权平均按当前均价。
// 库位按 planLocationOut 的方案分配，可能产生多条流水（跨库位出库）。
// 返回平均结转单价（分）。负库存开关关闭且库存不足 → 2001；指定库位不足 → 2004。
func (s *InventoryService) ApplyOut(ctx context.Context, tx *gorm.DB, whID, locID int64, p *model.Product, qty int64, docID *int64) (int64, error) {
	if qty <= 0 {
		return 0, errcode.ErrParam
	}
	inv, err := invOf(tx, p.ID, whID)
	if err != nil {
		return 0, err
	}
	allowNegative := model.GetBoolSetting(tx, "allow_negative_stock", false)
	if !allowNegative && inv.Quantity < qty {
		return 0, errcode.ErrStockNotEnough
	}

	// 先规划库位分配：失败在任何账变动之前返回
	lAllocs, err := planLocationOut(tx, whID, locID, p, qty, allowNegative)
	if err != nil {
		return 0, err
	}

	var bAllocs []batchAlloc
	var unitCost int64

	if p.CostMethod != "fifo" {
		unitCost = inv.AvgCost
		bAllocs = []batchAlloc{{nil, qty, unitCost}}
		inv.Quantity -= qty
		if inv.Quantity <= 0 {
			inv.TotalValue = inv.Quantity * inv.AvgCost / 1000
		} else {
			inv.TotalValue -= qty * unitCost / 1000
		}
	} else {
		// FIFO：按批次先进先出
		var batches []model.InventoryBatch
		if err := tx.Where("product_id = ? AND warehouse_id = ? AND quantity_in - quantity_out > 0", p.ID, whID).
			Order("id ASC").Find(&batches).Error; err != nil {
			return 0, err
		}
		remaining := qty
		var costSum int64
		for _, b := range batches {
			if remaining <= 0 {
				break
			}
			avail := b.QuantityIn - b.QuantityOut
			take := avail
			if remaining < avail {
				take = remaining
			}
			b.QuantityOut += take
			if err := tx.Save(&b).Error; err != nil {
				return 0, err
			}
			bid := b.ID
			bAllocs = append(bAllocs, batchAlloc{&bid, take, b.UnitCost})
			costSum += take * b.UnitCost
			remaining -= take
		}
		if remaining > 0 {
			// 批次不足（允许负库存时才会走到这里）：按当前均价结转
			bAllocs = append(bAllocs, batchAlloc{nil, remaining, inv.AvgCost})
			costSum += remaining * inv.AvgCost
			remaining = 0
		}
		unitCost = costSum / qty
		inv.Quantity -= qty
		inv.TotalValue = inv.Quantity * inv.AvgCost / 1000
	}

	if err := tx.Save(inv).Error; err != nil {
		return 0, err
	}
	// 扣减库位数量：负库存差额挂未分配库位，允许为负以暴露差异
	for _, a := range lAllocs {
		allowNeg := allowNegative && a.LocID == 0
		if err := changeLocQty(tx, p.ID, whID, a.LocID, -a.Qty, allowNeg); err != nil {
			return 0, err
		}
	}
	if err := emitOutMovements(tx, p, whID, bAllocs, lAllocs, docID); err != nil {
		return 0, err
	}
	return unitCost, nil
}

// Transfer 移库：同仓库内库位间搬移，不动仓库总量、不动成本。
// 只写一对流水（源库位 out / 目标库位 in），作业依据齐全。
func (s *InventoryService) Transfer(ctx context.Context, tx *gorm.DB, whID, fromLoc, toLoc int64, p *model.Product, qty int64, docID *int64) error {
	if qty <= 0 {
		return errcode.ErrParam
	}
	if fromLoc == toLoc {
		return errcode.New(1001, "移出库位与移入库位不能相同")
	}
	inv, err := invOf(tx, p.ID, whID)
	if err != nil {
		return err
	}
	if err := changeLocQty(tx, p.ID, whID, fromLoc, -qty, false); err != nil {
		return err
	}
	if err := changeLocQty(tx, p.ID, whID, toLoc, qty, false); err != nil {
		return err
	}
	cost := inv.AvgCost
	if err := addMovement(tx, p.ID, whID, fromLoc, nil, "out", qty, cost, docID); err != nil {
		return err
	}
	return addMovement(tx, p.ID, whID, toLoc, nil, "in", qty, cost, docID)
}

// ReverseDoc 作废回冲：把该单据产生的每条流水反向补一条（账面连续，不变式保持）。
// 移库单只回冲库位数量，不触碰仓库汇总（搬移本身不影响总量与成本）。
func (s *InventoryService) ReverseDoc(ctx context.Context, tx *gorm.DB, docID int64, docType string) error {
	isTransfer := docType == "transfer"
	var movements []model.InventoryMovement
	if err := tx.Where("document_id = ?", docID).Order("id DESC").Find(&movements).Error; err != nil {
		return err
	}
	for _, m := range movements {
		reverse := "out"
		if m.Direction == "out" {
			reverse = "in"
		}
		if isTransfer {
			delta := -m.Quantity // 原 in → 减；原 out → 加
			if m.Direction == "out" {
				delta = m.Quantity
			}
			if err := changeLocQty(tx, m.ProductID, m.WarehouseID, m.LocationID, delta, false); err != nil {
				if be, ok := err.(*errcode.BizError); ok && be.Code == 2004 && m.Direction == "in" {
					return errcode.New(3003, "移库回冲失败：搬入库位的货已被出库，无法原路退回。请改用反向移库单把剩余数量搬回")
				}
				return err
			}
			if err := addMovement(tx, m.ProductID, m.WarehouseID, m.LocationID, nil, reverse, m.Quantity, m.UnitCost, &docID); err != nil {
				return err
			}
			continue
		}

		var inv model.Inventory
		err := tx.Where("product_id = ? AND warehouse_id = ?", m.ProductID, m.WarehouseID).First(&inv).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(3003, "库存记录缺失，无法回冲")
		}
		if err != nil {
			return err
		}
		if m.Direction == "in" {
			// 回冲入库：扣回数量（PRD：库存已被卖出则阻止）
			if inv.Quantity < m.Quantity {
				return errcode.New(3003, "库存不足以回冲入库单（商品可能已售出），请先做销售退货或其他出库")
			}
			inv.Quantity -= m.Quantity
			inv.TotalValue -= m.Quantity * m.UnitCost / 1000
			if inv.Quantity == 0 {
				inv.AvgCost = 0
				inv.TotalValue = 0
			}
			if m.BatchID != nil {
				if err := tx.Model(&model.InventoryBatch{}).Where("id = ?", *m.BatchID).
					UpdateColumn("quantity_in", gorm.Expr("quantity_in - ?", m.Quantity)).Error; err != nil {
					return err
				}
			}
		} else {
			// 回冲出库：加回数量，按原结转成本
			inv.Quantity += m.Quantity
			inv.TotalValue += m.Quantity * m.UnitCost / 1000
			if inv.Quantity > 0 && inv.AvgCost == 0 {
				inv.AvgCost = m.UnitCost
			}
			if m.BatchID != nil {
				if err := tx.Model(&model.InventoryBatch{}).Where("id = ?", *m.BatchID).
					UpdateColumn("quantity_out", gorm.Expr("quantity_out - ?", m.Quantity)).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Save(&inv).Error; err != nil {
			return err
		}
		// 库位数量同步回冲
		locDelta := -m.Quantity
		if m.Direction == "out" {
			locDelta = m.Quantity
		}
		if err := changeLocQty(tx, m.ProductID, m.WarehouseID, m.LocationID, locDelta, false); err != nil {
			return err
		}
		if err := addMovement(tx, m.ProductID, m.WarehouseID, m.LocationID, m.BatchID, reverse, m.Quantity, m.UnitCost, &docID); err != nil {
			return err
		}
	}
	return nil
}
