import request from '../utils/request'

export interface SummaryRow {
  product_id: number; code: string; name: string; unit: string
  opening_qty: number; opening_val: number
  in_qty: number; in_val: number
  out_qty: number; out_val: number
  closing_qty: number; closing_val: number
}

export interface ProfitRow {
  doc_id: number; doc_no: string; doc_date: string; partner: string; doc_type: string
  revenue: number; cost: number; profit: number
}

export interface ProfitResult {
  rows: ProfitRow[]
  total: ProfitRow
}

export interface DashboardStats {
  today_sales: number; today_profit: number; inventory_val: number
  receivable: number; payable: number; today_docs: number; low_stock_count: number
  low_stock: { product_id: number; code: string; name: string; quantity: number; safe_stock: number }[]
  sales_trend: { date: string; sales: number }[]
  recent_docs: {
    id: number; doc_no: string; doc_type: string; partner_name: string
    total_amount: number; status: string; doc_date: string
  }[]
  // v0.13 库位容量/上下限预警
  location_alerts: {
    id: number; code: string; name: string
    capacity: number; min_qty: number; used_qty: number; usage_rate: number
    capacity_mode?: string; used_unit?: string // v0.17 容量计量方式与量纲
    status: string; status_text: string
  }[]
}

export interface StockRow {
  product_id: number; code: string; name: string; category_name: string; unit: string
  cost_method: string; is_batch: boolean
  quantity: number; avg_cost: number; total_value: number; safe_stock: number
}

export interface BatchRow {
  id: number; batch_no: string; expire_date: string | null
  quantity: number; unit_cost: number
}

export const reportApi = {
  summary: (params: { date_from: string; date_to: string }) =>
    request.get('/reports/summary', { params }) as any,
  profit: (params: { date_from: string; date_to: string }) =>
    request.get('/reports/profit', { params }) as any,
  dashboard: () => request.get('/dashboard/stats') as any,
  stocks: (keyword = '') => request.get('/inventory', { params: { keyword } }) as any,
  batches: (productId: number) => request.get(`/inventory/${productId}/batches`) as any,
}
