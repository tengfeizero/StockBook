package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/docno"
	"jxc/server/internal/pkg/errcode"
)

// 单据类型 → 库存方向
// none=不影响库存; in=入库方向; out=出库方向
type docStockEffect string

const (
	effectNone   docStockEffect = "none"
	effectIn     docStockEffect = "in"
	effectOut    docStockEffect = "out"
	effectAdjust docStockEffect = "adjust" // 盘点调整：数量=实盘数
)

var docEffects = map[string]docStockEffect{
	"purchase_order":  effectNone,
	"quotation":       effectNone,
	"purchase_in":     effectIn,
	"sales_return":    effectIn,
	"other_in":        effectIn,
	"sales_out":       effectOut,
	"purchase_return": effectOut,
	"other_out":       effectOut,
	"stocktake":       effectAdjust, // 盘点：确认时按实盘数与账面差额自动盘盈/盘亏
	"transfer":        effectNone,   // 移库：同仓库内库位搬移，confirmTx 单独处理（不影响总量与成本）
}

var docPartnerKinds = map[string]string{
	"purchase_order": "supplier", "purchase_in": "supplier", "purchase_return": "supplier",
	"quotation": "customer", "sales_out": "customer", "sales_return": "customer",
}

type DocumentService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *DocumentService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewDocumentService(db *gorm.DB) *DocumentService { return &DocumentService{conn: db} }

// ---------- 入参 ----------

type DocItemReq struct {
	ProductID    int64  `json:"product_id" binding:"required"`
	Quantity     int64  `json:"quantity" binding:"required"` // ×1000，正数
	Price        int64  `json:"price"`                       // 分：销售=售价；采购/其他入库=成本
	PriceName    string `json:"price_name"`
	BatchNo      string `json:"batch_no"`
	LocationID   int64  `json:"location_id"`    // 库位：普通单据=作业库位（不填入库带默认、出库自动分配）；移库单=移出库位
	ToLocationID int64  `json:"to_location_id"` // 移库单移入库位
	Remark       string `json:"remark"`
}

type DocReq struct {
	DocType    string       `json:"doc_type" binding:"required"`
	PartnerID  int64        `json:"partner_id"`
	DocDate    string       `json:"doc_date"` // 2006-01-02，默认今天
	Remark     string       `json:"remark"`
	RefDocID   int64        `json:"ref_doc_id"`
	Scope      string       `json:"scope"`       // 仅盘点："" 全仓（默认）/ location 按库位
	ConfirmNow bool         `json:"confirm_now"` // 开单即生效（require_confirm=false 时前端传 true）
	Items      []DocItemReq `json:"items" binding:"required,min=1"`
}

// ---------- 校验 ----------

func (s *DocumentService) validateAndBuild(ctx context.Context, tx *gorm.DB, req *DocReq, userID int64) (*model.Document, error) {
	if _, ok := docEffects[req.DocType]; !ok {
		return nil, errcode.ErrParam
	}
	wh, err := model.GetMainWarehouse(tx)
	if err != nil {
		return nil, err
	}
	docDate := time.Now()
	if req.DocDate != "" {
		d, err := time.ParseInLocation("2006-01-02", req.DocDate, time.Local)
		if err != nil {
			return nil, errcode.ErrParam
		}
		docDate = d
	}

	doc := &model.Document{
		DocType: req.DocType, Status: "draft",
		PartnerType: docPartnerKinds[req.DocType],
		PartnerID:   req.PartnerID, DocDate: docDate,
		Remark: req.Remark, RefDocID: req.RefDocID, CreatedBy: userID,
	}
	// 盘点范围：只有盘点单认 scope，其余单据忽略（防前端误传）
	if req.DocType == "stocktake" && req.Scope == "location" {
		doc.Scope = "location"
	}
	locScoped := doc.Scope == "location"
	seen := map[string]bool{}

	// 往来单位校验+快照
	if doc.PartnerType != "" {
		if req.PartnerID <= 0 {
			return nil, errcode.New(1001, "请选择"+partnerKindName(doc.PartnerType))
		}
		name, err := partnerName(tx, doc.PartnerType, req.PartnerID)
		if err != nil {
			return nil, err
		}
		doc.PartnerName = name
	}

	// 明细校验+快照
	totalQty, totalAmount := int64(0), int64(0)
	for i := range req.Items {
		it := &req.Items[i]
		var p model.Product
		if err := tx.First(&p, it.ProductID).Error; err != nil {
			return nil, errcode.New(4004, "商品不存在")
		}
		if p.Status != "active" {
			return nil, errcode.New(4002, "商品「"+p.Name+"」已停用")
		}
		if it.Quantity < 0 || (it.Quantity == 0 && req.DocType != "stocktake") {
			return nil, errcode.New(1001, "商品「"+p.Name+"」数量必须大于0")
		}
		if it.Quantity == 0 && req.DocType == "stocktake" && p.IsBatchEnabled && !locScoped {
			return nil, errcode.New(2002, "盘点数量为 0 时，批次商品请用其他出库清空批次，或填写实盘批次号")
		}
		if it.Price < 0 {
			return nil, errcode.New(1001, "商品「"+p.Name+"」单价不能为负")
		}
		// 入库类：批次商品必须录批次
		if docEffects[req.DocType] == effectIn && p.IsBatchEnabled && it.BatchNo == "" {
			return nil, errcode.New(2002, "商品「"+p.Name+"」启用批次管理，请填写批次号")
		}
		// 库位：移库单必须指定移出/移入（0 表示「未分配」，允许——存量上架要从未分配搬出）；
		// 入库与盘点未指定时带出商品默认库位；出库类不填即为 0，由库存引擎自动分配
		locID, locName, toLocID, toLocName := int64(0), "", int64(0), ""
		if locScoped {
			// 按库位盘点：库位必须显式给出（0=未分配，用于盘未上架的货），不做默认库位回填，
			// 否则「这个货架没货」会被带成默认库位，盘成别的架子的账
			lid, lname, err := locSnapshot(tx, it.LocationID)
			if err != nil {
				return nil, errcode.New(4004, "库位不存在")
			}
			key := fmt.Sprintf("%d:%d", p.ID, lid)
			if seen[key] {
				return nil, errcode.New(1001, "商品「"+p.Name+"」在同一库位重复出现，请合并为一行实盘数")
			}
			seen[key] = true
			locID, locName = lid, lname
		} else if req.DocType == "transfer" {
			if it.LocationID == it.ToLocationID {
				return nil, errcode.New(1001, "商品「"+p.Name+"」移出与移入库位不能相同")
			}
			lid, lname, err := locSnapshot(tx, it.LocationID)
			if err != nil {
				return nil, errcode.New(4004, "移出库位不存在")
			}
			tid, tname, err := locSnapshot(tx, it.ToLocationID)
			if err != nil {
				return nil, errcode.New(4004, "移入库位不存在")
			}
			locID, locName, toLocID, toLocName = lid, lname, tid, tname
		} else if it.LocationID > 0 {
			var loc model.Location
			if err := tx.First(&loc, it.LocationID).Error; err != nil {
				return nil, errcode.New(4004, "库位不存在")
			}
			locID, locName = loc.ID, loc.Name
		} else if e := docEffects[req.DocType]; (e == effectIn || e == effectAdjust) && p.DefaultLocationID > 0 {
			var loc model.Location
			if err := tx.First(&loc, p.DefaultLocationID).Error; err == nil {
				locID, locName = loc.ID, loc.Name
			}
		}
		doc.Items = append(doc.Items, model.DocumentItem{
			ProductID: p.ID, ProductCode: p.Code, ProductName: p.Name,
			Quantity: it.Quantity, Price: it.Price, PriceName: it.PriceName,
			BatchNo: it.BatchNo, LocationID: locID, LocationName: locName,
			ToLocationID: toLocID, ToLocationName: toLocName,
			Amount: it.Quantity * it.Price / 1000, Remark: it.Remark,
		})
		totalQty += it.Quantity
		totalAmount += it.Quantity * it.Price / 1000
	}
	doc.TotalQty, doc.TotalAmount = totalQty, totalAmount
	_ = wh
	return doc, nil
}

func partnerKindName(kind string) string {
	if kind == "supplier" {
		return "供应商"
	}
	return "客户"
}

// qtyText ×1000 数量 → 可读文本（去尾零）
func qtyText(q int64) string {
	s := fmt.Sprintf("%.3f", float64(q)/1000)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func signedQty(q int64) string {
	if q > 0 {
		return "+" + qtyText(q)
	}
	return qtyText(q)
}

func joinRemark(orig, extra string) string {
	if orig == "" {
		return extra
	}
	return orig + "；" + extra
}

func partnerName(tx *gorm.DB, kind string, id int64) (string, error) {
	if kind == "supplier" {
		var p model.Supplier
		if err := tx.First(&p, id).Error; err != nil {
			return "", errcode.New(4004, "供应商不存在")
		}
		return p.Name, nil
	}
	var p model.Customer
	if err := tx.First(&p, id).Error; err != nil {
		return "", errcode.New(4004, "客户不存在")
	}
	return p.Name, nil
}

// ---------- 创建（含确认） ----------

func (s *DocumentService) Create(ctx context.Context, req DocReq, userID int64) (*model.Document, error) {
	for attempt := 0; attempt < 3; attempt++ {
		doc, err := s.createOnce(ctx, req, userID)
		if err == errcode.ErrDocNoDup {
			continue // 单号并发冲突重试
		}
		return doc, err
	}
	return nil, errcode.ErrDocNoDup
}

func (s *DocumentService) createOnce(ctx context.Context, req DocReq, userID int64) (*model.Document, error) {
	var created *model.Document
	err := s.db(ctx).Transaction(func(tx *gorm.DB) error {
		doc, err := s.validateAndBuild(ctx, tx, &req, userID)
		if err != nil {
			return err
		}
		no, err := docno.Next(tx, req.DocType, doc.DocDate)
		if err != nil {
			return err
		}
		doc.DocNo = no
		// Omit 防止 GORM 关联自动插入 Items（下方手动控制插入时机）
		if err := tx.Omit("Items").Create(doc).Error; err != nil {
			return err
		}
		for i := range doc.Items {
			doc.Items[i].DocumentID = doc.ID
			if err := tx.Create(&doc.Items[i]).Error; err != nil {
				return err
			}
		}
		if req.ConfirmNow {
			if err := s.confirmTx(ctx, tx, doc); err != nil {
				return err
			}
		}
		created = doc
		return nil
	})
	return created, err
}

// ---------- 确认（生效，触发库存联动） ----------

func (s *DocumentService) Confirm(ctx context.Context, id int64, userID int64) error {
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.Document
		if err := tx.Preload("Items").First(&doc, id).Error; err != nil {
			return errcode.ErrNotFound
		}
		return s.confirmTx(ctx, tx, &doc)
	})
}

func (s *DocumentService) confirmTx(ctx context.Context, tx *gorm.DB, doc *model.Document) error {
	if doc.Status != "draft" {
		return errcode.ErrDocStatus
	}
	effect := docEffects[doc.DocType]
	// 移库单：库位间搬移，不动仓库总量与成本，单独分支处理
	if doc.DocType == "transfer" {
		wh, err := model.GetMainWarehouse(tx)
		if err != nil {
			return err
		}
		invSvc := &InventoryService{conn: s.db(ctx)}
		for i := range doc.Items {
			item := &doc.Items[i]
			var p model.Product
			if err := tx.First(&p, item.ProductID).Error; err != nil {
				return errcode.New(4004, "商品不存在")
			}
			// 移入库位容量校验（默认只提示，可到系统设置开启硬拦截）
			if err := checkCapacity(tx, wh.ID, item.ToLocationID, item.Quantity, item.ProductID); err != nil {
				return err
			}
			if err := invSvc.Transfer(ctx, tx, wh.ID, item.LocationID, item.ToLocationID, &p, item.Quantity, &doc.ID); err != nil {
				return err
			}
		}
	} else if effect != effectNone {
		wh, err := model.GetMainWarehouse(tx)
		if err != nil {
			return err
		}
		invSvc := &InventoryService{conn: s.db(ctx)}
		if effect == effectAdjust {
			// 盘点：整单只做一次账面快照 + 一次调账。
			// 不能放在下面的明细循环里——每行都重拍一次快照会把已调账商品的账面改成调整后数量，
			// 差异全被抹平（差异表/审批/复盘全部失真）。
			for i := range doc.Items {
				if err := snapshotStocktakeBook(tx, doc, wh.ID, &doc.Items[i]); err != nil {
					return err
				}
			}
			// 差异超阈值（v0.16）：整单转待审批、不调账
			if over, why := stocktakeOverThreshold(tx, doc, wh.ID); over {
				for i := range doc.Items {
					if err := tx.Save(&doc.Items[i]).Error; err != nil {
						return err
					}
				}
				doc.Status = "review"
				doc.ReviewStatus = "pending"
				doc.Remark = joinRemark(doc.Remark, "差异超阈值（"+why+"），待主管审批")
				return tx.Save(doc).Error
			}
			if err := s.applyStocktakeAdjust(ctx, tx, doc, wh.ID, invSvc); err != nil {
				return err
			}
		} else {
			for i := range doc.Items {
				item := &doc.Items[i]
				var p model.Product
				if err := tx.First(&p, item.ProductID).Error; err != nil {
					return errcode.New(4004, "商品不存在")
				}
				switch effect {
				case effectIn:
					// 成本 = 单价（采购入库/其他入库）；销售退货回冲原销售成本
					cost := item.Price
					if doc.DocType == "sales_return" {
						cost = s.originalSaleCost(ctx, tx, doc, item)
					}
					// 入库库位容量校验（盘点盘盈不受限，账实必须如实记录）
					if err := checkCapacity(tx, wh.ID, item.LocationID, item.Quantity, item.ProductID); err != nil {
						return err
					}
					if err := invSvc.ApplyIn(ctx, tx, wh.ID, item.LocationID, &p, item.Quantity, cost, item.BatchNo, &doc.ID); err != nil {
						return err
					}
					item.Cost = cost
				case effectOut:
					// 明细未指定库位时传 0，由引擎自动分配（默认库位优先）
					cost, err := invSvc.ApplyOut(ctx, tx, wh.ID, item.LocationID, &p, item.Quantity, &doc.ID)
					if err != nil {
						return err
					}
					item.Cost = cost
				}
				if err := tx.Save(item).Error; err != nil {
					return err
				}
			}
		}
	}
	now := time.Now()
	doc.Status = "confirmed"
	doc.ConfirmedAt = &now
	return tx.Save(doc).Error
}

// ---------- 盘点调账与差异审批（v0.16） ----------

// snapshotStocktakeBook 记录确认时点的账面数快照（差异表/复盘/审批都依赖它，事后取不到）
func snapshotStocktakeBook(tx *gorm.DB, doc *model.Document, whID int64, item *model.DocumentItem) error {
	if doc.Scope == "location" {
		li, err := locInvOf(tx, item.ProductID, whID, item.LocationID)
		if err != nil {
			return err
		}
		item.BookQty = li.Quantity
		return nil
	}
	inv, err := invOf(tx, item.ProductID, whID)
	if err != nil {
		return err
	}
	item.BookQty = inv.Quantity
	return nil
}

// stocktakeOverThreshold 盘点差异是否超过审批阈值（读系统设置 stocktake_review_*）。
// 差异率 = |diff| / 账面数（账面为 0 的行无基线，不参与差异率，由金额阈值兜底）；
// 差异金额按仓库均价估算（确认时成本尚未回填）。命中任一规则即整单待审批。
func stocktakeOverThreshold(tx *gorm.DB, doc *model.Document, whID int64) (bool, string) {
	if !model.GetBoolSetting(tx, "stocktake_review_enabled", false) {
		return false, ""
	}
	percent := model.GetIntSetting(tx, "stocktake_review_percent", 10)
	amount := model.GetIntSetting(tx, "stocktake_review_amount", 0)
	if percent <= 0 && amount <= 0 {
		return false, ""
	}
	for i := range doc.Items {
		item := &doc.Items[i]
		diff := item.Quantity - item.BookQty
		if diff == 0 {
			continue
		}
		if percent > 0 && item.BookQty > 0 {
			rate := absI64(diff) * 100 / item.BookQty
			if rate > int64(percent) {
				return true, fmt.Sprintf("%s 差异率 %d%%>阈值%d%%", item.ProductName, rate, percent)
			}
		}
		if amount > 0 {
			inv, err := invOf(tx, item.ProductID, whID)
			if err == nil && inv.AvgCost > 0 {
				amt := absI64(diff) * inv.AvgCost / 1000
				if amt >= int64(amount) {
					return true, fmt.Sprintf("%s 差异金额 ¥%.2f≥阈值", item.ProductName, float64(amt)/100)
				}
			}
		}
	}
	return false, ""
}

// applyStocktakeAdjust 执行盘点调账：差异 = 实盘数 - 明细已存的账面快照（BookQty）。
// 确认与审批通过共用本函数——都基于快照算差异，避免两次读账造成口径漂移。
func (s *DocumentService) applyStocktakeAdjust(ctx context.Context, tx *gorm.DB, doc *model.Document, whID int64, invSvc *InventoryService) error {
	for i := range doc.Items {
		item := &doc.Items[i]
		var p model.Product
		if err := tx.First(&p, item.ProductID).Error; err != nil {
			return errcode.New(4004, "商品不存在")
		}
		diff := item.Quantity - item.BookQty
		if doc.Scope == "location" {
			// 按库位盘点：差额就地调整该货架，仓库总量随之同步，成本仍按仓库均价
			if diff > 0 {
				inv, err := invOf(tx, item.ProductID, whID)
				if err != nil {
					return err
				}
				cost := inv.AvgCost
				if item.Price > 0 {
					cost = item.Price // 盘盈可指定成本价
				}
				batchNo := item.BatchNo
				if batchNo == "" {
					batchNo = "盘点盈"
				}
				if err := invSvc.ApplyIn(ctx, tx, whID, item.LocationID, &p, diff, cost, batchNo, &doc.ID); err != nil {
					return err
				}
				item.Cost = cost
			} else if diff < 0 {
				c, err := invSvc.ApplyOut(ctx, tx, whID, item.LocationID, &p, -diff, &doc.ID)
				if err != nil {
					return err
				}
				item.Cost = c
			}
			if diff != 0 {
				item.Remark = joinRemark(item.Remark, "库位「"+item.LocationName+"」账面"+qtyText(item.BookQty)+"，盈亏"+signedQty(diff))
			}
		} else {
			if diff > 0 {
				inv, err := invOf(tx, item.ProductID, whID)
				if err != nil {
					return err
				}
				cost := inv.AvgCost
				if item.Price > 0 {
					cost = item.Price
				}
				batchNo := item.BatchNo
				if batchNo == "" {
					batchNo = "盘点盈"
				}
				// 盘盈进明细指定库位（默认带出商品默认库位），未指定则进未分配
				if err := invSvc.ApplyIn(ctx, tx, whID, item.LocationID, &p, diff, cost, batchNo, &doc.ID); err != nil {
					return err
				}
				item.Cost = cost
			} else if diff < 0 {
				// 盘亏是仓库级差额，库位由引擎按默认库位优先自动分配
				c, err := invSvc.ApplyOut(ctx, tx, whID, 0, &p, -diff, &doc.ID)
				if err != nil {
					return err
				}
				item.Cost = c
			}
			if diff != 0 {
				item.Remark = joinRemark(item.Remark, "账面"+qtyText(item.BookQty)+"，盈亏"+signedQty(diff))
			}
		}
		if err := tx.Save(item).Error; err != nil {
			return err
		}
	}
	return nil
}

// Review 盘点差异审批：approve=true 按账面快照调账并生效；false 驳回作废（不调账，需重新盘点）
func (s *DocumentService) Review(ctx context.Context, id int64, approve bool, note string, userID int64) error {
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.Document
		if err := tx.Preload("Items").First(&doc, id).Error; err != nil {
			return errcode.ErrNotFound
		}
		if doc.DocType != "stocktake" || doc.Status != "review" {
			return errcode.ErrDocNotReview
		}
		now := time.Now()
		doc.ReviewBy = userID
		doc.ReviewAt = &now
		doc.ReviewNote = note
		if approve {
			wh, err := model.GetMainWarehouse(tx)
			if err != nil {
				return err
			}
			invSvc := &InventoryService{conn: s.db(ctx)}
			if err := s.applyStocktakeAdjust(ctx, tx, &doc, wh.ID, invSvc); err != nil {
				return err
			}
			doc.Status = "confirmed"
			doc.ReviewStatus = "approved"
			doc.ConfirmedAt = &now
			doc.Remark = joinRemark(doc.Remark, "主管审批通过"+noteText(note))
		} else {
			doc.Status = "voided"
			doc.ReviewStatus = "rejected"
			doc.VoidedAt = &now
			doc.VoidReason = "审批驳回：" + note
		}
		return tx.Save(&doc).Error
	})
}

func noteText(note string) string {
	if strings.TrimSpace(note) == "" {
		return ""
	}
	return "：" + note
}

func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// locSnapshot 库位快照：0 表示未分配（合法，用于存量上架）
func locSnapshot(tx *gorm.DB, id int64) (int64, string, error) {
	if id <= 0 {
		return 0, "未分配", nil
	}
	var loc model.Location
	if err := tx.First(&loc, id).Error; err != nil {
		return 0, "", errcode.New(4004, "库位不存在")
	}
	return loc.ID, loc.Name, nil
}

// originalSaleCost 销售退货取原销售单明细的结转成本；无原单则用当前均价
func (s *DocumentService) originalSaleCost(ctx context.Context, tx *gorm.DB, doc *model.Document, item *model.DocumentItem) int64 {
	if doc.RefDocID > 0 {
		var orig model.DocumentItem
		err := tx.Where("document_id = ? AND product_id = ?", doc.RefDocID, item.ProductID).First(&orig).Error
		if err == nil && orig.Cost > 0 {
			return orig.Cost
		}
	}
	var inv model.Inventory
	if err := tx.Where("product_id = ?", item.ProductID).First(&inv).Error; err == nil && inv.AvgCost > 0 {
		return inv.AvgCost
	}
	return item.Price
}

// ---------- 作废（回冲） ----------

func (s *DocumentService) Void(ctx context.Context, id int64, reason string) error {
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		var doc model.Document
		if err := tx.Preload("Items").First(&doc, id).Error; err != nil {
			return errcode.ErrNotFound
		}
		if doc.Status != "confirmed" && doc.Status != "review" {
			return errcode.New(3001, "只有已生效或待审批单据可以作废")
		}
		if doc.Status == "confirmed" && (docEffects[doc.DocType] != effectNone || doc.DocType == "transfer") {
			invSvc := &InventoryService{conn: s.db(ctx)}
			if err := invSvc.ReverseDoc(ctx, tx, doc.ID, doc.DocType); err != nil {
				return err
			}
		}
		if doc.Status == "review" {
			// 待审批单作废 = 开单人撤回：尚未调账，无需回冲
			doc.ReviewStatus = "rejected"
			doc.ReviewNote = joinRemark(doc.ReviewNote, "开单人作废撤回")
		}
		now := time.Now()
		doc.Status = "voided"
		doc.VoidedAt = &now
		doc.VoidReason = reason
		return tx.Save(&doc).Error
	})
}

// ---------- 查询 ----------

type DocListReq struct {
	DocType  string
	Status   string
	Keyword  string
	DateFrom string
	DateTo   string
	Page     int
	PageSize int
}

func (s *DocumentService) List(ctx context.Context, req DocListReq) ([]model.Document, int64, error) {
	q := s.db(ctx).Model(&model.Document{})
	if req.DocType != "" {
		q = q.Where("doc_type = ?", req.DocType)
	}
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	}
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		q = q.Where("doc_no LIKE ? OR partner_name LIKE ?", kw, kw)
	}
	if req.DateFrom != "" {
		q = q.Where("doc_date >= ?", req.DateFrom)
	}
	if req.DateTo != "" {
		q = q.Where("doc_date <= ?", req.DateTo+" 23:59:59")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Document
	err := q.Omit("items").Order("id DESC").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&list).Error
	return list, total, err
}

func (s *DocumentService) Get(ctx context.Context, id int64) (*model.Document, error) {
	var doc model.Document
	if err := s.db(ctx).Preload("Items").First(&doc, id).Error; err != nil {
		return nil, errcode.ErrNotFound
	}
	return &doc, nil
}
