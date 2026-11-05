package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"jxc/server/internal/model"
	"jxc/server/internal/pkg/errcode"
)

// ScaleService 条码秤标签解析（v0.11）
//
// 目标：散称商品开单时，扫码枪扫一下电子秤标签，自动带出商品 + 数量。
// 码制差异（品牌/型号/金额码或重量码）全部数据化到 scale_rules 表，
// 新增品牌只是新增一条规则，不需要改代码。
type ScaleService struct{ conn *gorm.DB }

// db 按请求上下文取数据库句柄：ctx 携带租户，GORM 租户插件据此隔离数据
func (s *ScaleService) db(ctx context.Context) *gorm.DB { return s.conn.WithContext(ctx) }

func NewScaleService(db *gorm.DB) *ScaleService { return &ScaleService{conn: db} }

// ---------------- 规则 CRUD ----------------

type ScaleRuleReq struct {
	Name           string `json:"name"`
	Brand          string `json:"brand"`
	Model          string `json:"model"`
	CodeLen        int    `json:"code_len"`
	Prefixes       string `json:"prefixes"`
	PluStart       int    `json:"plu_start"`
	PluLen         int    `json:"plu_len"`
	AmountStart    int    `json:"amount_start"`
	AmountLen      int    `json:"amount_len"`
	AmountDecimals int    `json:"amount_decimals"`
	WeightStart    int    `json:"weight_start"`
	WeightLen      int    `json:"weight_len"`
	WeightDecimals int    `json:"weight_decimals"`
	CheckType      string `json:"check_type"`
	Enabled        bool   `json:"enabled"`
	IsDefault      bool   `json:"is_default"`
	Sort           int    `json:"sort"`
	Remark         string `json:"remark"`
}

// normalize 填充未填项为行业常用默认值（金额 2 位小数、重量 3 位小数）
func (r *ScaleRuleReq) normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Prefixes = strings.TrimSpace(r.Prefixes)
	if r.CodeLen == 0 {
		r.CodeLen = 13
	}
	if r.PluStart == 0 {
		r.PluStart = 3
	}
	if r.PluLen == 0 {
		r.PluLen = 5
	}
	if r.AmountStart > 0 && r.AmountLen == 0 {
		r.AmountLen = 5
	}
	if r.WeightStart > 0 && r.WeightLen == 0 {
		r.WeightLen = 5
	}
	if r.AmountStart > 0 && r.AmountDecimals == 0 {
		r.AmountDecimals = 2
	}
	if r.WeightStart > 0 && r.WeightDecimals == 0 {
		r.WeightDecimals = 3
	}
	if r.CheckType == "" {
		r.CheckType = "mod10"
	}
}

func (r ScaleRuleReq) validate() error {
	if r.Name == "" {
		return errcode.New(1001, "规则名称必填")
	}
	if r.CodeLen < 4 || r.CodeLen > 40 {
		return errcode.New(1001, "条码位数应在 4~40 之间")
	}
	if r.PluStart < 1 || r.PluLen < 1 || r.PluStart+r.PluLen-1 > r.CodeLen {
		return errcode.New(1001, "PLU 段超出条码位数范围")
	}
	if r.AmountStart > 0 && (r.AmountStart+r.AmountLen-1 > r.CodeLen || r.AmountLen < 1) {
		return errcode.New(1001, "金额段超出条码位数范围")
	}
	if r.WeightStart > 0 && (r.WeightStart+r.WeightLen-1 > r.CodeLen || r.WeightLen < 1) {
		return errcode.New(1001, "重量段超出条码位数范围")
	}
	if r.AmountStart == 0 && r.WeightStart == 0 {
		return errcode.New(1001, "金额段与重量段至少要配置一项")
	}
	if r.AmountDecimals < 0 || r.AmountDecimals > 4 || r.WeightDecimals < 0 || r.WeightDecimals > 4 {
		return errcode.New(1001, "小数位数应在 0~4 之间")
	}
	if r.CheckType != "mod10" && r.CheckType != "none" {
		return errcode.New(1001, "校验方式仅支持 mod10 或 none")
	}
	if r.Prefixes != "" {
		for _, p := range strings.Split(r.Prefixes, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if len(p) > 3 || !allDigits(p) {
				return errcode.New(1001, "前缀应为 1~3 位数字，多个用英文逗号分隔")
			}
			if len(p) >= r.CodeLen {
				return errcode.New(1001, "前缀不能长于条码位数")
			}
		}
	}
	return nil
}

func (s *ScaleService) List(ctx context.Context) ([]model.ScaleRule, error) {
	var list []model.ScaleRule
	err := s.db(ctx).Order("sort ASC, id ASC").Find(&list).Error
	return list, err
}

// enabledRules 解析时只取启用规则；默认规则优先
func (s *ScaleService) enabledRules(ctx context.Context) ([]model.ScaleRule, error) {
	var list []model.ScaleRule
	err := s.db(ctx).Where("enabled = ?", true).Order("is_default DESC, sort ASC, id ASC").Find(&list).Error
	return list, err
}

func (r ScaleRuleReq) toModel() model.ScaleRule {
	return model.ScaleRule{
		Name: r.Name, Brand: r.Brand, Model: r.Model,
		CodeLen: r.CodeLen, Prefixes: r.Prefixes,
		PluStart: r.PluStart, PluLen: r.PluLen,
		AmountStart: r.AmountStart, AmountLen: r.AmountLen, AmountDecimals: r.AmountDecimals,
		WeightStart: r.WeightStart, WeightLen: r.WeightLen, WeightDecimals: r.WeightDecimals,
		CheckType: r.CheckType, Enabled: r.Enabled, IsDefault: r.IsDefault,
		Sort: r.Sort, Remark: r.Remark,
	}
}

func (s *ScaleService) clearDefault(ctx context.Context, exceptID int64) error {
	return s.db(ctx).Model(&model.ScaleRule{}).
		Where("is_default = ? AND id <> ?", true, exceptID).
		Update("is_default", false).Error
}

func (s *ScaleService) Create(ctx context.Context, req ScaleRuleReq) error {
	req.normalize()
	if err := req.validate(); err != nil {
		return err
	}
	rule := req.toModel()
	if rule.IsDefault {
		if err := s.clearDefault(ctx, 0); err != nil {
			return err
		}
	}
	return s.db(ctx).Create(&rule).Error
}

func (s *ScaleService) Update(ctx context.Context, id int64, req ScaleRuleReq) error {
	req.normalize()
	if err := req.validate(); err != nil {
		return err
	}
	var old model.ScaleRule
	if err := s.db(ctx).First(&old, id).Error; err != nil {
		return errcode.New(4004, "秤码规则不存在")
	}
	if req.IsDefault {
		if err := s.clearDefault(ctx, id); err != nil {
			return err
		}
	}
	return s.db(ctx).Model(&model.ScaleRule{}).Where("id = ?", id).Updates(map[string]any{
		"name": req.Name, "brand": req.Brand, "model": req.Model,
		"code_len": req.CodeLen, "prefixes": req.Prefixes,
		"plu_start": req.PluStart, "plu_len": req.PluLen,
		"amount_start": req.AmountStart, "amount_len": req.AmountLen, "amount_decimals": req.AmountDecimals,
		"weight_start": req.WeightStart, "weight_len": req.WeightLen, "weight_decimals": req.WeightDecimals,
		"check_type": req.CheckType, "enabled": req.Enabled, "is_default": req.IsDefault,
		"sort": req.Sort, "remark": req.Remark,
	}).Error
}

func (s *ScaleService) Delete(ctx context.Context, id int64) error {
	var rule model.ScaleRule
	if err := s.db(ctx).First(&rule, id).Error; err != nil {
		return errcode.New(4004, "秤码规则不存在")
	}
	if err := s.db(ctx).Delete(&model.ScaleRule{}, id).Error; err != nil {
		return err
	}
	// 删掉默认规则后，把排序最前的启用规则补为默认，避免"无默认"
	if rule.IsDefault {
		var next model.ScaleRule
		if err := s.db(ctx).Where("enabled = ?", true).Order("sort ASC, id ASC").First(&next).Error; err == nil {
			return s.db(ctx).Model(&model.ScaleRule{}).Where("id = ?", next.ID).Update("is_default", true).Error
		}
	}
	return nil
}

// ---------------- 解析 ----------------

type ScaleParseResult struct {
	Matched     bool   `json:"matched"`
	Code        string `json:"code"`
	Reason      string `json:"reason"` // 未匹配原因（给用户看）
	RuleID      int64  `json:"rule_id"`
	RuleName    string `json:"rule_name"`
	PLU         string `json:"plu"`
	Amount      int64  `json:"amount"`   // 金额（分），金额码时给出
	Quantity    int64  `json:"quantity"` // 数量 ×1000，重量码时给出
	Source      string `json:"source"`   // weight=重量段直接得出 / amount=按售价反算
	ProductID   int64  `json:"product_id"`
	ProductCode string `json:"product_code"`
	ProductName string `json:"product_name"`
	Unit        string `json:"unit"`
	Price       int64  `json:"price"` // 商品默认售价（分），金额码反算数量用
}

// Parse 解析秤码。返回 Matched=false 时 Reason 说明原因，调用方回退普通条码搜索。
func (s *ScaleService) Parse(ctx context.Context, code string) (*ScaleParseResult, error) {
	raw := strings.TrimSpace(code)
	res := &ScaleParseResult{Code: raw}
	digits := onlyDigits(raw)
	if digits == "" {
		res.Reason = "不是有效的数字条码"
		return res, nil
	}
	rules, err := s.enabledRules(ctx)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		res.Reason = "尚未配置秤码规则"
		return res, nil
	}

	var lastReason string
	for _, r := range rules {
		if r.CodeLen > 0 && len(digits) != r.CodeLen {
			continue
		}
		if !prefixMatch(r.Prefixes, digits) {
			continue
		}
		if r.CheckType == "mod10" && !mod10OK(digits) {
			lastReason = fmt.Sprintf("条码校验位不通过（规则：%s）", r.Name)
			continue
		}
		plu, ok := segment(digits, r.PluStart, r.PluLen)
		if !ok {
			continue
		}

		res.RuleID, res.RuleName, res.PLU = r.ID, r.Name, plu

		var p model.Product
		if err := s.db(ctx).Where("plu = ?", plu).First(&p).Error; err != nil {
			res.Reason = fmt.Sprintf("秤码解析成功（PLU=%s），但没有商品填写该 PLU 码", plu)
			return res, nil
		}
		if p.Status != "" && p.Status != "active" {
			res.Reason = fmt.Sprintf("PLU=%s 对应商品「%s」已停用", plu, p.Name)
			return res, nil
		}

		res.ProductID, res.ProductCode, res.ProductName, res.Unit = p.ID, p.Code, p.Name, p.Unit
		res.Price = s.defaultPrice(ctx, p)

		if r.WeightStart > 0 {
			if wd, ok := segment(digits, r.WeightStart, r.WeightLen); ok {
				if n, err := strconv.ParseInt(wd, 10, 64); err == nil {
					res.Quantity = scaleValue(n, r.WeightDecimals, 3)
				}
			}
		}
		if r.AmountStart > 0 {
			if ad, ok := segment(digits, r.AmountStart, r.AmountLen); ok {
				if n, err := strconv.ParseInt(ad, 10, 64); err == nil {
					res.Amount = scaleValue(n, r.AmountDecimals, 2)
				}
			}
		}

		// 纯金额码：数量 = 金额 / 售价（售价取商品默认价位）
		if res.Quantity == 0 && res.Amount > 0 {
			if res.Price > 0 {
				res.Quantity = res.Amount * 1000 / res.Price
			} else {
				res.Reason = fmt.Sprintf("商品「%s」没有售价，无法由金额反算数量", p.Name)
			}
		}
		if res.Quantity == 0 && res.Amount == 0 {
			lastReason = fmt.Sprintf("规则「%s」解析出的金额与重量均为 0", r.Name)
			continue
		}
		if res.Quantity > 0 && r.WeightStart > 0 {
			res.Source = "weight"
		} else if res.Amount > 0 {
			res.Source = "amount"
		}
		res.Matched = true
		return res, nil
	}

	if lastReason == "" {
		lastReason = "不是秤码（长度/前缀不匹配任何已启用规则）"
	}
	res.Reason = lastReason
	return res, nil
}

func (s *ScaleService) defaultPrice(ctx context.Context, p model.Product) int64 {
	var price model.ProductPrice
	if p.DefaultPriceID > 0 {
		if err := s.db(ctx).First(&price, p.DefaultPriceID).Error; err != nil {
			price = model.ProductPrice{}
		}
	}
	if price.ID == 0 {
		s.db(ctx).Where("product_id = ?", p.ID).Order("sort ASC, id ASC").First(&price)
	}
	return price.Price
}

// ---------------- 工具函数 ----------------

func onlyDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func prefixMatch(prefixes, code string) bool {
	if strings.TrimSpace(prefixes) == "" {
		return true
	}
	for _, p := range strings.Split(prefixes, ",") {
		p = strings.TrimSpace(p)
		if p != "" && strings.HasPrefix(code, p) {
			return true
		}
	}
	return false
}

// segment 按 1-based 起止取子串
func segment(code string, start, length int) (string, bool) {
	if start < 1 || length < 1 {
		return "", false
	}
	from, to := start-1, start-1+length
	if to > len(code) {
		return "", false
	}
	return code[from:to], true
}

// mod10OK EAN/UPC 通用校验：自右向左，倒数第二位起权重 3,1,3,1…
func mod10OK(code string) bool {
	n := len(code)
	if n < 2 {
		return false
	}
	sum, w := 0, 3
	for i := n - 2; i >= 0; i-- {
		d := int(code[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		sum += d * w
		if w == 3 {
			w = 1
		} else {
			w = 3
		}
	}
	check := (10 - sum%10) % 10
	return check == int(code[n-1]-'0')
}

// scaleValue 把 N（隐含 from 位小数）换算为以 to 位小数表示的整数
func scaleValue(n int64, from, to int) int64 {
	if from < 0 {
		from = 0
	}
	if from <= to {
		return n * pow10(to-from)
	}
	return n / pow10(from-to)
}

func pow10(n int) int64 {
	v := int64(1)
	for i := 0; i < n; i++ {
		v *= 10
	}
	return v
}

// ---------------- 内置预设 ----------------

// SeedScaleRules 首次启动写入三条行业常见码制（已有规则时不动）
func SeedScaleRules(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.ScaleRule{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	presets := []model.ScaleRule{
		{
			Name: "顶尖 Aclas LS · 13位金额码", Brand: "顶尖 Aclas", Model: "LS6 / LS2N / LS2NX",
			CodeLen: 13, Prefixes: "22",
			PluStart: 3, PluLen: 5,
			AmountStart: 8, AmountLen: 5, AmountDecimals: 2,
			CheckType: "mod10", Enabled: true, IsDefault: true, Sort: 1,
			Remark: "国内生鲜区最常见码制：22+PLU(5)+金额(分,5)+校验位；数量按商品售价反算",
		},
		{
			Name: "顶尖 Aclas LS · 13位重量码", Brand: "顶尖 Aclas", Model: "LS6 / LS2N / LS2NX",
			CodeLen: 13, Prefixes: "21",
			PluStart: 3, PluLen: 5,
			WeightStart: 8, WeightLen: 5, WeightDecimals: 3,
			CheckType: "mod10", Enabled: true, Sort: 2,
			Remark: "21+PLU(5)+重量(克,5)+校验位；直接得到重量，不依赖售价",
		},
		{
			Name: "大华 / 寺冈 · 18位码", Brand: "大华 / 寺冈 DIGI", Model: "支持 18 位码的机型",
			CodeLen: 18, Prefixes: "22",
			PluStart: 3, PluLen: 5,
			AmountStart: 8, AmountLen: 5, AmountDecimals: 2,
			WeightStart: 13, WeightLen: 5, WeightDecimals: 3,
			CheckType: "mod10", Enabled: true, Sort: 3,
			Remark: "金额与重量同时打在码里，重量优先、最准确；秤上改价也不影响库存数量",
		},
	}
	for i := range presets {
		if err := db.Create(&presets[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
