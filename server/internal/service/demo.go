package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// DemoService 演示数据一键初始化（支持多行业模板）
// 用途：给潜在客户/新店演示。流程：清空业务数据（保留用户/角色/系统设置/授权）→
// 写入档案与期初 → 通过单据引擎生成近两个月采购/销售/盘点等单据（保证库存、
// 成本、应收应付与报表数据全部自洽）→ 生成部分收付款形成余额。
// 行业差异全部收敛在 demo_profiles.go 的数据里，本文件只负责生成流程。
type DemoService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *DemoService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewDemoService(db *gorm.DB) *DemoService { return &DemoService{conn: db} }

type DemoSummary struct {
	Template       string `json:"template"`
	TemplateName   string `json:"template_name"`
	Categories     int64  `json:"categories"`
	Products       int64  `json:"products"`
	Suppliers      int64  `json:"suppliers"`
	Customers      int64  `json:"customers"`
	Locations      int64  `json:"locations"`
	PurchaseDocs   int64  `json:"purchase_docs"`
	SalesDocs      int64  `json:"sales_docs"`
	OtherDocs      int64  `json:"other_docs"`
	Payments       int64  `json:"payments"`
	StockValueView int64  `json:"stock_value_view"` // 元（展示用）
}

// DemoTemplateInfo 模板清单（前端选择场景用，不含商品明细）
type DemoTemplateInfo struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Scene    string `json:"scene"`
	Detail   string `json:"detail"`
	Shop     string `json:"shop"`
	Products int    `json:"products"`
	Sales    int    `json:"sales_docs"`
}

// Templates 可选演示模板
func (s *DemoService) Templates(ctx context.Context) []DemoTemplateInfo {
	out := make([]DemoTemplateInfo, 0, len(demoProfiles))
	for _, p := range demoProfiles {
		out = append(out, DemoTemplateInfo{
			Key: p.Key, Name: p.Name, Scene: p.Scene, Detail: p.Detail,
			Shop: p.ShopName, Products: len(p.Products), Sales: p.SalesDocs,
		})
	}
	return out
}

// HasTenants 是否存在 SaaS 租户：存在时禁止未登录的公开重置（否则会清空所有租户数据）
func (s *DemoService) HasTenants(ctx context.Context) bool {
	var n int64
	_ = s.conn.Model(&model.Tenant{}).Count(&n).Error
	return n > 0
}

// ---------- 主流程 ----------

func (s *DemoService) Reset(ctx context.Context, userID int64, templateKey string) (*DemoSummary, error) {
	prof, ok := demoProfileByKey(templateKey)
	if !ok {
		return nil, errcode.New(1001, "演示模板不存在："+templateKey)
	}
	// 1) 清空业务数据（硬删，保留 users/roles/settings/license/warehouses/audit_logs）
	tables := []interface{}{
		&model.DocumentItem{}, &model.Document{}, &model.InventoryMovement{},
		&model.InventoryBatch{}, &model.Inventory{}, &model.LocationInventory{}, &model.Payment{},
		&model.ProductPrice{}, &model.Product{}, &model.Category{},
		&model.Supplier{}, &model.Customer{}, &model.Location{},
	}
	if err := s.db(ctx).Transaction(func(tx *gorm.DB) error {
		for _, t := range tables {
			if err := tx.Unscoped().Where("1 = 1").Delete(t).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewSource(prof.Seed)) // 固定种子：同一模板每次生成的数据一致
	sum := &DemoSummary{Template: prof.Key, TemplateName: prof.Name}
	// 演示数据里部分货架接近满容，若开着「库位超容拦截」会把演示单据卡在 2005。
	// 演示初始化是展示场景，强制关闭拦截（用户之后可自行开启）。
	if err := model.SetSetting(s.db(ctx), "location_block_overflow", "false"); err != nil {
		return nil, err
	}
	adminID := userID
	if adminID <= 0 {
		var u model.User
		if err := s.db(ctx).Where("username = ?", "admin").First(&u).Error; err == nil {
			adminID = u.ID
		}
	}

	// 小票抬头（v0.14）：每个模板带一套自己的门店抬头，方便直接看 58mm 小票效果
	for k, v := range map[string]string{
		"shop_name":     prof.ShopName,
		"shop_phone":    prof.ShopPhone,
		"shop_addr":     prof.ShopAddr,
		"ticket_footer": prof.TicketFooter,
	} {
		if err := model.SetSetting(s.db(ctx), k, v); err != nil {
			return nil, err
		}
	}

	// 2) 库位
	locs := make([]model.Location, 0, len(prof.Locations))
	for _, l := range prof.Locations {
		loc := model.Location{
			Code: l.Code, Name: l.Name, Remark: l.Remark, CapacityMode: l.Mode,
			Capacity: l.Capacity, MinQty: l.MinQty, WarnRatio: 90,
		}
		if err := s.db(ctx).Create(&loc).Error; err != nil {
			return nil, err
		}
		locs = append(locs, loc)
	}
	sum.Locations = int64(len(locs))

	// 3) 分类
	cats := make([]model.Category, 0, len(prof.Cats))
	for i, c := range prof.Cats {
		cat := model.Category{Name: c.Name, Sort: i}
		if err := s.db(ctx).Create(&cat).Error; err != nil {
			return nil, err
		}
		cats = append(cats, cat)
	}
	sum.Categories = int64(len(cats))

	// 4) 供应商/客户
	for _, sp := range prof.Suppliers {
		if err := s.db(ctx).Create(&model.Supplier{
			Code: sp.Code, Name: sp.Name, Contact: sp.Contact, Phone: sp.Phone,
			Address: sp.Addr, OpeningPayable: sp.Opening,
		}).Error; err != nil {
			return nil, err
		}
	}
	sum.Suppliers = int64(len(prof.Suppliers))
	for _, cu := range prof.Customers {
		if err := s.db(ctx).Create(&model.Customer{
			Code: cu.Code, Name: cu.Name, Contact: cu.Contact, Phone: cu.Phone,
			Address: cu.Addr, OpeningReceivable: cu.Opening,
		}).Error; err != nil {
			return nil, err
		}
	}
	sum.Customers = int64(len(prof.Customers))

	// 5) 商品（默认价位=第一个层级，其余价位并存；批次商品带保质期）
	prodIDs := make([]int64, 0, len(prof.Products))
	prodCost := map[int64]int64{}
	priceMap := map[int64]map[string]int64{} // product_id → 价位名 → 价格
	for _, dp := range prof.Products {
		if dp.Cat < 0 || dp.Cat >= len(cats) {
			return nil, fmt.Errorf("演示模板 %s 商品 %s 的分类下标越界", prof.Key, dp.Code)
		}
		p := model.Product{
			Code: dp.Code, Barcode: dp.Barcode, Name: dp.Name, PLU: dp.PLU,
			CategoryID: cats[dp.Cat].ID, Unit: dp.Unit,
			CostMethod: "weighted_avg", IsBatchEnabled: dp.Batch, ShelfLifeDays: dp.ShelfDays,
			SafeStock: dp.Safe * 1000, Volume: dp.Volume, Weight: dp.Weight,
			Status: "active", DefaultLocationID: locs[prof.Cats[dp.Cat].Loc].ID,
		}
		if dp.Batch {
			// 批次商品走 FIFO：出库才会真正扣减批次结存，批次明细/临期预警才与库存对得上
			p.CostMethod = "fifo"
		}
		if err := s.db(ctx).Create(&p).Error; err != nil {
			return nil, err
		}
		prices := make([]model.ProductPrice, 0, len(prof.PriceTiers))
		pm := map[string]int64{}
		for i, tier := range prof.PriceTiers {
			pct := int64(tier.Pct)
			if i == 0 && tier.Jitter > 0 {
				pct += int64(rng.Intn(tier.Jitter + 1))
			}
			price := dp.Cost * pct / 100
			prices = append(prices, model.ProductPrice{ProductID: p.ID, Name: tier.Name, Price: price, Sort: i})
			pm[tier.Name] = price
		}
		if err := s.db(ctx).Create(&prices).Error; err != nil {
			return nil, err
		}
		if err := s.db(ctx).Model(&p).Update("default_price_id", prices[0].ID).Error; err != nil {
			return nil, err
		}
		prodIDs = append(prodIDs, p.ID)
		prodCost[p.ID] = dp.Cost
		priceMap[p.ID] = pm
	}
	sum.Products = int64(len(prodIDs))

	// 6) 期初库存（批次商品带批次号）
	openingSvc := NewOpeningService(s.db(ctx))
	openRows := make([]OpeningReq, 0, len(prodIDs))
	for i, pid := range prodIDs {
		dp := prof.Products[i]
		row := OpeningReq{
			ProductID: pid, Quantity: dp.Opening * 1000, UnitCost: dp.Cost,
			LocationID: locs[prof.Cats[dp.Cat].Loc].ID,
		}
		if dp.Batch {
			row.BatchNo = prof.batchNo(60)
		}
		openRows = append(openRows, row)
	}
	if err := openingSvc.Save(ctx, openRows); err != nil {
		return nil, fmt.Errorf("期初建账失败: %w", err)
	}

	// 7) 单据：先采购后销售，走单据引擎保证数据自洽
	docSvc := NewDocumentService(s.db(ctx))
	day := func(daysAgo int) string { return time.Now().AddDate(0, 0, -daysAgo).Format("2006-01-02") }
	supplierIDs := s.ids(ctx, &model.Supplier{})
	customerIDs := s.ids(ctx, &model.Customer{})
	wholesale := map[int64]bool{}
	for i, cu := range prof.Customers {
		wholesale[customerIDs[i]] = cu.Wholesale
	}

	// 采购入库：跨度前 2/3，成本价
	purchaseFrom, purchaseTo := prof.SpanDays, prof.SpanDays/2
	if purchaseTo < 1 {
		purchaseTo = 1
	}
	for i := 0; i < prof.PurchaseDocs; i++ {
		items := s.pick(ctx, rng, prof, prodIDs, prodCost, 2+rng.Intn(4), true)
		req := DocReq{
			DocType: "purchase_in", PartnerID: supplierIDs[rng.Intn(len(supplierIDs))],
			DocDate: day(spreadDay(purchaseFrom, purchaseTo, i, prof.PurchaseDocs, rng)), ConfirmNow: true,
			Remark: "正常补货", Items: items,
		}
		if _, err := docSvc.Create(ctx, req, adminID); err != nil {
			return nil, fmt.Errorf("生成采购单失败: %w", err)
		}
		sum.PurchaseDocs++
	}

	// 销售出库：按当前库存限流避免负库存，批发/会员客户走对应价位
	salesFrom := prof.SpanDays * 5 / 6
	if salesFrom < 2 {
		salesFrom = 2
	}
	for i := 0; i < prof.SalesDocs; i++ {
		cu := customerIDs[rng.Intn(len(customerIDs))]
		items := s.pickSales(ctx, rng, s.db(ctx), prodIDs, priceMap, 1+rng.Intn(4), prof.salesPriceName(wholesale[cu]))
		if len(items) == 0 {
			continue
		}
		d := spreadDay(salesFrom, 0, i, prof.SalesDocs, rng)
		if i >= prof.SalesDocs-3 { // 最后 3 张落在今天：首页看板「今日销售」不为 0
			d = 0
		}
		req := DocReq{
			DocType: "sales_out", PartnerID: cu, DocDate: day(d), ConfirmNow: true,
			Items: items,
		}
		if _, err := docSvc.Create(ctx, req, adminID); err != nil {
			return nil, fmt.Errorf("生成销售单失败: %w", err)
		}
		sum.SalesDocs++
	}

	// 其他入库（厂家赠品/产地赠品）/ 其他出库（自用耗材、生鲜损耗）
	nonBatch := s.nonBatchIdx(ctx, prof)
	if len(nonBatch) == 0 {
		nonBatch = []int{0}
	}
	inIdx := nonBatch[rng.Intn(len(nonBatch))]
	inQty := pickQty(rng, prof.Products[inIdx].Pack)
	inItem := DocItemReq{ProductID: prodIDs[inIdx], Quantity: inQty, Price: 0, Remark: prof.OtherInRemark}
	if prof.Products[inIdx].Batch {
		inItem.BatchNo = prof.batchNo(20)
	}
	_, _ = docSvc.Create(ctx, DocReq{
		DocType: "other_in", DocDate: day(prof.SpanDays / 3), ConfirmNow: true,
		Remark: prof.OtherInRemark, Items: []DocItemReq{inItem},
	}, adminID)

	outItems := make([]DocItemReq, 0, 2)
	for k := 0; k < 2; k++ {
		idx := nonBatch[rng.Intn(len(nonBatch))]
		if prodIDs[idx] == inItem.ProductID && len(nonBatch) > 1 {
			idx = nonBatch[(rng.Intn(len(nonBatch))+1)%len(nonBatch)]
		}
		outItems = append(outItems, DocItemReq{
			ProductID: prodIDs[idx], Quantity: pickQty(rng, prof.Products[idx].Pack), Price: prodCost[prodIDs[idx]],
			Remark: prof.OtherOutRemark,
		})
	}
	_, _ = docSvc.Create(ctx, DocReq{
		DocType: "other_out", DocDate: day(prof.SpanDays / 6), ConfirmNow: true,
		Remark: prof.OtherOutRemark, Items: outItems,
	}, adminID)
	sum.OtherDocs += 2

	// 盘点：随机抽若干商品，实盘=账面±1~3（批次商品避免盘成 0，否则需指定批次）
	var stItems []DocItemReq
	for _, idx := range rng.Perm(len(prodIDs)) {
		if len(stItems) >= prof.StocktakeItems {
			break
		}
		pid := prodIDs[idx]
		var inv model.Inventory
		if err := s.db(ctx).Where("product_id = ?", pid).First(&inv).Error; err != nil || inv.Quantity <= 3000 {
			continue
		}
		delta := int64(1 + rng.Intn(3))
		if rng.Intn(2) == 0 {
			delta = -delta
		}
		q := inv.Quantity + delta*1000
		if q <= 0 {
			continue
		}
		stItems = append(stItems, DocItemReq{ProductID: pid, Quantity: q})
	}
	if len(stItems) > 0 {
		if _, err := docSvc.Create(ctx, DocReq{
			DocType: "stocktake", DocDate: day(prof.SpanDays / 10), ConfirmNow: true,
			Remark: "月度抽盘", Items: stItems,
		}, adminID); err != nil {
			return nil, fmt.Errorf("生成盘点单失败: %w", err)
		}
		sum.OtherDocs++
	}

	// 报价单 2 张（草稿）+ 采购订单 1 张（草稿）—— 演示单据状态流转
	for i, remark := range prof.Quotations {
		_, _ = docSvc.Create(ctx, DocReq{
			DocType: "quotation", PartnerID: customerIDs[(i+1)%len(customerIDs)], DocDate: day(3 - i),
			Remark: remark, Items: s.pick(ctx, rng, prof, prodIDs, prodCost, 4, false),
		}, adminID)
		sum.OtherDocs++
	}
	_, _ = docSvc.Create(ctx, DocReq{
		DocType: "purchase_order", PartnerID: supplierIDs[0], DocDate: day(1),
		Remark: prof.PurchaseOrderRemark, Items: s.pick(ctx, rng, prof, prodIDs, prodCost, 3, false),
	}, adminID)
	sum.OtherDocs++

	// 8) 收付款：给部分销售单收款、部分采购单付款，形成应收应付余额
	finSvc := NewFinanceService(s.db(ctx))
	var sales, purchases []model.Document
	s.db(ctx).Where("doc_type = ? AND status = ?", "sales_out", "confirmed").Find(&sales)
	s.db(ctx).Where("doc_type = ? AND status = ?", "purchase_in", "confirmed").Find(&purchases)
	for _, d := range sales {
		if rng.Intn(10) < 6 { // 60% 已收款
			methods := []string{"cash", "wechat", "alipay", "bank"}
			_, err := finSvc.CreatePayment(ctx, PaymentReq{
				Kind: "receipt", PartnerID: d.PartnerID, Amount: d.TotalAmount,
				Method: methods[rng.Intn(len(methods))], DocDate: d.DocDate.Format("2006-01-02"),
				Remark: "货款结清",
			}, adminID)
			if err == nil {
				sum.Payments++
			}
		}
	}
	for _, d := range purchases {
		if rng.Intn(10) < 5 { // 50% 已付款
			_, err := finSvc.CreatePayment(ctx, PaymentReq{
				Kind: "payment", PartnerID: d.PartnerID, Amount: d.TotalAmount,
				Method: "bank", DocDate: d.DocDate.Format("2006-01-02"), Remark: "货款结清",
			}, adminID)
			if err == nil {
				sum.Payments++
			}
		}
	}

	var inv model.Inventory
	s.db(ctx).Model(&model.Inventory{}).
		Select("COALESCE(SUM(total_value),0) AS total_value").
		Scan(&inv)
	sum.StockValueView = inv.TotalValue / 100
	return sum, nil
}

// ---------- 辅助 ----------

// batchNo 批次号：生鲜按日（P20261104），其他按月（B202609）
func (p *demoProfile) batchNo(daysAgo int) string {
	t := time.Now().AddDate(0, 0, -daysAgo)
	if p.BatchDaily {
		return p.BatchPrefix + t.Format("20060102")
	}
	return p.BatchPrefix + t.Format("200601")
}

// salesPriceName 客户价位：批发客户优先批发价，其次会员价，最后默认（零售）价
func (p *demoProfile) salesPriceName(wholesale bool) string {
	if wholesale {
		for _, t := range p.PriceTiers {
			if t.Name == "批发价" {
				return "批发价"
			}
		}
		for _, t := range p.PriceTiers {
			if t.Name == "会员价" {
				return "会员价"
			}
		}
	}
	return p.PriceTiers[0].Name
}

// nonBatchIdx 非批次商品下标（用于其他出入库、避免必须录批次号）
func (s *DemoService) nonBatchIdx(ctx context.Context, p *demoProfile) []int {
	out := make([]int, 0, len(p.Products))
	for i, dp := range p.Products {
		if !dp.Batch {
			out = append(out, i)
		}
	}
	return out
}

// spreadDay 把 n 张单据均匀铺在 [from, to] 天前，带 0~1 天随机抖动
func spreadDay(from, to, i, n int, rng *rand.Rand) int {
	if n <= 1 || from <= to {
		return from
	}
	d := from - (from-to)*i/(n-1)
	if d < 0 {
		d = 0
	}
	if rng.Intn(2) == 1 && d > to {
		d--
	}
	return d
}

// pickQty 采购/其他出入库数量：0.5~1.5 倍采购批量，至少 1、最多 500（基本单位）
func pickQty(rng *rand.Rand, pack int64) int64 {
	if pack <= 0 {
		pack = 3
	}
	q := pack/2 + int64(rng.Intn(int(pack)+1)) + 1
	if q < 1 {
		q = 1
	}
	if q > 500 {
		q = 500
	}
	return q * 1000
}

func (s *DemoService) ids(ctx context.Context, m interface{}) []int64 {
	var out []int64
	s.db(ctx).Model(m).Pluck("id", &out)
	return out
}

// pick 采购/报价用：随机挑 n 个商品（cost=true 时用成本价）
func (s *DemoService) pick(ctx context.Context, rng *rand.Rand, prof *demoProfile, prodIDs []int64, prodCost map[int64]int64, n int, cost bool) []DocItemReq {
	seen := map[int]bool{}
	items := make([]DocItemReq, 0, n)
	for len(items) < n && len(seen) < len(prodIDs) {
		i := rng.Intn(len(prodIDs))
		if seen[i] {
			continue
		}
		seen[i] = true
		pid := prodIDs[i]
		dp := prof.Products[i]
		price := prodCost[pid]
		if !cost {
			price = price * int64(prof.PriceTiers[0].Pct+rng.Intn(prof.PriceTiers[0].Jitter+1)) / 100 // 报价按零售口径
		}
		item := DocItemReq{ProductID: pid, Quantity: pickQty(rng, dp.Pack), Price: price}
		if dp.Batch { // 批次商品入库必须带批次号
			item.BatchNo = prof.batchNo(10)
		}
		items = append(items, item)
	}
	return items
}

// pickSales 销售用：按当前实际库存限流避免负库存，并按客户类型带价位
func (s *DemoService) pickSales(ctx context.Context, rng *rand.Rand, db *gorm.DB, prodIDs []int64, priceMap map[int64]map[string]int64, n int, priceName string) []DocItemReq {
	items := make([]DocItemReq, 0, n)
	for try := 0; try < n*4 && len(items) < n; try++ {
		i := rng.Intn(len(prodIDs))
		pid := prodIDs[i]
		var inv model.Inventory
		if err := db.Where("product_id = ?", pid).First(&inv).Error; err != nil || inv.Quantity <= 0 {
			continue
		}
		maxQty := inv.Quantity / 4 // 单张最多动用库存的 1/4
		if maxQty <= 0 {
			continue
		}
		qty := int64(1 + rng.Intn(int(maxQty/1000)))
		if qty <= 0 {
			qty = 1
		}
		price := priceMap[pid][priceName]
		if price == 0 {
			price = priceMap[pid]["零售价"]
		}
		items = append(items, DocItemReq{
			ProductID: pid, Quantity: qty * 1000,
			Price: price, PriceName: priceName,
		})
	}
	return items
}
