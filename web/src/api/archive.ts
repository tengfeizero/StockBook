import request from '../utils/request'

// ============ 分类 ============
export const categoryApi = {
  list: (keyword = '') => request.get('/categories', { params: { keyword } }),
  create: (data: { name: string; parent_id?: number; sort?: number }) => request.post('/categories', data),
  update: (id: number, data: { name: string; parent_id?: number; sort?: number }) => request.put(`/categories/${id}`, data),
  remove: (id: number) => request.delete(`/categories/${id}`),
}

// ============ 商品 ============
export interface ProductPrice {
  name: string
  price: number // 分
  sort?: number
}
export interface ProductPayload {
  code: string
  barcode?: string
  plu?: string // 秤码 PLU（散称商品）
  name: string
  category_id?: number
  unit?: string
  cost_method: 'weighted_avg' | 'fifo'
  is_batch_enabled?: boolean
  shelf_life_days?: number
  safe_stock?: number
  volume?: number // 单件体积（升×1000，按基本单位；0=未登记）
  weight?: number // 单件重量（kg×1000，按基本单位；0=未登记）
  status?: string
  prices: ProductPrice[]
  default_price?: string
  default_location_id?: number // 默认库位
}

// ============ 秤码规则（v0.11） ============
export interface ScaleRule {
  id: number
  name: string
  brand: string
  model: string
  code_len: number
  prefixes: string
  plu_start: number
  plu_len: number
  amount_start: number
  amount_len: number
  amount_decimals: number
  weight_start: number
  weight_len: number
  weight_decimals: number
  check_type: 'mod10' | 'none'
  enabled: boolean
  is_default: boolean
  sort: number
  remark: string
}

export interface ScaleParseResult {
  matched: boolean
  code: string
  reason: string
  rule_id: number
  rule_name: string
  plu: string
  amount: number // 分
  quantity: number // ×1000
  source: 'weight' | 'amount' | ''
  product_id: number
  product_code: string
  product_name: string
  unit: string
  price: number // 分
}

export const scaleApi = {
  list: () => request.get('/scale-rules'),
  create: (data: Partial<ScaleRule>) => request.post('/scale-rules', data),
  update: (id: number, data: Partial<ScaleRule>) => request.put(`/scale-rules/${id}`, data),
  remove: (id: number) => request.delete(`/scale-rules/${id}`),
  parse: (code: string) => request.post('/scale/parse', { code }),
}

// ============ 库位（v0.12 数量账 + v0.13 容量预警） ============
export interface Location {
  id: number
  code: string
  name: string
  remark: string
  sort: number
  capacity: number // 容量上限（×1000；0=不限）
  min_qty: number // 下限（×1000；0=不限）
  warn_ratio: number // 近满预警阈值 %
  capacity_mode?: 'qty' | 'volume' | 'weight' // 容量计量方式（v0.17）
  // 以下为后端计算的实时占用
  used_qty?: number // 占用合计（×1000，量纲随 capacity_mode）
  unknown_skus?: number // volume/weight 模式下未登记体积/重量的在库商品数
  used_unit?: string // 件 / L / kg
  sku_count?: number
  usage_rate?: number // 百分比；未设容量为 -1
  status?: 'normal' | 'near_full' | 'full' | 'over' | 'under' | 'empty'
  status_text?: string
}

export interface LocationPayload {
  code?: string
  name: string
  remark?: string
  sort?: number
  capacity_mode?: 'qty' | 'volume' | 'weight'
  capacity?: number
  min_qty?: number
  warn_ratio?: number
}

export interface LocationStockRow {
  location_id: number
  location_name: string
  location_code: string
  product_id: number
  product_code: string
  product_name: string
  unit: string
  barcode: string
  plu: string
  quantity: number
}

export const locationApi = {
  list: () =>
    request.get('/locations') as unknown as Promise<Location[]>,
  alerts: () =>
    request.get('/locations/alerts') as unknown as Promise<Location[]>,
  create: (data: LocationPayload) => request.post('/locations', data),
  update: (id: number, data: LocationPayload) => request.put(`/locations/${id}`, data),
  remove: (id: number) => request.delete(`/locations/${id}`),
  // 库位库存（v0.12）：location_id 不传=全部，0=未分配
  stock: (params: { location_id?: number; keyword?: string }) =>
    request.get('/locations/stock', { params }) as unknown as Promise<LocationStockRow[]>,
}

export const productApi = {
  list: (params: { page?: number; page_size?: number; keyword?: string; category_id?: number; status?: string }) =>
    request.get('/products', { params }),
  get: (id: number) => request.get(`/products/${id}`),
  create: (data: ProductPayload) => request.post('/products', data),
  update: (id: number, data: ProductPayload) => request.put(`/products/${id}`, data),
  remove: (id: number) => request.delete(`/products/${id}`),
}

// ============ 供应商 / 客户 ============
export interface PartnerPayload {
  code: string
  name: string
  contact?: string
  phone?: string
  address?: string
  opening?: number // 分
  remark?: string
}

export const partnerApi = {
  list: (kind: 'supplier' | 'customer', params: { page?: number; page_size?: number; keyword?: string }) =>
    request.get(`/partners/${kind}`, { params }),
  create: (kind: 'supplier' | 'customer', data: PartnerPayload) => request.post(`/partners/${kind}`, data),
  update: (kind: 'supplier' | 'customer', id: number, data: PartnerPayload) => request.put(`/partners/${kind}/${id}`, data),
  remove: (kind: 'supplier' | 'customer', id: number) => request.delete(`/partners/${kind}/${id}`),
}

// ============ 期初库存 ============
export interface OpeningStockRow {
  product_id: number; code: string; name: string; unit: string
  is_batch_enabled: boolean; quantity: number; avg_cost: number
}
export interface OpeningReq {
  location_id?: number
  product_id: number
  quantity: number // ×1000
  unit_cost: number // 分
  batch_no?: string
}

export const openingApi = {
  stocks: () => request.get('/opening/stocks'),
  save: (rows: OpeningReq[]) => request.post('/opening', { rows }),
}

// ============ 批量导入 ============
export interface ImportResult {
  success: number; skipped: number
  errors: { row: number; msg: string }[]
}

export const importApi = {
  products: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request.post('/import/products', fd)
  },
  partners: (kind: 'supplier' | 'customer', file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return request.post(`/import/partners/${kind}`, fd)
  },
  template: (kind: 'products' | 'supplier' | 'customer') =>
    request.get(`/import/template/${kind}`, { responseType: 'blob' }),
}
