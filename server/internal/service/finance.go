package service

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// FinanceService 应收应付：余额 = 期初 + 单据发生额(销售出库/采购入库 − 退货) − 已收付金额
// 余额不落库、实时聚合计算，天然与单据/收付款单一致，无同步问题
type FinanceService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *FinanceService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewFinanceService(db *gorm.DB) *FinanceService { return &FinanceService{conn: db} }

// ---------- 收付款单 ----------

type PaymentReq struct {
	Kind      string `json:"kind" binding:"required"` // receipt | payment
	PartnerID int64  `json:"partner_id" binding:"required"`
	Amount    int64  `json:"amount" binding:"required"` // 分 > 0
	Method    string `json:"method"`
	DocDate   string `json:"doc_date"`
	Remark    string `json:"remark"`
}

var paymentMethods = map[string]bool{"cash": true, "wechat": true, "alipay": true, "bank": true}

func (s *FinanceService) CreatePayment(ctx context.Context, req PaymentReq, userID int64) (*model.Payment, error) {
	if req.Kind != "receipt" && req.Kind != "payment" {
		return nil, errcode.ErrParam
	}
	if req.Amount <= 0 {
		return nil, errcode.New(1001, "金额必须大于0")
	}
	if req.Method != "" && !paymentMethods[req.Method] {
		return nil, errcode.ErrParam
	}
	partnerType := "customer"
	if req.Kind == "payment" {
		partnerType = "supplier"
	}
	name, err := partnerName(s.db(ctx), partnerType, req.PartnerID)
	if err != nil {
		return nil, err
	}
	docDate := time.Now()
	if req.DocDate != "" {
		d, e := time.ParseInLocation("2006-01-02", req.DocDate, time.Local)
		if e != nil {
			return nil, errcode.ErrParam
		}
		docDate = d
	}

	p := &model.Payment{
		Kind: req.Kind, PartnerType: partnerType, PartnerID: req.PartnerID,
		PartnerName: name, Amount: req.Amount, Method: req.Method,
		DocDate: docDate, Remark: req.Remark, Status: "confirmed", CreatedBy: userID,
	}
	err = s.db(ctx).Transaction(func(tx *gorm.DB) error {
		no, e := s.nextNo(ctx, tx, req.Kind, docDate)
		if e != nil {
			return e
		}
		p.DocNo = no
		return tx.Create(p).Error
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// nextNo 收付款单号：SK20261104001 / FK20261104001（编号独立于 documents 表计数）
func (s *FinanceService) nextNo(ctx context.Context, tx *gorm.DB, kind string, d time.Time) (string, error) {
	prefix := "SK"
	if kind == "payment" {
		prefix = "FK"
	}
	base := prefix + d.Format("20060102")
	var count int64
	if err := tx.Model(&model.Payment{}).Where("doc_no LIKE ?", base+"%").Count(&count).Error; err != nil {
		return "", err
	}
	for n := count + 1; n <= count+100; n++ {
		no := fmt.Sprintf("%s%03d", base, n)
		var dup int64
		if err := tx.Model(&model.Payment{}).Where("doc_no = ?", no).Count(&dup).Error; err != nil {
			return "", err
		}
		if dup == 0 {
			return no, nil
		}
	}
	return "", errcode.New(3002, "单号生成冲突，请重试")
}

func (s *FinanceService) VoidPayment(ctx context.Context, id int64, reason string) error {
	return s.db(ctx).Transaction(func(tx *gorm.DB) error {
		var p model.Payment
		if err := tx.First(&p, id).Error; err != nil {
			return errcode.ErrNotFound
		}
		if p.Status != "confirmed" {
			return errcode.New(3001, "只有已确认的收付款单可以作废")
		}
		p.Status = "voided"
		p.VoidReason = reason
		return tx.Save(&p).Error
	})
}

type PaymentListReq struct {
	Kind      string
	PartnerID int64
	Page      int
	PageSize  int
}

func (s *FinanceService) ListPayments(ctx context.Context, req PaymentListReq) ([]model.Payment, int64, error) {
	q := s.db(ctx).Model(&model.Payment{})
	if req.Kind != "" {
		q = q.Where("kind = ?", req.Kind)
	}
	if req.PartnerID > 0 {
		q = q.Where("partner_id = ?", req.PartnerID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Payment
	err := q.Order("id DESC").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&list).Error
	return list, total, err
}

// ---------- 余额总览 ----------

type PartnerBalance struct {
	PartnerID int64  `json:"partner_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Opening   int64  `json:"opening"` // 期初（分）
	Biz       int64  `json:"biz"`     // 本期发生：销售出库+ / 退货−（或采购方向）
	Settled   int64  `json:"settled"` // 已收/已付（有效收付款单合计）
	Balance   int64  `json:"balance"` // 余额 = opening + biz − settled；>0 为欠款
}

// Balances kind=customer 返回应收（客户欠我们）；kind=supplier 返回应付（我们欠供应商）
func (s *FinanceService) Balances(ctx context.Context, kind string) ([]PartnerBalance, error) {
	partnerType := kind
	var bizTypes struct{ plus, minus string }
	if kind == "customer" {
		bizTypes = struct{ plus, minus string }{"sales_out", "sales_return"}
	} else if kind == "supplier" {
		bizTypes = struct{ plus, minus string }{"purchase_in", "purchase_return"}
	} else {
		return nil, errcode.ErrParam
	}

	// 单据发生额
	type bizRow struct {
		PartnerID int64
		Sum       int64
	}
	var bizRows []bizRow
	if err := s.db(ctx).Model(&model.Document{}).
		Select("partner_id, COALESCE(SUM(CASE WHEN doc_type = ? THEN total_amount WHEN doc_type = ? THEN -total_amount ELSE 0 END),0) as sum",
			bizTypes.plus, bizTypes.minus).
		Where("partner_type = ? AND status = ? AND partner_id > 0", partnerType, "confirmed").
		Group("partner_id").Scan(&bizRows).Error; err != nil {
		return nil, err
	}
	bizMap := map[int64]int64{}
	for _, r := range bizRows {
		bizMap[r.PartnerID] = r.Sum
	}

	// 已收付金额
	var payRows []bizRow
	if err := s.db(ctx).Model(&model.Payment{}).
		Select("partner_id, COALESCE(SUM(amount),0) as sum").
		Where("kind = ? AND status = ?", "receipt", "confirmed").
		Group("partner_id").Scan(&payRows).Error; err != nil {
		return nil, err
	}
	if kind == "supplier" {
		payRows = nil
		if err := s.db(ctx).Model(&model.Payment{}).
			Select("partner_id, COALESCE(SUM(amount),0) as sum").
			Where("kind = ? AND status = ?", "payment", "confirmed").
			Group("partner_id").Scan(&payRows).Error; err != nil {
			return nil, err
		}
	}
	payMap := map[int64]int64{}
	for _, r := range payRows {
		payMap[r.PartnerID] = r.Sum
	}

	// 档案列表（含期初）
	var result []PartnerBalance
	if kind == "customer" {
		var cs []model.Customer
		if err := s.db(ctx).Find(&cs).Error; err != nil {
			return nil, err
		}
		for _, c := range cs {
			result = append(result, s.balanceRow(ctx, c.ID, c.Code, c.Name, c.OpeningReceivable, bizMap, payMap))
		}
	} else {
		var ss []model.Supplier
		if err := s.db(ctx).Find(&ss).Error; err != nil {
			return nil, err
		}
		for _, c := range ss {
			result = append(result, s.balanceRow(ctx, c.ID, c.Code, c.Name, c.OpeningPayable, bizMap, payMap))
		}
	}
	return result, nil
}

func (s *FinanceService) balanceRow(ctx context.Context, id int64, code, name string, opening int64, biz, pay map[int64]int64) PartnerBalance {
	b := PartnerBalance{PartnerID: id, Code: code, Name: name, Opening: opening}
	b.Biz = biz[id]
	b.Settled = pay[id]
	b.Balance = opening + b.Biz - b.Settled
	return b
}
