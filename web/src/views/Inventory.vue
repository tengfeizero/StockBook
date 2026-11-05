<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { reportApi, type StockRow, type BatchRow } from '../api/report'
import { locationApi } from '../api/archive'
import { fenToYuan } from '../utils/money'

const tab = ref<'product' | 'location'>('product')
const rows = ref<StockRow[]>([])
const loading = ref(false)
const keyword = ref('')
const batchMap = reactive<Record<number, BatchRow[]>>({})

// 按库位视图（v0.12 库位数量账）
interface LocStockRow {
  location_id: number; location_name: string; location_code: string
  product_id: number; product_code: string; product_name: string
  unit: string; barcode: string; plu: string; quantity: number
}
const locRows = ref<LocStockRow[]>([])
const locList = ref<{ id: number; name: string; code: string }[]>([])
const locFilter = ref<number | undefined>(undefined) // undefined=全部，0=未分配
const locLoading = ref(false)

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')

async function load() {
  loading.value = true
  try {
    rows.value = (await reportApi.stocks(keyword.value)) || []
  } finally {
    loading.value = false
  }
}

async function loadLocStock() {
  locLoading.value = true
  try {
    const params: any = { keyword: keyword.value }
    if (locFilter.value !== undefined) params.location_id = locFilter.value
    locRows.value = ((await locationApi.stock(params)) as any) || []
  } finally {
    locLoading.value = false
  }
}

async function switchTab(v: any) {
  if (v === 'location') {
    if (!locList.value.length) locList.value = ((await locationApi.list()) as any) || []
    await loadLocStock()
  } else {
    await load()
  }
}

function search() {
  if (tab.value === 'location') loadLocStock()
  else load()
}

// 库位合计：当前筛选范围内按库位汇总
const locSummary = computed(() => {
  const m = new Map<string, { name: string; skus: number; qty: number }>()
  for (const r of locRows.value) {
    const key = String(r.location_id)
    const cur = m.get(key) || { name: r.location_name || '未分配', skus: 0, qty: 0 }
    cur.skus += 1
    cur.qty += r.quantity
    m.set(key, cur)
  }
  return [...m.values()]
})
const unassignedCount = computed(() => locRows.value.filter((r) => r.location_id === 0).length)

async function onExpand(row: StockRow, expanded: StockRow[]) {
  if (!expanded.find((r) => r.product_id === row.product_id) || !row.is_batch) return
  if (batchMap[row.product_id]) return
  batchMap[row.product_id] = (await reportApi.batches(row.product_id)) || []
}

const costMethodText = (m: string) => (m === 'fifo' ? 'FIFO' : '加权平均')
const safeStatus = (row: StockRow) => {
  if (row.safe_stock <= 0) return null
  return row.quantity < row.safe_stock ? 'below' : 'ok'
}

onMounted(load)
</script>

<template>
  <el-card shadow="never">
    <el-tabs v-model="tab" @tab-change="switchTab">
      <el-tab-pane label="按商品" name="product" />
      <el-tab-pane label="按库位" name="location" />
    </el-tabs>

    <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center">
      <el-input
        v-model="keyword" placeholder="编码/名称/条码/PLU" style="width: 220px" clearable
        @keyup.enter="search()" @clear="search()"
      />
      <el-select v-if="tab === 'location'" v-model="locFilter" clearable placeholder="全部库位" style="width: 180px" @change="loadLocStock">
        <el-option label="未分配（待上架）" :value="0" />
        <el-option v-for="l in locList" :key="l.id" :label="l.code ? `${l.code} ${l.name}` : l.name" :value="l.id" />
      </el-select>
      <el-button type="primary" @click="search()">查询</el-button>
      <template v-if="tab === 'location'">
        <el-tag v-for="s in locSummary" :key="s.name" type="info" effect="plain">
          {{ s.name }}：{{ s.skus }} 个商品 / {{ qtyText(s.qty) }}
        </el-tag>
      </template>
    </div>

    <!-- 按商品 -->
    <el-table v-if="tab === 'product'" :data="rows" v-loading="loading" border @expand-change="onExpand">
      <el-table-column type="expand">
        <template #default="{ row }">
          <div v-if="row.is_batch" style="padding: 8px 24px">
            <el-table :data="batchMap[row.product_id] || []" border size="small">
              <el-table-column prop="batch_no" label="批次号" min-width="120" />
              <el-table-column label="剩余数量" width="110" align="right">
                <template #default="s">{{ qtyText(s.row.quantity) }}</template>
              </el-table-column>
              <el-table-column label="批次成本" width="110" align="right">
                <template #default="s">¥{{ fenToYuan(s.row.unit_cost) }}</template>
              </el-table-column>
              <el-table-column label="到期日" width="120">
                <template #default="s">{{ s.row.expire_date || '—' }}</template>
              </el-table-column>
            </el-table>
          </div>
          <div v-else style="padding: 8px 24px; color: var(--el-text-color-secondary)">非批次管理商品</div>
        </template>
      </el-table-column>
      <el-table-column prop="code" label="编码" width="110" />
      <el-table-column prop="name" label="商品名称" min-width="150" />
      <el-table-column prop="category_name" label="分类" width="100" />
      <el-table-column prop="unit" label="单位" width="70" />
      <el-table-column label="库存数量" width="110" align="right">
        <template #default="{ row }">
          <b :style="{ color: row.quantity < 0 ? 'var(--el-color-danger)' : undefined }">{{ qtyText(row.quantity) }}</b>
        </template>
      </el-table-column>
      <el-table-column label="成本均价" width="110" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.avg_cost) }}</template>
      </el-table-column>
      <el-table-column label="库存金额" width="120" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.total_value) }}</template>
      </el-table-column>
      <el-table-column label="成本法" width="90">
        <template #default="{ row }">{{ costMethodText(row.cost_method) }}</template>
      </el-table-column>
      <el-table-column label="安全库存" width="110">
        <template #default="{ row }">
          <template v-if="safeStatus(row) === 'below'">
            <el-tag type="danger" size="small">低于安全线</el-tag>
          </template>
          <template v-else-if="safeStatus(row) === 'ok'">
            <el-tag type="success" size="small">正常</el-tag>
          </template>
          <span v-else>—</span>
        </template>
      </el-table-column>
    </el-table>

    <!-- 按库位 -->
    <template v-else>
      <el-alert
        v-if="unassignedCount > 0"
        type="warning" :closable="false" show-icon style="margin-bottom: 12px"
        title="存在「未分配」库存：这些货还没有登记在具体货架上。到「库存作业 → 移库」把它们移到实际货架，库位账才完整。"
      />
      <el-table :data="locRows" v-loading="locLoading" border :default-sort="{ prop: 'location_id', order: 'ascending' }">
        <el-table-column label="库位" width="150">
          <template #default="{ row }">
            <el-tag v-if="row.location_id === 0" type="warning" size="small">未分配</el-tag>
            <span v-else>{{ row.location_code ? row.location_code + ' ' : '' }}{{ row.location_name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="product_code" label="编码" width="110" />
        <el-table-column prop="product_name" label="商品名称" min-width="180" />
        <el-table-column prop="unit" label="单位" width="70" />
        <el-table-column prop="plu" label="PLU" width="90">
          <template #default="{ row }">{{ row.plu || '—' }}</template>
        </el-table-column>
        <el-table-column label="数量" width="120" align="right">
          <template #default="{ row }">
            <b :style="{ color: row.quantity < 0 ? 'var(--el-color-danger)' : undefined }">{{ qtyText(row.quantity) }}</b>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!locLoading && !locRows.length" style="padding: 24px; text-align: center; color: var(--el-text-color-placeholder)">
        暂无库位库存记录
      </div>
    </template>
  </el-card>
</template>
