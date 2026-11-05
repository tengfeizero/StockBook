import request from '../utils/request'

export interface Payment {
  id: number
  doc_no: string
  kind: 'receipt' | 'payment'
  partner_type: string
  partner_id: number
  partner_name: string
  amount: number // 分
  method: string
  doc_date: string
  remark: string
  status: string
  void_reason: string
}

export interface PartnerBalance {
  partner_id: number
  code: string
  name: string
  opening: number
  biz: number
  settled: number
  balance: number
}

export const financeApi = {
  balances: (kind: 'customer' | 'supplier') =>
    request.get('/finance/balances', { params: { kind } }) as any,
  payments: (params: { kind?: string; partner_id?: number; page?: number; page_size?: number }) =>
    request.get('/finance/payments', { params }) as any,
  createPayment: (data: {
    kind: 'receipt' | 'payment'
    partner_id: number
    amount: number
    method?: string
    doc_date?: string
    remark?: string
  }) => request.post('/finance/payments', data) as any,
  voidPayment: (id: number, reason: string) =>
    request.post(`/finance/payments/${id}/void`, { reason }) as any,
}

export const methodText: Record<string, string> = {
  cash: '现金', wechat: '微信', alipay: '支付宝', bank: '银行转账',
}
