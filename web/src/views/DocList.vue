<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { docApi, docTypeConfig, statusText, type Doc } from '../api/doc'
import { fenToYuan } from '../utils/money'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const canReview = computed(() => auth.hasPerm(['system']))

const list = ref<Doc[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({
  page: 1, page_size: 20,
  doc_type: '', status: '', keyword: '',
  date_from: '', date_to: '',
})

const detailVisible = ref(false)
const detail = ref<Doc | null>(null)

async function load() {
  loading.value = true
  try {
    const data: any = await docApi.list(query)
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

async function showDetail(row: Doc) {
  detail.value = (await docApi.get(row.id)) as any
  detailVisible.value = true
}

async function confirm(row: Doc) {
  await ElMessageBox.confirm(`确认让单据 ${row.doc_no} 生效并联动库存？`, '提示', { type: 'warning' })
  await docApi.confirm(row.id)
  ElMessage.success('已生效')
  load()
}

async function reviewDoc(row: Doc, approve: boolean) {
  const d = row
  if (!d) return
  try {
    if (approve) {
      await ElMessageBox.confirm(
        `通过盘点单 ${d.doc_no} 的差异审批？通过后将按账面快照调账生效。`,
        '审批确认', { confirmButtonText: '通过', type: 'warning' }
      )
      await docApi.review(d.id, 'approve')
      ElMessage.success('已通过，库存已按差异调账')
    } else {
      const { value } = await ElMessageBox.prompt(
        `驳回盘点单 ${d.doc_no}？驳回后不调账，单据作废，需重新盘点。`,
        '审批驳回', { inputPlaceholder: '驳回原因（建议填写）', type: 'warning' }
      )
      await docApi.review(d.id, 'reject', value || '')
      ElMessage.success('已驳回，单据作废，库存未调整')
    }
    detailVisible.value = false
    load()
  } catch { /* 用户取消 */ }
}

async function voidDoc(row: Doc) {
  const pending = row.status === 'review'
  const { value } = await ElMessageBox.prompt(
    pending
      ? `待审批盘点单 ${row.doc_no} 尚未调账，作废即撤回本次盘点。请输入作废原因：`
      : `作废 ${row.doc_no}，库存将自动回冲。请输入作废原因：`,
    '作废单据', { inputPlaceholder: '作废原因（可空）', type: 'warning' }
  )
  await docApi.void(row.id, value || '')
  ElMessage.success(pending ? '已撤回（未调账）' : '已作废，库存已回冲')
  load()
}

const typeTitle = (t: string) => docTypeConfig[t]?.title || t
const statusType = (s: string) => (s === 'confirmed' ? 'success' : s === 'voided' ? 'danger' : s === 'review' ? 'warning' : 'info')

const router = useRouter()
const route = useRoute()

function printDoc() {
  if (detail.value) router.push(`/print/${detail.value.id}`)
}

// 58mm 小票（v0.14）：任意单据都可打，草稿/作废会在票面标注不作结算凭证
function printTicket() {
  if (detail.value) router.push(`/print/${detail.value.id}/ticket`)
}

// 盘点差异表（v0.15）
const qtyText = (q: number | undefined) => ((q || 0) / 1000).toFixed(3).replace(/\.?0+$/, '')
const signedQty = (q: number) => (q > 0 ? '+' : '') + qtyText(q)
const hasDiff = computed(() =>
  (detail.value?.items || []).some((it) => (it.book_qty || 0) !== it.quantity),
)
function openDiff() {
  if (detail.value) router.push(`/print/${detail.value.id}/diff`)
}
// 复盘：跳到盘点页，只盘这张单的差异项，提交后生成关联初盘的新盘点单
function startRecheck() {
  if (detail.value) router.push({ path: '/stocktake', query: { ref: String(detail.value.id) } })
}

// 支持从开单页「查看全部」带参数进入：/doc-list?doc_type=purchase_in&keyword=CGRK...
function applyRouteQuery() {
  query.doc_type = (route.query.doc_type as string) || ''
  query.keyword = (route.query.keyword as string) || ''
  query.page = 1
}

watch(() => route.query, () => {
  applyRouteQuery()
  load()
})

onMounted(() => {
  applyRouteQuery()
  load()
})
</script>

<template>
  <el-card shadow="never">
    <div style="margin-bottom: 12px; display: flex; gap: 8px; flex-wrap: wrap">
      <el-select v-model="query.doc_type" placeholder="全部类型" clearable style="width: 140px" @change="query.page = 1; load()">
        <el-option v-for="(c, t) in docTypeConfig" :key="t" :label="c.title" :value="t" />
      </el-select>
      <el-select v-model="query.status" placeholder="全部状态" clearable style="width: 120px" @change="query.page = 1; load()">
        <el-option label="草稿" value="draft" />
        <el-option label="已生效" value="confirmed" />
        <el-option label="待审批" value="review" />
        <el-option label="已作废" value="voided" />
      </el-select>
      <el-input v-model="query.keyword" placeholder="单号/往来单位" style="width: 180px" clearable @keyup.enter="query.page = 1; load()" />
      <el-date-picker v-model="query.date_from" type="date" placeholder="日期从" value-format="YYYY-MM-DD" style="width: 140px" />
      <el-date-picker v-model="query.date_to" type="date" placeholder="到" value-format="YYYY-MM-DD" style="width: 130px" />
      <el-button type="primary" @click="query.page = 1; load()">查询</el-button>
    </div>

    <el-table :data="list" v-loading="loading" border>
      <el-table-column prop="doc_no" label="单号" width="170" />
      <el-table-column label="类型" width="100">
        <template #default="{ row }">{{ typeTitle(row.doc_type) }}</template>
      </el-table-column>
      <el-table-column prop="partner_name" label="往来单位" min-width="140" />
      <el-table-column label="日期" width="110">
        <template #default="{ row }">{{ (row.doc_date || '').slice(0, 10) }}</template>
      </el-table-column>
      <el-table-column label="数量" width="100" align="right">
        <template #default="{ row }">{{ (row.total_qty / 1000).toFixed(3).replace(/\.?0+$/, '') }}</template>
      </el-table-column>
      <el-table-column label="金额" width="120" align="right">
        <template #default="{ row }">¥{{ fenToYuan(row.total_amount) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="showDetail(row)">详情</el-button>
          <el-button v-if="row.status === 'draft'" link type="success" @click="confirm(row)">生效</el-button>
          <el-button v-if="row.status === 'review' && canReview" link type="success" @click="reviewDoc(row, true)">通过</el-button>
          <el-button v-if="row.status === 'review' && canReview" link type="danger" @click="reviewDoc(row, false)">驳回</el-button>
          <el-button v-if="row.status === 'confirmed' || row.status === 'review'" link type="danger" @click="voidDoc(row)">作废</el-button>
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

  <el-drawer v-model="detailVisible" :title="detail ? `${typeTitle(detail.doc_type)} ${detail.doc_no}` : ''" size="640px">
    <template v-if="detail">
      <el-descriptions :column="2" border size="small" style="margin-bottom: 16px">
        <el-descriptions-item label="往来单位">{{ detail.partner_name || '—' }}</el-descriptions-item>
        <el-descriptions-item label="日期">{{ detail.doc_date?.slice(0, 10) }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ statusText[detail.status] }}</el-descriptions-item>
        <el-descriptions-item label="备注">{{ detail.remark || '—' }}</el-descriptions-item>
        <el-descriptions-item v-if="detail.status === 'voided'" label="作废原因" :span="2">{{ detail.void_reason }}</el-descriptions-item>
        <el-descriptions-item v-if="detail.review_status === 'pending'" label="审批" :span="2">
          <el-tag type="warning" size="small">差异超阈值，待主管审批（库存暂未调整）</el-tag>
        </el-descriptions-item>
        <el-descriptions-item v-else-if="detail.review_status === 'approved'" label="审批" :span="2">
          已通过<span v-if="detail.review_note">：{{ detail.review_note }}</span>
        </el-descriptions-item>
        <el-descriptions-item v-else-if="detail.review_status === 'rejected'" label="审批" :span="2">
          已驳回<span v-if="detail.review_note">：{{ detail.review_note }}</span>
        </el-descriptions-item>
      </el-descriptions>
      <el-table :data="detail.items" border size="small">
        <el-table-column prop="product_code" label="编码" width="90" />
        <el-table-column prop="product_name" label="商品" min-width="140" />
        <el-table-column v-if="detail.doc_type === 'stocktake'" label="账面" width="76" align="right">
          <template #default="{ row }">{{ qtyText(row.book_qty) }}</template>
        </el-table-column>
        <el-table-column :label="detail.doc_type === 'stocktake' ? '实盘' : '数量'" width="80" align="right">
          <template #default="{ row }">{{ qtyText(row.quantity) }}</template>
        </el-table-column>
        <el-table-column v-if="detail.doc_type === 'stocktake'" label="差异" width="80" align="right">
          <template #default="{ row }">
            <b v-if="row.book_qty !== row.quantity"
              :style="{ color: row.quantity > row.book_qty ? 'var(--el-color-success)' : 'var(--el-color-danger)' }">
              {{ signedQty(row.quantity - row.book_qty) }}
            </b>
            <span v-else style="color: var(--el-text-color-secondary)">0</span>
          </template>
        </el-table-column>
        <el-table-column label="单价" width="90" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.price) }}</template>
        </el-table-column>
        <el-table-column label="成本" width="90" align="right">
          <template #default="{ row }">{{ row.cost ? '¥' + fenToYuan(row.cost) : '—' }}</template>
        </el-table-column>
        <el-table-column label="批次" width="80">
          <template #default="{ row }">{{ row.batch_no || '—' }}</template>
        </el-table-column>
        <el-table-column label="库位" min-width="150">
          <template #default="{ row }">
            <template v-if="detail?.doc_type === 'transfer'">
              {{ row.location_name || '未分配' }} → {{ row.to_location_name || '未分配' }}
            </template>
            <template v-else>{{ row.location_name || '—' }}</template>
          </template>
        </el-table-column>
        <el-table-column label="金额" width="100" align="right">
          <template #default="{ row }">¥{{ fenToYuan(row.amount) }}</template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 12px; text-align: right">
        合计：<b style="color: var(--el-color-danger)">¥{{ fenToYuan(detail.total_amount) }}</b>
      </div>
      <div style="margin-top: 12px; text-align: right">
        <el-button v-if="detail.doc_type === 'stocktake'" type="warning" plain @click="openDiff">差异表（打印/导出）</el-button>
        <el-button v-if="detail.status === 'review' && canReview" type="success" @click="reviewDoc(detail, true)">通过并调账</el-button>
        <el-button v-if="detail.status === 'review' && canReview" type="danger" plain @click="reviewDoc(detail, false)">驳回</el-button>
        <el-button v-if="detail.doc_type === 'stocktake' && detail.status === 'confirmed' && hasDiff" type="success" plain @click="startRecheck">
          按差异项复盘
        </el-button>
        <el-button type="primary" plain @click="printDoc">打印 A4 单据</el-button>
        <el-button type="primary" @click="printTicket">打印小票</el-button>
      </div>
    </template>
  </el-drawer>
</template>
