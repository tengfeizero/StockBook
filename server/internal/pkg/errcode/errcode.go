package errcode

import "fmt"

// BizError 业务错误（对应 CONVENTIONS.md 错误码段）
type BizError struct {
	Code int
	Msg  string
}

func (e *BizError) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Msg) }

func New(code int, msg string) *BizError { return &BizError{Code: code, Msg: msg} }

// 通用 1xxx
var (
	ErrParam        = New(1001, "参数错误")
	ErrUnauth       = New(1002, "未登录或登录已过期")
	ErrForbidden    = New(1003, "无权限")
	ErrNotFound     = New(1004, "资源不存在")
	ErrWrongCreds   = New(1005, "用户名或密码错误")
	ErrUserDisabled = New(1006, "账号已停用")
)

// 库存 2xxx
var (
	ErrStockNotEnough = New(2001, "库存不足")
	ErrBatchMissing   = New(2002, "启用批次的商品必须录入批次")
	ErrCostMethod     = New(2003, "成本方式冲突")
	ErrLocStockShort  = New(2004, "库位库存不足")
	ErrLocFull        = New(2005, "库位容量不足")
)

// 单据 3xxx
var (
	ErrDocStatus    = New(3001, "单据状态不允许该操作")
	ErrDocNoDup     = New(3002, "单号生成冲突，请重试")
	ErrVoidReversal = New(3003, "作废回冲失败")
	ErrDocNotReview = New(3004, "该单据不在待审批状态")
)

// 档案 4xxx
var (
	ErrCodeDup    = New(4001, "编码已存在")
	ErrProductOff = New(4002, "商品已停用")
)

// 租户/订阅 51xx（5001 保留给授权过期）
var (
	ErrTenantNotFound = New(5101, "企业编码不存在")
	ErrTenantDisabled = New(5102, "该企业账号已停用，请联系服务商")
	ErrTenantExpired  = New(5103, "订阅已到期：请在 系统管理→订阅与计费 续费后继续操作")
	ErrTenantCodeUsed = New(5104, "企业编码已被占用，请换一个")
	ErrUserQuota      = New(5105, "已达当前套餐的用户数上限，请升级套餐")
	ErrPlanNotFound   = New(5106, "套餐不存在或已下架")
)
