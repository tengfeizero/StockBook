package model

// Category 商品分类
type Category struct {
	Base
	Name     string `gorm:"size:50;not null" json:"name"`
	ParentID int64  `gorm:"index;default:0" json:"parent_id"` // 0=顶级，预留二级
	Sort     int    `gorm:"default:0" json:"sort"`
}

func (Category) TableName() string { return "categories" }

// Product 商品（对应 DATA-MODEL.md products 表）
type Product struct {
	Base
	Code              string         `gorm:"size:50;index" json:"code"`
	Barcode           string         `gorm:"size:50;index" json:"barcode"`
	PLU               string         `gorm:"size:20;index" json:"plu"` // 秤码 PLU（散称商品在电子秤上的货号），可空
	Name              string         `gorm:"size:100;not null;index" json:"name"`
	CategoryID        int64          `json:"category_id"`
	Unit              string         `gorm:"size:20" json:"unit"`
	CostMethod        string         `gorm:"size:20;not null;default:weighted_avg" json:"cost_method"` // weighted_avg / fifo
	IsBatchEnabled    bool           `gorm:"default:false" json:"is_batch_enabled"`
	ShelfLifeDays     int            `gorm:"default:0" json:"shelf_life_days"`
	SafeStock         int64          `gorm:"default:0" json:"safe_stock"` // ×1000 存储
	Volume            int64          `gorm:"default:0" json:"volume"`     // 单件体积（升 ×1000，按基本单位；0=未登记，库位按体积计量时该商品不计占用）
	Weight            int64          `gorm:"default:0" json:"weight"`     // 单件重量（kg ×1000，按基本单位；0=未登记）
	Status            string         `gorm:"size:20;default:active" json:"status"`
	DefaultPriceID    int64          `json:"default_price_id"`
	DefaultLocationID int64          `gorm:"default:0" json:"default_location_id"` // 默认库位（开单自动带出，可改）
	Prices            []ProductPrice `gorm:"foreignKey:ProductID" json:"prices,omitempty"`
}

func (Product) TableName() string { return "products" }

// ProductPrice 商品价位（多价位）
type ProductPrice struct {
	Base
	ProductID int64  `gorm:"index;not null" json:"product_id"`
	Name      string `gorm:"size:50;not null" json:"name"`
	Price     int64  `json:"price"` // 分
	Sort      int    `gorm:"default:0" json:"sort"`
}

func (ProductPrice) TableName() string { return "product_prices" }

// Location 库位/货架档案。v0.12 起参与库位数量账（location_inventories）。
// v0.13 增加容量与上下限：容量按「占用数量合计」计（与商品基本单位同一量纲，
// 适合五金/便利店这类单件为主的仓库），0 表示不限制，不做体积/重量换算。
type Location struct {
	Base
	Code         string `gorm:"size:50" json:"code"`
	Name         string `gorm:"size:100;not null" json:"name"`
	Remark       string `gorm:"size:200" json:"remark"`
	Sort         int    `gorm:"default:0" json:"sort"`
	CapacityMode string `gorm:"size:20;default:qty" json:"capacity_mode"` // 容量计量：qty 数量 / volume 体积 / weight 重量
	Capacity     int64  `gorm:"default:0" json:"capacity"`                // 容量上限（与计量方式同量纲 ×1000；0=不限）
	MinQty       int64  `gorm:"default:0" json:"min_qty"`                 // 下限（同量纲 ×1000；0=不限）
	WarnRatio    int    `gorm:"default:90" json:"warn_ratio"`             // 近满预警阈值百分比（0~100，默认 90）
}

func (Location) TableName() string { return "locations" }

// Supplier 供应商
type Supplier struct {
	Base
	Code           string `gorm:"size:50;index" json:"code"`
	Name           string `gorm:"size:100;not null;index" json:"name"`
	Contact        string `gorm:"size:50" json:"contact"`
	Phone          string `gorm:"size:30" json:"phone"`
	Address        string `gorm:"size:200" json:"address"`
	OpeningPayable int64  `gorm:"default:0" json:"opening_payable"` // 期初应付，分
	Remark         string `gorm:"size:200" json:"remark"`
}

func (Supplier) TableName() string { return "suppliers" }

// Customer 客户
type Customer struct {
	Base
	Code              string `gorm:"size:50;index" json:"code"`
	Name              string `gorm:"size:100;not null;index" json:"name"`
	Contact           string `gorm:"size:50" json:"contact"`
	Phone             string `gorm:"size:30" json:"phone"`
	Address           string `gorm:"size:200" json:"address"`
	OpeningReceivable int64  `gorm:"default:0" json:"opening_receivable"` // 期初应收，分
	Remark            string `gorm:"size:200" json:"remark"`
}

func (Customer) TableName() string { return "customers" }
