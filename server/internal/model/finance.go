package model

import "time"

// Payment 收付款单：receipt=收款单（客户→我们），payment=付款单（我们→供应商）
// 余额实时计算（期初 + 单据发生 − 收付款），不在档案上存余额字段，避免同步问题
type Payment struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	TenantID    int64     `gorm:"index;default:0" json:"tenant_id"`
	DocNo       string    `gorm:"size:32;index" json:"doc_no"`
	Kind        string    `gorm:"size:16;index" json:"kind"`         // receipt | payment
	PartnerType string    `gorm:"size:16;index" json:"partner_type"` // customer | supplier
	PartnerID   int64     `gorm:"index" json:"partner_id"`
	PartnerName string    `gorm:"size:64" json:"partner_name"`
	Amount      int64     `json:"amount"`                // 分，恒为正
	Method      string    `gorm:"size:16" json:"method"` // cash|wechat|alipay|bank
	DocDate     time.Time `json:"doc_date"`
	Remark      string    `gorm:"size:255" json:"remark"`
	Status      string    `gorm:"size:16;index;default:confirmed" json:"status"` // confirmed | voided
	VoidReason  string    `gorm:"size:255" json:"void_reason"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}
