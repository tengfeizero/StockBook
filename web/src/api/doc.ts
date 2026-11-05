import request from '../utils/request'

export interface DocItemReq {
  product_id: number
  quantity: number // ×1000
  price: number // 分
  price_name?: string
  batch_no?: string
  location_id?: number // 库位：普通单据=作业库位（入库不填带默认、出库自动分配）；移库单=移出库位
  to_location_id?: number // 移库单移入库位
  remark?: string
}

export interface DocReq {
  doc_type: string
  partner_id?: number
  doc_date?: string
  remark?: string
  ref_doc_id?: number
  scope?: 'location' // 盘点范围：不传=全仓；location=按库位逐架盘
  confirm_now: boolean
  items: DocItemReq[]
}

export interface DocItem {
  id: number; product_id: number; product_code: string; product_name: string
  quantity: number; price: number; cost: number; price_name: string
  book_qty: number // 盘点单：确认时账面数（×1000），其他单据恒 0
  batch_no: string; location_id: number; location_name: string
  to_location_id: number; to_location_name: string
  amount: number; remark: string
}

export interface Doc {
  id: number; doc_no: string; doc_type: string; status: string
  partner_type: string; partner_id: number; partner_name: string
  doc_date: string; total_qty: number; total_amount: number
  remark: string; void_reason: string; ref_doc_id: number
  scope?: string
  // 盘点差异审批（v0.16）
  review_status?: string // '' 无 / pending 待审批 / approved 已通过 / rejected 已驳回
  review_note?: string
  review_by?: number
  review_at?: string
  items?: DocItem[]
}

export const docApi = {
  create: (data: DocReq) => request.post('/docs', data) as any,
  confirm: (id: number) => request.post(`/docs/${id}/confirm`),
  void: (id: number, reason: string) => request.post(`/docs/${id}/void`, { reason }),
  review: (id: number, action: 'approve' | 'reject', note = '') =>
    request.post(`/docs/${id}/review`, { action, note }),
  list: (params: {
    page?: number; page_size?: number; doc_type?: string; status?: string
    keyword?: string; date_from?: string; date_to?: string
  }) => request.get('/docs', { params }),
  get: (id: number) => request.get(`/docs/${id}`),
}

// 单据类型配置（与后端 service/document.go 对应）
export const docTypeConfig: Record<string, {
  title: string; partner: 'supplier' | 'customer' | ''
  affectsStock: boolean; priceLabel: string; batchRequired: boolean
  hidePrice?: boolean // 移库单无金额
  locNote?: string   // 库位列提示
}> = {
  purchase_order: { title: '采购订单', partner: 'supplier', affectsStock: false, priceLabel: '订货单价', batchRequired: false },
  purchase_in: { title: '采购入库', partner: 'supplier', affectsStock: true, priceLabel: '采购单价', batchRequired: true, locNote: '不填按商品默认库位' },
  sales_out: { title: '销售出库', partner: 'customer', affectsStock: true, priceLabel: '销售单价', batchRequired: false, locNote: '不填自动分配' },
  quotation: { title: '报价单', partner: 'customer', affectsStock: false, priceLabel: '报价', batchRequired: false },
  purchase_return: { title: '采购退货', partner: 'supplier', affectsStock: true, priceLabel: '退货单价', batchRequired: false, locNote: '不填自动分配' },
  sales_return: { title: '销售退货', partner: 'customer', affectsStock: true, priceLabel: '退货单价', batchRequired: true, locNote: '不填按商品默认库位' },
  other_in: { title: '其他入库', partner: '', affectsStock: true, priceLabel: '成本单价', batchRequired: false, locNote: '不填按商品默认库位' },
  other_out: { title: '其他出库', partner: '', affectsStock: true, priceLabel: '成本单价', batchRequired: false, locNote: '不填自动分配' },
  stocktake: { title: '盘点单', partner: '', affectsStock: true, priceLabel: '盘盈成本', batchRequired: false, locNote: '全仓口径：实盘数=全仓数量，此处为盘盈入库库位' },
  transfer: { title: '移库单', partner: '', affectsStock: true, priceLabel: '', batchRequired: false, hidePrice: true },
}

export const statusText: Record<string, string> = {
  draft: '草稿', confirmed: '已生效', voided: '已作废', review: '待审批',
}
