<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { locationApi, type Location } from '../api/archive'

const router = useRouter()
const list = ref<Location[]>([])
const loading = ref(false)
const dialog = ref(false)
const isEdit = ref(false)
const form = reactive({
  id: 0, code: '', name: '', remark: '', sort: 0,
  capacity_mode: 'qty' as 'qty' | 'volume' | 'weight',
  capacity: 0 as number | undefined, // 展示单位（随计量方式），保存时 ×1000
  min_qty: 0 as number | undefined,
  warn_ratio: 90,
})

const unitOf = (m?: string) => (m === 'volume' ? 'L' : m === 'weight' ? 'kg' : '件')
const capModeText: Record<string, string> = { qty: '按数量（件）', volume: '按体积（升 L）', weight: '按重量（kg）' }

// 库位在库汇总（v0.12 数量账）：后端 /locations 已带 used_qty/sku_count/status
const unassigned = ref<{ skus: number; qty: number } | null>(null)
const qtyText = (q: number) => (q / 1000).toFixed(3).replace(/\.?0+$/, '')

const alerts = computed(() => list.value.filter((l) => l.status && !['normal', 'empty'].includes(l.status)))
const overCount = computed(() => list.value.filter((l) => l.status === 'over').length)

const statusType: Record<string, string> = {
  over: 'danger', full: 'danger', near_full: 'warning', under: 'warning', empty: 'info', normal: 'success',
}

async function load() {
  loading.value = true
  try {
    list.value = (await locationApi.list()) || []
    const rows: any[] = (await locationApi.stock({})) || []
    const un = { skus: 0, qty: 0 }
    for (const r of rows) {
      if (r.location_id !== 0) continue
      un.skus += 1
      un.qty += r.quantity
    }
    unassigned.value = un.skus > 0 ? un : null
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  Object.assign(form, { id: 0, code: '', name: '', remark: '', sort: list.value.length, capacity_mode: 'qty', capacity: 0, min_qty: 0, warn_ratio: 90 })
  dialog.value = true
}

function openEdit(row: Location) {
  isEdit.value = true
  Object.assign(form, {
    id: row.id, code: row.code, name: row.name, remark: row.remark, sort: row.sort,
    capacity_mode: row.capacity_mode || 'qty',
    capacity: row.capacity ? row.capacity / 1000 : 0,
    min_qty: row.min_qty ? row.min_qty / 1000 : 0,
    warn_ratio: row.warn_ratio || 90,
  })
  dialog.value = true
}

async function save() {
  if (!form.name.trim()) {
    ElMessage.warning('请填写库位名称')
    return
  }
  const data = {
    code: form.code, name: form.name.trim(), remark: form.remark, sort: form.sort,
    capacity_mode: form.capacity_mode,
    capacity: Math.round((form.capacity || 0) * 1000),
    min_qty: Math.round((form.min_qty || 0) * 1000),
    warn_ratio: form.warn_ratio || 90,
  }
  if (isEdit.value) {
    await locationApi.update(form.id, data)
  } else {
    await locationApi.create(data)
  }
  ElMessage.success('已保存')
  dialog.value = false
  load()
}

async function remove(row: Location) {
  await ElMessageBox.confirm(`确定删除库位「${row.name}」？`, '删除确认', { type: 'warning' })
  await locationApi.remove(row.id)
  ElMessage.success('已删除')
  load()
}

// 进度条颜色跟着状态走
function barStatus(row: Location) {
  if (row.status === 'over' || row.status === 'full') return 'exception'
  if (row.status === 'near_full') return 'warning'
  return 'success'
}

onMounted(load)
</script>

<template>
  <div>
    <el-alert
      type="info" :closable="false" show-icon
      title="库位按货架/仓位记账：入库进指定库位、出库自动扣减、移库单在货架间搬货、盘点可按货架逐架盘；可设容量上限做货位预警。"
      style="margin-bottom: 12px"
    />
    <el-alert
      v-if="alerts.length > 0"
      :type="overCount > 0 ? 'error' : 'warning'" :closable="false" show-icon style="margin-bottom: 12px"
      :title="`货位预警：${alerts.length} 个货位需要关注${overCount > 0 ? `（其中 ${overCount} 个已超出容量）` : ''}`"
    >
      <div style="font-size: 12px; line-height: 1.8">
        <span v-for="a in alerts" :key="a.id" style="margin-right: 14px">
          <b>{{ a.name }}</b>
          {{ a.status_text }}
          <template v-if="a.capacity">（{{ qtyText(a.used_qty || 0) }}{{ unitOf(a.capacity_mode) }}/{{ qtyText(a.capacity) }}{{ unitOf(a.capacity_mode) }}，{{ a.usage_rate }}%）</template>
        </span>
      </div>
    </el-alert>
    <el-alert
      v-if="unassigned && unassigned.skus > 0"
      type="warning" :closable="false" show-icon style="margin-bottom: 12px"
      :title="`有 ${unassigned.skus} 个商品共 ${qtyText(unassigned.qty)} 处于「未分配」状态（不知道具体货架），建议用移库单上架。`"
    >
      <template #default>
        <el-button link type="primary" @click="router.push('/transfer')">去移库 →</el-button>
      </template>
    </el-alert>
    <div style="margin-bottom: 12px">
      <el-button type="primary" @click="openCreate">新建库位</el-button>
      <span style="margin-left: 12px; color: var(--el-text-color-secondary); font-size: 12px">
        容量计量可选「数量 / 体积 / 重量」：混装多单位的货架建议按体积或重量计（需先在商品档案登记单件体积/重量）。
      </span>
    </div>
    <el-table :data="list" border v-loading="loading" empty-text="暂无库位，点击左上角新建">
      <el-table-column prop="code" label="编码" width="100" />
      <el-table-column prop="name" label="库位名称" min-width="140" />
      <el-table-column label="在库" width="150" align="center">
        <template #default="{ row }">
          <template v-if="row.sku_count">
            <span>{{ row.sku_count }} 个商品</span>
            <span style="color: var(--el-text-color-secondary)"> / 占用 {{ qtyText(row.used_qty || 0) }}{{ unitOf(row.capacity_mode) }}</span>
          </template>
          <span v-else style="color: var(--el-text-color-placeholder)">空</span>
        </template>
      </el-table-column>
      <el-table-column label="容量占用" width="220">
        <template #default="{ row }">
          <template v-if="row.capacity">
            <el-progress
              :percentage="Math.min(row.usage_rate || 0, 100)" :status="barStatus(row) as any"
              :stroke-width="8" :format="() => `${row.usage_rate}%`"
            />
            <div style="font-size: 11px; color: var(--el-text-color-secondary)">
              {{ qtyText(row.used_qty || 0) }}{{ unitOf(row.capacity_mode) }} / {{ qtyText(row.capacity) }}{{ unitOf(row.capacity_mode) }}
              <el-tag v-if="row.unknown_skus" size="small" type="warning" style="margin-left: 4px">
                {{ row.unknown_skus }} 个商品未登记{{ row.capacity_mode === 'weight' ? '重量' : '体积' }}
              </el-tag>
            </div>
          </template>
          <span v-else style="color: var(--el-text-color-placeholder); font-size: 12px">未设容量</span>
        </template>
      </el-table-column>
      <el-table-column label="计量方式" width="120" align="center">
        <template #default="{ row }">
          <span style="font-size: 12px">{{ capModeText[row.capacity_mode || 'qty'] || row.capacity_mode }}</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag v-if="row.status" size="small" :type="(statusType[row.status] || 'info') as any">{{ row.status_text }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="140" />
      <el-table-column prop="sort" label="排序" width="70" align="center" />
      <el-table-column label="操作" width="130" align="center">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="isEdit ? '编辑库位' : '新建库位'" width="480px">
      <el-form label-width="110px">
        <el-form-item label="编码"><el-input v-model="form.code" placeholder="选填，如 A-01" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="form.name" placeholder="如 A区-工具货架" /></el-form-item>
        <el-form-item label="容量计量方式">
          <el-radio-group v-model="form.capacity_mode">
            <el-radio-button value="qty">数量</el-radio-button>
            <el-radio-button value="volume">体积（升）</el-radio-button>
            <el-radio-button value="weight">重量（kg）</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="form.capacity_mode === 'volume' ? '体积上限(L)' : form.capacity_mode === 'weight' ? '重量上限(kg)' : '数量上限'">
          <el-input-number v-model="form.capacity" :min="0" :precision="3" style="width: 160px" />
          <span style="margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary)">0 = 不限</span>
        </el-form-item>
        <el-form-item :label="form.capacity_mode === 'volume' ? '体积下限(L)' : form.capacity_mode === 'weight' ? '重量下限(kg)' : '数量下限'">
          <el-input-number v-model="form.min_qty" :min="0" :precision="3" style="width: 160px" />
          <span style="margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary)">低于则提醒（0 = 不限）</span>
        </el-form-item>
        <el-form-item v-if="form.capacity_mode !== 'qty'" label-width="110px">
          <span style="font-size: 12px; color: var(--el-text-color-secondary)">
            体积/重量按商品档案的「单件体积/重量 × 在库数量」合计；未登记的商品不计占用，列表会提示补档案。
          </span>
        </el-form-item>
        <el-form-item label="近满预警阈值">
          <el-input-number v-model="form.warn_ratio" :min="1" :max="100" style="width: 120px" />
          <span style="margin-left: 8px; font-size: 12px; color: var(--el-text-color-secondary)">% ，默认 90</span>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="form.remark" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="form.sort" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>
