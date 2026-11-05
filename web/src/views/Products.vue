<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { categoryApi, locationApi, productApi, type ProductPayload } from '../api/archive'
import ImportDialog from '../components/ImportDialog.vue'
import { fenToYuan } from '../utils/money'

interface Category { id: number; name: string }
interface PriceRow { name: string; price: number } // price 单位：分
interface ProductRow {
  id: number; code: string; barcode: string; plu: string; name: string; category_id: number; unit: string
  cost_method: string; is_batch_enabled: boolean; shelf_life_days: number; safe_stock: number
  volume: number; weight: number
  status: string; default_price_id: number; default_location_id: number
  prices: (PriceRow & { id: number })[]
}

const list = ref<ProductRow[]>([])
const total = ref(0)
const loading = ref(false)
const categories = ref<Category[]>([])
const locations = ref<{ id: number; name: string }[]>([])
const query = reactive({ page: 1, page_size: 20, keyword: '', category_id: undefined as number | undefined })

const dialog = ref(false)
const importVisible = ref(false)
const isEdit = ref(false)
const saving = ref(false)
const form = reactive({
  id: 0, code: '', barcode: '', plu: '', name: '', category_id: undefined as number | undefined,
  unit: '', cost_method: 'weighted_avg' as 'weighted_avg' | 'fifo',
  is_batch_enabled: false, shelf_life_days: 0, safe_stock_yuan: 0, volume_l: 0, weight_kg: 0, status: 'active',
  prices: [{ name: '零售价', price: 0 }] as PriceRow[],
  default_price: '零售价',
  default_location_id: undefined as number | undefined,
})

async function load() {
  loading.value = true
  try {
    const data: any = await productApi.list(query)
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function loadCategories() {
  categories.value = (await categoryApi.list()) as any
}

async function loadLocations() {
  locations.value = ((await locationApi.list()) as any) || []
}

function openCreate() {
  isEdit.value = false
  Object.assign(form, {
    id: 0, code: '', barcode: '', plu: '', name: '', category_id: undefined, unit: '',
    cost_method: 'weighted_avg', is_batch_enabled: false, shelf_life_days: 0,
    safe_stock_yuan: 0, volume_l: 0, weight_kg: 0, status: 'active',
    prices: [{ name: '零售价', price: 0 }], default_price: '零售价',
    default_location_id: undefined,
  })
  dialog.value = true
}

function openEdit(row: ProductRow) {
  isEdit.value = true
  Object.assign(form, {
    id: row.id, code: row.code, barcode: row.barcode, plu: row.plu || '', name: row.name,
    category_id: row.category_id || undefined, unit: row.unit,
    cost_method: row.cost_method, is_batch_enabled: row.is_batch_enabled,
    shelf_life_days: row.shelf_life_days, safe_stock_yuan: row.safe_stock / 1000,
    volume_l: (row.volume || 0) / 1000, weight_kg: (row.weight || 0) / 1000,
    status: row.status,
    prices: row.prices.map((p) => ({ name: p.name, price: p.price })),
    default_price: row.prices.find((p) => p.id === row.default_price_id)?.name || row.prices[0]?.name || '',
    default_location_id: row.default_location_id || undefined,
  })
  dialog.value = true
}

function addPrice() { form.prices.push({ name: '', price: 0 }) }
function removePrice(i: number) { form.prices.splice(i, 1) }

async function submit() {
  if (!form.code.trim() || !form.name.trim()) return ElMessage.warning('编码和名称必填')
  if (form.prices.some((p) => !p.name.trim())) return ElMessage.warning('价位名称不能为空')
  saving.value = true
  const payload: ProductPayload = {
    code: form.code, barcode: form.barcode, plu: form.plu, name: form.name,
    category_id: form.category_id, unit: form.unit, cost_method: form.cost_method,
    is_batch_enabled: form.is_batch_enabled, shelf_life_days: form.shelf_life_days,
    safe_stock: Math.round(form.safe_stock_yuan * 1000),
    volume: Math.round((form.volume_l || 0) * 1000), weight: Math.round((form.weight_kg || 0) * 1000),
    status: form.status,
    prices: form.prices.map((p) => ({ name: p.name, price: p.price })),
    default_price: form.default_price,
    default_location_id: form.default_location_id || 0,
  }
  try {
    if (isEdit.value) await productApi.update(form.id, payload)
    else await productApi.create(payload)
    ElMessage.success('保存成功')
    dialog.value = false
    load()
  } finally {
    saving.value = false
  }
}

async function remove(row: ProductRow) {
  await ElMessageBox.confirm(`确定删除商品「${row.name}」？`, '提示', { type: 'warning' })
  await productApi.remove(row.id)
  ElMessage.success('已删除')
  load()
}

const categoryName = (id: number) => categories.value.find((c) => c.id === id)?.name || '—'
const locationName = (id: number) => locations.value.find((l) => l.id === id)?.name || '—'
const priceText = (row: ProductRow) => row.prices.map((p) => `${p.name} ${fenToYuan(p.price)}`).join(' / ')

onMounted(() => { load(); loadCategories(); loadLocations() })
</script>

<template>
  <el-card shadow="never">
    <div style="margin-bottom: 12px; display: flex; gap: 8px; flex-wrap: wrap">
      <el-button type="primary" @click="openCreate">新建商品</el-button>
      <el-button @click="importVisible = true">批量导入</el-button>
      <el-input v-model="query.keyword" placeholder="名称/编码/条码" style="width: 200px" clearable @keyup.enter="query.page = 1; load()" />
      <el-select v-model="query.category_id" placeholder="全部分类" clearable style="width: 140px" @change="query.page = 1; load()">
        <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
      <el-button @click="query.page = 1; load()">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="code" label="编码" width="100" />
      <el-table-column prop="name" label="名称" min-width="160" />
      <el-table-column label="分类" width="110">
        <template #default="{ row }">{{ categoryName(row.category_id) }}</template>
      </el-table-column>
      <el-table-column prop="unit" label="单位" width="70" />
      <el-table-column prop="barcode" label="条码" width="130" />
      <el-table-column prop="plu" label="PLU" width="90">
        <template #default="{ row }">
          <span v-if="row.plu" style="font-family: monospace">{{ row.plu }}</span>
          <span v-else style="color: var(--el-text-color-placeholder)">—</span>
        </template>
      </el-table-column>
      <el-table-column label="价位" min-width="200">
        <template #default="{ row }">{{ priceText(row) }}</template>
      </el-table-column>
      <el-table-column label="成本法" width="110">
        <template #default="{ row }">{{ row.cost_method === 'fifo' ? '先进先出' : '加权平均' }}</template>
      </el-table-column>
      <el-table-column label="批次" width="70">
        <template #default="{ row }">{{ row.is_batch_enabled ? '是' : '—' }}</template>
      </el-table-column>
      <el-table-column label="默认库位" width="110">
        <template #default="{ row }">{{ row.default_location_id ? locationName(row.default_location_id) : '—' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
            {{ row.status === 'active' ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      v-model:current-page="query.page"
      :page-size="query.page_size"
      :total="total"
      layout="total, prev, pager, next"
      style="margin-top: 12px; justify-content: flex-end"
      @current-change="load"
    />
  </el-card>

    <ImportDialog v-model="importVisible" kind="products" title="商品" @success="load" />

  <el-dialog v-model="dialog" :title="isEdit ? '编辑商品' : '新建商品'" width="680px" top="5vh">
    <el-form label-width="90px">
      <el-row :gutter="12">
        <el-col :span="12"><el-form-item label="编码" required><el-input v-model="form.code" /></el-form-item></el-col>
        <el-col :span="12"><el-form-item label="条码"><el-input v-model="form.barcode" /></el-form-item></el-col>
        <el-col :span="12">
          <el-form-item label="PLU 码">
            <el-input v-model="form.plu" placeholder="电子秤上的货号，如 00123，可空" maxlength="20" />
          </el-form-item>
        </el-col>
        <el-col :span="12"><el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item></el-col>
        <el-col :span="6"><el-form-item label="分类">
          <el-select v-model="form.category_id" clearable style="width: 100%">
            <el-option v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item></el-col>
        <el-col :span="6"><el-form-item label="单位"><el-input v-model="form.unit" /></el-form-item></el-col>
        <el-col :span="12"><el-form-item label="成本方式">
          <el-radio-group v-model="form.cost_method" :disabled="isEdit">
            <el-radio value="weighted_avg">加权平均</el-radio>
            <el-radio value="fifo">先进先出</el-radio>
          </el-radio-group>
        </el-form-item></el-col>
        <el-col :span="12"><el-form-item label="批次管理">
          <el-switch v-model="form.is_batch_enabled" :disabled="form.cost_method === 'fifo'" />
          <span v-if="form.cost_method === 'fifo'" style="margin-left: 8px; color: var(--el-text-color-secondary); font-size: 12px">FIFO 商品强制开启</span>
        </el-form-item></el-col>
        <el-col :span="12" v-if="form.is_batch_enabled">
          <el-form-item label="保质期(天)"><el-input-number v-model="form.shelf_life_days" :min="0" style="width: 100%" /></el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="安全库存"><el-input-number v-model="form.safe_stock_yuan" :min="0" :precision="3" style="width: 100%" /></el-form-item>
          <el-form-item label="单件体积(升)"><el-input-number v-model="form.volume_l" :min="0" :precision="3" style="width: 100%" /></el-form-item>
        </el-col>
        <el-col :span="12"><el-form-item label="默认库位">
          <el-select v-model="form.default_location_id" clearable placeholder="选填，开单自动带出" style="width: 100%">
            <el-option v-for="l in locations" :key="l.id" :label="l.name" :value="l.id" />
          </el-select>
        </el-form-item></el-col>
        <el-col :span="12"><el-form-item label="状态">
          <el-switch v-model="form.status" active-value="active" inactive-value="disabled" active-text="启用" inactive-text="停用" />
        </el-form-item></el-col>
      </el-row>

      <el-form-item label="价位">
        <div style="width: 100%">
          <div v-for="(p, i) in form.prices" :key="i" style="display: flex; gap: 8px; margin-bottom: 8px">
            <el-input v-model="p.name" placeholder="价位名(零售价/批发价…)" style="width: 180px" />
            <el-input-number v-model="p.price" :min="0" :precision="2" :step="1" style="width: 140px" />
            <el-radio v-model="form.default_price" :value="p.name">默认</el-radio>
            <el-button link type="danger" :disabled="form.prices.length <= 1" @click="removePrice(i)">删除</el-button>
          </div>
          <el-button link type="primary" @click="addPrice">+ 添加价位</el-button>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialog = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
    </template>
  </el-dialog>
</template>
