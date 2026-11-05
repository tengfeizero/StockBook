<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { locationApi, openingApi, productApi, type OpeningStockRow, type OpeningReq } from '../api/archive'
import { fenToYuan } from '../utils/money'

interface ProductOption { id: number; code: string; name: string; unit: string; is_batch_enabled: boolean; default_location_id?: number }

const stocks = ref<OpeningStockRow[]>([])
const loading = ref(false)
const saving = ref(false)

// 录入行
interface Row { product: ProductOption; qty: number; cost_yuan: number; batch_no: string; location_id: number | undefined }
const rows = ref<Row[]>([])
const searchKeyword = ref('')
const searchOptions = ref<ProductOption[]>([])
const searching = ref(false)

async function load() {
  loading.value = true
  try {
    stocks.value = (await openingApi.stocks()) as any
  } finally {
    loading.value = false
  }
}

// 远程搜商品（排除已建账）
async function searchProduct(kw: string) {
  if (!kw) return
  searching.value = true
  try {
    const data: any = await productApi.list({ keyword: kw, page: 1, page_size: 10 })
    const built = new Set(stocks.value.filter((s) => s.quantity > 0).map((s) => s.product_id))
    searchOptions.value = data.list.filter((p: any) => !built.has(p.id))
  } finally {
    searching.value = false
  }
}

function addRow(p: ProductOption) {
  if (rows.value.some((r) => r.product.id === p.id)) return
  rows.value.push({ product: p, qty: 0, cost_yuan: 0, batch_no: '', location_id: p.default_location_id || undefined })
  searchKeyword.value = ''
  searchOptions.value = []
}

function removeRow(i: number) {
  rows.value.splice(i, 1)
}

async function save() {
  if (!rows.value.length) return ElMessage.warning('请先添加商品')
  if (rows.value.some((r) => r.qty <= 0)) return ElMessage.warning('期初数量必须大于 0')
  if (rows.value.some((r) => r.product.is_batch_enabled && !r.batch_no.trim()))
    return ElMessage.warning('启用批次的商品需填写批次号（不填将默认「期初」）')
  saving.value = true
  const payload: OpeningReq[] = rows.value.map((r) => ({
    product_id: r.product.id,
    quantity: Math.round(r.qty * 1000),
    unit_cost: Math.round(r.cost_yuan * 100),
    batch_no: r.batch_no,
    location_id: r.location_id || 0,
  }))
  try {
    await openingApi.save(payload)
    ElMessage.success('期初建账成功')
    rows.value = []
    load()
  } finally {
    saving.value = false
  }
}

const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')

// 库位列表（v0.12：期初可指定存放货架，不选则记为未分配）
const locations = ref<{ id: number; name: string }[]>([])

onMounted(() => {
  load()
  locationApi.list().then((l: any) => (locations.value = l || [])).catch(() => {})
})
</script>

<template>
  <el-card shadow="never" style="margin-bottom: 16px">
    <template #header>期初建账录入</template>
    <el-select
      v-model="searchKeyword"
      filterable
      remote
      :remote-method="searchProduct"
      :loading="searching"
      placeholder="搜索商品名称/编码/条码，添加到期初清单"
      style="width: 420px"
      value-key="id"
      @change="(v: any) => v && addRow(v)"
    >
      <el-option v-for="p in searchOptions" :key="p.id" :label="`${p.code} ${p.name}`" :value="p" />
    </el-select>

    <el-table v-if="rows.length" :data="rows" border style="margin-top: 12px">
      <el-table-column label="商品" min-width="200">
        <template #default="{ row }">{{ row.product.code }} {{ row.product.name }}</template>
      </el-table-column>
      <el-table-column prop="product.unit" label="单位" width="70" />
      <el-table-column label="期初数量" width="160">
        <template #default="{ row }">
          <el-input-number v-model="row.qty" :min="0" :precision="3" style="width: 130px" />
        </template>
      </el-table-column>
      <el-table-column label="期初单价(元)" width="160">
        <template #default="{ row }">
          <el-input-number v-model="row.cost_yuan" :min="0" :precision="2" style="width: 130px" />
        </template>
      </el-table-column>
      <el-table-column label="批次号" width="180">
        <template #default="{ row }">
          <el-input v-if="row.product.is_batch_enabled" v-model="row.batch_no" placeholder="默认「期初」" />
          <span v-else style="color: var(--el-text-color-placeholder)">—</span>
        </template>
      </el-table-column>
      <el-table-column v-if="locations.length" label="库位" width="160">
        <template #default="{ row }">
          <el-select v-model="row.location_id" size="small" clearable placeholder="未分配">
            <el-option v-for="l in locations" :key="l.id" :label="l.name" :value="l.id" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="80">
        <template #default="{ $index }">
          <el-button link type="danger" @click="removeRow($index)">移除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 12px">
      <el-button type="primary" :loading="saving" :disabled="!rows.length" @click="save">保存期初建账</el-button>
      <span style="margin-left: 12px; color: var(--el-text-color-secondary); font-size: 12px">
        已建账的商品不能重复录入；后续库存调整请用「其他出入库」单
      </span>
    </div>
  </el-card>

  <el-card shadow="never">
    <template #header>当前库存（{{ stocks.length }} 个商品）</template>
    <el-table :data="stocks" v-loading="loading" border>
      <el-table-column prop="code" label="编码" width="110" />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column prop="unit" label="单位" width="70" />
      <el-table-column label="库存数量" width="120" align="right">
        <template #default="{ row }">{{ qtyText(row.quantity) }}</template>
      </el-table-column>
      <el-table-column label="库存成本均价" width="130" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.avg_cost) }}</template>
      </el-table-column>
      <el-table-column label="批次" width="80">
        <template #default="{ row }">{{ row.is_batch_enabled ? '是' : '—' }}</template>
      </el-table-column>
    </el-table>
  </el-card>
</template>
