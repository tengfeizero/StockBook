<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { reportApi, type StockRow } from '../api/report'
import { docApi, type Doc, type DocItem } from '../api/doc'
import { locationApi, productApi, type Location, type LocationStockRow } from '../api/archive'
import { yuanToFen } from '../utils/money'

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')

// ---------------- 范围 ----------------
const scope = ref<'warehouse' | 'location'>('warehouse')
function onScopeChange() {
  if (scope.value === 'location' && curLoc.value === null && locs.value.length > 0) {
    pickLoc(locs.value[0].id)
  }
}

// ---------------- 全仓盘点（原逻辑） ----------------
const rows = ref<StockRow[]>([])
const loading = ref(false)
const keyword = ref('')
const actuals = reactive<Record<number, { actual: string; cost: string }>>({})

async function load() {
  loading.value = true
  try {
    rows.value = (await reportApi.stocks(keyword.value)) || []
    // 复盘：初盘有差异但当前账上已为 0 的商品（盘亏过），库存查询不一定返回 → 补一行账面 0
    if (isRecheck.value && refDoc.value?.scope !== 'location' && !keyword.value) {
      for (const it of refItems.value) {
        if (rows.value.some((r) => r.product_id === it.product_id)) continue
        rows.value.push({
          product_id: it.product_id, code: it.product_code, name: it.product_name,
          category_name: '', unit: '', cost_method: '', is_batch: false,
          quantity: 0, avg_cost: 0, total_value: 0, safe_stock: 0,
        })
      }
    }
    for (const r of rows.value) {
      if (!actuals[r.product_id]) actuals[r.product_id] = { actual: '', cost: '' }
    }
  } finally {
    loading.value = false
  }
}

function diffOf(row: StockRow): number | null {
  const a = actuals[row.product_id]
  if (!a || a.actual === '') return null
  const v = Math.round(parseFloat(a.actual) * 1000)
  if (isNaN(v)) return null
  return v - row.quantity
}

const changedRows = computed(() => rows.value.filter((r) => diffOf(r) !== null && diffOf(r) !== 0))

// ---------------- 按货架盘点（v0.13） ----------------
const locs = ref<Location[]>([])
const curLoc = ref<number | null>(null)
const locRows = ref<LocationStockRow[]>([])
const locLoading = ref(false)
// 已录入的实盘数：`库位:商品` → { actual, cost }（切换货架不丢）
const locActs = reactive<Record<string, { actual: string; cost: string }>>({})
// 各库位账面量（用于算盈亏，避免切换货架后再查一次接口）
const qtyMap = reactive<Record<string, number>>({})
// 手工添加的商品（空货架上发现多出来的货 → 盘盈）
const extras = reactive<Record<number, LocationStockRow[]>>({})

const akey = (locId: number, pid: number) => `${locId}:${pid}`
// 模板里用的 key：未选货架时给空串（表格此时不渲染）
const curKey = (pid: number) => (curLoc.value === null ? '' : akey(curLoc.value, pid))

const curLocInfo = computed(() => locs.value.find((l) => l.id === curLoc.value) || null)

async function loadLocs() {
  locs.value = (await locationApi.list()) || []
  if (scope.value !== 'location' || curLoc.value !== null || locs.value.length === 0) return
  // 复盘优先定位到初盘第一个差异货架，省去用户自己找
  const refLoc = isRecheck.value ? refItems.value[0]?.location_id : undefined
  if (refLoc === 0) pickLoc(0)
  else if (refLoc != null && locs.value.some((l) => l.id === refLoc)) pickLoc(refLoc)
  else pickLoc(locs.value[0].id)
}

async function loadLocRows(locId: number) {
  locLoading.value = true
  try {
    const data = (await locationApi.stock({ location_id: locId })) || []
    for (const r of data) {
      qtyMap[akey(locId, r.product_id)] = r.quantity
      if (!locActs[akey(locId, r.product_id)]) locActs[akey(locId, r.product_id)] = { actual: '', cost: '' }
    }
    const ex = (extras[locId] || []).filter((e) => !data.some((r) => r.product_id === e.product_id))
    const list: LocationStockRow[] = [...data, ...ex]
    // 复盘：初盘该货架有差异、当前架上已无此货（盘亏到 0）→ 补一行账面 0，否则无法复盘
    if (isRecheck.value) {
      for (const it of refItems.value) {
        if (it.location_id !== locId) continue
        if (list.some((r) => r.product_id === it.product_id)) continue
        qtyMap[akey(locId, it.product_id)] = 0
        if (!locActs[akey(locId, it.product_id)]) locActs[akey(locId, it.product_id)] = { actual: '', cost: '' }
        list.push({
          location_id: locId, location_name: '', location_code: '',
          product_id: it.product_id, product_code: it.product_code,
          product_name: it.product_name, unit: '', barcode: '', plu: '', quantity: 0,
        })
      }
    }
    locRows.value = list
  } finally {
    locLoading.value = false
  }
}

function pickLoc(id: number | null) {
  curLoc.value = id
  if (id !== null) loadLocRows(id)
}

function locDiff(row: LocationStockRow): number | null {
  if (curLoc.value === null) return null
  const a = locActs[akey(curLoc.value, row.product_id)]
  if (!a || a.actual === '') return null
  const v = Math.round(parseFloat(a.actual) * 1000)
  if (isNaN(v)) return null
  return v - (qtyMap[akey(curLoc.value, row.product_id)] ?? row.quantity ?? 0)
}

// 本次要提交的差异行（跨库位汇总）
const locChanged = computed(() => {
  const out: { loc: number; name: string; pid: number; qty: number; cost: number }[] = []
  for (const [k, v] of Object.entries(locActs)) {
    if (v.actual === '') continue
    const qty = Math.round(parseFloat(v.actual) * 1000)
    if (isNaN(qty)) continue
    const base = qtyMap[k]
    if (base === undefined || qty === base) continue
    const [locS, pidS] = k.split(':')
    const loc = Number(locS)
    out.push({
      loc, name: locs.value.find((l) => l.id === loc)?.name || '未分配',
      pid: Number(pidS), qty, cost: v.cost ? yuanToFen(v.cost) : 0,
    })
  }
  return out
})

const locCount = computed(() => new Set(locChanged.value.map((c) => c.loc)).size)

// 视图：复盘模式下只显示初盘有差异的行（复盘=核对差异，不重盘整仓）
const viewRows = computed(() =>
  isRecheck.value
    ? rows.value.filter((r) => refItems.value.some((it) => it.product_id === r.product_id))
    : rows.value,
)
const viewLocRows = computed(() =>
  isRecheck.value
    ? locRows.value.filter((r) =>
        refItems.value.some((it) => it.location_id === curLoc.value && it.product_id === r.product_id),
      )
    : locRows.value,
)
// 已录入项数（含账实相符）：盘点单会记录全部已盘行，差异只是其中一部分
const warehouseScanned = computed(
  () => rows.value.filter((r) => (actuals[r.product_id]?.actual ?? '') !== '').length,
)
const locScanned = computed(() => Object.values(locActs).filter((v) => v.actual !== '').length)
const curChanged = computed(() =>
  curLoc.value === null ? 0 : locChanged.value.filter((c) => c.loc === curLoc.value).length,
)

// 添加商品（盘盈：货架上多出来的、账面没有的货）
const addDlg = ref(false)
const searchKw = ref('')
const searchOpts = ref<any[]>([])
const searching = ref(false)
async function searchProducts(kw: string) {
  searching.value = true
  try {
    const data: any = await productApi.list({ keyword: kw, page: 1, page_size: 20, status: 'active' })
    searchOpts.value = data.list || []
  } finally {
    searching.value = false
  }
}
function addProduct(p: any) {
  if (curLoc.value === null) return
  if (locRows.value.some((r) => r.product_id === p.id)) {
    ElMessage.info('该商品已在本货架列表中')
    return
  }
  const k = akey(curLoc.value, p.id)
  qtyMap[k] = 0
  locActs[k] = { actual: '', cost: '' }
  const row: LocationStockRow = {
    location_id: curLoc.value, location_name: '', location_code: '',
    product_id: p.id, product_code: p.code, product_name: p.name,
    unit: p.unit || '', barcode: p.barcode || '', plu: p.plu || '', quantity: 0,
  }
  extras[curLoc.value] = [...(extras[curLoc.value] || []), row]
  locRows.value = [...locRows.value, row]
  addDlg.value = false
  searchKw.value = ''
}

function resetLocActs() {
  for (const k of Object.keys(locActs)) delete locActs[k]
  for (const k of Object.keys(extras)) delete extras[Number(k)]
  for (const k of Object.keys(qtyMap)) delete qtyMap[k]
}

function clearLocActs() {
  resetLocActs()
  if (curLoc.value !== null) loadLocRows(curLoc.value)
  ElMessage.success('已清空本次录入')
}

// ---------------- 复盘模式（v0.15） ----------------
// 从盘点单详情「按差异项复盘」进入：/stocktake?ref=<初盘单 id>
// 只盘初盘有差异的商品；账面数取当前库存（初盘已调整后的账），复盘数再按新差额调整
const route = useRoute()
const refDoc = ref<Doc | null>(null)
const refItems = ref<DocItem[]>([])
const isRecheck = computed(() => !!refDoc.value)

async function loadRefDoc() {
  const refId = Number(route.query.ref || 0)
  if (!refId) return
  const d: any = await docApi.get(refId)
  // 初盘差异行：实盘数 ≠ 账面数（book_qty 为确认时账面快照）
  const items: DocItem[] = (d.items || []).filter((it: DocItem) => it.book_qty !== it.quantity)
  refDoc.value = d
  refItems.value = items
  scope.value = d.scope === 'location' ? 'location' : 'warehouse'
  for (const it of items) {
    if (d.scope === 'location') {
      const k = akey(it.location_id, it.product_id)
      if (!locActs[k]) locActs[k] = { actual: '', cost: '' }
    } else if (!actuals[it.product_id]) {
      actuals[it.product_id] = { actual: '', cost: '' }
    }
  }
}

// ---------------- 提交 ----------------
// 盘点单记录**全部已盘行**（含账实相符），差异靠 book_qty 与实盘数对比得出；
// 只写差异行会丢掉「盘过哪些」，事后既无法复盘也无法证明盘过
function scannedWarehouseItems() {
  const out: { product_id: number; quantity: number; price: number }[] = []
  for (const r of rows.value) {
    const a = actuals[r.product_id]
    if (!a || a.actual === '') continue
    const qty = Math.round(parseFloat(a.actual) * 1000)
    if (isNaN(qty)) continue
    // 盘盈成本只对盘盈行有意义；无差异行不填价，避免放大单据金额
    out.push({ product_id: r.product_id, quantity: qty, price: qty > r.quantity && a.cost ? yuanToFen(a.cost) : 0 })
  }
  return out
}

function scannedLocationItems() {
  const out: { product_id: number; quantity: number; price: number; location_id: number }[] = []
  for (const [k, v] of Object.entries(locActs)) {
    if (v.actual === '') continue
    const qty = Math.round(parseFloat(v.actual) * 1000)
    if (isNaN(qty)) continue
    const base = qtyMap[k]
    if (base === undefined) continue
    const [locS, pidS] = k.split(':')
    out.push({
      product_id: Number(pidS), quantity: qty,
      price: qty > base && v.cost ? yuanToFen(v.cost) : 0,
      location_id: Number(locS),
    })
  }
  return out
}

async function submitWarehouse() {
  const items = scannedWarehouseItems()
  if (items.length === 0) {
    ElMessage.warning('没有录入实盘数（留空=不盘）')
    return
  }
  const res: any = await docApi.create({
    doc_type: 'stocktake',
    confirm_now: true,
    remark: isRecheck.value ? `复盘（初盘 ${refDoc.value?.doc_no}）` : '库存盘点（全仓）',
    ref_doc_id: refDoc.value?.id,
    items,
  })
  if (res.status === 'review') {
    ElMessage.warning(`盘点单 ${res.doc_no} 差异超阈值，已提交主管审批，库存暂未调整`)
  } else {
    ElMessage.success(`盘点单 ${res.doc_no} 已生效：已盘 ${items.length} 项、差异 ${changedRows.value.length} 项`)
  }
  for (const r of rows.value) if (actuals[r.product_id]) actuals[r.product_id] = { actual: '', cost: '' }
  load()
}

async function submitLocation() {
  const items = scannedLocationItems()
  if (items.length === 0) {
    ElMessage.warning('没有录入实盘数（留空=不盘）')
    return
  }
  const res: any = await docApi.create({
    doc_type: 'stocktake',
    scope: 'location',
    confirm_now: true,
    remark: isRecheck.value
      ? `复盘（初盘 ${refDoc.value?.doc_no}，${locCount.value} 个货架）`
      : `按货架盘点（${locCount.value} 个货架）`,
    ref_doc_id: refDoc.value?.id,
    items,
  })
  if (res.status === 'review') {
    ElMessage.warning(`盘点单 ${res.doc_no} 差异超阈值，已提交主管审批，库存暂未调整`)
  } else {
    ElMessage.success(`盘点单 ${res.doc_no} 已生效：已盘 ${items.length} 项、差异 ${locChanged.value.length} 项`)
  }
  resetLocActs()
  await loadLocs()
  if (curLoc.value !== null) loadLocRows(curLoc.value)
}

onMounted(async () => {
  await loadRefDoc() // 复盘模式要先知道初盘差异项，再加载账面
  load()
  loadLocs()
})
</script>

<template>
  <el-card shadow="never">
    <template #header>
      <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap">
        <span style="font-weight: 600">库存盘点</span>
        <el-radio-group v-model="scope" size="small" @change="onScopeChange">
          <el-radio-button value="warehouse">全仓盘点</el-radio-button>
          <el-radio-button value="location">按货架盘点</el-radio-button>
        </el-radio-group>
        <span style="color: var(--el-text-color-secondary); font-size: 12px">
          {{ scope === 'warehouse'
            ? '按「商品全仓总量」盘：实盘数填整个仓库的实际数量，差额自动盘盈/盘亏'
            : '按「货架」盘：逐架录入实盘数，差额只调该货架；可切换多个货架后一次提交' }}
        </span>
      </div>
    </template>

    <!-- 复盘：只盘初盘差异项，提交后生成关联初盘的新盘点单 -->
    <el-alert
      v-if="isRecheck" type="warning" show-icon :closable="false" style="margin-bottom: 12px"
      :title="`复盘模式 · 初盘 ${refDoc?.doc_no}（${(refDoc?.doc_date || '').slice(0, 10)}）共 ${refItems.length} 项差异`"
    >
      <span style="font-size: 12px">
        只列出初盘有差异的商品；账面数是初盘调整后的<b>当前库存</b>，录入复盘实盘数后按新差额调整。
        提交生成关联初盘的新盘点单，差异表可对比两张单核对复盘结果。
      </span>
    </el-alert>

    <!-- ============ 全仓盘点 ============ -->
    <template v-if="scope === 'warehouse'">
      <div style="margin-bottom: 10px; display: flex; gap: 8px">
        <el-input v-model="keyword" placeholder="编码/名称" style="width: 200px" size="small" clearable @keyup.enter="load()" />
        <el-button size="small" @click="load()">刷新</el-button>
      </div>

      <el-table :data="viewRows" v-loading="loading" border size="small">
        <el-table-column prop="code" label="编码" width="110" />
        <el-table-column prop="name" label="商品名称" min-width="150" />
        <el-table-column prop="unit" label="单位" width="70" />
        <el-table-column label="账面数量" width="110" align="right">
          <template #default="{ row }">{{ qtyText(row.quantity) }}</template>
        </el-table-column>
        <el-table-column label="实盘数量" width="130">
          <template #default="{ row }">
            <el-input v-model="actuals[row.product_id].actual" placeholder="留空=不盘" size="small" type="number" />
          </template>
        </el-table-column>
        <el-table-column label="盈亏" width="110" align="right">
          <template #default="{ row }">
            <template v-if="diffOf(row) !== null">
              <b :style="{ color: diffOf(row)! > 0 ? 'var(--el-color-success)' : diffOf(row)! < 0 ? 'var(--el-color-danger)' : undefined }">
                {{ diffOf(row)! > 0 ? '+' : '' }}{{ qtyText(diffOf(row)!) }}
              </b>
            </template>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column label="盘盈成本价(元,可选)" width="150">
          <template #default="{ row }">
            <el-input v-model="actuals[row.product_id].cost" placeholder="默认账面均价" size="small" type="number" :disabled="!diffOf(row) || diffOf(row)! <= 0" />
          </template>
        </el-table-column>
      </el-table>

      <div style="margin-top: 16px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap">
        <span>已盘 <b>{{ warehouseScanned }}</b> 项 · 差异 <b>{{ changedRows.length }}</b> 项</span>
        <el-button type="primary" @click="submitWarehouse">保存并生效（调整库存）</el-button>
        <span style="color: var(--el-text-color-secondary); font-size: 12px">
          生成盘点单（PD 单号）并立即生效；作废该单可回冲盘点结果。
        </span>
      </div>
    </template>

    <!-- ============ 按货架盘点 ============ -->
    <template v-else>
      <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 10px; flex-wrap: wrap">
        <span style="font-size: 13px">当前货架</span>
        <el-select :model-value="curLoc" placeholder="选择货架" style="width: 220px" size="small" @update:model-value="pickLoc">
          <el-option v-for="l in locs" :key="l.id" :value="l.id" :label="`${l.code ? l.code + ' · ' : ''}${l.name}`" />
          <el-option :value="0" label="未分配（没有上架的货）" />
        </el-select>
        <template v-if="curLocInfo">
          <el-tag v-if="curLocInfo.status && curLocInfo.status !== 'normal'" size="small"
            :type="curLocInfo.status === 'over' ? 'danger' : curLocInfo.status === 'under' ? 'warning' : 'warning'">
            {{ curLocInfo.status_text }}
          </el-tag>
          <span v-if="curLocInfo.capacity" style="font-size: 12px; color: var(--el-text-color-secondary)">
            占用 {{ qtyText(curLocInfo.used_qty || 0) }} / 容量 {{ qtyText(curLocInfo.capacity) }}
            （{{ curLocInfo.usage_rate }}%）
          </span>
          <span v-else style="font-size: 12px; color: var(--el-text-color-placeholder)">未设容量</span>
        </template>
        <el-button v-if="!isRecheck" size="small" @click="addDlg = true; searchProducts('')">添加商品</el-button>
        <span style="font-size: 12px; color: var(--el-text-color-secondary)">
          货架上有多出来的货（账面没有）时用「添加商品」登记，实盘数直接填数量
        </span>
      </div>

      <el-table v-if="curLoc !== null" :data="viewLocRows" v-loading="locLoading" border size="small" empty-text="该货架暂无在库商品（可用「添加商品」登记盘盈）">
        <el-table-column label="编码" width="110">
          <template #default="{ row }">{{ row.product_code }}</template>
        </el-table-column>
        <el-table-column label="商品名称" min-width="150">
          <template #default="{ row }">{{ row.product_name }}</template>
        </el-table-column>
        <el-table-column label="单位" width="70">
          <template #default="{ row }">{{ row.unit }}</template>
        </el-table-column>
        <el-table-column label="本架账面" width="110" align="right">
          <template #default="{ row }">
            {{ qtyText(qtyMap[curKey(row.product_id)] ?? row.quantity) }}
          </template>
        </el-table-column>
        <el-table-column label="实盘数量" width="130">
          <template #default="{ row }">
            <el-input v-model="locActs[curKey(row.product_id)].actual" placeholder="留空=不盘" size="small" type="number" />
          </template>
        </el-table-column>
        <el-table-column label="盈亏" width="110" align="right">
          <template #default="{ row }">
            <template v-if="locDiff(row) !== null">
              <b :style="{ color: locDiff(row)! > 0 ? 'var(--el-color-success)' : locDiff(row)! < 0 ? 'var(--el-color-danger)' : undefined }">
                {{ locDiff(row)! > 0 ? '+' : '' }}{{ qtyText(locDiff(row)!) }}
              </b>
            </template>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column label="盘盈成本价(元,可选)" width="150">
          <template #default="{ row }">
            <el-input v-model="locActs[curKey(row.product_id)].cost" placeholder="默认账面均价" size="small" type="number"
              :disabled="!locDiff(row) || locDiff(row)! <= 0" />
          </template>
        </el-table-column>
      </el-table>

      <div style="margin-top: 16px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap">
        <span>本货架差异：<b>{{ curChanged }}</b> 项</span>
        <span>汇总：已录入 <b>{{ locScanned }}</b> 项 · 差异 <b>{{ locChanged.length }}</b> 项，涉及 <b>{{ locCount }}</b> 个货架</span>
        <el-button type="primary" @click="submitLocation">保存并生效（按库位调整）</el-button>
        <el-button v-if="locChanged.length > 0" @click="clearLocActs">清空录入</el-button>
        <span style="color: var(--el-text-color-secondary); font-size: 12px">
          切换货架不会丢失已录入数据；提交后生成一张盘点单，明细带库位，作废可整单回冲。
        </span>
      </div>
    </template>

    <!-- 添加商品（盘盈） -->
    <el-dialog v-model="addDlg" title="添加商品到当前货架" width="480px">
      <el-select
        v-model="searchKw" filterable remote clearable style="width: 100%"
        placeholder="输入编码/名称/条码/PLU 搜索商品"
        :remote-method="searchProducts" :loading="searching" @change="(v: any) => { const p = searchOpts.find((o) => o.id === v); if (p) addProduct(p) }"
      >
        <el-option v-for="p in searchOpts" :key="p.id" :value="p.id" :label="`${p.code} ${p.name}`" />
      </el-select>
      <div style="margin-top: 8px; font-size: 12px; color: var(--el-text-color-secondary)">
        适用于「货架上有货、账面没有」的盘盈场景，添加后把实盘数填上即可。
      </div>
    </el-dialog>
  </el-card>
</template>
