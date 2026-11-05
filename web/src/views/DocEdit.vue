<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { docApi, docTypeConfig, statusText, type DocReq } from '../api/doc'
import { locationApi, partnerApi, productApi, scaleApi } from '../api/archive'
import { fenToYuan, yuanToFen } from '../utils/money'

const props = defineProps<{ docType: string }>()
const cfg = computed(() => docTypeConfig[props.docType])
const router = useRouter()
const isTransfer = computed(() => props.docType === 'transfer')
// 入库方向单据：未选库位时带出商品默认库位；出库交给后端自动分配
const INBOUND_DOCS = new Set(['purchase_in', 'sales_return', 'other_in', 'stocktake'])
const autoLoc = computed(() => INBOUND_DOCS.has(props.docType))

interface PriceOpt { id: number; name: string; price: number }
interface ProductOpt {
  id: number; code: string; name: string; unit: string
  is_batch_enabled: boolean; default_location_id?: number; prices?: PriceOpt[]
}
interface Row {
  product: ProductOpt
  qty: number
  price_yuan: number
  price_name: string
  batch_no: string
  location_id: number | undefined
  to_location_id: number | undefined
  remark: string
}

// 表头
const partnerOptions = ref<{ id: number; name: string; code: string }[]>([])
const partnerId = ref<number | undefined>()
const docDate = ref(new Date().toISOString().slice(0, 10))
const remark = ref('')
const saving = ref(false)

// 库位（M6：开单可选，入库自动带出商品默认库位）
const locations = ref<{ id: number; name: string }[]>([])

// 明细行
const rows = ref<Row[]>([])
const productKeyword = ref('')
const productOptions = ref<ProductOpt[]>([])
const searchingProduct = ref(false)
const searchingPartner = ref(false)

async function searchPartner(kw: string) {
  if (!cfg.value.partner) return
  searchingPartner.value = true
  try {
    const data: any = await partnerApi.list(cfg.value.partner, { keyword: kw, page: 1, page_size: 20 })
    partnerOptions.value = data.list
  } finally {
    searchingPartner.value = false
  }
}

async function searchProduct(kw: string) {
  if (!kw) return
  // 秤码优先：扫电子秤标签（纯数字 8~20 位）先按秤码规则解析，命中即带出商品与重量
  if (/^\d{8,20}$/.test(kw.trim()) && (await tryScaleCode(kw.trim()))) return
  searchingProduct.value = true
  try {
    const data: any = await productApi.list({ keyword: kw, page: 1, page_size: 10, status: 'active' })
    productOptions.value = data.list
  } finally {
    searchingProduct.value = false
  }
}

// 秤码解析：散称商品扫码即带出商品 + 数量（重量码直接取重量，金额码按售价反算）
function qtyFromScale(r: any, priceYuan: number): number {
  if (r.source === 'amount' && priceYuan > 0) return r.amount / 100 / priceYuan
  return r.quantity / 1000
}

async function tryScaleCode(code: string): Promise<boolean> {
  let r: any
  try {
    r = await scaleApi.parse(code)
  } catch {
    return false
  }
  if (!r?.matched) return false // 不是秤码则回退普通商品搜索
  try {
    const p: any = await productApi.get(r.product_id)
    const exist = rows.value.find((x) => x.product.id === p.id)
    let qty: number
    if (exist) {
      qty = qtyFromScale(r, exist.price_yuan)
      exist.qty = Math.round((exist.qty + qty) * 1000) / 1000
    } else {
      addProduct(p)
      const row = rows.value[rows.value.length - 1]
      qty = qtyFromScale(r, row.price_yuan)
      row.qty = Math.round(qty * 1000) / 1000
    }
    const shown = (Math.round(qty * 1000) / 1000).toString()
    if (r.source === 'amount') {
      ElMessage.success(`秤码识别：${p.name} × ${shown}${p.unit || ''}（金额 ¥${(r.amount / 100).toFixed(2)} 按售价反算，可修改数量）`)
    } else {
      ElMessage.success(`秤码识别：${p.name} × ${shown}${p.unit || ''}（${r.rule_name}）`)
    }
    return true
  } catch {
    return false
  }
}

function addProduct(p: any) {
  if (rows.value.some((r) => r.product.id === p.id)) {
    ElMessage.info('该商品已在清单中')
    return
  }
  const prices: PriceOpt[] = p.prices || []
  const defaultPrice = prices.find((x) => x.id === p.default_price_id) || prices[0]
  rows.value.push({
    product: p, qty: 0,
    price_yuan: defaultPrice ? defaultPrice.price / 100 : 0,
    price_name: defaultPrice?.name || '',
    batch_no: '',
    location_id: autoLoc.value ? (p.default_location_id || undefined) : undefined,
    to_location_id: undefined,
    remark: '',
  })
  productKeyword.value = ''
  productOptions.value = []
}

function pickPrice(row: Row, name: string) {
  const opt = row.product.prices?.find((x) => x.name === name)
  if (opt) row.price_yuan = opt.price / 100
}

function removeRow(i: number) {
  rows.value.splice(i, 1)
}

const totalQty = computed(() => rows.value.reduce((s, r) => s + r.qty, 0))
const totalAmount = computed(() => rows.value.reduce((s, r) => s + Math.round(r.qty * r.price_yuan * 100) / 100, 0))

// 库位列表（开单自动带出用）
locationApi.list().then((l: any) => (locations.value = l || [])).catch(() => {})

// 本单据类型的最近单据（开单时可参考历史，点击跳转单据查询定位）
interface RecentDoc {
  id: number; doc_no: string; doc_date: string
  partner_name: string; total_amount: number; status: string
}
const recentDocs = ref<RecentDoc[]>([])

async function loadRecent() {
  try {
    const data: any = await docApi.list({ doc_type: props.docType, page: 1, page_size: 8 })
    recentDocs.value = data.list || []
  } catch {
    recentDocs.value = []
  }
}
loadRecent()

function goDocList(docNo?: string) {
  router.push({ path: '/doc-list', query: { doc_type: props.docType, ...(docNo ? { keyword: docNo } : {}) } })
}

// 8 种单据共用本组件：路由切换时组件被复用，必须重置表头与明细，
// 否则上一张单的往来单位/商品明细会残留在新单据里
watch(() => props.docType, () => {
  partnerId.value = undefined
  partnerOptions.value = []
  docDate.value = new Date().toISOString().slice(0, 10)
  remark.value = ''
  rows.value = []
  productKeyword.value = ''
  productOptions.value = []
  loadRecent()
})

async function submit(confirmNow: boolean) {
  if (cfg.value.partner && !partnerId.value) return ElMessage.warning(`请选择${cfg.value.partner === 'supplier' ? '供应商' : '客户'}`)
  if (!rows.value.length) return ElMessage.warning('请添加商品明细')
  for (const r of rows.value) {
    if (r.qty <= 0) return ElMessage.warning(`「${r.product.name}」数量必须大于0`)
    if (!cfg.value.hidePrice && r.price_yuan < 0) return ElMessage.warning(`「${r.product.name}」单价不能为负`)
    if (cfg.value.batchRequired && r.product.is_batch_enabled && !r.batch_no.trim())
      return ElMessage.warning(`「${r.product.name}」启用批次管理，请填写批次号`)
    if (isTransfer.value) {
      if (r.location_id == null) return ElMessage.warning(`「${r.product.name}」请选择移出库位`)
      if (r.to_location_id == null) return ElMessage.warning(`「${r.product.name}」请选择移入库位`)
      if (r.location_id === r.to_location_id) return ElMessage.warning(`「${r.product.name}」移出与移入库位不能相同`)
    }
  }
  saving.value = true
  const req: DocReq = {
    doc_type: props.docType,
    partner_id: partnerId.value,
    doc_date: docDate.value,
    remark: remark.value,
    confirm_now: confirmNow,
    items: rows.value.map((r) => ({
      product_id: r.product.id,
      quantity: Math.round(r.qty * 1000),
      price: yuanToFen(r.price_yuan),
      price_name: r.price_name,
      batch_no: r.batch_no,
      location_id: r.location_id || 0,
      to_location_id: r.to_location_id || 0,
      remark: r.remark,
    })),
  }
  try {
    const data: any = await docApi.create(req)
    // 销售方向单据已生效时，直接给「打印 58mm 小票」入口（收银场景一步出票）
    const ticketable = confirmNow && ['sales_out', 'sales_return', 'quotation'].includes(props.docType)
    if (ticketable) {
      try {
        await ElMessageBox.confirm(
          `单号 ${data.doc_no}，已生效。是否打印 58mm 小票？`,
          `${cfg.value.title}成功`,
          { confirmButtonText: '打印小票', cancelButtonText: '去单据查询', type: 'success' }
        )
        router.push(`/print/${data.id}/ticket`)
      } catch {
        router.push('/doc-list')
      }
      return
    }
    const statusLine = data.status === 'review'
      ? `，差异超阈值已转主管审批（库存暂未调整）`
      : confirmNow ? '，已生效' : '，保存为草稿'
    await ElMessageBox.alert(
      `单号 ${data.doc_no}${statusLine}`,
      `${cfg.value.title}成功`,
      { confirmButtonText: '去单据查询' }
    )
    router.push('/doc-list')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <span style="font-weight: 600">{{ cfg.title }}开单</span>
      <el-tag v-if="!cfg.affectsStock" size="small" type="info" style="margin-left: 8px">不影响库存</el-tag>
      <el-tag v-else-if="isTransfer" size="small" type="warning" style="margin-left: 8px">只调库位，不改库存总量</el-tag>
    </template>

    <el-form inline label-width="80px">
      <el-form-item :label="cfg.partner === 'supplier' ? '供应商' : '客户'" v-if="cfg.partner" required>
        <el-select
          v-model="partnerId"
          filterable remote :remote-method="searchPartner" :loading="searchingPartner"
          placeholder="搜索名称/编码" style="width: 260px"
        >
          <el-option v-for="p in partnerOptions" :key="p.id" :label="`${p.code} ${p.name}`" :value="p.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="单据日期">
        <el-date-picker v-model="docDate" type="date" value-format="YYYY-MM-DD" style="width: 150px" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="remark" style="width: 260px" maxlength="200" />
      </el-form-item>
    </el-form>

    <el-select
      v-model="productKeyword"
      filterable remote :remote-method="searchProduct" :loading="searchingProduct"
      placeholder="搜索商品添加明细（名称/编码/条码）"
      style="width: 460px; margin-bottom: 12px"
      value-key="id"
      @change="(v: any) => v && addProduct(v)"
    >
      <el-option v-for="p in productOptions" :key="p.id" :label="`${p.code} ${p.name}`" :value="p" />
    </el-select>

    <el-table v-if="rows.length" :data="rows" border>
      <el-table-column label="商品" min-width="200">
        <template #default="{ row }">{{ row.product.code }} {{ row.product.name }}</template>
      </el-table-column>
      <el-table-column prop="product.unit" label="单位" width="70" />
      <el-table-column label="数量" width="150">
        <template #default="{ row }">
          <el-input-number v-model="row.qty" :min="0" :precision="3" style="width: 130px" />
        </template>
      </el-table-column>
      <el-table-column v-if="!cfg.hidePrice && rows.some((r) => r.product.prices?.length)" label="价位" width="140">
        <template #default="{ row }">
          <el-select v-model="row.price_name" size="small" @change="pickPrice(row, row.price_name)" :disabled="!row.product.prices?.length">
            <el-option v-for="p in row.product.prices" :key="p.id" :label="p.name" :value="p.name" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column v-if="!cfg.hidePrice" :label="cfg.priceLabel + '(元)'" width="160">
        <template #default="{ row }">
          <el-input-number v-model="row.price_yuan" :min="0" :precision="2" style="width: 140px" />
        </template>
      </el-table-column>
      <el-table-column v-if="!cfg.hidePrice" label="金额(元)" width="110" align="right">
        <template #default="{ row }">{{ (row.qty * row.price_yuan).toFixed(2) }}</template>
      </el-table-column>
      <el-table-column v-if="!isTransfer" label="批次号" width="150">
        <template #default="{ row }">
          <el-input v-if="row.product.is_batch_enabled" v-model="row.batch_no" size="small" placeholder="必填" />
          <span v-else style="color: var(--el-text-color-placeholder)">—</span>
        </template>
      </el-table-column>
      <el-table-column v-if="locations.length && isTransfer" label="移出库位" width="170">
        <template #default="{ row }">
          <el-select v-model="row.location_id" size="small" filterable placeholder="选择库位">
            <el-option label="未分配（待上架）" :value="0" />
            <el-option v-for="l in locations" :key="l.id" :label="l.name" :value="l.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column v-if="locations.length && isTransfer" label="移入库位" width="170">
        <template #default="{ row }">
          <el-select v-model="row.to_location_id" size="small" filterable placeholder="选择库位">
            <el-option label="未分配（待上架）" :value="0" />
            <el-option v-for="l in locations" :key="l.id" :label="l.name" :value="l.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column v-if="locations.length && !isTransfer" label="库位" width="170">
        <template #default="{ row }">
          <el-select v-model="row.location_id" size="small" clearable :placeholder="cfg.locNote || '自动分配'">
            <el-option v-for="l in locations" :key="l.id" :label="l.name" :value="l.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="80" fixed="right">
        <template #default="{ $index }">
          <el-button link type="danger" @click="removeRow($index)">移除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 16px; display: flex; align-items: center; gap: 24px">
      <span>合计数量：<b>{{ totalQty.toFixed(3).replace(/\.?0+$/, '') }}</b></span>
      <span v-if="!cfg.hidePrice">合计金额：<b style="color: var(--el-color-danger); font-size: 16px">¥{{ totalAmount.toFixed(2) }}</b></span>
      <el-button type="primary" :loading="saving" :disabled="!rows.length" @click="submit(true)">
        {{ isTransfer ? '保存并执行移库' : cfg.affectsStock ? '保存并生效（联动库存）' : '保存并生效' }}
      </el-button>
      <el-button :loading="saving" :disabled="!rows.length" @click="submit(false)">存草稿</el-button>
    </div>

    <el-divider />
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
      <span style="font-weight: 600">{{ cfg.title }} · 最近单据</span>
      <el-button link type="primary" @click="goDocList()">查看全部 →</el-button>
    </div>
    <el-table :data="recentDocs" size="small" border @row-click="(row: RecentDoc) => goDocList(row.doc_no)" style="cursor: pointer">
      <el-table-column prop="doc_no" label="单号" width="190" />
      <el-table-column label="日期" width="110">
        <template #default="{ row }">{{ (row.doc_date || '').slice(0, 10) }}</template>
      </el-table-column>
      <el-table-column prop="partner_name" label="往来单位" min-width="160" />
      <el-table-column label="金额" width="120" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.total_amount) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.status === 'confirmed' ? 'success' : row.status === 'voided' ? 'danger' : 'info'" size="small">
            {{ statusText[row.status] || row.status }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
