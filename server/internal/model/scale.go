package model

// ScaleRule 条码秤标签解析规则（v0.11）
//
// 生鲜/散称商品在电子秤上称重后打印的是 EAN-13「店内码」，码里同时包含
// 商品 PLU 与重量或金额。不同品牌/型号的码制（前缀、分段、小数位）不一样，
// 因此这里把码制做成数据配置，新增品牌只需加一条规则，不改代码。
//
// 段位置均从 1 开始计数（人读习惯），解析时统一转成 0-based 切片。
// Amount*/Weight* 为 0 表示该码制不含该段。
type ScaleRule struct {
	Base
	Name           string `gorm:"size:50;not null" json:"name"` // 规则名，如「顶尖 LS 13位金额码」
	Brand          string `gorm:"size:50" json:"brand"`         // 品牌，如 顶尖 Aclas
	Model          string `gorm:"size:100" json:"model"`        // 适用型号，如 LS6 / LS2N
	CodeLen        int    `json:"code_len"`                     // 条码总位数，0=不限制
	Prefixes       string `gorm:"size:100" json:"prefixes"`     // 前缀，逗号分隔（如 "21,22"），空=不限
	PluStart       int    `json:"plu_start"`                    // PLU 段起始位（1-based）
	PluLen         int    `json:"plu_len"`                      // PLU 段长度
	AmountStart    int    `json:"amount_start"`                 // 金额段起始位，0=无
	AmountLen      int    `json:"amount_len"`
	AmountDecimals int    `json:"amount_decimals"` // 金额小数位（2=分）
	WeightStart    int    `json:"weight_start"`    // 重量段起始位，0=无
	WeightLen      int    `json:"weight_len"`
	WeightDecimals int    `json:"weight_decimals"`           // 重量小数位（3=克，即 kg 保留 3 位）
	CheckType      string `gorm:"size:20" json:"check_type"` // mod10 / none
	Enabled        bool   `json:"enabled"`
	IsDefault      bool   `json:"is_default"`
	Sort           int    `json:"sort"`
	Remark         string `gorm:"size:200" json:"remark"`
}

func (ScaleRule) TableName() string { return "scale_rules" }
