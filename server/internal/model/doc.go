package model

import "time"

// Document 单据主表（10 类单据共用，对应 DATA-MODEL.md）
type Document struct {
	Base
	DocNo       string     `gorm:"size:30;index" json:"doc_no"`
	DocType     string     `gorm:"size:20;index" json:"doc_type"`
	Status      string     `gorm:"size:20;index;default:draft" json:"status"` // draft / confirmed / voided
	Scope       string     `gorm:"size:20" json:"scope"`                      // 盘点范围："" 全仓（默认）/ location 按库位逐架盘
	PartnerType string     `gorm:"size:20" json:"partner_type"`               // supplier / customer / ""
	PartnerID   int64      `json:"partner_id"`
	PartnerName string     `gorm:"size:100" json:"partner_name"` // 快照
	DocDate     time.Time  `json:"doc_date"`
	TotalQty    int64      `json:"total_qty"`    // ×1000
	TotalAmount int64      `json:"total_amount"` // 分
	Remark      string     `gorm:"size:500" json:"remark"`
	ConfirmedAt *time.Time `json:"confirmed_at"`
	VoidedAt    *time.Time `json:"voided_at"`
	VoidReason  string     `gorm:"size:200" json:"void_reason"`
	// 盘点差异审批（v0.16）：差异超阈值的盘点单确认后转待审批，主管通过后才调账
	ReviewStatus string         `gorm:"size:20;index;default:''" json:"review_status"` // "" 无 / pending 待审批 / approved 已通过 / rejected 已驳回
	ReviewNote   string         `gorm:"size:200" json:"review_note"`                   // 审批意见 / 驳回原因
	ReviewBy     int64          `json:"review_by"`
	ReviewAt     *time.Time     `json:"review_at"`
	RefDocID     int64          `gorm:"default:0" json:"ref_doc_id"` // 关联原单（退货用）
	CreatedBy    int64          `json:"created_by"`
	Items        []DocumentItem `gorm:"foreignKey:DocumentID" json:"items,omitempty"`
}

func (Document) TableName() string { return "documents" }

// DocumentItem 单据明细
type DocumentItem struct {
	Base
	DocumentID     int64  `gorm:"index;not null" json:"document_id"`
	ProductID      int64  `gorm:"index;not null" json:"product_id"`
	ProductCode    string `gorm:"size:50" json:"product_code"`  // 快照
	ProductName    string `gorm:"size:100" json:"product_name"` // 快照
	Quantity       int64  `json:"quantity"`                     // 一律正数，×1000；盘点单=实盘数
	BookQty        int64  `gorm:"default:0" json:"book_qty"`    // 盘点单：确认时的账面数（×1000），其他单据恒 0
	Price          int64  `json:"price"`                        // 单价（分）；盘点/出库成本类单据语义见 service
	Cost           int64  `json:"cost"`                         // 确认时回填的结转/计入单位成本（分）
	PriceName      string `gorm:"size:50" json:"price_name"`
	BatchNo        string `gorm:"size:50" json:"batch_no"`          // 入库类批次商品必填
	LocationID     int64  `gorm:"default:0" json:"location_id"`     // 库位（移库单=移出库位；0=未分配）
	LocationName   string `gorm:"size:100" json:"location_name"`    // 快照
	ToLocationID   int64  `gorm:"default:0" json:"to_location_id"`  // 移库单移入库位（其他单据恒 0）
	ToLocationName string `gorm:"size:100" json:"to_location_name"` // 快照
	Amount         int64  `json:"amount"`                           // 数量×单价（分）
	Remark         string `gorm:"size:200" json:"remark"`
}

func (DocumentItem) TableName() string { return "document_items" }

// Setting 系统设置（租户内 key-value；主键 = 租户 + 键，各租户互不干扰）
type Setting struct {
	TenantID int64  `gorm:"primaryKey;autoIncrement:false" json:"tenant_id"`
	Key      string `gorm:"primaryKey;size:50" json:"key"`
	Value    string `gorm:"size:2000" json:"value"`
}

func (Setting) TableName() string { return "tenant_settings" }
